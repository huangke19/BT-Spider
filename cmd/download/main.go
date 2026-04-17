// bt-download 是 BT-Spider 的无头（headless）命令行下载器。
//
// 适用场景：
//   - 脚本 / CI / AI 助手（hermes、Claude Code 等）通过子进程调用
//   - 不需要交互终端（TTY）
//   - 输出按行流式打印，可用 `--json` 切换为结构化 JSON
//
// 用法:
//
//	bt-download [选项] <magnet 链接>          # 直接下载磁力链接
//	bt-download [选项] <搜索关键词...>         # 搜索并下载（默认选做种数最高的）
//
// 示例:
//
//	bt-download 'magnet:?xt=urn:btih:...'
//	bt-download --pick 2 "The Bourne Supremacy 2004"
//	bt-download --json --dir /tmp/dl "Ubuntu 24.04"
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/huangke/bt-spider/config"
	"github.com/huangke/bt-spider/engine"
	"github.com/huangke/bt-spider/pkg/httputil"
	"github.com/huangke/bt-spider/pkg/logger"
	"github.com/huangke/bt-spider/pkg/utils"
	"github.com/huangke/bt-spider/search/pipeline"
	"github.com/huangke/bt-spider/search/providers"
)

var (
	flagDir      = flag.String("dir", "", "下载目录（默认使用 config.json 或 ~/Downloads/BT-Spider）")
	flagPick     = flag.Int("pick", 1, "搜索模式下选第几个结果（按做种数排序）")
	flagJSON     = flag.Bool("json", false, "以 JSON 每行输出（便于脚本/AI 解析）")
	flagInterval = flag.Duration("interval", 2*time.Second, "进度输出间隔")
	flagTopN     = flag.Int("show", 5, "搜索模式下预览前 N 条候选")
)

func usage() {
	fmt.Fprintf(os.Stderr, `bt-download — BT-Spider 无头下载器

用法:
  %s [选项] <magnet 链接>
  %s [选项] <搜索关键词...>

选项:
`, os.Args[0], os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() == 0 {
		usage()
		os.Exit(2)
	}
	arg := strings.Join(flag.Args(), " ")

	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		emit("error", map[string]any{"msg": "配置加载失败: " + err.Error()})
		os.Exit(1)
	}
	if *flagDir != "" {
		cfg.DownloadDir = *flagDir
	}

	if err := logger.Init(cfg.LogDir, cfg.LogLevel); err != nil {
		emit("warn", map[string]any{"msg": "日志系统初始化失败: " + err.Error()})
	}
	if err := pipeline.SetSearchAuditDBPath(cfg.SearchDBPath); err != nil {
		emit("warn", map[string]any{"msg": "搜索审计数据库初始化失败: " + err.Error()})
	}
	logger.Info("bt-download start", "mode", "headless", "arg", arg, "download_dir", cfg.DownloadDir)

	// 启动时异步预热常用 provider 的 TLS/DNS 连接（决策 D5）
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		httputil.Preheat(ctx, httputil.DefaultPreheatHosts())
	}()

	eng, err := engine.New(cfg)
	if err != nil {
		emit("error", map[string]any{"msg": err.Error()})
		os.Exit(1)
	}
	defer eng.Close()

	// Ctrl+C 时干净退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		emit("canceled", map[string]any{"msg": "用户中断"})
		eng.Close()
		os.Exit(130)
	}()

	magnet, name, err := resolveMagnet(arg)
	if err != nil {
		emit("error", map[string]any{"msg": err.Error()})
		os.Exit(1)
	}
	emit("picked", map[string]any{"name": name, "dir": cfg.DownloadDir})

	dl, err := eng.AddMagnetAsync(magnet)
	if err != nil {
		emit("error", map[string]any{"msg": err.Error()})
		os.Exit(1)
	}

	for {
		snap := dl.Snapshot()
		switch snap.State {
		case engine.StateWaitingMeta:
			emit("meta", map[string]any{"peers": snap.Peers})
		case engine.StateDownloading:
			percent := 0.0
			if snap.Total > 0 {
				percent = float64(snap.Completed) / float64(snap.Total) * 100
			}
			emit("progress", map[string]any{
				"percent":   fmt.Sprintf("%.1f", percent),
				"completed": utils.FormatBytes(snap.Completed),
				"total":     utils.FormatBytes(snap.Total),
				"speed":     utils.FormatBytes(int64(snap.Speed)) + "/s",
				"peers":     snap.Peers,
				"eta":       etaStr(snap.ETA),
			})
		case engine.StateSeeding:
			emit("seeding", map[string]any{
				"uploaded":    utils.FormatBytes(snap.Uploaded),
				"share_ratio": fmt.Sprintf("%.2f", snap.ShareRatio),
				"speed":       utils.FormatBytes(int64(snap.Speed)) + "/s",
				"peers":       snap.Peers,
				"elapsed":     etaStr(snap.SeedElapsed),
			})
		case engine.StateDone:
			emit("done", map[string]any{
				"name": snap.Name,
				"dir":  cfg.DownloadDir,
			})
			return
		case engine.StateFailed:
			emit("error", map[string]any{"msg": snap.Err})
			os.Exit(1)
		case engine.StateCanceled:
			emit("canceled", map[string]any{"msg": "任务被取消"})
			os.Exit(130)
		}
		time.Sleep(*flagInterval)
	}
}

