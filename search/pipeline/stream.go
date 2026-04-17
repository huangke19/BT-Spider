package pipeline

import (
	"context"
	"strings"
	"time"

	"github.com/huangke/bt-spider/pkg/logger"
	"github.com/huangke/bt-spider/search"
)

// StreamUpdate 流式搜索的单次推送。
type StreamUpdate struct {
	Provider string          // 来源 provider；"cache" 表示缓存命中
	Results  []search.Result // 当前累积的完整列表（已去重/排序）
	Err      error           // 单 provider 错误，不影响整体
	Done     bool            // 是否为终结信号
}

// SearchStream 并发搜索所有 provider，一旦有 provider 返回就推送当前累积结果。
// channel 在所有 provider 完成或 ctx 取消后关闭。
func SearchStream(ctx context.Context, keyword string, providers []search.Provider, timeout time.Duration) <-chan StreamUpdate {
	out := make(chan StreamUpdate, 16)

	go func() {
		defer close(out)

		if timeout <= 0 {
			timeout = DefaultSearchTimeout
		}

		// 缓存命中：直接 Done
		if cached, ok := CacheGet(keyword); ok {
			logger.Info("search cache hit (stream)", "keyword", keyword, "count", len(cached))
			select {
			case out <- StreamUpdate{Provider: "cache", Results: cached, Done: true}:
			case <-ctx.Done():
			}
			return
		}

		strictQuery, strictMode := parseStrictMovieQuery(keyword)
		runID := auditStartRun(keyword, timeout, strictMode, len(providers))

		type providerResult struct {
			name    string
			results []search.Result
			err     error
			dur     time.Duration
		}
		ch := make(chan providerResult, len(providers))
		pending := make(map[string]struct{}, len(providers))
		for _, p := range providers {
			pending[p.Name()] = struct{}{}
			go func(p search.Provider) {
				start := time.Now()
				results, err := p.Search(keyword, 0)
				ch <- providerResult{name: p.Name(), results: results, err: err, dur: time.Since(start)}
			}(p)
		}

		timer := time.NewTimer(timeout)
		defer timer.Stop()

		var accumulated []search.Result
		flush := func() []search.Result {
			return finalizeResults(append([]search.Result(nil), accumulated...), keyword, strictMode, strictQuery)
		}

		for len(pending) > 0 {
			select {
			case <-ctx.Done():
				auditFinishRun(runID, "canceled", len(accumulated), "ctx canceled")
				return
			case pr := <-ch:
				delete(pending, pr.name)
				auditRecordProviderResult(runID, pr.name, pr.dur, pr.results, pr.err)
				if pr.err != nil {
					logger.Warn("search stream provider error", "provider", pr.name, "err", pr.err)
					select {
					case out <- StreamUpdate{Provider: pr.name, Err: pr.err}:
					case <-ctx.Done():
						return
					}
					continue
				}
				accumulated = append(accumulated, pr.results...)
				snapshot := flush()
				select {
				case out <- StreamUpdate{Provider: pr.name, Results: snapshot}:
				case <-ctx.Done():
					return
				}
			case <-timer.C:
				pendingList := sortedKeys(pending)
				logger.Warn("search stream timeout", "keyword", keyword, "pending", strings.Join(pendingList, ", "))
				for provider := range pending {
					auditRecordProviderTimeout(runID, provider, timeout)
				}
				final := flush()
				if len(final) > 0 {
					CachePut(keyword, final)
				}
				select {
				case out <- StreamUpdate{Results: final, Done: true}:
				case <-ctx.Done():
				}
				auditFinishRun(runID, "partial_timeout", len(final), "")
				return
			}
		}

		final := flush()
		if len(final) > 0 {
			CachePut(keyword, final)
		}
		select {
		case out <- StreamUpdate{Results: final, Done: true}:
		case <-ctx.Done():
		}
		if len(final) > 0 {
			auditFinishRun(runID, "success", len(final), "")
		} else {
			auditFinishRun(runID, "no_results", 0, "")
		}
	}()

	return out
}
