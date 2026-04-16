package query

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"time"

	"github.com/huangke/bt-spider/pkg/httputil"
)

var tmdbClient = httputil.NewResilientClient(httputil.WithTimeout(5 * time.Second))

type tmdbSearchResponse struct {
	Results []tmdbMovie `json:"results"`
}

type tmdbMovie struct {
	Title       string  `json:"title"`
	ReleaseDate string  `json:"release_date"`
	VoteCount   int     `json:"vote_count"`
	Popularity  float64 `json:"popularity"`
}

// SearchTMDB 用片名查询 TMDB，返回英文标准标题 + 年份。
func SearchTMDB(query, apiKey string) (movieMeta, bool) {
	endpoint := fmt.Sprintf(
		"https://api.themoviedb.org/3/search/movie?query=%s&language=en-US&page=1",
		url.QueryEscape(query),
	)

	body, err := tmdbClient.GetWithHeaders(endpoint, map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Accept":        "application/json",
	})
	if err != nil {
		return movieMeta{}, false
	}

	var result tmdbSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return movieMeta{}, false
	}
	if len(result.Results) == 0 {
		return movieMeta{}, false
	}

	sort.SliceStable(result.Results, func(i, j int) bool {
		if result.Results[i].VoteCount != result.Results[j].VoteCount {
			return result.Results[i].VoteCount > result.Results[j].VoteCount
		}
		return result.Results[i].Popularity > result.Results[j].Popularity
	})

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
