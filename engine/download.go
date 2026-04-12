package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
)

// Download 表示一个活跃的下载任务
type Download struct {
	ID        string // info hash
	Name      string
	TotalSize int64
	Torrent   *torrent.Torrent

	mu       sync.Mutex
	canceled bool
}

// Progress 返回下载进度信息
func (d *Download) Progress() (completed int64, total int64, peers int, done bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.canceled {
		return 0, d.TotalSize, 0, true
	}
	total = d.TotalSize
	completed = d.Torrent.BytesCompleted()
	stats := d.Torrent.Stats()
	peers = stats.ActivePeers
	done = total > 0 && completed >= total
	return
}

// Cancel 取消下载
func (d *Download) Cancel() {
	d.mu.Lock()
	d.canceled = true
	d.mu.Unlock()
	d.Torrent.Drop()
}

// IsCanceled 是否已取消
func (d *Download) IsCanceled() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.canceled
}

// AddMagnetAsync 异步添加磁力链接，返回 Download 对象
func (e *Engine) AddMagnetAsync(magnet string) (*Download, error) {
	t, err := e.client.AddMagnet(magnet)
	if err != nil {
		return nil, fmt.Errorf("解析磁力链接失败: %w", err)
	}

	// 注入最新 tracker 列表
	if e.trackers != nil {
		t.AddTrackers(e.trackers.Get())
	}

	dl := &Download{
		Torrent: t,
	}

	// 等待元数据（异步）
	go func() {
		select {
		case <-t.GotInfo():
			info := t.Info()
			dl.mu.Lock()
			dl.ID = t.InfoHash().HexString()
			dl.Name = info.BestName()
			dl.TotalSize = info.TotalLength()
			dl.mu.Unlock()
			t.DownloadAll()
		case <-time.After(2 * time.Minute):
			dl.mu.Lock()
			dl.canceled = true
			dl.mu.Unlock()
			t.Drop()
		}
	}()

	return dl, nil
}

// GetActiveTorrents 返回所有活跃 torrent 信息
func (e *Engine) GetActiveTorrents() []*torrent.Torrent {
	return e.client.Torrents()
}
