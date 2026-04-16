package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/huangke/bt-spider/config"
	"github.com/huangke/bt-spider/pkg/logger"
)

// DownloadState 下载任务状态
type DownloadState int

const (
	StateWaitingMeta DownloadState = iota
	StateDownloading
	StateSeeding
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
	case StateSeeding:
		return "做种中"
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

	onEvent func(Event) // Engine 注入的事件回调

	uploadedAtCompletion int64
	uploadedTotal        int64
	shareRatio           float64
	seedStartedAt        time.Time
	seedElapsed          time.Duration
	policy               DownloadPolicy
}

type DownloadPolicy struct {
	Seed           bool
	SeedRatioLimit float64
	SeedTimeLimit  time.Duration
}

func policyFromConfig(cfg *config.Config) DownloadPolicy {
	seedTimeLimit, _ := cfg.SeedTimeLimitDuration()
	return DownloadPolicy{
		Seed:           cfg.Seed,
		SeedRatioLimit: cfg.SeedRatioLimit,
		SeedTimeLimit:  seedTimeLimit,
	}
}

// DownloadSnapshot UI 读取用的只读快照
type DownloadSnapshot struct {
	ID          string
	Name        string
	State       DownloadState
	Completed   int64
	Total       int64
	Peers       int
	Speed       float64 // bytes/sec
	ETA         time.Duration
	Err         string
	Uploaded    int64
	ShareRatio  float64
	SeedElapsed time.Duration
}

