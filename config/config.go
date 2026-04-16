package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DownloadDir       string `json:"download_dir"`
	MaxResults        int    `json:"max_results"`
	MaxConns          int    `json:"max_conns"`
	ListenPort        int    `json:"listen_port"`
	Seed              bool   `json:"seed"`
	EnableTrackerList bool   `json:"enable_tracker_list"`
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		DownloadDir:       filepath.Join(home, "Downloads", "BT-Spider"),
		MaxResults:        100,
		MaxConns:          80,
		ListenPort:        0,
		Seed:              false,
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
}
