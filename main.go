package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/huangke/bt-spider/app"
	"github.com/huangke/bt-spider/config"
	"github.com/huangke/bt-spider/engine"
	"github.com/huangke/bt-spider/pkg/logger"
	"github.com/huangke/bt-spider/search/pipeline"
	"github.com/huangke/bt-spider/tui"
)

func main() {
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 配置加载失败: %v\n", err)
		os.Exit(1)
	}

	// 命令行参数可覆盖下载目录
	for _, arg := range os.Args[1:] {
		cfg.DownloadDir = arg
		break
	}

	if err := logger.Init(cfg.LogDir, cfg.LogLevel); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  日志系统初始化失败（将继续运行但不写日志）: %v\n", err)
	}
	if err := pipeline.SetSearchAuditDBPath(cfg.SearchDBPath); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  搜索审计数据库初始化失败（将继续运行但不记录搜索明细）: %v\n", err)
	}
	logger.Info("bt-spider start", "mode", "tui", "download_dir", cfg.DownloadDir)

	eng, err := engine.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 启动失败: %v\n", err)
		os.Exit(1)
	}
	defer eng.Close()

	a := app.New(eng, nil)

	p := tea.NewProgram(tui.New(a), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ TUI 运行出错: %v\n", err)
		os.Exit(1)
	}
}
