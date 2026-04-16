package engine

import "github.com/anacrolix/torrent"

// TorrentHandle 抽象 Download 对底层 torrent 库的依赖，方便单元测试。
type TorrentHandle interface {
	BytesCompleted() int64
	ActivePeers() int
	BytesUploaded() int64
	Drop()
}

// realHandle 包装 *torrent.Torrent 实现 TorrentHandle。
type realHandle struct {
	t *torrent.Torrent
}

func (h *realHandle) BytesCompleted() int64 { return h.t.BytesCompleted() }

func (h *realHandle) ActivePeers() int { return h.t.Stats().ActivePeers }

func (h *realHandle) BytesUploaded() int64 {
	stats := h.t.Stats()
	c := stats.BytesWrittenData
	return c.Int64()
}

func (h *realHandle) Drop() { h.t.Drop() }
