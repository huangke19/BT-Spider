package search

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/huangke/bt-spider/pkg/logger"
)

const DefaultSearchTimeout = 8 * time.Second

// Result 搜索结果
type Result struct {
	Name     string `json:"name"`
	Size     string `json:"size"`
	Seeders  int    `json:"seeders"`
	Leechers int    `json:"leechers"`
	Magnet   string `json:"magnet"`
	Source   string `json:"source"`
	InfoHash string `json:"info_hash"`
}

// Provider 搜索源接口
type Provider interface {
	Name() string
	Search(keyword string, page int) ([]Result, error)
}

// DefaultProviders 返回所有内置的 torrent 搜索源
func DefaultProviders() []Provider {
	return []Provider{
		NewApiBay(),
		NewBtDig(),
		NewBT4G(),
		NewYTS(),
		NewEZTV(),
		NewNyaa(),
		NewLeet337x(),
		NewTorrentKitty(),
	}
}

// Search 使用所有可用源并发搜索关键词，合并去重，按做种数排序
func Search(keyword string, providers []Provider) ([]Result, error) {
	return SearchWithTimeout(keyword, providers, DefaultSearchTimeout)
}

// SearchWithTimeout 使用所有可用源并发搜索关键词，并在 timeout 到期后返回已拿到的结果。
func SearchWithTimeout(keyword string, providers []Provider, timeout time.Duration) ([]Result, error) {
	if timeout <= 0 {
		timeout = DefaultSearchTimeout
	}

	strictQuery, strictMode := parseStrictMovieQuery(keyword)

	type providerResult struct {
		name    string
		results []Result
		err     error
	}
	ch := make(chan providerResult, len(providers))
	pending := make(map[string]struct{}, len(providers))
	for _, p := range providers {
		pending[p.Name()] = struct{}{}
		go func(p Provider) {
			results, err := p.Search(keyword, 0)
			ch <- providerResult{name: p.Name(), results: results, err: err}
		}(p)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	logger.Debug("search start", "keyword", keyword, "providers", len(providers), "timeout", timeout)

	var allResults []Result
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

func finalizeResults(allResults []Result, keyword string, strictMode bool, strictQuery strictMovieQuery) []Result {
	if strictMode {
		return finalizeStrictMovieResults(allResults, strictQuery)
	}

	// 关键词相关性过滤：名字必须包含至少一个关键词（过滤无关结果）
	allResults = filterByKeyword(allResults, keyword)

	// 按 info_hash 去重
	allResults = dedup(allResults)

	// 对来源无做种数的结果（Seeders == -1），用 UDP tracker scrape 查询真实做种数
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
					allResults[i].Seeders = 0 // scrape 失败视为无做种
				}
			}
		}
	}

	// 只保留确认有做种的，避免下载死种
	var seeded []Result
	for _, r := range allResults {
		if r.Seeders > 0 {
			seeded = append(seeded, r)
		}
	}

	// 按做种数降序
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

// dedup 按 info_hash 去重，保留做种数更高的；若胜者大小未知则从其他来源借用。
func dedup(results []Result) []Result {
	seen := make(map[string]int) // info_hash -> index in output
	var out []Result

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
				// 胜者大小未知时，从之前的记录里借用已知大小
				if existing.Size == "未知" && knownSize != "未知" {
					existing.Size = knownSize
				}
			} else if existing.Size == "未知" && r.Size != "未知" {
				// 做种数不更高，但能补充大小
				existing.Size = r.Size
			}
		} else {
			seen[hash] = len(out)
			out = append(out, r)
		}
	}
	return out
}

// isCJK 判断字符是否为 CJK（中日韩）字符
func isCJK(r rune) bool {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF: // CJK 统一汉字
		return true
	case r >= 0x3400 && r <= 0x4DBF: // CJK 扩展 A
		return true
	case r >= 0x3040 && r <= 0x309F: // 平假名
		return true
	case r >= 0x30A0 && r <= 0x30FF: // 片假名
		return true
	case r >= 0xAC00 && r <= 0xD7AF: // 谚文音节
		return true
	}
	return false
}

// tokenize 按语言敏感方式把关键词拆分为 token：
//   - 连续 CJK 字符：切成 bigram（如"谍影重重" → 谍影/影重/重重）；单字段单独成 token
//   - 非 CJK（ASCII 等）：按空格分词，保留长度 ≥ 3 的词
func tokenize(keyword string) []string {
	keyword = strings.ToLower(keyword)
	runes := []rune(keyword)
	var tokens []string

	i := 0
	for i < len(runes) {
		if isCJK(runes[i]) {
			start := i
			for i < len(runes) && isCJK(runes[i]) {
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
			for i < len(runes) && !isCJK(runes[i]) {
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

// filterByKeyword 过滤掉名字里没有足够关键词的结果
// 策略：把关键词拆成 token（CJK 用 bigram，ASCII 按空格分词），至少一半的 token 需出现在标题中
func filterByKeyword(results []Result, keyword string) []Result {
	tokens := tokenize(keyword)
	if len(tokens) == 0 {
		return results
	}

	// 至少需要匹配的 token 数：超过一半
	minMatch := len(tokens)/2 + 1
	if minMatch > len(tokens) {
		minMatch = len(tokens)
	}

	var out []Result
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

	// 过滤后为空才退回原始结果（保守兜底，防止过滤算法偏差吞掉所有结果）
	if len(out) == 0 {
		return results
	}
	return out
}

// BuildMagnet 从 info_hash 构建磁力链接
func BuildMagnet(infoHash, name string) string {
	magnet := fmt.Sprintf("magnet:?xt=urn:btih:%s", infoHash)
	if name != "" {
		magnet += "&dn=" + name
	}
	// 添加常用 tracker
	trackers := []string{
		"udp://tracker.opentrackr.org:1337/announce",
		"udp://open.stealth.si:80/announce",
		"udp://tracker.torrent.eu.org:451/announce",
		"udp://tracker.bittor.pw:1337/announce",
		"udp://tracker.openbittorrent.com:6969/announce",
	}
	for _, tr := range trackers {
		magnet += "&tr=" + tr
	}
	return magnet
}
