# CHANGELOG


## 2026-04-17（搜索源清理）

- **移除无用搜索源**：基于 `search_history.db` 审计库的实测数据分析：
  - **BTDigg**：23/23 次全部失败（HTTP 429 限速 + 熔断器），删除 `btdig.go`
  - **Nyaa**：26/26 次成功但返回 0 条结果（动漫源，电影搜索无用），删除 `nyaa.go`
  - **YTS**：删除后恢复，留待后续验证
  - 从 `DefaultProviders()` 注册表移除 BTDigg/Nyaa
  - 从 `sourceTrustScore()` 移除已删源的信任分
- 剩余 5 个搜索源：ThePirateBay、BT4G、1337x、TorrentKitty、YTS
- 更新 README.md 搜索源数量描述


## 2026-04-17

### 架构重构（Day 3a – Day 9）

**Day 3a：app/ 业务编排层**
- 新增 `app/app.go`，作为 TUI/CLI 与 engine/search 之间的唯一依赖边界
- TUI 不再直接 import `engine` / `search`，所有业务能力通过 `app.App` 暴露

**Day 4-5：search/ 拆包**
- `search/` 平铺结构拆分为三个子包：
  - `search/providers/`：8 个搜索源独立实现 + `DefaultProviders()` 注册表
  - `search/query/`：用户输入 → 标准化搜索词（本地别名解析、NLP、TMDB、Groq）
  - `search/pipeline/`：搜索编排、去重、关键词过滤、严格电影评分、BEP 15 做种数补全
  - `search/types.go`：共享 `Result`、`Provider` 接口、`MovieResolution`
  - `search/parse.go`：共享解析工具（IsCJK、ParseMovieTitleYear 等）

**Day 6：ResilientClient**
- 新增 `pkg/httputil/resilient.go`：带指数退避 + jitter 重试、per-host 熔断器的 HTTP 客户端
- 所有 8 个 provider 及 `search/query/tmdb.go` 全部切换，移除重复的手动 HTTP 样板代码
- 配套 3 个单元测试（重试成功、4xx 不重试、熔断器开启/半开/重置）

**Day 7-8：Engine 离散事件流 + TUI 事件驱动**
- 新增 `engine/event.go`：定义 `Event` 类型与 6 种 `EventType`（MetaReceived / DownloadDone / SeedingStarted / SeedingStopped / Failed / Canceled）
- Engine 内置 buffered channel（容量 64），状态变更即时 emit，channel 满时静默丢弃不阻塞生产者
- `app.App` 新增 `WaitEvent()` 方法，向上层暴露事件流
- TUI 新增 `eventCmd`：通过 goroutine 阻塞等待事件，触发后立即刷新快照并回到等待；保留 500ms 轮询以持续刷新下载进度

**Day 9：TorrentHandle 接口 + 状态机单元测试**
- 新增 `engine/handle.go`：`TorrentHandle` 接口（`BytesCompleted` / `ActivePeers` / `BytesUploaded` / `Drop`），`realHandle` 包装真实 torrent；`Download` 依赖接口而非具体类型
- 新增 `engine/download_test.go`：9 个状态机单元测试，使用 `mockHandle` 完全脱离 BT 客户端：
  - 初始状态、快照字段正确性
  - Cancel（含幂等）、setFailed + 事件断言
  - watchLifecycle 完整生命周期：无做种完成、做种达分享率停、做种达时间限停、取消中断生命周期



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
