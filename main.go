package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/huangke/bt-spider/config"
	"github.com/huangke/bt-spider/engine"
	"github.com/huangke/bt-spider/tui"
)

func main() {
	cfg, _ := config.LoadConfig("config.json")
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	// 命令行参数可覆盖下载目录
	for _, arg := range os.Args[1:] {
		cfg.DownloadDir = arg
		break
	}

	eng, err := engine.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 启动失败: %v\n", err)
		os.Exit(1)
	}
	defer eng.Close()

	p := tea.NewProgram(tui.New(eng), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ TUI 运行出错: %v\n", err)
		os.Exit(1)
	}
}
