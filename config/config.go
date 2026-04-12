package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	DownloadDir string
	ListenAddr  string
	MaxConns    int
	Seed        bool
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		DownloadDir: filepath.Join(home, "Downloads", "BT-Spider"),
		ListenAddr:  ":6881",
		MaxConns:    80,
		Seed:        false,
	}
}
