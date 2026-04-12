package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/huangke/bt-spider/config"
	"github.com/huangke/bt-spider/engine"
)

func main() {
	fmt.Println("🕷  BT-Spider v0.1.0")
	fmt.Println("粘贴磁力链接开始下载，输入 quit 退出")
	fmt.Println()

	cfg := config.DefaultConfig()

	// 支持命令行参数指定下载目录
	if len(os.Args) > 1 {
		cfg.DownloadDir = os.Args[1]
	}

	eng, err := engine.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 启动失败: %v\n", err)
		os.Exit(1)
	}
	defer eng.Close()

	// 优雅退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n👋 正在退出...")
		eng.Close()
		os.Exit(0)
	}()

	fmt.Printf("💾 下载目录: %s\n\n", cfg.DownloadDir)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("magnet> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch strings.ToLower(input) {
		case "quit", "exit", "q":
			fmt.Println("👋 再见!")
			return
		default:
			if !strings.HasPrefix(input, "magnet:") {
				fmt.Println("⚠️  请输入有效的磁力链接 (magnet:?xt=...)")
				continue
			}

			if err := eng.AddMagnet(input); err != nil {
				fmt.Fprintf(os.Stderr, "❌ 下载失败: %v\n", err)
			}
			fmt.Println()
		}
	}
}
