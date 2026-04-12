package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	DownloadDir      string  `json:"download_dir"`
	ListenAddr       string  `json:"listen_addr"`
	MaxConns         int     `json:"max_conns"`
	Seed              bool    `json:"seed"`
	EnableTrackerList bool    `json:"enable_tracker_list"`
	TelegramBotToken string  `json:"telegram_bot_token"`
	AllowedUserIDs   []int64 `json:"allowed_user_ids"`
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		DownloadDir: filepath.Join(home, "Downloads", "BT-Spider"),
		ListenAddr:  ":6881",
		MaxConns:    80,
		Seed:              false,
		EnableTrackerList: true,
	}
}

// LoadConfig 从文件加载配置，不存在则用默认值
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

	// 环境变量覆盖
	if token := os.Getenv("BT_TELEGRAM_BOT_TOKEN"); token != "" {
		cfg.TelegramBotToken = token
	}

	return cfg, nil
}

// HasTelegram 是否配置了 Telegram Bot
func (c *Config) HasTelegram() bool {
	return strings.TrimSpace(c.TelegramBotToken) != ""
}

// IsAllowedUser 检查用户是否有权限（空列表 = 不限制）
func (c *Config) IsAllowedUser(userID int64) bool {
	if len(c.AllowedUserIDs) == 0 {
		return true
	}
	for _, id := range c.AllowedUserIDs {
		if id == userID {
			return true
		}
	}
	return false
}
