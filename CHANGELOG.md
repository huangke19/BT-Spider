# CHANGELOG

## 2026-04-17（性能优化）

### Added
- 全局共享 HTTP Transport（`pkg/httputil/transport.go`），所有 provider 复用连接池（HTTP/2 + keep-alive）
- 启动时异步预热常用 provider 域名的 TLS/DNS 连接（`httputil.Preheat`）
- 搜索专用低延迟客户端 `NewSearchClient`：0 重试、2 次失败触发熔断、5s 超时
- 24h 内存搜索缓存（LRU，最多 256 条）：`search/pipeline/cache.go`
- 流式搜索 API：`search/pipeline/SearchStream` + `app/SearchStream`，provider 返回即推送
- TUI 接入流式搜索：结果逐步刷新，不阻塞输入；支持按需补全单条资源大小（DHT）
- TMDB 查询结果缓存（7 天，含 miss 缓存，请求超时 800ms）
- 磁力链接内置 tracker 列表扩充至 17 条

### Changed
- 搜索不再同步调用 `ResolveSizes`，改为下载时按需单条补全
- 搜索路径移除非严格模式下的同步 `ScrapeSeeders` 阻塞
- 审计数据库写入改为异步队列（容量 1024），退出时 drain 保证落盘

### Removed
- 1337x 从 `DefaultProviders()` 移除（详情页 HTML 抓取带来 5–15s 延迟）

---

## 2026-04-17（架构重构）

### Added
- `app/app.go`：业务编排层，TUI / CLI 与 engine/search 的唯一依赖边界，暴露 `Search`、`AddMagnet`、`CancelDownload`、`WaitEvent` 等方法
- `search/` 拆分为三个子包：
  - `search/providers/`：6 个搜索源实现 + `DefaultProviders()` 注册表（默认启用 4 个）
  - `search/query/`：本地别名解析 → 严格格式解析 → TMDB → Groq 识别链
  - `search/pipeline/`：搜索编排、去重、关键词过滤、严格电影评分、UDP 做种数补全
- `pkg/httputil/resilient.go`：`ResilientClient`，带指数退避 + jitter 重试、per-host 熔断器
- `engine/event.go`：6 种离散事件类型（MetaReceived / DownloadDone / SeedingStarted / SeedingStopped / Failed / Canceled）；Engine 内置容量 64 的 buffered channel
- `engine/handle.go`：`TorrentHandle` 接口，`Download` 依赖接口而非具体 torrent 类型
- `engine/download_test.go`：9 个状态机单元测试（初始状态、Cancel 幂等、setFailed 事件断言、watchLifecycle 完整生命周期）
- 结构化日志（JSON）：所有核心流程、HTTP 请求、provider 调用、下载状态机均有 JSON 日志；写入 `~/Library/Logs/BT-Spider/bt-spider-YYYY-MM-DD.log`

---

## 2026-03-28

- `movie` 命令：本地别名库 + 严格格式解析，识别中英文片名后直接发起搜索
- 做种自动停止：支持按分享率（`seed_ratio_limit`）或保种时长（`seed_time_limit`）两种条件

---

## 2026-03-01

- 项目初始化：多源聚合搜索、TUI 交互模式、无头 CLI（`bt-download`）
