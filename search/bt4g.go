package search

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// BT4G 基于 BT4G RSS 接口的搜索源
type BT4G struct {
	baseURL string
	client  *http.Client
}

func NewBT4G() *BT4G {
	return &BT4G{
		baseURL: "https://bt4gprx.com",
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (b *BT4G) Name() string {
	return "BT4G"
}

// RSS XML 结构
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

func (b *BT4G) Search(keyword string, page int) ([]Result, error) {
	searchURL := fmt.Sprintf("%s/search?q=%s&page=rss&bt4g_order_by=seeders",
		b.baseURL, url.QueryEscape(keyword))

	req, err := http.NewRequest(http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

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

	var results []Result
	for _, item := range rss.Channel.Items {
		// 从链接或描述中提取 info_hash
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

		// 从描述中提取大小
		size := extractSize(item.Description)

		result := Result{
			Name:     name,
			Size:     size,
			Seeders:  1, // RSS 不提供精确做种数，但按 seeders 排序所以存在即有
			Leechers: 0,
			InfoHash: infoHash,
			Source:   b.Name(),
			Magnet:   BuildMagnet(infoHash, url.QueryEscape(name)),
		}
		results = append(results, result)
	}

	return results, nil
}

// extractHash 从文本中提取 40 位十六进制 info_hash
func extractHash(text string) string {
	match := bt4gHashPattern.FindString(text)
	return strings.ToUpper(match)
}

var bt4gSizePattern = regexp.MustCompile(`(?i)([\d.]+)\s*(GB|MB|KB|TB|B)\b`)

// extractSize 从描述文本中提取文件大小
func extractSize(desc string) string {
	match := bt4gSizePattern.FindStringSubmatch(desc)
	if match != nil {
		return match[1] + " " + strings.ToUpper(match[2])
	}
	return "未知"
}
