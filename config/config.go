package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	MaxResults int `json:"max_results"` // 每次搜索最多显示条数，默认 20
}

func DefaultConfig() *Config {
	return &Config{
		MaxResults: 20,
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
