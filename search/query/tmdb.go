package query

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/huangke/bt-spider/pkg/httputil"
)

var tmdbClient = httputil.NewResilientClient(httputil.WithTimeout(800 * time.Millisecond))

// --- TMDB 响应缓存 ---

type tmdbCacheItem struct {
	meta   movieMeta
	ok     bool
	expiry time.Time
}

const tmdbCacheTTL = 7 * 24 * time.Hour

var (
	tmdbCacheMu sync.RWMutex
	tmdbCache   = make(map[string]tmdbCacheItem)
)

func tmdbNormalizeKey(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}

func tmdbCacheGet(key string) (movieMeta, bool, bool) {
	tmdbCacheMu.RLock()
	item, ok := tmdbCache[key]
	tmdbCacheMu.RUnlock()
	if !ok {
		return movieMeta{}, false, false
	}
	if time.Now().After(item.expiry) {
		return movieMeta{}, false, false
	}
	return item.meta, item.ok, true
}

func tmdbCachePut(key string, meta movieMeta, ok bool) {
	tmdbCacheMu.Lock()
	tmdbCache[key] = tmdbCacheItem{meta: meta, ok: ok, expiry: time.Now().Add(tmdbCacheTTL)}
	tmdbCacheMu.Unlock()
}

type tmdbSearchResponse struct {
	Results []tmdbMovie `json:"results"`
}

type tmdbMovie struct {
	Title       string  `json:"title"`
	ReleaseDate string  `json:"release_date"`
	VoteCount   int     `json:"vote_count"`
	Popularity  float64 `json:"popularity"`
}

// SearchTMDB 用片名查询 TMDB，返回英文标准标题 + 年份。
// 结果缓存 7 天（含 miss），第二次查询同一输入耗时 < 1ms。
func SearchTMDB(query, apiKey string) (movieMeta, bool) {
	key := tmdbNormalizeKey(query)
	if meta, ok, hit := tmdbCacheGet(key); hit {
		return meta, ok
	}

	endpoint := fmt.Sprintf(
		"https://api.themoviedb.org/3/search/movie?query=%s&language=en-US&page=1",
		url.QueryEscape(query),
	)

	body, err := tmdbClient.GetWithHeaders(endpoint, map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Accept":        "application/json",
	})
	if err != nil {
		// 网络错误不缓存，下次继续重试
		return movieMeta{}, false
	}

	var result tmdbSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return movieMeta{}, false
	}
	if len(result.Results) == 0 {
		// 无结果也缓存（避免反复重试）
		tmdbCachePut(key, movieMeta{}, false)
		return movieMeta{}, false
	}

	sort.SliceStable(result.Results, func(i, j int) bool {
		if result.Results[i].VoteCount != result.Results[j].VoteCount {
			return result.Results[i].VoteCount > result.Results[j].VoteCount
		}
		return result.Results[i].Popularity > result.Results[j].Popularity
	})

	movie := result.Results[0]
	year := ""
	if len(movie.ReleaseDate) >= 4 {
		year = movie.ReleaseDate[:4]
	}
	if movie.Title == "" {
		tmdbCachePut(key, movieMeta{}, false)
		return movieMeta{}, false
	}
	meta := movieMeta{Title: movie.Title, Year: year}
	tmdbCachePut(key, meta, true)
	return meta, true
}
