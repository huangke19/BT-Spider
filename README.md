# BT-Spider 🕷

个人 BT 下载工具，支持磁力链接下载、多源搜索、Telegram Bot 远程控制。

## 功能

- 磁力链接解析与下载，实时进度显示
- 多搜索源聚合（ApiBay / BTDigg / BT4G / YTS / EZTV / Nyaa），按做种数排序、自动去重
- Telegram Bot 远程控制：搜索、下载、查看状态、取消任务
- 公共 Tracker 列表自动拉取与定时刷新（24h）
- 用户权限控制（Telegram 白名单）
- 优雅退出（Ctrl+C）

## 快速开始

### CLI 模式

```bash
# 编译
go build -o bt-spider .

# 运行（默认下载到 ~/Downloads/BT-Spider/）
./bt-spider

# 指定下载目录
./bt-spider /path/to/download
```

启动后进入交互式 `bt>` 提示符，支持以下用法：

```
# 搜索关键词
bt> search 黑客帝国

# 搜索结果会编号列出，输入序号即可开始下载
bt> 1

# 也可直接粘贴磁力链接下载
bt> magnet:?xt=urn:btih:xxxxx...
```

### 代理支持

搜索功能自动读取系统代理环境变量，无需改代码：

```bash
export HTTPS_PROXY=http://127.0.0.1:7890
export HTTP_PROXY=http://127.0.0.1:7890
./bt-spider
```

### Telegram Bot 模式

1. 复制配置文件并填入 Bot Token：

```bash
cp config.example.json config.json
```

```json
{
  "download_dir": "",
  "listen_addr": ":6881",
  "max_conns": 80,
  "seed": false,
  "telegram_bot_token": "YOUR_BOT_TOKEN_HERE",
  "allowed_user_ids": []
}
```

- `download_dir`：下载目录，留空则默认 `~/Downloads/BT-Spider/`
- `allowed_user_ids`：Telegram 用户 ID 白名单，空数组表示不限制
- `seed`：下载完成后是否做种
- `max_conns`：每个 torrent 最大连接数

2. 也可通过环境变量设置 Token：

```bash
export BT_TELEGRAM_BOT_TOKEN="***"
```

### Bot 命令

| 命令 | 说明 |
|------|------|
| `/search <关键词>` | 搜索磁力资源 |
| `/s <关键词>` | 搜索（简写） |
| `/status` | 查看下载状态 |
| `/cancel` | 取消当前下载 |
| `/help` | 显示帮助 |

直接发送关键词也会触发搜索，直接发送磁力链接会开始下载。

## 搜索源

| 源 | 类型 | 接口 |
|-----|------|------|
| ApiBay (TPB) | 综合 | JSON API |
| BTDigg | 综合 (DHT) | HTML 解析 |
| BT4G | 综合 | RSS |
| YTS | 电影 | JSON API |
| EZTV | 美剧 | JSON API |
| Nyaa | 动漫 | RSS |

## 项目结构

```
.
├── main.go              # CLI 入口
├── config/config.go     # 配置管理
├── engine/
│   ├── engine.go        # BT 下载引擎
│   ├── download.go      # 下载任务管理
│   └── trackers.go      # 公共 Tracker 列表
├── search/
│   ├── search.go        # 搜索聚合与去重
│   ├── apibay.go        # ThePirateBay
│   ├── btdig.go         # BTDigg
│   ├── bt4g.go          # BT4G
│   ├── yts.go           # YTS
│   ├── eztv.go          # EZTV
│   └── nyaa.go          # Nyaa
├── bot/bot.go           # Telegram Bot
└── config.example.json  # 配置示例
```

## 技术栈

- [anacrolix/torrent](https://github.com/anacrolix/torrent) — BT 协议实现
- [telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) — Telegram Bot SDK
