package engine

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const trackerListURL = "https://trackerslist.com/best.txt"

var fallbackTrackers = []string{
	"udp://tracker.opentrackr.org:1337/announce",
	"udp://open.tracker.cl:1337/announce",
	"udp://tracker.openbittorrent.com:6969/announce",
	"udp://opentracker.i2p.rocks:6969/announce",
}

// TrackerList 管理公开 tracker 列表，定时刷新
type TrackerList struct {
	mu       sync.RWMutex
	trackers []string
	stopCh   chan struct{}
}

// NewTrackerList 创建并异步拉取 tracker 列表，每 24 小时刷新
func NewTrackerList() *TrackerList {
	tl := &TrackerList{
		trackers: fallbackTrackers,
		stopCh:   make(chan struct{}),
	}

	// 异步拉取，不阻塞启动
	go tl.run()

	return tl
}

func (tl *TrackerList) run() {
	// 启动时立即拉取一次
	tl.refresh()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tl.refresh()
		case <-tl.stopCh:
			return
		}
	}
}

func (tl *TrackerList) refresh() {
	trackers, err := fetchTrackers()
	if err != nil {
		log.Printf("⚠️  拉取 tracker 列表失败: %v，使用 fallback 列表", err)
		tl.mu.Lock()
		tl.trackers = fallbackTrackers
		tl.mu.Unlock()
		return
	}

	tl.mu.Lock()
	tl.trackers = trackers
	tl.mu.Unlock()
	log.Printf("✅ 已更新 tracker 列表（%d 条）", len(trackers))
}

// Get 返回当前 tracker 列表（格式：[][]string，每个 tracker 一个 tier）
func (tl *TrackerList) Get() [][]string {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	result := make([][]string, len(tl.trackers))
	for i, tr := range tl.trackers {
		result[i] = []string{tr}
	}
	return result
}

// Stop 停止定时刷新
func (tl *TrackerList) Stop() {
	close(tl.stopCh)
}

func fetchTrackers() ([]string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(trackerListURL)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取失败: %w", err)
	}

	var trackers []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			trackers = append(trackers, line)
		}
	}

	if len(trackers) == 0 {
		return nil, fmt.Errorf("返回列表为空")
	}

	return trackers, nil
}
