#!/bin/bash
# 编译所有可执行文件
set -e

echo "[build] 编译主程序 (TUI) ..."
go build -o bt-spider .

echo "[build] 编译无头下载器 ..."
go build -o bt-download ./cmd/download

echo "[build] 全部完成！"
