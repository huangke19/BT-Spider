package search

import (
	"github.com/huangke/bt-spider/config"
	"github.com/huangke/bt-spider/pkg/logger"
)

// Resolver 电影查询解析器：把自然语言/模糊输入转成规范化的 MovieResolution。
// 一个 Resolver 只关心自己认不认识这个输入（ok=true 即命中，链路短路）。
//
// 新增解析层 = 新增一个实现 + 注册进 Chain，不改已有 Resolver。
type Resolver interface {
	Name() string
	Resolve(input string) (MovieResolution, bool)
}

// Chain 是 Resolver 的有序责任链：按注册顺序尝试，第一个命中即返回。
// 命中与否都会打日志，方便排查为什么某个输入没被识别。
type Chain struct {
	resolvers []Resolver
}

func NewChain(rs ...Resolver) *Chain {
	return &Chain{resolvers: rs}
}

// Resolve 按顺序询问每个 resolver；全部失败时返回 false。
func (c *Chain) Resolve(input string) (MovieResolution, bool) {
	for _, r := range c.resolvers {
		if res, ok := r.Resolve(input); ok {
			logger.Info("resolver hit", "resolver", r.Name(), "input", input, "query", res.Query)
			return res, true
		}
		logger.Debug("resolver miss", "resolver", r.Name(), "input", input)
	}
	return MovieResolution{}, false
}

// --- 三个内置 Resolver，薄包装现有函数，不重写任何业务逻辑 ---

// aliasResolver 走本地别名库 + 严格格式识别。同步、无网络、最快。
type aliasResolver struct{}

func (aliasResolver) Name() string { return "alias" }

func (aliasResolver) Resolve(input string) (MovieResolution, bool) {
	return ResolveMovieSearchInput(input)
}

// tmdbResolver 走 TMDB API。key 为空时自动 miss。
type tmdbResolver struct{ apiKey string }

func (r tmdbResolver) Name() string { return "tmdb" }

func (r tmdbResolver) Resolve(input string) (MovieResolution, bool) {
	if r.apiKey == "" {
		return MovieResolution{}, false
	}
	meta, ok := SearchTMDB(input, r.apiKey)
	if !ok {
		return MovieResolution{}, false
	}
	query := formatMovieQuery(meta.Title, meta.Year) + " 1080P"
	return MovieResolution{
		Query:   query,
		Display: "TMDB 解析: " + query,
	}, true
}

// groqResolver 走 Groq LLM。key 为空时自动 miss。
type groqResolver struct{ apiKey string }

func (r groqResolver) Name() string { return "groq" }

func (r groqResolver) Resolve(input string) (MovieResolution, bool) {
	if r.apiKey == "" {
		return MovieResolution{}, false
	}
	meta, ok := ResolveWithGroq(input, r.apiKey)
	if !ok {
		return MovieResolution{}, false
	}
	query := formatMovieQuery(meta.Title, meta.Year) + " 1080P"
	return MovieResolution{
		Query:   query,
		Display: "AI 解析: " + query,
	}, true
}

// DefaultResolverChain 按 alias → tmdb → groq 顺序构造默认链。
// 优先级原则：越快、越确定的越靠前。
func DefaultResolverChain(cfg *config.Config) *Chain {
	return NewChain(
		aliasResolver{},
		tmdbResolver{apiKey: cfg.TMDBApiKey},
		groqResolver{apiKey: cfg.GroqApiKey},
	)
}
