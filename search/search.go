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

// Search 使用所有可用源搜索关键词，合并去重，按做种数排序
func Search(keyword string, providers []Provider) ([]Result, error) {
	var allResults []Result
	var lastErr error

	for _, p := range providers {
		results, err := p.Search(keyword, 0)
		if err != nil {
			lastErr = err
			continue
		}
		allResults = append(allResults, results...)
	}

	if len(allResults) == 0 && lastErr != nil {
		return nil, fmt.Errorf("所有搜索源失败: %w", lastErr)
	}

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
