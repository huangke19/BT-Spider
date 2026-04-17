package engine

import (
	"sync"
	"time"

	"github.com/huangke/bt-spider/pkg/logger"
	"github.com/huangke/bt-spider/pkg/utils"
	"github.com/huangke/bt-spider/search"
)

// ResolveSizes 对 Size 为"未知"的结果，通过 DHT 拉取 torrent 元数据来补充大小。
// 并发请求，超时后直接返回（size 仍为"未知"）。
func (e *Engine) ResolveSizes(results []search.Result, timeout time.Duration) []search.Result {
	out := make([]search.Result, len(results))
	copy(out, results)

	unknown := 0
	for _, r := range out {
		if r.Size == "未知" && r.Magnet != "" {
			unknown++
		}
	}
	if unknown == 0 {
		return out
	}
	logger.Debug("resolve sizes start", "total", len(out), "unknown", unknown, "timeout", timeout)

	resolved := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := range out {
		if out[i].Size != "未知" || out[i].Magnet == "" {
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			size := e.fetchSize(out[idx].Magnet, timeout)
			if size != "" {
				out[idx].Size = size
				mu.Lock()
				resolved++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	logger.Debug("resolve sizes done", "resolved", resolved, "unknown_remaining", unknown-resolved)
	return out
}

// ResolveSizeOne 对单个磁力同步拉取 size。失败返回空串，不 panic。
func (e *Engine) ResolveSizeOne(magnet string, timeout time.Duration) string {
	if magnet == "" {
		return ""
	}
	return e.fetchSize(magnet, timeout)
}

// fetchSize 添加磁力链接、等待元数据、读取总大小，随即 drop torrent。
func (e *Engine) fetchSize(magnet string, timeout time.Duration) string {
	t, err := e.client.AddMagnet(magnet)
	if err != nil {
		logger.Debug("fetchSize add magnet failed", "err", err)
		return ""
	}
	defer t.Drop()

	select {
	case <-t.GotInfo():
		total := t.Info().TotalLength()
		if total > 0 {
			return utils.FormatBytes(total)
		}
		return ""
	case <-time.After(timeout):
		logger.Debug("fetchSize timeout", "infohash", t.InfoHash().HexString()[:12])
		return ""
	}
}
