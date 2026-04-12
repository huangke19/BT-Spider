package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	fmt.Println("输入 magnet 链接下载，search <关键词> 搜索，book <书名> 搜电子书，quit 退出")
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

		case strings.HasPrefix(strings.ToLower(input), "book "):
			keyword := strings.TrimSpace(input[5:])
			if keyword == "" {
				fmt.Println("⚠️  请输入书名或作者")
				continue
			}
			fmt.Printf("📚 搜索电子书: %s\n", keyword)
			books, err := zlibSearch(keyword)
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
			// 尝试解析为序号（search 后续选择）
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
					if err := zlibDownload(b["id"], cfg.DownloadDir); err != nil {
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

// zlibBin 返回 zlib 可执行文件路径（优先 ~/bin/zlib，其次 PATH）
func zlibBin() string {
	home, _ := os.UserHomeDir()
	local := home + "/bin/zlib"
	if _, err := os.Stat(local); err == nil {
		return local
	}
	if p, err := exec.LookPath("zlib"); err == nil {
		return p
	}
	return "zlib"
}

// zlibSession 读取 zlib CLI 保存的 session（cookies + domain）
type zlibSession struct {
	Cookies map[string]string `json:"cookies"`
	Domain  string            `json:"domain"`
}

func loadZlibSession() (*zlibSession, error) {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(home + "/.config/zlib/session.json")
	if err != nil {
		return nil, fmt.Errorf("未找到 zlib session，请先运行: ~/bin/zlib login")
	}
	var s zlibSession
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// zlibSearch 通过 zlib CLI 搜索电子书，解析表格输出
func zlibSearch(keyword string) ([]map[string]string, error) {
	// 确认 session 存在
	if _, err := loadZlibSession(); err != nil {
		return nil, err
	}
	cmd := exec.Command(zlibBin(), "search", keyword, "-n", "15")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zlib search 失败: %w", err)
	}
	return parseZlibTable(string(out)), nil
}

// parseZlibTable 解析 zlib search 输出的 box-drawing 表格
// 格式: │ # │ ID │ Title │ Authors │ Year │ Format │ Size │
func parseZlibTable(output string) []map[string]string {
	var books []map[string]string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "│") {
			continue
		}
		// 去掉首尾 │，按 │ 分割
		line = strings.Trim(line, "│")
		cols := strings.Split(line, "│")
		if len(cols) < 7 {
			continue
		}
		num := strings.TrimSpace(cols[0])
		// 跳过表头
		if num == "#" || num == "" {
			continue
		}
		// 验证第一列是数字
		if _, err := strconv.Atoi(num); err != nil {
			continue
		}
		id := strings.TrimSpace(cols[1])
		title := strings.TrimSpace(cols[2])
		author := strings.TrimSpace(cols[3])
		year := strings.TrimSpace(cols[4])
		format := strings.TrimSpace(cols[5])
		size := strings.TrimSpace(cols[6])
		if id == "" || title == "" {
			continue
		}
		books = append(books, map[string]string{
			"id":      id,
			"title":   title,
			"authors": author,
			"format":  format,
			"year":    year,
			"size":    size,
		})
	}
	return books
}

// zlibDownload 调用 zlib download <id>，通过 script 提供伪 TTY
func zlibDownload(id, destDir string) error {
	if destDir == "" {
		home, _ := os.UserHomeDir()
		destDir = home + "/Documents/Books"
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	cmd := exec.Command("script", "-q", "/dev/null",
		zlibBin(), "download", id, "-d", destDir)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
