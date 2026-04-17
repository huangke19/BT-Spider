package providers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/huangke/bt-spider/pkg/httputil"
	"github.com/huangke/bt-spider/search"
)

type YTS struct {
	baseURL string
	client  *httputil.ResilientClient
}

func NewYTS() *YTS {
	return &YTS{
		baseURL: "https://yts.mx/api/v2",
		client:  httputil.NewSearchClient(httputil.WithTimeout(3 * time.Second)),
	}
}

func (y *YTS) Name() string {
	return "YTS"
}

type ytsResponse struct {
	Data ytsData `json:"data"`
}

type ytsData struct {
	Movies []ytsMovie `json:"movies"`
}

type ytsMovie struct {
	Title    string       `json:"title_long"`
	Torrents []ytsTorrent `json:"torrents"`
}

type ytsTorrent struct {
	Hash    string `json:"hash"`
	Quality string `json:"quality"`
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

	var resp ytsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	var results []search.Result
	for _, m := range resp.Data.Movies {
		for _, t := range m.Torrents {
			if t.Hash == "" {
				continue
			}
			name := fmt.Sprintf("%s [%s] [%s]", m.Title, t.Quality, t.Size)
			results = append(results, search.Result{
				Name:     name,
				Size:     t.Size,
				Seeders:  t.Seeds,
				Leechers: t.Peers,
				InfoHash: t.Hash,
				Source:   y.Name(),
				Magnet:   search.BuildMagnet(t.Hash, url.QueryEscape(name)),
			})
		}
	}
	return results, nil
}
