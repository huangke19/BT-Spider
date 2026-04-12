package engine

import (
	"fmt"
	"os"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/huangke/bt-spider/config"
	"github.com/huangke/bt-spider/pkg/utils"
)

type Engine struct {
	client   *torrent.Client
	cfg      *config.Config
	trackers *TrackerList
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

	eng := &Engine{
		client: client,
		cfg:    cfg,
	}

	// 启动 tracker 列表自动刷新
	if cfg.EnableTrackerList {
		tl := NewTrackerList()
		// 等待初始拉取完成
		time.Sleep(2 * time.Second)
		eng.trackers = tl
	}

	return eng, nil
}

// AddMagnet 添加磁力链接并开始下载
func (e *Engine) AddMagnet(magnet string) error {
	t, err := e.client.AddMagnet(magnet)
	if err != nil {
		return fmt.Errorf("解析磁力链接失败: %w", err)
	}

	// 注入最新 tracker 列表
	if e.trackers != nil {
		t.AddTrackers(e.trackers.Get())
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
	fmt.Printf("📁 大小: %s\n", utils.FormatBytes(info.TotalLength()))
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
			eta = utils.FormatDuration(time.Duration(secs) * time.Second)
		}

		percent := float64(completed) / float64(total) * 100
		bar := utils.ProgressBar(percent, 30)

		// 清行并输出：进度条 百分比 | 已下载/总大小 | 速度 | ETA | peers
		fmt.Printf("\r\033[K%s %.1f%% | %s/%s | %s/s | ETA %s | ↓ %d peers",
			bar, percent,
			utils.FormatBytes(completed), utils.FormatBytes(total),
			utils.FormatBytes(int64(avgSpeed)),
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

func (e *Engine) Close() {
	if e.trackers != nil {
		e.trackers.Stop()
	}
	if e.client != nil {
		e.client.Close()
	}
}
