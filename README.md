# BT-Spider 🕷

磁力链接搜索工具。聚合多个 BT 搜索源，按做种数排序，输入序号获取磁力链接。

## 功能

- 聚合 6 个搜索源：ApiBay、BTDigg、BT4G、YTS、EZTV、Nyaa
- 并发搜索，自动按 info_hash 去重，按做种数降序排列
- 输入序号直接输出磁力链接，粘贴到任意 BT 客户端下载
- 代理支持（HTTP_PROXY / HTTPS_PROXY）
- 零外部依赖，纯标准库

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
| `<序号>` | 输出该条目的磁力链接 |
| `quit` / `q` | 退出 |

### 示例

```
bt> search Inception 1080p
🔍 搜索: Inception 1080p

找到 100 个结果（按做种数排序）:

  [1] Inception (2010) 1080p BrRip x264 - 1.85GB - YIFY
      1.85 GB | Seeders: 428 | Leechers: 134 | ThePirateBay
  [2] Inception.2010.1080p.BluRay.DDP5.1.x265.10bit-GalaxyRG265
      2.87 GB | Seeders: 182 | Leechers: 76 | ThePirateBay
  ...

bt> 1
🔗 Inception (2010) 1080p BrRip x264 - 1.85GB - YIFY
magnet:?xt=urn:btih:...
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
│   └── config.go        # 配置（max_results）
├── search/
│   ├── search.go        # Provider 接口、并发搜索、去重、排序
│   ├── apibay.go        # ThePirateBay（JSON API）
│   ├── btdig.go         # BTDigg（HTML 爬取）
│   ├── bt4g.go          # BT4G（RSS）
│   ├── yts.go           # YTS（JSON API，电影）
│   ├── eztv.go          # EZTV（JSON API，剧集）
│   └── nyaa.go          # Nyaa（RSS，动漫）
└── pkg/
    ├── httputil/
    │   └── client.go    # 共享 HTTP 客户端（代理、UA、超时）
    └── utils/
        └── format.go    # FormatBytes

```

## 相关项目

| 项目 | 说明 |
|------|------|
| [BT-Books](https://github.com/huangke19/BT-Books) | 电子书下载工具（Z-Library） |
| [BT-Music](https://github.com/huangke19/BT-Music) | 音乐下载工具（B站 yt-dlp + BT搜索） |

## 许可证

MIT
