package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/huangke/bt-spider/config"
	"github.com/huangke/bt-spider/engine"
	"github.com/huangke/bt-spider/web"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "Web UI 监听地址")
	dir := flag.String("dir", "", "下载目录（默认使用 config.json 或 ~/Downloads/BT-Spider）")
	flag.Parse()

	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 配置加载失败: %v\n", err)
		os.Exit(1)
	}
	if *dir != "" {
		cfg.DownloadDir = *dir
	}

	eng, err := engine.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 启动失败: %v\n", err)
		os.Exit(1)
	}
	defer eng.Close()

	srv := &http.Server{
		Addr:    *addr,
		Handler: web.New(eng).Handler(),
	}

	log.Printf("BT-Spider Web UI running at http://%s", *addr)
	log.Printf("Download directory: %s", cfg.DownloadDir)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "❌ Web UI 运行出错: %v\n", err)
		os.Exit(1)
	}
}
