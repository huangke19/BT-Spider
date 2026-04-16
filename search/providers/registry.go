package providers

import "github.com/huangke/bt-spider/search"

// DefaultProviders 返回所有内置的 torrent 搜索源
func DefaultProviders() []search.Provider {
	return []search.Provider{
		NewApiBay(),
		NewBtDig(),
		NewBT4G(),
		NewYTS(),
		NewNyaa(),
		NewLeet337x(),
		NewTorrentKitty(),
	}
}
