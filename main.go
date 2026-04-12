package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/huangke/bt-spider/bot"
	"github.com/huangke/bt-spider/config"
	"github.com/huangke/bt-spider/engine"
	"github.com/huangke/bt-spider/search"
)

func main() {
	fmt.Println("🕷  BT-Spider v0.2.0")
	fmt.Println("输入 magnet 链接下载，search <关键词> 搜索，book <书名> 搜电子书，quit 退出")
	fmt.Println()

	// 检测 bot 模式：--bot 参数或 BT_TELEGRAM_BOT_TOKEN 环境变量
	botMode := false
	for _, arg := range os.Args[1:] {
		if arg == "--bot" || arg == "-bot" {
			botMode = true
		}
	}
	if os.Getenv("BT_TELEGRAM_BOT_TOKEN") != "" {
		botMode = true
	}

	// 先尝试从配置文件加载
	cfg, _ := config.LoadConfig("config.json")
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	// 命令行参数覆盖下载目录
	for _, arg := range os.Args[1:] {
		if arg != "--bot" && arg != "-bot" {
			cfg.DownloadDir = arg
			break
		}
	}

	eng, err := engine.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 启动失败: %v\n", err)
		os.Exit(1)
	}
	defer eng.Close()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Bot 模式
	if botMode || cfg.HasTelegram() {
		if !cfg.HasTelegram() {
			fmt.Fprintf(os.Stderr, "❌ Bot 模式需要配置 Telegram Bot Token（config.json 或 BT_TELEGRAM_BOT_TOKEN 环境变量）\n")
			os.Exit(1)
		}
		b, err := bot.New(cfg, eng)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Bot 启动失败: %v\n", err)
			os.Exit(1)
		}
		go b.Run()
		fmt.Println("🤖 Bot 模式已启动，按 Ctrl+C 退出")
		<-sigCh
		fmt.Println("\n👋 正在退出...")
		b.Stop()
		return
	}

	// 交互模式
	go func() {
		<-sigCh
		fmt.Println("\n👋 正在退出...")
		eng.Close()
		os.Exit(0)
	}()

	fmt.Printf("💾 下载目录: %s\n\n", cfg.DownloadDir)

	scanner := bufio.NewScanner(os.Stdin)
	var lastResults []search.Result
	var lastBooks []map[string]string

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

		case strings.HasPrefix(strings.ToLower(input), "book "):
			keyword := strings.TrimSpace(input[5:])
			if keyword == "" {
				fmt.Println("⚠️  请输入书名或作者")
				continue
			}
			fmt.Printf("📚 搜索电子书: %s\n", keyword)
			books, err := search.ZlibSearch(keyword)
			if err != nil {
				fmt.Printf("❌ 搜索失败: %v\n", err)
				continue
			}
			if len(books) == 0 {
				fmt.Println("未找到相关电子书")
				continue
			}
			fmt.Printf("\n找到 %d 个结果:\n\n", len(books))
			for i, b := range books {
				author := b["authors"]
				if author == "" {
					author = "未知作者"
				}
				fmt.Printf("  [%d] %s\n      %s | %s | %s | %s\n",
					i+1, b["title"], author, b["format"], b["size"], b["year"])
			}
			fmt.Println("\n输入序号下载（回车跳过）: ")
			lastBooks = books

		case strings.HasPrefix(input, "magnet:"):
			if err := eng.AddMagnet(input); err != nil {
				fmt.Fprintf(os.Stderr, "❌ 下载失败: %v\n", err)
			}
			fmt.Println()

		default:
			// 尝试解析为序号（search/book 后续选择）
			if num, err := strconv.Atoi(input); err == nil {
				if num >= 1 && num <= len(lastResults) {
					r := lastResults[num-1]
					fmt.Printf("⬇️  下载: %s\n", r.Name)
					if err := eng.AddMagnet(r.Magnet); err != nil {
						fmt.Fprintf(os.Stderr, "❌ 下载失败: %v\n", err)
					}
					fmt.Println()
				} else if num >= 1 && num <= len(lastBooks) {
					b := lastBooks[num-1]
					fmt.Printf("⬇️  下载电子书: %s\n", b["title"])
					if err := search.ZlibDownload(b["id"], cfg.DownloadDir); err != nil {
						fmt.Fprintf(os.Stderr, "❌ 下载失败: %v\n", err)
					}
					fmt.Println()
				} else {
					fmt.Println("⚠️  序号超出范围")
				}
			} else {
				fmt.Println("⚠️  未知命令。输入 search <关键词> 搜索，book <书名> 搜电子书，或粘贴 magnet 链接下载")
			}
		}
	}
}
