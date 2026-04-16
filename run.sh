#!/bin/bash
# 运行 BT-Spider 各模式
set -e

MODE=${1:-tui}

case "$MODE" in
  tui)
    echo "[run] 启动 TUI 主程序..."
    ./bt-spider
    ;;
  download)
    echo "[run] 启动无头下载器..."
    shift
    ./bt-download "$@"
    ;;
  *)
    echo "用法: $0 [tui|download] [参数...]"
    exit 1
    ;;
esac
