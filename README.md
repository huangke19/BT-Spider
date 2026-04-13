# BT-Spider 🕷

磁力搜索 + BT 下载工具。聚合多个搜索源，按做种数排序，输入序号直接下载。

## 功能

- 聚合 7 个搜索源：ApiBay、BTDigg、BT4G、YTS、EZTV、Nyaa、1337x
- 并发搜索，自动去重，按做种数降序排列
- 输入序号直接开始下载，实时显示进度条
- 支持直接粘贴磁力链接下载
- 自动拉取 tracker 列表，提升连接成功率
- 代理支持（HTTP_PROXY / HTTPS_PROXY）

## 快速开始

### 编译

```bash
go build -o bt-spider .
```

### 运行

```bash
# 需要代理访问 BT 搜索源
HTTPS_PROXY=http://127.0.0.1:7890 ./bt-spider
```

### 命令

| 命令 | 说明 |
|------|------|
| `search <关键词>` | 搜索，列出结果 |
| `<序号>` | 下载对应条目（显示实时进度） |
| `magnet:?xt=...` | 直接粘贴磁力链接下载 |
| `quit` / `q` | 退出 |

### 示例

```
bt> search Inception 1080p
🔍 搜索: Inception 1080p

找到 104 个结果（按做种数排序）:

  [1] Inception 2010 1080p BrRip x264 1 85GB YIFY
      1.9 GB | Seeders: 922 | Leechers: 64 | 1337x
  [2] Inception 2010 720p BrRip x264 YIFY
      1.1 GB | Seeders: 393 | Leechers: 48 | 1337x
  ...

bt> 1
⬇  正在下载: Inception 2010 1080p BrRip x264 1 85GB YIFY
进度: [████████████████████░░░░░░░░░░] 68.3% | 1.3 GB / 1.9 GB | ↓ 3.2 MB/s
```

## 搜索源

| 来源 | 类型 | 接口 |
|------|------|------|
| ApiBay (TPB) | 综合 | JSON API |
| BTDigg | 综合 (DHT) | JSON API |
| BT4G | 综合 | RSS |
| YTS | 电影 | JSON API |
| EZTV | 剧集 | JSON API |
| Nyaa | 动漫 | RSS |
| 1337x | 综合 | HTML 爬取 |

## 下载

文件保存至 `~/Downloads/BT-Spider/`，可在 `config/config.go` 中修改路径。

下载时会自动从 [trackerslist.com](https://trackerslist.com/best.txt) 拉取最新 tracker 列表（每 24 小时刷新），提升连接成功率。

若 30 秒内未能获取种子元数据（通常是 peer 数不足），会提示换下一个。

## 代理

```bash
export HTTPS_PROXY=http://127.0.0.1:7890
./bt-spider
```

## 项目结构

```
.
├── main.go              # CLI 入口，交互式 REPL
├── config/
│   └── config.go        # 配置（下载目录、最大结果数、连接数等）
├── engine/
│   ├── engine.go        # 下载引擎（磁力→BT，实时进度条）
│   ├── download.go      # 异步下载管理
│   └── trackers.go      # Tracker 列表自动更新
├── search/
│   ├── search.go        # Provider 接口、并发搜索、去重、排序、关键词过滤
│   ├── apibay.go        # ThePirateBay（JSON API）
│   ├── btdig.go         # BTDigg（HTML 爬取）
│   ├── bt4g.go          # BT4G（RSS）
│   ├── yts.go           # YTS（JSON API，电影）
│   ├── eztv.go          # EZTV（JSON API，剧集）
│   ├── nyaa.go          # Nyaa（RSS，动漫）
│   └── 1337x.go         # 1337x（HTML 爬取，两步：列表页+详情页）
└── pkg/
    ├── httputil/
    │   └── client.go    # 共享 HTTP 客户端（代理、UA、超时）
    └── utils/
        └── format.go    # FormatBytes, FormatDuration, ProgressBar
```

## 相关项目

| 项目 | 说明 |
|------|------|
| [BT-Books](https://github.com/huangke19/BT-Books) | 电子书下载工具（Z-Library） |
| [BT-Music](https://github.com/huangke19/BT-Music) | 音乐下载工具（B站 yt-dlp + BT搜索） |

## 许可证

MIT
