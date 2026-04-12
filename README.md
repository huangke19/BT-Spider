# BT-Spider

个人 BT 下载工具，支持多源搜索、电子书下载和 Telegram Bot。

## 功能

- 磁力链接下载，实时显示进度（速度、预计时间、Peer 数）
- 聚合 6 个搜索源：ApiBay、BTDigg、BT4G、YTS、EZTV、Nyaa
- 并发搜索所有源，自动按 info_hash 去重
- 电子书搜索和下载（通过 Z-Library / zlib CLI）
- Telegram Bot：远程搜索、下载、查看状态、取消任务
- 公共 Tracker 列表自动刷新（24 小时）
- 代理支持（HTTP_PROXY / HTTPS_PROXY）
- 用户权限控制（Telegram 白名单）

## 快速开始

### 编译

```bash
go build -o bt-spider .
```

### CLI 模式

```bash
# 默认下载目录：~/Downloads/BT-Spider/
./bt-spider

# 自定义下载目录
./bt-spider /path/to/download
```

交互命令：

```
bt> search matrix          # 搜索种子
bt> 1                      # 下载第 1 条结果
bt> magnet:?xt=urn:btih:...  # 下载磁力链接
bt> book 三毛 撒哈拉的故事    # 搜索电子书
bt> quit                   # 退出
```

### Bot 模式

```bash
# 通过配置文件
cp config.example.json config.json
# 编辑 config.json，设置 telegram_bot_token
./bt-spider --bot

# 或通过环境变量
export BT_TELEGRAM_BOT_TOKEN="***"
./bt-spider --bot
```

Bot 命令：

| 命令 | 说明 |
|------|------|
| `/search <关键词>` | 搜索种子 |
| `/s <关键词>` | 搜索（简写） |
| `/status` | 查看下载状态 |
| `/cancel` | 取消下载 |
| `/help` | 帮助 |

直接发送关键词即可搜索，发送磁力链接即可下载。

### 代理

```bash
export HTTPS_PROXY=http://127.0.0.1:7890
export HTTP_PROXY=http://127.0.0.1:7890
./bt-spider
```

## 配置

参见 [config.example.json](config.example.json)：

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `download_dir` | `~/Downloads/BT-Spider/` | 下载目录 |
| `listen_addr` | `:6881` | BT 监听地址 |
| `max_conns` | `80` | 每个种子最大连接数 |
| `seed` | `false` | 下载完成后是否做种 |
| `enable_tracker_list` | `true` | 自动获取公共 Tracker 列表 |
| `telegram_bot_token` | `""` | Telegram Bot Token |
| `allowed_user_ids` | `[]` | Telegram 用户 ID 白名单（空 = 不限制） |

## 搜索源

| 来源 | 类型 | 接口 | 备注 |
|------|------|------|------|
| ApiBay (TPB) | 综合 | JSON API | 自动过滤无关结果 |
| BTDigg | 综合 (DHT) | HTML 爬取 | 无精确做种数 |
| BT4G | 综合 | RSS | 有 Cloudflare 保护 |
| YTS | 电影 | JSON API | 高质量电影种子 |
| EZTV | 剧集 | JSON API | 客户端关键词过滤 |
| Nyaa | 动漫 | RSS | 日本 / 动漫内容 |

## 电子书下载

需要安装 [heartleo/zlib](https://github.com/heartleo/zlib) CLI：

```bash
# 安装
GOPATH=/tmp/gopath go install github.com/heartleo/zlib/cmd/zlib@latest
cp /tmp/gopath/bin/zlib ~/bin/zlib

# 登录（需要代理）
HTTPS_PROXY=http://127.0.0.1:7890 ~/bin/zlib login --email your@email.com --password yourpass
```

电子书默认保存到 `~/Documents/Books/`。

## 项目结构

```
.
├── main.go                # CLI 入口，交互式 REPL，Bot 启动
├── config/
│   └── config.go          # 配置加载、默认值、环境变量覆盖
├── engine/
│   ├── engine.go          # BT 下载引擎（同步下载 + 进度显示）
│   ├── download.go        # 异步下载任务（Bot 模式用）
│   └── trackers.go        # 公共 Tracker 列表自动刷新（24h）
├── search/
│   ├── search.go          # Provider 接口、并发搜索、去重、DefaultProviders()
│   ├── apibay.go          # ThePirateBay（JSON API）
│   ├── btdig.go           # BTDigg（HTML 爬取）
│   ├── bt4g.go            # BT4G（RSS）
│   ├── yts.go             # YTS（JSON API，电影）
│   ├── eztv.go            # EZTV（JSON API，剧集）
│   ├── nyaa.go            # Nyaa（RSS，动漫）
│   ├── book.go            # BookProvider 接口、SearchBooks 聚合
│   ├── zlib.go            # Z-Library 电子书搜索 / 下载（通过 zlib CLI）
│   ├── zlibrary.go        # Z-Library 网页搜索链接
│   └── annasarchive.go    # Anna's Archive 网页搜索链接
├── bot/
│   └── bot.go             # Telegram Bot（搜索、下载、进度、文件发送）
├── pkg/
│   ├── utils/
│   │   └── format.go      # FormatBytes、FormatDuration、ProgressBar、Truncate
│   └── httputil/
│       └── client.go      # 共享 HTTP 客户端工厂（代理、UA、超时）
├── config.example.json
├── .gitignore
├── LICENSE
└── README.md
```

## 架构

```
┌─────────┐     ┌─────────┐
│  CLI    │     │Telegram │
│ (main)  │     │  Bot    │
└────┬────┘     └────┬────┘
     │               │
     │  search.DefaultProviders()
     │               │
     ▼               ▼
┌─────────────────────────┐
│     search (Provider)   │  并发搜索、去重、排序
│  ApiBay│BtDig│BT4G│...  │
└────────────┬────────────┘
             │
             ▼
┌─────────────────────────┐
│     engine (Engine)     │  种子下载、进度显示、Tracker 管理
│  AddMagnet│AddMagnetAsync│
└────────────┬────────────┘
             │
             ▼
┌─────────────────────────┐
│   pkg/httputil + utils  │  共享 HTTP 客户端、格式化工具
└─────────────────────────┘
```

## 技术栈

- [anacrolix/torrent](https://github.com/anacrolix/torrent) - BT 协议
- [telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) - Telegram Bot SDK
- [heartleo/zlib](https://github.com/heartleo/zlib) - Z-Library CLI

## 许可证

MIT
