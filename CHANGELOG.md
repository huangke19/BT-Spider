# CHANGELOG


## 2026-04-16

- 日志系统全面升级：所有核心流程、错误、HTTP 请求、provider 搜索、下载状态机均有结构化日志，便于排查问题。
- engine/trackers.go、cmd/web/main.go 等历史遗留 log.Printf 均已迁移为结构化日志
- 1337x/TorrentKitty hash 支持 base32/hex，兼容更多磁力链
- EZTV 已移除（API 不支持关键词搜索，实际无效）
- Web API 日志支持 HTTP 状态码、耗时、路径、方法，分 info/warn/error 级别。
- DHT 补全大小、provider 并发搜索、下载失败/超时等场景均有详细日志。
- 兼容原有 config.json 配置，日志默认写入 ~/Library/Logs/BT-Spider/bt-spider-YYYY-MM-DD.log。
- 代码结构优化，便于后续扩展。

## 2026-04-10

- 支持 Web UI，浏览器可直接管理下载任务。
- 支持多 tracker 自动刷新，提升连接率。

## 2026-03-28

- 支持严格电影搜索（movie 命令/格式），自动识别中英文别名、年份、分辨率。
- 支持做种率/保种时长自动停止。

## 2026-03-01

- 项目初始化，支持多源聚合搜索、TUI/Headless 两种模式。
