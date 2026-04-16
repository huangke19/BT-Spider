package providers

import "github.com/huangke/bt-spider/search"

// DefaultProviders 返回所有内置的 torrent 搜索源
func DefaultProviders() []search.Provider {
	return []search.Provider{
		NewApiBay(),
		NewBT4G(),
		NewLeet337x(),
		NewTorrentKitty(),
		NewYTS(),
	}
}
