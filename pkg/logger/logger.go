// Package logger 提供 BT-Spider 的全局结构化日志。
//
// 日志以 JSON 格式写入按天切分的文件（如 bt-spider-2026-04-16.log），
// 方便用 jq / grep 做事后排查。全局 logger 在 Init 之前会输出到
// io.Discard，避免未初始化时污染终端。
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	global     = slog.New(slog.NewJSONHandler(io.Discard, nil))
	currentLog string // 当前日志文件的绝对路径，供 UI/提示用
)

// Init 按指定目录和级别初始化全局 logger。
// logDir 为空时回落到 ~/Library/Logs/BT-Spider/。
// level 接受 debug / info / warn / error，不识别时默认 info。
func Init(logDir, level string) error {
	if logDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("获取 HOME 失败: %w", err)
		}
		logDir = filepath.Join(home, "Library", "Logs", "BT-Spider")
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("创建日志目录 %s 失败: %w", logDir, err)
	}

	filename := filepath.Join(logDir, fmt.Sprintf("bt-spider-%s.log", time.Now().Format("2006-01-02")))
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: parseLevel(level)})
	global = slog.New(handler)
	currentLog = filename

	global.Info("logger initialized",
		"path", filename,
		"level", strings.ToLower(level),
	)
	return nil
}

// Path 返回当前日志文件路径（Init 之前为空字符串）。
func Path() string { return currentLog }

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// 薄封装，和 slog 保持一致的 key-value 调用方式。
// 用法：logger.Info("search done", "keyword", kw, "count", n)

func Debug(msg string, args ...any) { global.Debug(msg, args...) }
func Info(msg string, args ...any)  { global.Info(msg, args...) }
func Warn(msg string, args ...any)  { global.Warn(msg, args...) }
func Error(msg string, args ...any) { global.Error(msg, args...) }

// With 返回一个绑定了固定字段的子 logger，便于在同一业务流中串联日志。
func With(args ...any) *slog.Logger { return global.With(args...) }
