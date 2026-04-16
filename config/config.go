package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	DownloadDir       string  `json:"download_dir"`
	MaxResults        int     `json:"max_results"`
	MaxConns          int     `json:"max_conns"`
	ListenPort        int     `json:"listen_port"`
	Seed              bool    `json:"seed"`
	SeedRatioLimit    float64 `json:"seed_ratio_limit"`
	SeedTimeLimit     string  `json:"seed_time_limit"`
	EnableTrackerList bool    `json:"enable_tracker_list"`
	TMDBApiKey        string  `json:"tmdb_api_key"`
	GroqApiKey        string  `json:"groq_api_key"`
	LogDir            string  `json:"log_dir"`        // 空字符串 = ~/Library/Logs/BT-Spider/
	LogLevel          string  `json:"log_level"`      // debug / info / warn / error，默认 info
	SearchDBPath      string  `json:"search_db_path"` // 空字符串 = ~/Library/Application Support/BT-Spider/search_history.db
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		DownloadDir:       filepath.Join(home, "Downloads", "BT-Spider"),
		MaxResults:        100,
		MaxConns:          80,
		ListenPort:        0,
		Seed:              false,
		SeedRatioLimit:    1.0,
		SeedTimeLimit:     "30m",
		EnableTrackerList: true,
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	cfg.normalize()

	return cfg, nil
}

func (c *Config) normalize() {
	defaults := DefaultConfig()
	if c.DownloadDir == "" {
		c.DownloadDir = defaults.DownloadDir
	}
	if c.MaxResults <= 0 {
		c.MaxResults = defaults.MaxResults
	}
	if c.MaxConns <= 0 {
		c.MaxConns = defaults.MaxConns
	}
	if c.ListenPort < 0 || c.ListenPort > 65535 {
		c.ListenPort = defaults.ListenPort
	}
	if c.SeedRatioLimit < 0 {
		c.SeedRatioLimit = defaults.SeedRatioLimit
	}
	if strings.TrimSpace(c.SeedTimeLimit) == "" {
		c.SeedTimeLimit = defaults.SeedTimeLimit
	}
	if _, err := c.SeedTimeLimitDuration(); err != nil {
		c.SeedTimeLimit = defaults.SeedTimeLimit
	}
}

func (c *Config) SeedTimeLimitDuration() (time.Duration, error) {
	if strings.TrimSpace(c.SeedTimeLimit) == "" {
		return 0, nil
	}
	return time.ParseDuration(c.SeedTimeLimit)
}
