package providers

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/huangke/bt-spider/pkg/httputil"
	"github.com/huangke/bt-spider/search"
)

// Nyaa 基于 Nyaa.si RSS 的搜索源（动漫/日语资源）
type Nyaa struct {
	baseURL string
	client  *httputil.ResilientClient
}

func NewNyaa() *Nyaa {
	return &Nyaa{
		baseURL: "https://nyaa.si",
		client:  httputil.NewResilientClient(httputil.WithTimeout(3 * time.Second)),
	}
}

func (n *Nyaa) Name() string {
	return "Nyaa"
}

type nyaaRSS struct {
	XMLName xml.Name    `xml:"rss"`
	Channel nyaaChannel `xml:"channel"`
}

type nyaaChannel struct {
	Items []nyaaItem `xml:"item"`
}

type nyaaItem struct {
	Title    string `xml:"title"`
	Link     string `xml:"link"`
	GUID     string `xml:"guid"`
	Seeders  string `xml:"http://nyaa.si/xmlns/nyaa seeders"`
	Leechers string `xml:"http://nyaa.si/xmlns/nyaa leechers"`
	Size     string `xml:"http://nyaa.si/xmlns/nyaa size"`
	InfoHash string `xml:"http://nyaa.si/xmlns/nyaa infoHash"`
}

var nyaaHashPattern = regexp.MustCompile(`(?i)[0-9a-f]{40}`)

func (n *Nyaa) Search(keyword string, page int) ([]search.Result, error) {
	searchURL := fmt.Sprintf("%s/?page=rss&q=%s&s=seeders&o=desc",
		n.baseURL, url.QueryEscape(keyword))

	body, err := n.client.Get(searchURL)
	if err != nil {
		return nil, err
	}

	var rss nyaaRSS
	if err := xml.Unmarshal(body, &rss); err != nil {
		return nil, fmt.Errorf("解析 RSS 失败: %w", err)
	}

	var results []search.Result
	for _, item := range rss.Channel.Items {
		name := strings.TrimSpace(item.Title)
		if name == "" {
			continue
		}

		infoHash := item.InfoHash
		if infoHash == "" {
			infoHash = nyaaHashPattern.FindString(item.Link)
			if infoHash == "" {
				infoHash = nyaaHashPattern.FindString(item.GUID)
			}
		}
		if infoHash == "" {
			continue
		}

		seeders, _ := strconv.Atoi(item.Seeders)
		leechers, _ := strconv.Atoi(item.Leechers)

		size := item.Size
		if size == "" {
			size = "未知"
		}

		result := search.Result{
			Name:     name,
			Size:     size,
			Seeders:  seeders,
			Leechers: leechers,
			InfoHash: strings.ToUpper(infoHash),
			Source:   n.Name(),
			Magnet:   search.BuildMagnet(strings.ToUpper(infoHash), url.QueryEscape(name)),
		}
		results = append(results, result)
	}

	return results, nil
}
