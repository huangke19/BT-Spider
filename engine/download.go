package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
)

// DownloadState 下载任务状态
type DownloadState int

const (
	StateWaitingMeta DownloadState = iota
	StateDownloading
	StateDone
	StateFailed
	StateCanceled
)

func (s DownloadState) String() string {
	switch s {
	case StateWaitingMeta:
		return "等待元数据"
	case StateDownloading:
		return "下载中"
	case StateDone:
		return "已完成"
	case StateFailed:
		return "失败"
	case StateCanceled:
		return "已取消"
	}
	return "未知"
}

// Download 表示一个活跃的下载任务
type Download struct {
	ID        string
	Magnet    string
	Torrent   *torrent.Torrent
	CreatedAt time.Time

	mu        sync.Mutex
	name      string
	totalSize int64
	state     DownloadState
	errMsg    string

	// 速度追踪（EWMA）
	lastBytes int64
	lastTime  time.Time
	speedEWMA float64
}

// DownloadSnapshot UI 读取用的只读快照
type DownloadSnapshot struct {
	ID        string
	Name      string
	State     DownloadState
	Completed int64
	Total     int64
	Peers     int
	Speed     float64 // bytes/sec
	ETA       time.Duration
	Err       string
}

// Snapshot 返回当前状态快照，并更新内部速度追踪
func (d *Download) Snapshot() DownloadSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()

	snap := DownloadSnapshot{
		ID:    d.ID,
		Name:  d.name,
		State: d.state,
		Total: d.totalSize,
		Err:   d.errMsg,
	}

	if d.Torrent != nil && (d.state == StateDownloading || d.state == StateDone) {
		snap.Completed = d.Torrent.BytesCompleted()
		stats := d.Torrent.Stats()
		snap.Peers = stats.ActivePeers
	}

	// EWMA 速度（alpha=0.3）
	now := time.Now()
	if d.state == StateDownloading && !d.lastTime.IsZero() {
		dt := now.Sub(d.lastTime).Seconds()
		if dt > 0 {
			inst := float64(snap.Completed-d.lastBytes) / dt
			if inst < 0 {
				inst = 0
			}
			if d.speedEWMA == 0 {
				d.speedEWMA = inst
			} else {
				d.speedEWMA = 0.3*inst + 0.7*d.speedEWMA
			}
		}
	}
	d.lastBytes = snap.Completed
	d.lastTime = now
	snap.Speed = d.speedEWMA

	if snap.Speed > 0 && snap.Completed < snap.Total {
		secs := float64(snap.Total-snap.Completed) / snap.Speed
		snap.ETA = time.Duration(secs) * time.Second
	}

	// 自动转 Done
	if d.state == StateDownloading && snap.Total > 0 && snap.Completed >= snap.Total {
		d.state = StateDone
		snap.State = StateDone
	}

	return snap
}

func (d *Download) setState(s DownloadState) {
	d.mu.Lock()
	d.state = s
	d.mu.Unlock()
}

func (d *Download) setFailed(msg string) {
	d.mu.Lock()
	d.state = StateFailed
	d.errMsg = msg
	d.mu.Unlock()
}

func (d *Download) State() DownloadState {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state
}

// Cancel 取消下载并释放资源
func (d *Download) Cancel() {
	d.mu.Lock()
	if d.state == StateCanceled || d.state == StateDone {
		d.mu.Unlock()
		return
	}
	d.state = StateCanceled
	d.mu.Unlock()
	if d.Torrent != nil {
		d.Torrent.Drop()
	}
}

// AddMagnetAsync 异步添加磁力链接并注册到 Engine
func (e *Engine) AddMagnetAsync(magnet string) (*Download, error) {
	t, err := e.client.AddMagnet(magnet)
	if err != nil {
		return nil, fmt.Errorf("解析磁力链接失败: %w", err)
	}

	if e.trackers != nil {
		t.AddTrackers(e.trackers.Get())
	}

	// 先用 magnet dn 或 infohash 当占位名
	placeholder := t.Name()
	if placeholder == "" {
		placeholder = t.InfoHash().HexString()[:12] + "..."
	}

	dl := &Download{
		ID:        t.InfoHash().HexString(),
		Magnet:    magnet,
		Torrent:   t,
		CreatedAt: time.Now(),
		name:      placeholder,
		state:     StateWaitingMeta,
	}

	e.registerDownload(dl)

	go func() {
		select {
		case <-t.GotInfo():
			info := t.Info()
			dl.mu.Lock()
			if dl.state == StateCanceled {
				dl.mu.Unlock()
				return
			}
			dl.name = info.BestName()
			dl.totalSize = info.TotalLength()
			dl.state = StateDownloading
			dl.mu.Unlock()
			t.DownloadAll()
		case <-time.After(2 * time.Minute):
			dl.setFailed("获取元数据超时（2分钟）")
			t.Drop()
		}
	}()

	return dl, nil
}
