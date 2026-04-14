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
	Seed              bool   `json:"seed"`
	EnableTrackerList bool   `json:"enable_tracker_list"`
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		DownloadDir:       filepath.Join(home, "Downloads", "BT-Spider"),
		MaxResults:        100,
		MaxConns:          80,
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

	return cfg, nil
}
