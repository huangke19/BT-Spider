package engine

import (
	"fmt"
	"os"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/huangke/bt-spider/config"
)

type Engine struct {
	client *torrent.Client
	cfg    *config.Config
}

func New(cfg *config.Config) (*Engine, error) {
	// 确保下载目录存在
	if err := os.MkdirAll(cfg.DownloadDir, 0755); err != nil {
		return nil, fmt.Errorf("创建下载目录失败: %w", err)
	}

	clientCfg := torrent.NewDefaultClientConfig()
	clientCfg.DataDir = cfg.DownloadDir
	clientCfg.ListenPort = 6881
	clientCfg.Seed = cfg.Seed
	clientCfg.EstablishedConnsPerTorrent = cfg.MaxConns

	client, err := torrent.NewClient(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("创建BT客户端失败: %w", err)
	}

	return &Engine{
		client: client,
		cfg:    cfg,
	}, nil
}

// AddMagnet 添加磁力链接并开始下载
func (e *Engine) AddMagnet(magnet string) error {
	t, err := e.client.AddMagnet(magnet)
	if err != nil {
		return fmt.Errorf("解析磁力链接失败: %w", err)
	}

	fmt.Println("⏳ 正在获取元数据...")

	// 等待元数据，带超时
	select {
	case <-t.GotInfo():
	case <-time.After(2 * time.Minute):
		t.Drop()
		return fmt.Errorf("获取元数据超时（2分钟）")
	}

	info := t.Info()
	fmt.Printf("📦 名称: %s\n", info.BestName())
	fmt.Printf("📁 大小: %s\n", formatBytes(info.TotalLength()))
	fmt.Printf("📂 文件数: %d\n", len(info.UpvertedFiles()))
	fmt.Printf("💾 保存到: %s\n", e.cfg.DownloadDir)
	fmt.Println()

	// 开始下载所有文件
	t.DownloadAll()

	// 显示进度
	e.showProgress(t)

	return nil
}

// showProgress 实时显示下载进度
func (e *Engine) showProgress(t *torrent.Torrent) {
	for {
		stats := t.Stats()
		total := t.Info().TotalLength()
		completed := t.BytesCompleted()

		if total == 0 {
			time.Sleep(time.Second)
			continue
		}

		percent := float64(completed) / float64(total) * 100
		bar := progressBar(percent, 30)

		fmt.Printf("\r%s %.1f%% | %s/%s | ↓ %d peers",
			bar, percent,
			formatBytes(completed), formatBytes(total),
			stats.ActivePeers,
		)

		if completed >= total {
			fmt.Printf("\n\n✅ 下载完成！\n")
			return
		}

		time.Sleep(time.Second)
	}
}

func (e *Engine) Close() {
	if e.client != nil {
		e.client.Close()
	}
}

// progressBar 生成进度条
func progressBar(percent float64, width int) string {
	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}
	bar := make([]rune, width)
	for i := range bar {
		if i < filled {
			bar[i] = '█'
		} else {
			bar[i] = '░'
		}
	}
	return string(bar)
}

// formatBytes 格式化字节大小
func formatBytes(b int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