// Snapshot 返回当前状态快照，并更新内部速度追踪
func (d *Download) Snapshot() DownloadSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()

	snap := DownloadSnapshot{
		ID:          d.ID,
		Name:        d.name,
		State:       d.state,
		Total:       d.totalSize,
		Err:         d.errMsg,
		Uploaded:    d.uploadedTotal,
		ShareRatio:  d.shareRatio,
		SeedElapsed: d.seedElapsed,
	}

	if d.Torrent != nil && (d.state == StateDownloading || d.state == StateSeeding || d.state == StateDone) {
		snap.Completed = d.Torrent.BytesCompleted()
		stats := d.Torrent.Stats()
		snap.Peers = stats.ActivePeers
		snap.Uploaded = stats.BytesWrittenData.Int64()
	}

	// EWMA 速度（alpha=0.3）
	now := time.Now()
	if (d.state == StateDownloading || d.state == StateSeeding) && !d.lastTime.IsZero() {
		dt := now.Sub(d.lastTime).Seconds()
		if dt > 0 {
			currentBytes := snap.Completed
			if d.state == StateSeeding {
				currentBytes = snap.Uploaded
			}
			inst := float64(currentBytes-d.lastBytes) / dt
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
	if d.state == StateSeeding {
		d.lastBytes = snap.Uploaded
	} else {
		d.lastBytes = snap.Completed
	}
	d.lastTime = now
	snap.Speed = d.speedEWMA

	if snap.Speed > 0 && snap.Completed < snap.Total {
		secs := float64(snap.Total-snap.Completed) / snap.Speed
		snap.ETA = time.Duration(secs) * time.Second
	}
	if d.seedStartedAt.IsZero() == false {
		snap.SeedElapsed = time.Since(d.seedStartedAt).Round(time.Second)
		d.seedElapsed = snap.SeedElapsed
	}
	if d.totalSize > 0 {
		uploadedSinceComplete := snap.Uploaded - d.uploadedAtCompletion
		if uploadedSinceComplete < 0 {
			uploadedSinceComplete = 0
		}
		d.uploadedTotal = uploadedSinceComplete
		snap.Uploaded = uploadedSinceComplete
		snap.ShareRatio = float64(uploadedSinceComplete) / float64(d.totalSize)
		d.shareRatio = snap.ShareRatio
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
	logger.Error("download failed", "id", d.ID, "name", d.name, "reason", msg)
	d.emitEvent(EventFailed, msg)
}

func (d *Download) startSeeding(uploadedAtCompletion int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.state == StateCanceled || d.state == StateFailed || d.state == StateDone {
		return
	}
	if d.seedStartedAt.IsZero() {
		d.seedStartedAt = time.Now()
		d.uploadedAtCompletion = uploadedAtCompletion
		d.uploadedTotal = 0
		d.shareRatio = 0
		d.lastBytes = 0
		d.lastTime = time.Time{}
		d.speedEWMA = 0
	}
	d.state = StateSeeding
}

func (d *Download) markDone() {
	d.mu.Lock()
	if d.state != StateCanceled && d.state != StateFailed {
		d.state = StateDone
	}
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
	logger.Info("download canceled", "id", d.ID, "name", d.name)
	d.emitEvent(EventCanceled, "")
	if d.Torrent != nil {
		d.Torrent.Drop()
	}
}

// AddMagnetAsync 异步添加磁力链接并注册到 Engine
func (e *Engine) AddMagnetAsync(magnet string) (*Download, error) {
	return e.AddMagnetWithPolicyAsync(magnet, policyFromConfig(e.cfg))
}

// AddMagnetWithPolicyAsync 使用指定做种策略添加磁力链接。
func (e *Engine) AddMagnetWithPolicyAsync(magnet string, policy DownloadPolicy) (*Download, error) {
	t, err := e.client.AddMagnet(magnet)
	if err != nil {
		logger.Error("add magnet failed", "err", err)
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
		policy:    policy,
		onEvent:   e.emit,
	}

	e.registerDownload(dl)
	logger.Info("download queued", "id", dl.ID, "name", dl.name, "seed", policy.Seed)

	go func() {
		select {
		case <-t.GotInfo():
			info := t.Info()
			dl.mu.Lock()
			if dl.state == StateCanceled {
				dl.mu.Unlock()
				logger.Info("download canceled before meta", "id", dl.ID)
				return
			}
			dl.name = info.BestName()
			dl.totalSize = info.TotalLength()
			dl.state = StateDownloading
			dl.mu.Unlock()
			logger.Info("download meta received", "id", dl.ID, "name", dl.name, "size", dl.totalSize)
			dl.emitEvent(EventMetaReceived, dl.name)
			t.DownloadAll()
			go dl.watchLifecycle()
		case <-time.After(2 * time.Minute):
			dl.setFailed("获取元数据超时（2分钟）")
			logger.Warn("download meta timeout", "id", dl.ID, "name", dl.name)
			t.Drop()
		}
	}()

	return dl, nil
}

func (d *Download) watchLifecycle() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		d.mu.Lock()
		state := d.state
		t := d.Torrent
		totalSize := d.totalSize
		seedStartedAt := d.seedStartedAt
		policy := d.policy
		d.mu.Unlock()

		if t == nil || state == StateCanceled || state == StateFailed || state == StateDone {
			return
		}

		completed := t.BytesCompleted()
		stats := t.Stats()
		uploaded := stats.BytesWrittenData.Int64()

		if totalSize <= 0 || completed < totalSize {
			continue
		}

		if !policy.Seed {
			logger.Info("download done", "id", d.ID, "name", d.name, "size", totalSize)
			d.markDone()
			d.emitEvent(EventDownloadDone, "")
			return
		}

		if state != StateSeeding {
			logger.Info("download done, start seeding", "id", d.ID, "name", d.name, "size", totalSize)
			d.startSeeding(uploaded)
			d.emitEvent(EventSeedingStarted, "")
			continue
		}

		uploadedSinceComplete := uploaded - d.uploadedAtCompletionValue()
		if uploadedSinceComplete < 0 {
			uploadedSinceComplete = 0
		}

		if policy.SeedRatioLimit > 0 && totalSize > 0 {
			ratio := float64(uploadedSinceComplete) / float64(totalSize)
			if ratio >= policy.SeedRatioLimit {
				logger.Info("seeding done (ratio limit)", "id", d.ID, "name", d.name,
					"ratio", ratio, "limit", policy.SeedRatioLimit)
				d.markDone()
				d.emitEvent(EventSeedingStopped, fmt.Sprintf("ratio %.2f", ratio))
				t.Drop()
				return
			}
		}
		if policy.SeedTimeLimit > 0 && !seedStartedAt.IsZero() && time.Since(seedStartedAt) >= policy.SeedTimeLimit {
			logger.Info("seeding done (time limit)", "id", d.ID, "name", d.name,
				"elapsed", time.Since(seedStartedAt).Round(time.Second))
			d.markDone()
			d.emitEvent(EventSeedingStopped, "时间限制")
			t.Drop()
			return
		}
	}
}

func (d *Download) uploadedAtCompletionValue() int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.uploadedAtCompletion
}

func (d *Download) emitEvent(typ EventType, detail string) {
	if d.onEvent != nil {
		d.onEvent(Event{
			Type:       typ,
			DownloadID: d.ID,
			Name:       d.name,
			Detail:     detail,
		})
	}
}
