package search

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

// 用于 scrape 的 UDP tracker（BEP 15）。这些 tracker 健康、响应快。
var scrapeTrackers = []string{
	"udp://tracker.opentrackr.org:1337",
	"udp://open.demonii.com:1337",
	"udp://tracker.torrent.eu.org:451",
	"udp://open.stealth.si:80",
}

const scrapeBatchSize = 70 // BEP 15 限制一次最多 74 个 infohash

// ScrapeSeeders 查询多个 tracker 获取每个 infohash 的真实做种数。
// 每个 hash 取所有 tracker 中最高的值（任何一个 tracker 有数据即可信）。
// 返回 infohash（大写 hex）-> seeders。
func ScrapeSeeders(infoHashes []string, timeout time.Duration) map[string]int {
	result := make(map[string]int, len(infoHashes))
	if len(infoHashes) == 0 {
		return result
	}

	// 去重 + 解码 hex
	seen := make(map[string]bool)
	var hashBytes [][20]byte
	var hashHex []string
	for _, h := range infoHashes {
		hu := strings.ToUpper(h)
		if seen[hu] || len(hu) != 40 {
			continue
		}
		var b [20]byte
		if _, err := hex.Decode(b[:], []byte(hu)); err != nil {
			continue
		}
		seen[hu] = true
		hashBytes = append(hashBytes, b)
		hashHex = append(hashHex, hu)
		result[hu] = 0
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, tr := range scrapeTrackers {
		wg.Add(1)
		go func(tr string) {
			defer wg.Done()
			for i := 0; i < len(hashBytes); i += scrapeBatchSize {
				end := i + scrapeBatchSize
				if end > len(hashBytes) {
					end = len(hashBytes)
				}
				counts, err := scrapeUDP(tr, hashBytes[i:end], timeout)
				if err != nil {
					return // 该 tracker 该批次失败，尝试下一个 tracker
				}
				mu.Lock()
				for j, c := range counts {
					hu := hashHex[i+j]
					if c > result[hu] {
						result[hu] = c
					}
				}
				mu.Unlock()
			}
		}(tr)
	}
	wg.Wait()
	return result
}

func scrapeUDP(trackerURL string, hashes [][20]byte, timeout time.Duration) ([]int, error) {
	u, err := url.Parse(trackerURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "udp" {
		return nil, fmt.Errorf("仅支持 udp tracker")
	}

	conn, err := net.DialTimeout("udp", u.Host, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))

	// --- connect 请求 ---
	transID := rand.Uint32()
	connReq := make([]byte, 16)
	binary.BigEndian.PutUint64(connReq[0:], 0x41727101980) // 协议魔数
	binary.BigEndian.PutUint32(connReq[8:], 0)             // action = connect
	binary.BigEndian.PutUint32(connReq[12:], transID)
	if _, err := conn.Write(connReq); err != nil {
		return nil, err
	}
	connResp := make([]byte, 16)
	n, err := conn.Read(connResp)
	if err != nil {
		return nil, err
	}
	if n < 16 ||
		binary.BigEndian.Uint32(connResp[0:4]) != 0 ||
		binary.BigEndian.Uint32(connResp[4:8]) != transID {
		return nil, fmt.Errorf("connect 响应无效")
	}
	connectionID := binary.BigEndian.Uint64(connResp[8:16])

	// --- scrape 请求 ---
	transID2 := rand.Uint32()
	scrapeReq := make([]byte, 16+20*len(hashes))
	binary.BigEndian.PutUint64(scrapeReq[0:], connectionID)
	binary.BigEndian.PutUint32(scrapeReq[8:], 2) // action = scrape
	binary.BigEndian.PutUint32(scrapeReq[12:], transID2)
	for i, h := range hashes {
		copy(scrapeReq[16+i*20:], h[:])
	}
	if _, err := conn.Write(scrapeReq); err != nil {
		return nil, err
	}

	scrapeResp := make([]byte, 8+12*len(hashes))
	n, err = conn.Read(scrapeResp)
	if err != nil {
		return nil, err
	}
	if n < 8 ||
		binary.BigEndian.Uint32(scrapeResp[0:4]) != 2 ||
		binary.BigEndian.Uint32(scrapeResp[4:8]) != transID2 {
		return nil, fmt.Errorf("scrape 响应无效")
	}

	counts := make([]int, len(hashes))
	for i := range hashes {
		off := 8 + i*12
		if off+12 > n {
			break
		}
		counts[i] = int(binary.BigEndian.Uint32(scrapeResp[off : off+4]))
	}
	return counts, nil
}
