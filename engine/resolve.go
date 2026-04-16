package engine

import (
	"sync"
	"time"

	"github.com/huangke/bt-spider/pkg/utils"
	"github.com/huangke/bt-spider/search"
)

// ResolveSizes 对 Size 为"未知"的结果，通过 DHT 拉取 torrent 元数据来补充大小。
// 并发请求，超时后直接返回（size 仍为"未知"）。
func (e *Engine) ResolveSizes(results []search.Result, timeout time.Duration) []search.Result {
	out := make([]search.Result, len(results))
	copy(out, results)

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
			}
		}(i)
	}
	wg.Wait()
	return out
}

// fetchSize 添加磁力链接、等待元数据、读取总大小，随即 drop torrent。
func (e *Engine) fetchSize(magnet string, timeout time.Duration) string {
	t, err := e.client.AddMagnet(magnet)
	if err != nil {
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
		return ""
	}
}
