package search

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ApiBay 基于 ThePirateBay API 的搜索源
type ApiBay struct {
	baseURL string
	client  *http.Client
}

func NewApiBay() *ApiBay {
	return &ApiBay{
		baseURL: "https://apibay.org",
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (a *ApiBay) Name() string {
	return "ThePirateBay"
}

type apiBayResult struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	InfoHash string `json:"info_hash"`
	Leechers string `json:"leechers"`
	Seeders  string `json:"seeders"`
	Size     string `json:"size"`
	Category string `json:"category"`
}

func (a *ApiBay) Search(keyword string, page int) ([]Result, error) {
	apiURL := fmt.Sprintf("%s/q.php?q=%s&cat=", a.baseURL, url.QueryEscape(keyword))

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "BT-Spider/1.0")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 返回 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var apiResults []apiBayResult
	if err := json.Unmarshal(body, &apiResults); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	var results []Result
	for _, r := range apiResults {
		// apibay 返回 id=0 name="No results" 表示无结果
		if r.ID == "0" || r.Name == "No results returned" {
			continue
		}

		seeders, _ := strconv.Atoi(r.Seeders)
		leechers, _ := strconv.Atoi(r.Leechers)
		sizeBytes, _ := strconv.ParseInt(r.Size, 10, 64)

		result := Result{
			Name:     r.Name,
			Size:     formatSize(sizeBytes),
			Seeders:  seeders,
			Leechers: leechers,
			InfoHash: r.InfoHash,
			Source:   a.Name(),
		}
		result.Magnet = BuildMagnet(r.InfoHash, url.QueryEscape(r.Name))

		results = append(results, result)
	}

	// apibay 搜不到时会返回热门列表，需要过滤掉无关结果
	results = filterRelevant(results, keyword)

	return results, nil
}

// filterRelevant 过滤与关键词不相关的结果。
// apibay 在搜索无结果时会返回全站热门，导致中文等关键词搜出无关内容。
func filterRelevant(results []Result, keyword string) []Result {
	tokens := keywordTokens(keyword)
	if len(tokens) == 0 {
		return results
	}

	var filtered []Result
	for _, r := range results {
		nameLower := strings.ToLower(r.Name)
		for _, tok := range tokens {
			if strings.Contains(nameLower, tok) {
				filtered = append(filtered, r)
				break
			}
		}
	}
	return filtered
}

// keywordTokens 将关键词拆分为用于匹配的 token 列表（全小写）。
// 中文按单字拆分（连续中文字符作为整体），英文按空格拆分。
func keywordTokens(keyword string) []string {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return nil
	}

	var tokens []string

	// 提取连续的中文子串
	var cjk []rune
	for _, r := range keyword {
		if unicode.Is(unicode.Han, r) {
			cjk = append(cjk, r)
		} else {
			if len(cjk) > 0 {
				tokens = append(tokens, string(cjk))
				cjk = nil
			}
		}
	}
	if len(cjk) > 0 {
		tokens = append(tokens, string(cjk))
	}

	// 提取英文单词（按空格/标点分割，过滤短词）
	for _, word := range strings.Fields(keyword) {
		// 去掉非字母数字
		cleaned := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return r
			}
			return -1
		}, word)
		if len(cleaned) >= 2 && !unicode.Is(unicode.Han, []rune(cleaned)[0]) {
			tokens = append(tokens, cleaned)
		}
	}

	return tokens
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
