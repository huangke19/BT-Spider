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

func doSearch(keyword string) {
	fmt.Printf("🔍 搜索: %s\n", keyword)
	providers := []search.Provider{
		search.NewApiBay(),
		search.NewBtDig(),
		search.NewBT4G(),
		search.NewYTS(),
		search.NewEZTV(),
		search.NewNyaa(),
	}
	results, err := search.Search(keyword, providers)
	if err != nil {
		fmt.Printf("❌ 搜索失败: %v\n", err)
		return
	}
	if len(results) == 0 {
		fmt.Println("未找到有做种的结果")
		return
	}
	fmt.Printf("\n找到 %d 个结果（按做种数排序）:\n\n", len(results))
	limit := 20
	if len(results) < limit {
		limit = len(results)
	}
	for i, r := range results[:limit] {
		fmt.Printf("  [%d] %s\n      %s | Seeders: %d | Leechers: %d | %s\n",
			i+1, r.Name, r.Size, r.Seeders, r.Leechers, r.Source)
	}
	fmt.Println()
}

func main() {
	fmt.Println("🕷  BT-Spider v0.2.0")
	fmt.Println("输入 magnet 链接下载，search <关键词> 搜索，quit 退出")
	fmt.Println()

	cfg := config.DefaultConfig()

	if len(os.Args) > 1 {
		cfg.DownloadDir = os.Args[1]
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
			providers := []search.Provider{
				search.NewApiBay(),
				search.NewBtDig(),
				search.NewBT4G(),
				search.NewYTS(),
				search.NewEZTV(),
				search.NewNyaa(),
			}
			results, err := search.Search(keyword, providers)
			if err != nil {
				fmt.Printf("❌ 搜索失败: %v\n", err)
				continue
			}
			if len(results) == 0 {
				fmt.Println("未找到有做种的结果")
				continue
			}
			lastResults = results
			fmt.Printf("\n找到 %d 个结果（按做种数排序）:\n\n", len(results))
			limit := 20
			if len(results) < limit {
				limit = len(results)
			}
			for i, r := range results[:limit] {
				fmt.Printf("  [%d] %s\n      %s | Seeders: %d | Leechers: %d | %s\n",
					i+1, r.Name, r.Size, r.Seeders, r.Leechers, r.Source)
			}
			fmt.Println("\n输入序号下载（回车跳过）: ")

		case strings.HasPrefix(input, "magnet:"):
			if err := eng.AddMagnet(input); err != nil {
				fmt.Fprintf(os.Stderr, "❌ 下载失败: %v\n", err)
			}
			fmt.Println()

		default:
			// 尝试解析为序号（search 后续选择）
			if num, err := strconv.Atoi(input); err == nil && num >= 1 && num <= len(lastResults) {
				r := lastResults[num-1]
				fmt.Printf("⬇️  下载: %s\n", r.Name)
				if err := eng.AddMagnet(r.Magnet); err != nil {
					fmt.Fprintf(os.Stderr, "❌ 下载失败: %v\n", err)
				}
				fmt.Println()
			} else {
				fmt.Println("⚠️  未知命令。输入 search <关键词> 搜索，或粘贴 magnet 链接下载")
			}
		}
	}
}
