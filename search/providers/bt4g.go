package providers

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/huangke/bt-spider/pkg/httputil"
	"github.com/huangke/bt-spider/search"
)

// BT4G 基于 BT4G RSS 接口的搜索源
type BT4G struct {
	baseURL string
	client  *http.Client
}

func NewBT4G() *BT4G {
	return &BT4G{
		baseURL: "https://bt4gprx.com",
		client:  httputil.NewClient(httputil.DefaultTimeout),
	}
}

func (b *BT4G) Name() string {
	return "BT4G"
}

type bt4gRSS struct {
	XMLName xml.Name    `xml:"rss"`
	Channel bt4gChannel `xml:"channel"`
}

type bt4gChannel struct {
	Items []bt4gItem `xml:"item"`
}

type bt4gItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
}

var bt4gHashPattern = regexp.MustCompile(`(?i)[0-9a-f]{40}`)

func (b *BT4G) Search(keyword string, page int) ([]search.Result, error) {
	searchURL := fmt.Sprintf("%s/search?q=%s&page=rss&bt4g_order_by=seeders",
		b.baseURL, url.QueryEscape(keyword))

	req, err := http.NewRequest(http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", httputil.DefaultUA)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("返回 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var rss bt4gRSS
	if err := xml.Unmarshal(body, &rss); err != nil {
		return nil, fmt.Errorf("解析 RSS 失败: %w", err)
	}

	var results []search.Result
	for _, item := range rss.Channel.Items {
		infoHash := extractHash(item.Link)
		if infoHash == "" {
			infoHash = extractHash(item.Description)
		}
		if infoHash == "" {
			continue
		}

		name := strings.TrimSpace(item.Title)
		if name == "" {
			continue
		}

		size := extractSize(item.Description)

		result := search.Result{
			Name:     name,
			Size:     size,
			Seeders:  -1,
			Leechers: 0,
			InfoHash: infoHash,
			Source:   b.Name(),
			Magnet:   search.BuildMagnet(infoHash, url.QueryEscape(name)),
		}
		results = append(results, result)
	}

	return results, nil
}

func extractHash(text string) string {
	match := bt4gHashPattern.FindString(text)
	return strings.ToUpper(match)
}

var bt4gSizePattern = regexp.MustCompile(`(?i)([\d.]+)\s*(GB|MB|KB|TB|B)\b`)

func extractSize(desc string) string {
	match := bt4gSizePattern.FindStringSubmatch(desc)
	if match != nil {
		return match[1] + " " + strings.ToUpper(match[2])
	}
	return "未知"
}
