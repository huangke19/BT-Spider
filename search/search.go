package search

import (
	"fmt"
	"sort"
	"strings"
)

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
	}
}

// Search 使用所有可用源并发搜索关键词，合并去重，按做种数排序
func Search(keyword string, providers []Provider) ([]Result, error) {
	type providerResult struct {
		results []Result
		err     error
	}
	ch := make(chan providerResult, len(providers))
	for _, p := range providers {
		go func(p Provider) {
			results, err := p.Search(keyword, 0)
			ch <- providerResult{results, err}
		}(p)
	}
	var allResults []Result
	var lastErr error
	for range providers {
		pr := <-ch
		if pr.err != nil {
			lastErr = pr.err
			continue
		}
		allResults = append(allResults, pr.results...)
	}

	if len(allResults) == 0 && lastErr != nil {
		return nil, fmt.Errorf("所有搜索源失败: %w", lastErr)
	}

	// 关键词相关性过滤：名字必须包含至少一个关键词（过滤无关结果）
	allResults = filterByKeyword(allResults, keyword)

	// 按 info_hash 去重
	allResults = dedup(allResults)

	// 过滤无做种的
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

	return seeded, nil
}

// dedup 按 info_hash 去重，保留做种数更高的
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
			if r.Seeders > out[idx].Seeders {
				out[idx] = r
			}
		} else {
			seen[hash] = len(out)
			out = append(out, r)
		}
	}
	return out
}

// filterByKeyword 过滤掉名字里没有足够关键词的结果
// 策略：关键词里至少一半的词（长度>=3）必须出现在标题中
func filterByKeyword(results []Result, keyword string) []Result {
	words := strings.Fields(strings.ToLower(keyword))
	// 只保留长度>=3的词
	var meaningful []string
	for _, w := range words {
		if len(w) >= 3 {
			meaningful = append(meaningful, w)
		}
	}
	if len(meaningful) == 0 {
		return results
	}

	// 至少需要匹配的词数：超过一半
	minMatch := len(meaningful)/2 + 1
	if minMatch > len(meaningful) {
		minMatch = len(meaningful)
	}

	var out []Result
	for _, r := range results {
		nameLower := strings.ToLower(r.Name)
		matched := 0
		for _, w := range meaningful {
			if strings.Contains(nameLower, w) {
				matched++
			}
		}
		if matched >= minMatch {
			out = append(out, r)
		}
	}

	// 如果过滤后结果为空，退回原始结果
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
