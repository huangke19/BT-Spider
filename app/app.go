// Package app 是 UI（TUI / CLI）共用的业务编排层。
//
// 职责边界：
//   - 编排 engine + search 两个底层包，提供「业务能力」方法
//   - UI 只依赖本包，不直接 import engine / search 的函数
//   - 不持有 UI 状态，不关心渲染
//
// 未来扩展（如缓存搜索结果、按角色控制可见 provider、搜索熔断等）都加在本包。
package app

import (
	"time"

	"github.com/huangke/bt-spider/engine"
	"github.com/huangke/bt-spider/search"
	"github.com/huangke/bt-spider/search/pipeline"
	"github.com/huangke/bt-spider/search/providers"
	"github.com/huangke/bt-spider/search/query"
)

// sizeResolveTimeout 对搜索结果中 Size 未知的条目，用 DHT 补全的单次超时。
const sizeResolveTimeout = 8 * time.Second

// ---- 类型重导出（让 UI 层只 import app，不直接依赖 engine / search） ----

type SearchResult = search.Result
type MovieResolution = search.MovieResolution
type DownloadSnapshot = engine.DownloadSnapshot
type DownloadState = engine.DownloadState

const (
	StateWaitingMeta = engine.StateWaitingMeta
	StateDownloading = engine.StateDownloading
	StateSeeding     = engine.StateSeeding
	StateDone        = engine.StateDone
	StateFailed      = engine.StateFailed
	StateCanceled    = engine.StateCanceled
)

// App 业务编排入口。
type App struct {
	engine    *engine.Engine
	providers []search.Provider
}

// New 构造 App。providers 传 nil 时使用 search.DefaultProviders()。
func New(eng *engine.Engine, provs []search.Provider) *App {
	if provs == nil {
		provs = providers.DefaultProviders()
	}
	return &App{
		engine:    eng,
		providers: provs,
	}
}

// --- 搜索 / 识别 ---

// Search 执行一次搜索，并对 Size 未知的结果自动通过 DHT 补全大小。
func (a *App) Search(keyword string) ([]search.Result, error) {
	results, err := pipeline.Search(keyword, a.providers)
	if err != nil {
		return nil, err
	}
	if len(results) > 0 {
		results = a.engine.ResolveSizes(results, sizeResolveTimeout)
	}
	return results, nil
}

// ResolveLocal 使用本地别名 + 严格格式识别解析输入（同步，无网络）。
func (a *App) ResolveLocal(input string) (search.MovieResolution, bool) {
	return query.ResolveMovieSearchInput(input)
}

// ResolveNLP 走完整 NLP pipeline（可能调用 TMDB / Groq，延迟 200~500ms）。
func (a *App) ResolveNLP(input string) (search.MovieResolution, bool) {
	return query.NLPResolve(input, a.engine.Config())
}

// --- 下载 ---

// AddMagnet 添加磁力链接到下载队列。返回的 error 已带用户可读消息。
func (a *App) AddMagnet(magnet string) error {
	_, err := a.engine.AddMagnetAsync(magnet)
	return err
}

// CancelDownload 按 ID 取消并从列表中移除。返回 false 表示未找到。
func (a *App) CancelDownload(id string) bool {
	return a.engine.RemoveDownload(id)
}

// ListDownloads 返回所有下载任务的快照。
func (a *App) ListDownloads() []engine.DownloadSnapshot {
	return a.engine.ListDownloads()
}

// ClearFinished 清理已完成/失败/取消的任务，返回被清理的数量。
func (a *App) ClearFinished() int {
	return a.engine.ClearFinished()
}

// --- 元信息 ---

// DownloadDir 当前下载目录（供 UI 显示）。
func (a *App) DownloadDir() string {
	return a.engine.Config().DownloadDir
}
