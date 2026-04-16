package search

import "fmt"

// Result 搜索结果
type Result struct {
	Name     string `json:"name"`
	Size     string `json:"size"`
	Seeders  int    `json:"seeders"`
	Leechers int    `json:"leechers"`
	Magnet   string `json:"magnet"`
	Source   string `json:"source"`
	InfoHash string `json:"info_hash"`
}

// Provider 搜索源接口
type Provider interface {
	Name() string
	Search(keyword string, page int) ([]Result, error)
}

// MovieResolution 已解析的电影搜索查询
type MovieResolution struct {
	Query   string
	Display string
}

// BuildMagnet 从 info_hash 构建磁力链接
func BuildMagnet(infoHash, name string) string {
	magnet := fmt.Sprintf("magnet:?xt=urn:btih:%s", infoHash)
	if name != "" {
		magnet += "&dn=" + name
	}
	trackers := []string{
		"udp://tracker.opentrackr.org:1337/announce",
		"udp://open.stealth.si:80/announce",
		"udp://tracker.torrent.eu.org:451/announce",
		"udp://tracker.bittor.pw:1337/announce",
		"udp://tracker.openbittorrent.com:6969/announce",
	}
	for _, tr := range trackers {
		magnet += "&tr=" + tr
	}
	return magnet
}
