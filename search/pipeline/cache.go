package pipeline

import (
	"strings"
	"sync"
	"time"

	"github.com/huangke/bt-spider/search"
)

// SearchCacheTTL 搜索结果缓存有效期（决策 D3：1 天）。
const SearchCacheTTL = 24 * time.Hour

// SearchCacheMaxEntries 内存 LRU 最大条目数。
const SearchCacheMaxEntries = 256

type cacheEntry struct {
	results []search.Result
	expiry  time.Time
}

type searchCache struct {
	mu      sync.Mutex
	entries map[string]*cacheEntry
	order   []string // FIFO 近似 LRU
}

var defaultCache = &searchCache{
	entries: make(map[string]*cacheEntry),
}

func normalizeKey(keyword string) string {
	return strings.ToLower(strings.TrimSpace(keyword))
}

func (c *searchCache) Get(keyword string) ([]search.Result, bool) {
	key := normalizeKey(keyword)
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiry) {
		delete(c.entries, key)
		return nil, false
	}
	out := make([]search.Result, len(e.results))
	copy(out, e.results)
	return out, true
}

func (c *searchCache) Put(keyword string, results []search.Result) {
	if len(results) == 0 {
		return
	}
	key := normalizeKey(keyword)
	c.mu.Lock()
	defer c.mu.Unlock()

	cp := make([]search.Result, len(results))
	copy(cp, results)
	if _, exists := c.entries[key]; !exists {
		c.order = append(c.order, key)
	}
	c.entries[key] = &cacheEntry{
		results: cp,
		expiry:  time.Now().Add(SearchCacheTTL),
	}

	for len(c.order) > SearchCacheMaxEntries {
		evict := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, evict)
	}
}

func (c *searchCache) Invalidate(keyword string) {
	key := normalizeKey(keyword)
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// --- 包级 API ---

// CacheGet 从缓存获取搜索结果。
func CacheGet(keyword string) ([]search.Result, bool) { return defaultCache.Get(keyword) }

// CachePut 写入搜索结果缓存。
func CachePut(keyword string, results []search.Result) { defaultCache.Put(keyword, results) }

// CacheInvalidate 使某个关键词的缓存失效。
func CacheInvalidate(keyword string) { defaultCache.Invalidate(keyword) }
