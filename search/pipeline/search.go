package pipeline

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/huangke/bt-spider/pkg/logger"
	"github.com/huangke/bt-spider/search"
)

const DefaultSearchTimeout = 8 * time.Second

// Search 使用所有可用源并发搜索关键词，合并去重，按做种数排序
func Search(keyword string, providers []search.Provider) ([]search.Result, error) {
	return SearchWithTimeout(keyword, providers, DefaultSearchTimeout)
}

// SearchWithTimeout 使用所有可用源并发搜索关键词，并在 timeout 到期后返回已拿到的结果。
func SearchWithTimeout(keyword string, providers []search.Provider, timeout time.Duration) ([]search.Result, error) {
	if timeout <= 0 {
		timeout = DefaultSearchTimeout
	}

	strictQuery, strictMode := parseStrictMovieQuery(keyword)

	type providerResult struct {
		name    string
		results []search.Result
		err     error
	}
	ch := make(chan providerResult, len(providers))
	pending := make(map[string]struct{}, len(providers))
	for _, p := range providers {
		pending[p.Name()] = struct{}{}
		go func(p search.Provider) {
			results, err := p.Search(keyword, 0)
			ch <- providerResult{name: p.Name(), results: results, err: err}
		}(p)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	logger.Debug("search start", "keyword", keyword, "providers", len(providers), "timeout", timeout)

	var allResults []search.Result
	var errs []string
	for range providers {
		select {
		case pr := <-ch:
			delete(pending, pr.name)
			if pr.err != nil {
				logger.Warn("search provider error", "provider", pr.name, "keyword", keyword, "err", pr.err)
				errs = append(errs, fmt.Sprintf("%s: %v", pr.name, pr.err))
				continue
			}
			logger.Debug("search provider done", "provider", pr.name, "keyword", keyword, "count", len(pr.results))
			allResults = append(allResults, pr.results...)
		case <-timer.C:
			logger.Warn("search timeout", "keyword", keyword, "pending", strings.Join(sortedKeys(pending), ", "))
			results := finalizeResults(allResults, keyword, strictMode, strictQuery)
			if len(results) > 0 {
				logger.Info("search done (partial)", "keyword", keyword, "count", len(results))
				return results, nil
			}
			return nil, fmt.Errorf("搜索超时（%s），未及时返回结果；仍在等待: %s", timeout, strings.Join(sortedKeys(pending), ", "))
		}
	}

	results := finalizeResults(allResults, keyword, strictMode, strictQuery)
	if len(results) > 0 {
		logger.Info("search done", "keyword", keyword, "count", len(results))
		return results, nil
	}
	if len(errs) > 0 {
		logger.Warn("search all providers failed", "keyword", keyword, "errors", len(errs))
		return nil, fmt.Errorf("所有搜索源失败: %s", strings.Join(errs, "; "))
	}
	logger.Info("search done (no results)", "keyword", keyword)
	return nil, nil
}

func finalizeResults(allResults []search.Result, keyword string, strictMode bool, strictQuery strictMovieQuery) []search.Result {
	if strictMode {
		return finalizeStrictMovieResults(allResults, strictQuery)
	}

	allResults = filterByKeyword(allResults, keyword)
	allResults = dedup(allResults)

	var unknownHashes []string
	for _, r := range allResults {
		if r.Seeders == -1 && r.InfoHash != "" {
			unknownHashes = append(unknownHashes, r.InfoHash)
		}
	}
	if len(unknownHashes) > 0 {
		scraped := ScrapeSeeders(unknownHashes, 3*time.Second)
		for i := range allResults {
			if allResults[i].Seeders == -1 {
				if c, ok := scraped[strings.ToUpper(allResults[i].InfoHash)]; ok {
					allResults[i].Seeders = c
				} else {
					allResults[i].Seeders = 0
				}
			}
		}
	}

	var seeded []search.Result
	for _, r := range allResults {
		if r.Seeders > 0 {
			seeded = append(seeded, r)
		}
	}

	sort.Slice(seeded, func(i, j int) bool {
		return seeded[i].Seeders > seeded[j].Seeders
	})

	return seeded
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func dedup(results []search.Result) []search.Result {
	seen := make(map[string]int)
	var out []search.Result

	for _, r := range results {
		hash := strings.ToLower(r.InfoHash)
		if hash == "" {
			out = append(out, r)
			continue
		}
		if idx, ok := seen[hash]; ok {
			existing := &out[idx]
			if r.Seeders > existing.Seeders {
				knownSize := existing.Size
				*existing = r
				if existing.Size == "未知" && knownSize != "未知" {
					existing.Size = knownSize
				}
			} else if existing.Size == "未知" && r.Size != "未知" {
				existing.Size = r.Size
			}
		} else {
			seen[hash] = len(out)
			out = append(out, r)
		}
	}
	return out
}

func tokenize(keyword string) []string {
	keyword = strings.ToLower(keyword)
	runes := []rune(keyword)
	var tokens []string

	i := 0
	for i < len(runes) {
		if search.IsCJK(runes[i]) {
			start := i
			for i < len(runes) && search.IsCJK(runes[i]) {
				i++
			}
			seg := runes[start:i]
			if len(seg) == 1 {
				tokens = append(tokens, string(seg))
			} else {
				for j := 0; j+1 < len(seg); j++ {
					tokens = append(tokens, string(seg[j:j+2]))
				}
			}
		} else {
			start := i
			for i < len(runes) && !search.IsCJK(runes[i]) {
				i++
			}
			for _, w := range strings.Fields(string(runes[start:i])) {
				if utf8.RuneCountInString(w) >= 3 {
					tokens = append(tokens, w)
				}
			}
		}
	}
	return tokens
}

func filterByKeyword(results []search.Result, keyword string) []search.Result {
	tokens := tokenize(keyword)
	if len(tokens) == 0 {
		return results
	}

	minMatch := len(tokens)/2 + 1
	if minMatch > len(tokens) {
		minMatch = len(tokens)
	}

	var out []search.Result
	for _, r := range results {
		nameLower := strings.ToLower(r.Name)
		matched := 0
		for _, t := range tokens {
			if strings.Contains(nameLower, t) {
				matched++
			}
		}
		if matched >= minMatch {
			out = append(out, r)
		}
	}

	if len(out) == 0 {
		return results
	}
	return out
}
