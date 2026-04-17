#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

MODE="${1:-tui}"
if [[ $# -gt 0 ]]; then
  shift
fi

usage() {
  cat <<EOF
用法:
  $0 [tui] [TUI 参数...]
  $0 download [bt-download 参数...]

说明:
  - 监听 Go 源码变化
  - 变更后自动重新编译并重启进程
  - 依赖 air: go install github.com/air-verse/air@latest
EOF
}

resolve_air() {
  if command -v air >/dev/null 2>&1; then
    command -v air
    return 0
  fi

  local gobin
  gobin="$(go env GOBIN 2>/dev/null || true)"
  if [[ -n "$gobin" && -x "$gobin/air" ]]; then
    echo "$gobin/air"
    return 0
  fi

  local gopath
  gopath="$(go env GOPATH 2>/dev/null || true)"
  if [[ -n "$gopath" && -x "$gopath/bin/air" ]]; then
    echo "$gopath/bin/air"
    return 0
  fi

  return 1
}

ensure_air() {
  AIR_BIN="$(resolve_air || true)"
  if [[ -n "${AIR_BIN:-}" ]]; then
    return 0
  fi

  echo "[dev] 未检测到 air，请先安装:"
  echo "      go install github.com/air-verse/air@latest"
  exit 1
}

case "$MODE" in
  -h|--help|help)
    usage
    exit 0
    ;;
esac

ensure_air
mkdir -p "$ROOT_DIR/tmp"

case "$MODE" in
  tui)
    exec "$AIR_BIN" --build.cmd "go build -o ./tmp/bt-spider-dev ." --build.entrypoint "./tmp/bt-spider-dev" -- "$@"
    ;;
  download)
    exec "$AIR_BIN" --build.cmd "go build -o ./tmp/bt-download-dev ./cmd/download" --build.entrypoint "./tmp/bt-download-dev" -- "$@"
    ;;
  *)
    echo "[dev] 未知模式: $MODE" >&2
    usage >&2
    exit 1
    ;;
esac
