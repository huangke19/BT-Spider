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

// TorrentKitty 基于 torrentkitty.tv 的搜索源，磁力聚合站
type TorrentKitty struct {
	baseURL string
	client  *http.Client
}

func NewTorrentKitty() *TorrentKitty {
	return &TorrentKitty{
		baseURL: "https://www.torrentkitty.tv",
		client:  httputil.NewClient(httputil.DefaultTimeout),
	}
}

func (t *TorrentKitty) Name() string {
	return "TorrentKitty"
}

var (
	tkRowPattern    = regexp.MustCompile(`(?s)<tr[^>]*>(.+?)</tr>`)
	tkNamePattern   = regexp.MustCompile(`<td class="name">([^<]+)</td>`)
	tkMagnetPattern = regexp.MustCompile(`href="(magnet:\?xt=urn:btih:[^"]+)"`)
)

func (t *TorrentKitty) Search(keyword string, page int) ([]search.Result, error) {
	searchURL := fmt.Sprintf("%s/search/%s", t.baseURL, url.PathEscape(keyword))

	req, err := http.NewRequest(http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", httputil.DefaultUA)

	resp, err := t.client.Do(req)
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

	html := string(body)

	rows := tkRowPattern.FindAllStringSubmatch(html, -1)
	var results []search.Result
	for _, row := range rows {
		cell := row[1]

		mName := tkNamePattern.FindStringSubmatch(cell)
		mMagnet := tkMagnetPattern.FindStringSubmatch(cell)

		if mName == nil || mMagnet == nil {
			continue
		}

		name := strings.TrimSpace(mName[1])
		magnet := strings.TrimSpace(mMagnet[1])
		size := "未知"

		infoHash := extractHashFromMagnet(magnet)
		if infoHash == "" {
			continue
		}

		results = append(results, search.Result{
			Name:     name,
			Size:     size,
			Seeders:  -1,
			Leechers: 0,
			InfoHash: infoHash,
			Source:   t.Name(),
			Magnet:   magnet,
		})
	}

	return results, nil
}

var magnetHashPattern = regexp.MustCompile(`(?i)urn:btih:([0-9a-f]{40})`)

func extractHashFromMagnet(magnet string) string {
	match := magnetHashPattern.FindStringSubmatch(magnet)
	if len(match) >= 2 {
		return strings.ToUpper(match[1])
	}
	return ""
}
