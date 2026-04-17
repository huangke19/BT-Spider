package providers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/huangke/bt-spider/pkg/httputil"
	"github.com/huangke/bt-spider/pkg/utils"
	"github.com/huangke/bt-spider/search"
)

// ApiBay 基于 ThePirateBay API 的搜索源
type ApiBay struct {
	baseURL string
	client  *httputil.ResilientClient
}

func NewApiBay() *ApiBay {
	return &ApiBay{
		baseURL: "https://apibay.org",
		client:  httputil.NewSearchClient(),
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

func (a *ApiBay) Search(keyword string, page int) ([]search.Result, error) {
	apiURL := fmt.Sprintf("%s/q.php?q=%s&cat=", a.baseURL, url.QueryEscape(keyword))

	body, err := a.client.Get(apiURL)
	if err != nil {
		return nil, err
	}

	var apiResults []apiBayResult
	if err := json.Unmarshal(body, &apiResults); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	var results []search.Result
	for _, r := range apiResults {
		if r.ID == "0" || r.Name == "No results returned" {
			continue
		}

		seeders, _ := strconv.Atoi(r.Seeders)
		leechers, _ := strconv.Atoi(r.Leechers)
		sizeBytes, _ := strconv.ParseInt(r.Size, 10, 64)

		result := search.Result{
			Name:     r.Name,
			Size:     utils.FormatBytes(sizeBytes),
			Seeders:  seeders,
			Leechers: leechers,
			InfoHash: r.InfoHash,
			Source:   a.Name(),
		}
		result.Magnet = search.BuildMagnet(r.InfoHash, url.QueryEscape(r.Name))

		results = append(results, result)
	}

	results = filterRelevant(results, keyword)

	return results, nil
}

func filterRelevant(results []search.Result, keyword string) []search.Result {
	tokens := keywordTokens(keyword)
	if len(tokens) == 0 {
		return results
	}

	var filtered []search.Result
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

func keywordTokens(keyword string) []string {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return nil
	}

	var tokens []string

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

	for _, word := range strings.Fields(keyword) {
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
