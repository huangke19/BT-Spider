package engine

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/huangke/bt-spider/pkg/logger"
)

const trackerListURL = "https://trackerslist.com/best.txt"

var fallbackTrackers = []string{
	"udp://tracker.opentrackr.org:1337/announce",
	"udp://open.tracker.cl:1337/announce",
	"udp://tracker.openbittorrent.com:6969/announce",
	"udp://opentracker.i2p.rocks:6969/announce",
}

type TrackerList struct {
	mu       sync.RWMutex
	trackers []string
	stopCh   chan struct{}
}

func NewTrackerList() *TrackerList {
	tl := &TrackerList{
		trackers: fallbackTrackers,
		stopCh:   make(chan struct{}),
	}
	go tl.run()
	return tl
}

func (tl *TrackerList) run() {
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
		logger.Warn("tracker refresh failed", "err", err)
		tl.mu.Lock()
		tl.trackers = fallbackTrackers
		tl.mu.Unlock()
		return
	}
	tl.mu.Lock()
	tl.trackers = trackers
	tl.mu.Unlock()
	logger.Info("tracker list updated", "count", len(trackers))
}

func (tl *TrackerList) Get() [][]string {
	tl.mu.RLock()
	defer tl.mu.RUnlock()
	result := make([][]string, len(tl.trackers))
	for i, tr := range tl.trackers {
		result[i] = []string{tr}
	}
	return result
}

func (tl *TrackerList) Stop() {
	close(tl.stopCh)
}

func fetchTrackers() ([]string, error) {
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
	}
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
