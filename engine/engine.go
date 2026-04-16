package engine

import (
	"fmt"
	"os"
	"sync"

	"github.com/anacrolix/torrent"
	"github.com/huangke/bt-spider/config"
)

type Engine struct {
	client   *torrent.Client
	cfg      *config.Config
	trackers *TrackerList

	mu        sync.RWMutex
	downloads []*Download
}

func New(cfg *config.Config) (*Engine, error) {
	if err := os.MkdirAll(cfg.DownloadDir, 0755); err != nil {
		return nil, fmt.Errorf("创建下载目录失败: %w", err)
	}

	clientCfg := torrent.NewDefaultClientConfig()
	clientCfg.DataDir = cfg.DownloadDir
	clientCfg.ListenPort = cfg.ListenPort
	clientCfg.Seed = cfg.Seed
	clientCfg.EstablishedConnsPerTorrent = cfg.MaxConns

	client, err := torrent.NewClient(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("创建BT客户端失败: %w", err)
	}

	eng := &Engine{
		client: client,
		cfg:    cfg,
	}

	if cfg.EnableTrackerList {
		eng.trackers = NewTrackerList()
	}

	return eng, nil
}

// Config 返回 engine 配置（UI 用来显示下载目录等）
func (e *Engine) Config() *config.Config {
	return e.cfg
}

// registerDownload 把新任务加入注册表
func (e *Engine) registerDownload(d *Download) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.downloads = append(e.downloads, d)
}

// ListDownloads 返回所有下载任务的快照，按创建时间排序
func (e *Engine) ListDownloads() []DownloadSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]DownloadSnapshot, 0, len(e.downloads))
	for _, d := range e.downloads {
		out = append(out, d.Snapshot())
	}
	return out
}

// RemoveDownload 按 ID 取消并从列表中移除
func (e *Engine) RemoveDownload(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, d := range e.downloads {
		if d.ID == id {
			d.Cancel()
			e.downloads = append(e.downloads[:i], e.downloads[i+1:]...)
			return true
		}
	}
	return false
}

// ClearFinished 清理已完成/失败/取消的任务
func (e *Engine) ClearFinished() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	kept := e.downloads[:0]
	removed := 0
	for _, d := range e.downloads {
		s := d.State()
		if s == StateDone || s == StateFailed || s == StateCanceled {
			removed++
			continue
		}
		kept = append(kept, d)
	}
	e.downloads = kept
	return removed
}

func (e *Engine) Close() {
	e.mu.Lock()
	for _, d := range e.downloads {
		d.Cancel()
	}
	e.mu.Unlock()

	if e.trackers != nil {
		e.trackers.Stop()
	}
	if e.client != nil {
		e.client.Close()
	}
}
