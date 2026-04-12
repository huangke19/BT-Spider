package search

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ZlibBin returns the path to the zlib executable (prefers ~/bin/zlib, falls back to PATH).
func ZlibBin() string {
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

// zlibSession stores the zlib CLI session (cookies + domain).
type zlibSession struct {
	Cookies map[string]string `json:"cookies"`
	Domain  string            `json:"domain"`
}

// LoadZlibSession reads the zlib CLI saved session.
func LoadZlibSession() (*zlibSession, error) {
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

// ZlibSearch searches for ebooks via the zlib CLI and parses the table output.
func ZlibSearch(keyword string) ([]map[string]string, error) {
	// 确认 session 存在
	if _, err := LoadZlibSession(); err != nil {
		return nil, err
	}
	cmd := exec.Command(ZlibBin(), "search", keyword, "-n", "15")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zlib search 失败: %w", err)
	}
	return ParseZlibTable(string(out)), nil
}

// ParseZlibTable parses the box-drawing table output from zlib search.
// Format: │ # │ ID │ Title │ Authors │ Year │ Format │ Size │
func ParseZlibTable(output string) []map[string]string {
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

// ZlibDownload calls zlib download <id> via script to provide a pseudo TTY.
func ZlibDownload(id, destDir string) error {
	if destDir == "" {
		home, _ := os.UserHomeDir()
		destDir = home + "/Documents/Books"
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	cmd := exec.Command("script", "-q", "/dev/null",
		ZlibBin(), "download", id, "-d", destDir)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
