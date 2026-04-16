package query

import (
	"github.com/huangke/bt-spider/config"
	"github.com/huangke/bt-spider/pkg/logger"
	"github.com/huangke/bt-spider/search"
)

// Resolver 电影查询解析器接口
type Resolver interface {
	Name() string
	Resolve(input string) (search.MovieResolution, bool)
}

// Chain 是 Resolver 的有序责任链
type Chain struct {
	resolvers []Resolver
}

func NewChain(rs ...Resolver) *Chain {
	return &Chain{resolvers: rs}
}

func (c *Chain) Resolve(input string) (search.MovieResolution, bool) {
	for _, r := range c.resolvers {
		if res, ok := r.Resolve(input); ok {
			logger.Info("resolver hit", "resolver", r.Name(), "input", input, "query", res.Query)
			return res, true
		}
		logger.Debug("resolver miss", "resolver", r.Name(), "input", input)
	}
	return search.MovieResolution{}, false
}

// --- 三个内置 Resolver ---

type aliasResolver struct{}

func (aliasResolver) Name() string { return "alias" }

func (aliasResolver) Resolve(input string) (search.MovieResolution, bool) {
	return ResolveMovieSearchInput(input)
}

type tmdbResolver struct{ apiKey string }

func (r tmdbResolver) Name() string { return "tmdb" }

func (r tmdbResolver) Resolve(input string) (search.MovieResolution, bool) {
	if r.apiKey == "" {
		return search.MovieResolution{}, false
	}
	meta, ok := SearchTMDB(input, r.apiKey)
	if !ok {
		return search.MovieResolution{}, false
	}
	query := formatMovieQuery(meta.Title, meta.Year) + " 1080P"
	return search.MovieResolution{
		Query:   query,
		Display: "TMDB 解析: " + query,
	}, true
}

type groqResolver struct{ apiKey string }

func (r groqResolver) Name() string { return "groq" }

func (r groqResolver) Resolve(input string) (search.MovieResolution, bool) {
	if r.apiKey == "" {
		return search.MovieResolution{}, false
	}
	meta, ok := ResolveWithGroq(input, r.apiKey)
	if !ok {
		return search.MovieResolution{}, false
	}
	query := formatMovieQuery(meta.Title, meta.Year) + " 1080P"
	return search.MovieResolution{
		Query:   query,
		Display: "AI 解析: " + query,
	}, true
}

// DefaultResolverChain 按 alias → tmdb → groq 顺序构造默认链。
func DefaultResolverChain(cfg *config.Config) *Chain {
	return NewChain(
		aliasResolver{},
		tmdbResolver{apiKey: cfg.TMDBApiKey},
		groqResolver{apiKey: cfg.GroqApiKey},
	)
}
