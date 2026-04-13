package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/huangke/bt-spider/config"
	"github.com/huangke/bt-spider/search"
)

const version = "0.3.0"

func main() {
	fmt.Printf("🕷  BT-Spider v%s\n", version)
	fmt.Println("命令: search <关键词>  搜索  |  <序号>  复制磁力链接  |  quit 退出")
	fmt.Println()

	cfg, _ := config.LoadConfig("config.json")
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

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
			fmt.Println()

		default:
			num, err := strconv.Atoi(input)
			if err != nil {
				fmt.Println("⚠️  未知命令。输入 search <关键词> 搜索，或输入序号获取磁力链接")
				continue
			}
			if num < 1 || num > len(lastResults) {
				fmt.Println("⚠️  序号超出范围")
				continue
			}
			r := lastResults[num-1]
			fmt.Printf("🔗 %s\n%s\n\n", r.Name, r.Magnet)
		}
	}
}
