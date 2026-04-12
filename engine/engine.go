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

// showProgress 实时显示下载进度（含速度和 ETA）
func (e *Engine) showProgress(t *torrent.Torrent) {
	var prevCompleted int64
	prevTime := time.Now()
	// 滑动窗口计算平均速度（最近 10 秒）
	type sample struct {
		bytes int64
		time  time.Time
	}
	samples := make([]sample, 0, 10)

	for {
		stats := t.Stats()
		total := t.Info().TotalLength()
		completed := t.BytesCompleted()

		if total == 0 {
			time.Sleep(time.Second)
			continue
		}

		now := time.Now()
		elapsed := now.Sub(prevTime).Seconds()

		// 计算瞬时速度
		var speed float64
		if elapsed > 0 {
			speed = float64(completed-prevCompleted) / elapsed
		}

		// 更新滑动窗口
		samples = append(samples, sample{bytes: completed, time: now})
		if len(samples) > 10 {
			samples = samples[1:]
		}

		// 用滑动窗口计算平均速度（更平滑）
		var avgSpeed float64
		if len(samples) >= 2 {
			first := samples[0]
			last := samples[len(samples)-1]
			dt := last.time.Sub(first.time).Seconds()
			if dt > 0 {
				avgSpeed = float64(last.bytes-first.bytes) / dt
			}
		}
		if avgSpeed <= 0 {
			avgSpeed = speed
		}

		prevCompleted = completed
		prevTime = now

		// 计算 ETA
		remaining := total - completed
		eta := "计算中..."
		if avgSpeed > 0 && remaining > 0 {
			secs := float64(remaining) / avgSpeed
			eta = formatDuration(time.Duration(secs) * time.Second)
		}

		percent := float64(completed) / float64(total) * 100
		bar := progressBar(percent, 30)

		// 清行并输出：进度条 百分比 | 已下载/总大小 | 速度 | ETA | peers
		fmt.Printf("\r\033[K%s %.1f%% | %s/%s | %s/s | ETA %s | ↓ %d peers",
			bar, percent,
			formatBytes(completed), formatBytes(total),
			formatBytes(int64(avgSpeed)),
			eta,
			stats.ActivePeers,
		)

		if completed >= total {
			fmt.Printf("\n\n✅ 下载完成！\n")
			return
		}

		time.Sleep(time.Second)
	}
}

// formatDuration 格式化剩余时间
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
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
