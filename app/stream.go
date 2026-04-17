package app

import (
	"context"
	"time"

	"github.com/huangke/bt-spider/search/pipeline"
)

// SearchUpdate 流式搜索的一次推送（向 UI 转发）。
type SearchUpdate struct {
	Provider string
	Results  []SearchResult
	Err      error
	Done     bool
}

// SearchStream 发起流式搜索。UI 每收到一条 Update 就刷新展示。
// timeout <= 0 时使用默认 8s。
func (a *App) SearchStream(ctx context.Context, keyword string, timeout time.Duration) <-chan SearchUpdate {
	out := make(chan SearchUpdate, 16)
	go func() {
		defer close(out)
		for u := range pipeline.SearchStream(ctx, keyword, a.providers, timeout) {
			select {
			case out <- SearchUpdate{
				Provider: u.Provider,
				Results:  u.Results,
				Err:      u.Err,
				Done:     u.Done,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
