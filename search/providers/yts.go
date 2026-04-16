package providers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/huangke/bt-spider/pkg/httputil"
	"github.com/huangke/bt-spider/search"
)

// YTS 基于 YTS JSON API 的搜索源（电影资源）
type YTS struct {
	baseURL string
	client  *httputil.ResilientClient
}

func NewYTS() *YTS {
	return &YTS{
		baseURL: "https://yts.mx/api/v2",
		client:  httputil.NewResilientClient(httputil.WithTimeout(3 * time.Second)),
	}
}

func (y *YTS) Name() string {
	return "YTS"
}

type ytsResponse struct {
	Status string  `json:"status"`
	Data   ytsData `json:"data"`
}

type ytsData struct {
	MovieCount int        `json:"movie_count"`
	Movies     []ytsMovie `json:"movies"`
}

type ytsMovie struct {
	Title    string       `json:"title"`
	Year     int          `json:"year"`
	Torrents []ytsTorrent `json:"torrents"`
}

type ytsTorrent struct {
	Hash    string `json:"hash"`
	Quality string `json:"quality"`
	Type    string `json:"type"`
	Size    string `json:"size"`
	Seeds   int    `json:"seeds"`
	Peers   int    `json:"peers"`
}

func (y *YTS) Search(keyword string, page int) ([]search.Result, error) {
	apiURL := fmt.Sprintf("%s/list_movies.json?query_term=%s&sort_by=seeds&order_by=desc&limit=50&page=%d",
		y.baseURL, url.QueryEscape(keyword), page+1)

	body, err := y.client.Get(apiURL)
	if err != nil {
		return nil, err
	}

	var ytsResp ytsResponse
	if err := json.Unmarshal(body, &ytsResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if ytsResp.Status != "ok" {
		return nil, fmt.Errorf("API 状态异常: %s", ytsResp.Status)
	}

	var results []search.Result
	for _, movie := range ytsResp.Data.Movies {
		for _, t := range movie.Torrents {
			name := fmt.Sprintf("%s (%d) [%s] [%s]", movie.Title, movie.Year, t.Quality, t.Type)
			result := search.Result{
				Name:     name,
				Size:     t.Size,
				Seeders:  t.Seeds,
				Leechers: t.Peers,
				InfoHash: t.Hash,
				Source:   y.Name(),
				Magnet:   search.BuildMagnet(t.Hash, url.QueryEscape(name)),
			}
			results = append(results, result)
		}
	}

	return results, nil
}
