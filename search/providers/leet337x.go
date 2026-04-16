package providers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/huangke/bt-spider/pkg/httputil"
	"github.com/huangke/bt-spider/search"
)

const leet337BaseURL = "https://www.1337xx.to"

type Leet337x struct {
	client *http.Client
}

func NewLeet337x() *Leet337x {
	return &Leet337x{client: httputil.NewClient(15 * time.Second)}
}

func (l *Leet337x) Name() string { return "1337x" }

func (l *Leet337x) Search(keyword string, page int) ([]search.Result, error) {
	if page < 1 {
		page = 1
	}
	slug := strings.ReplaceAll(keyword, " ", "-")
	u := fmt.Sprintf("%s/search/%s/%d/", leet337BaseURL, url.PathEscape(slug), page)
	body, err := l.get(u)
	if err != nil {
		return nil, err
	}

	reHref := regexp.MustCompile(`href="(/torrent/\d+/([^/"]+)/)"`)
	reSeeds := regexp.MustCompile(`<td class="coll-2 seeds">(\d+)</td>`)
	reLeeches := regexp.MustCompile(`<td class="coll-3 leeches">(\d+)</td>`)

	hrefs := reHref.FindAllStringSubmatch(body, -1)
	seeds := reSeeds.FindAllStringSubmatch(body, -1)
	leeches := reLeeches.FindAllStringSubmatch(body, -1)

	type listItem struct {
		href    string
		slug    string
		seeders int
		leeches int
	}

	limit := 10
	if len(hrefs) < limit {
		limit = len(hrefs)
	}

	items := make([]listItem, 0, limit)
	for i := 0; i < limit; i++ {
		s := 0
		lc := 0
		if i < len(seeds) {
			fmt.Sscanf(seeds[i][1], "%d", &s)
		}
		if i < len(leeches) {
			fmt.Sscanf(leeches[i][1], "%d", &lc)
		}
		items = append(items, listItem{
			href:    hrefs[i][1],
			slug:    hrefs[i][2],
			seeders: s,
			leeches: lc,
		})
	}

	results := make([]search.Result, len(items))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)

	reMagnet := regexp.MustCompile(`(magnet:\?xt=urn:btih:[A-Za-z0-9]+[^"'\s]*)`)
	reSize := regexp.MustCompile(`<strong>Total size</strong><span>([^<]+)</span>`)
	reHash := regexp.MustCompile(`urn:btih:([A-Za-z0-9]+)`)

	for i, item := range items {
		wg.Add(1)
		go func(idx int, it listItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			detailURL := leet337BaseURL + it.href
			detail, err := l.get(detailURL)
			if err != nil {
				return
			}

			mMagnet := reMagnet.FindStringSubmatch(detail)
			mSize := reSize.FindStringSubmatch(detail)
			if mMagnet == nil {
				return
			}

			magnet := mMagnet[1]
			mHash := reHash.FindStringSubmatch(magnet)
			infoHash := ""
			if mHash != nil {
				infoHash = strings.ToUpper(mHash[1])
			}

			size := ""
			if mSize != nil {
				size = strings.TrimSpace(mSize[1])
			}

			name := strings.ReplaceAll(it.slug, "-", " ")

			results[idx] = search.Result{
				Name:     name,
				Size:     size,
				Seeders:  it.seeders,
				Leechers: it.leeches,
				Magnet:   magnet,
				InfoHash: infoHash,
				Source:   "1337x",
			}
		}(i, item)
	}
	wg.Wait()

	var out []search.Result
	for _, r := range results {
		if r.Magnet != "" {
			out = append(out, r)
		}
	}
	return out, nil
}

func (l *Leet337x) get(u string) (string, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := l.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}
