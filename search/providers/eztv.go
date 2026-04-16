package providers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/huangke/bt-spider/pkg/httputil"
	"github.com/huangke/bt-spider/pkg/utils"
	"github.com/huangke/bt-spider/search"
)

// EZTV 基于 EZTV JSON API 的搜索源（美剧资源）
type EZTV struct {
	baseURL string
	client  *httputil.ResilientClient
}

func NewEZTV() *EZTV {
	return &EZTV{
		baseURL: "https://eztv.re/api",
		client:  httputil.NewResilientClient(httputil.WithMaxBody(4 << 20)),
	}
}

func (e *EZTV) Name() string {
	return "EZTV"
}

type eztvResponse struct {
	TorrentsCount int           `json:"torrents_count"`
	Limit         int           `json:"limit"`
	Page          int           `json:"page"`
	Torrents      []eztvTorrent `json:"torrents"`
}

type eztvTorrent struct {
	Hash      string `json:"hash"`
	Filename  string `json:"filename"`
	Title     string `json:"title"`
	SizeBytes string `json:"size_bytes"`
	Seeds     int    `json:"seeds"`
	Peers     int    `json:"peers"`
	MagnetURL string `json:"magnet_url"`
}

func (e *EZTV) Search(keyword string, page int) ([]search.Result, error) {
	apiURL := fmt.Sprintf("%s/get-torrents?limit=100&page=%d", e.baseURL, page+1)

	body, err := e.client.Get(apiURL)
	if err != nil {
		return nil, err
	}

	var ezResp eztvResponse
	if err := json.Unmarshal(body, &ezResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	keywordLower := eztvNormalize(keyword)
	var results []search.Result
	for _, t := range ezResp.Torrents {
		if !eztvMatchKeyword(t.Title, keywordLower) && !eztvMatchKeyword(t.Filename, keywordLower) {
			continue
		}

		name := t.Title
		if name == "" {
			name = t.Filename
		}
		if name == "" || t.Hash == "" {
			continue
		}

		sizeBytes, _ := strconv.ParseInt(t.SizeBytes, 10, 64)

		result := search.Result{
			Name:     name,
			Size:     utils.FormatBytes(sizeBytes),
			Seeders:  t.Seeds,
			Leechers: t.Peers,
			InfoHash: t.Hash,
			Source:   e.Name(),
		}
		if t.MagnetURL != "" {
			result.Magnet = t.MagnetURL
		} else {
			result.Magnet = search.BuildMagnet(t.Hash, url.QueryEscape(name))
		}

		results = append(results, result)
	}

	return results, nil
}

func eztvNormalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func eztvMatchKeyword(text, keyword string) bool {
	return strings.Contains(strings.ToLower(text), keyword)
}
