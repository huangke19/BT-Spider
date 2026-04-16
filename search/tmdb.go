package search

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/huangke/bt-spider/pkg/httputil"
)

var tmdbClient = httputil.NewClient(5 * time.Second)

type tmdbSearchResponse struct {
	Results []tmdbMovie `json:"results"`
}

type tmdbMovie struct {
	Title       string `json:"title"`
	ReleaseDate string `json:"release_date"` // "2014-04-04"
}

// SearchTMDB 用片名（支持中文）查询 TMDB，返回英文标准标题 + 年份。
// apiKey 应为 TMDB API Read Access Token（Bearer token）。
func SearchTMDB(query, apiKey string) (movieMeta, bool) {
	endpoint := fmt.Sprintf(
		"https://api.themoviedb.org/3/search/movie?api_key=%s&query=%s&language=en-US&page=1",
		apiKey, url.QueryEscape(query),
	)

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return movieMeta{}, false
	}
	req.Header.Set("Accept", "application/json")

	resp, err := tmdbClient.Do(req)
	if err != nil {
		return movieMeta{}, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return movieMeta{}, false
	}

	var result tmdbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return movieMeta{}, false
	}
	if len(result.Results) == 0 {
		return movieMeta{}, false
	}

	movie := result.Results[0]
	year := ""
	if len(movie.ReleaseDate) >= 4 {
		year = movie.ReleaseDate[:4]
	}
	if movie.Title == "" {
		return movieMeta{}, false
	}
	return movieMeta{Title: movie.Title, Year: year}, true
}
