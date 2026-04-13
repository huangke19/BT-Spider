package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
)

// Download 表示一个活跃的下载任务
type Download struct {
	ID        string
	Name      string
	TotalSize int64
	Torrent   *torrent.Torrent

	mu       sync.Mutex
	canceled bool
}

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

func (d *Download) Cancel() {
	d.mu.Lock()
	d.canceled = true
	d.mu.Unlock()
	d.Torrent.Drop()
}

func (d *Download) IsCanceled() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.canceled
}

// AddMagnetAsync 异步添加磁力链接
func (e *Engine) AddMagnetAsync(magnet string) (*Download, error) {
	t, err := e.client.AddMagnet(magnet)
	if err != nil {
		return nil, fmt.Errorf("解析磁力链接失败: %w", err)
	}

	if e.trackers != nil {
		t.AddTrackers(e.trackers.Get())
	}

	dl := &Download{Torrent: t}

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

func (e *Engine) GetActiveTorrents() []*torrent.Torrent {
	return e.client.Torrents()
}