// resolveMagnet 把输入解析成 magnet 链接。
// 如果是 magnet 开头，直接返回；否则执行搜索并按 --pick 选择。
func resolveMagnet(arg string) (magnet, name string, err error) {
	if strings.HasPrefix(arg, "magnet:") {
		return arg, "(magnet link)", nil
	}

	emit("search", map[string]any{"keyword": arg})
	results, err := pipeline.Search(arg, providers.DefaultProviders())
	if err != nil {
		return "", "", fmt.Errorf("搜索失败: %w", err)
	}
	if len(results) == 0 {
		return "", "", fmt.Errorf("关键词 %q 没有找到有做种的结果", arg)
	}
	if *flagPick < 1 || *flagPick > len(results) {
		return "", "", fmt.Errorf("--pick=%d 超出结果范围 [1,%d]", *flagPick, len(results))
	}

	top := *flagTopN
	if top > len(results) {
		top = len(results)
	}
	for i, r := range results[:top] {
		emit("result", map[string]any{
			"index":    i + 1,
			"name":     r.Name,
			"size":     r.Size,
			"seeders":  r.Seeders,
			"leechers": r.Leechers,
			"source":   r.Source,
		})
	}

	chosen := results[*flagPick-1]
	return chosen.Magnet, chosen.Name, nil
}

// emit 按一行一条输出事件：JSON 模式输出结构化数据，文本模式输出人类可读行。
// 两种模式都是行缓冲、立即刷新，便于被调用方流式读取。
func emit(event string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["event"] = event
	data["ts"] = time.Now().Format(time.RFC3339)

	if *flagJSON {
		b, _ := json.Marshal(data)
		fmt.Println(string(b))
		return
	}

	switch event {
	case "search":
		fmt.Printf("[search] keyword=%q\n", data["keyword"])
	case "result":
		fmt.Printf("[result] [%d] %s | %s | S:%d L:%d | %s\n",
			data["index"], data["name"], data["size"],
			data["seeders"], data["leechers"], data["source"])
	case "picked":
		fmt.Printf("[picked] name=%q dir=%s\n", data["name"], data["dir"])
	case "meta":
		fmt.Printf("[meta] 等待元数据...  peers=%d\n", data["peers"])
	case "progress":
		fmt.Printf("[progress] %s%%  %s/%s  ↓ %s  peers=%d  ETA %s\n",
			data["percent"], data["completed"], data["total"],
			data["speed"], data["peers"], data["eta"])
	case "seeding":
		fmt.Printf("[seeding] ↑ %s  ratio=%s  speed=%s  peers=%d  elapsed %s\n",
			data["uploaded"], data["share_ratio"], data["speed"], data["peers"], data["elapsed"])
	case "done":
		fmt.Printf("[done] ✅ %s -> %s\n", data["name"], data["dir"])
	case "error":
		fmt.Printf("[error] ❌ %s\n", data["msg"])
	case "canceled":
		fmt.Printf("[canceled] %s\n", data["msg"])
	}
}

func etaStr(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	return utils.FormatDuration(d)
}
