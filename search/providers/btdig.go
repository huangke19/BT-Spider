package providers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/huangke/bt-spider/pkg/httputil"
	"github.com/huangke/bt-spider/search"
)

// BtDig 基于 BTDigg DHT 搜索引擎
type BtDig struct {
	baseURL string
	client  *http.Client
}

func NewBtDig() *BtDig {
	return &BtDig{
		baseURL: "https://btdig.com",
		client:  httputil.NewClient(httputil.DefaultTimeout),
	}
}

func (b *BtDig) Name() string {
	return "BTDigg"
}

func (b *BtDig) Search(keyword string, page int) ([]search.Result, error) {
	searchURL := fmt.Sprintf("%s/search?q=%s&p=%d&order=0",
		b.baseURL, url.QueryEscape(keyword), page)

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

	return b.parseHTML(string(body))
}

var (
	btdigNamePattern  = regexp.MustCompile(`<div class="torrent_name">.*?<a href="/([0-9a-fA-F]{40})"[^>]*>(.+?)</a>`)
	btdigSizePattern  = regexp.MustCompile(`<span class="torrent_size"[^>]*>([^<]+)</span>`)
)

func (b *BtDig) parseHTML(html string) ([]search.Result, error) {
	var results []search.Result

	entries := strings.Split(html, `<div class="one_result">`)
	if len(entries) <= 1 {
		entries = strings.Split(html, `<div class="torrent_name">`)
	}

	for _, entry := range entries[1:] {
		nameMatch := btdigNamePattern.FindStringSubmatch(entry)
		if nameMatch == nil {
			hashPattern := regexp.MustCompile(`/([0-9a-fA-F]{40})`)
			hashMatch := hashPattern.FindStringSubmatch(entry)
			if hashMatch == nil {
				continue
			}

			titlePattern := regexp.MustCompile(`>([^<]{5,})</a>`)
			titleMatch := titlePattern.FindStringSubmatch(entry)
			name := "Unknown"
			if titleMatch != nil {
				name = strings.TrimSpace(titleMatch[1])
			}

			nameMatch = []string{"", hashMatch[1], name}
		}

		infoHash := nameMatch[1]
		name := strings.TrimSpace(nameMatch[2])
		name = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(name, "")
		name = strings.TrimSpace(name)

		if name == "" || infoHash == "" {
			continue
		}

		size := "未知"
		sizeMatch := btdigSizePattern.FindStringSubmatch(entry)
		if sizeMatch != nil {
			size = strings.TrimSpace(sizeMatch[1])
		}

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
