package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/huangke/bt-spider/config"
	"github.com/huangke/bt-spider/engine"
	"github.com/huangke/bt-spider/search"
)

const version = "0.5.0"

func main() {
	fmt.Printf("🕷  BT-Spider v%s\n", version)
	fmt.Println("命令: search <关键词>  搜索  |  <序号>  下载  |  magnet:...  直接下载  |  quit 退出")
	fmt.Println()

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
	var lastResults []search.Result

	for {
		fmt.Print("bt> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch {
		case strings.ToLower(input) == "quit" || strings.ToLower(input) == "exit" || strings.ToLower(input) == "q":
			fmt.Println("👋 再见!")
			return

		case strings.HasPrefix(strings.ToLower(input), "search "):
			keyword := strings.TrimSpace(input[7:])
			if keyword == "" {
				fmt.Println("⚠️  请输入搜索关键词")
				continue
			}
			fmt.Printf("🔍 搜索: %s\n", keyword)
			results, err := search.Search(keyword, search.DefaultProviders())
			if err != nil {
				fmt.Printf("❌ 搜索失败: %v\n", err)
				continue
			}
			if len(results) == 0 {
				fmt.Println("未找到有做种的结果")
				continue
			}
			lastResults = results
			limit := cfg.MaxResults
			if len(results) < limit {
				limit = len(results)
			}
			fmt.Printf("\n找到 %d 个结果（按做种数排序）:\n\n", len(results))
			for i, r := range results[:limit] {
				fmt.Printf("  [%d] %s\n      %s | Seeders: %d | Leechers: %d | %s\n",
					i+1, r.Name, r.Size, r.Seeders, r.Leechers, r.Source)
			}
			fmt.Println("\n输入序号下载:")

		case strings.HasPrefix(input, "magnet:"):
			if err := eng.AddMagnet(input); err != nil {
				fmt.Fprintf(os.Stderr, "❌ 下载失败: %v\n", err)
			}
			fmt.Println()

		default:
			num, err := strconv.Atoi(input)
			if err != nil {
				fmt.Println("⚠️  未知命令。输入 search <关键词> 搜索，输入序号下载，或粘贴 magnet 链接直接下载")
				continue
			}
			if num < 1 || num > len(lastResults) {
				fmt.Println("⚠️  序号超出范围")
				continue
			}
			r := lastResults[num-1]
			fmt.Printf("⬇️  下载: %s\n", r.Name)
			if err := eng.AddMagnet(r.Magnet); err != nil {
				fmt.Fprintf(os.Stderr, "❌ 下载失败: %v\n", err)
			}
			fmt.Println()
		}
	}
}
