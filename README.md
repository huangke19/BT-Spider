# BT-Spider 🕷

磁力搜索 + BT 下载工具。聚合多个搜索源，按做种数排序；**支持交互式 TUI** 与 **无头命令行** 两种使用方式。

## 功能

- 聚合 8 个搜索源：ApiBay、BTDigg、BT4G、YTS、EZTV、Nyaa、1337x、TorrentKitty
- 并发搜索、自动去重、按做种数降序排列
- 搜索带总超时保护，慢源不会一直拖住整体结果
- **TUI 实时界面**：多任务进度条同屏刷新，边下边搜不阻塞
- **Headless CLI**：供脚本 / AI 助手通过子进程调用，支持 JSON 流式输出
- 中文搜索：CJK 关键词采用 bigram 分词，避免因无空格导致的误过滤
- 自动拉取 tracker 列表（每 24h 刷新），提升连接成功率
- 代理支持（`HTTP_PROXY` / `HTTPS_PROXY`）

## 快速开始

### 编译

```bash
# 主程序（TUI）
go build -o bt-spider .

# 无头下载器
go build -o bt-download ./cmd/download
```

### TUI 模式（交互式，推荐日常使用）

```bash
HTTPS_PROXY=http://127.0.0.1:7890 ./bt-spider
```

进入后：

```
bt> search The Bourne Supremacy 2004
bt> 1                    # 下载第 1 条结果
bt> 2                    # 再加一个任务，两个并行下载
bt> c 1                  # 取消任务 #1
bt> clear                # 清理已结束任务
bt> q                    # 退出
```

TUI 界面会每 500ms 刷新，所有任务的进度条 / 速度 / peers / ETA 同屏实时显示。

| TUI 命令 | 说明 |
|----------|------|
| `search <关键词>` | 搜索（异步，不阻塞输入） |
| `<序号>` | 下载搜索结果中的对应条目 |
| `magnet:?xt=...` | 直接添加磁力链接 |
| `c <下载序号>` | 取消指定下载任务 |
| `clear` | 清理已完成 / 失败 / 取消的任务 |
| `q` / `quit` / `Ctrl+C` | 退出 |

> ⚠️ **TUI 需要真实终端（TTY）**。通过管道、AI 助手子进程、后台服务等非 TTY 环境调用会报错，请改用 Headless 模式。

### Headless 模式（脚本 / AI 调用）

进度以**一行一条**的流式日志输出（非 TTY 友好）。

```bash
# 关键词搜索 + 下载做种数最高的
./bt-download "The Bourne Supremacy 2004"

# 指定选第 2 个结果
./bt-download --pick 2 "Ubuntu 24.04"

# 直接下载磁力链接
./bt-download 'magnet:?xt=urn:btih:90FD7709140B1C82C32E6014FB1F99A317DB68A3'

# JSON 输出（脚本解析友好）
./bt-download --json --dir /tmp/dl "Ubuntu 24.04"
```

**文本输出样例：**

```
[search] keyword="The Bourne Supremacy 2004"
[result] [1] The Bourne Supremacy (2004) 1080p BrRip x264 YIFY | 1.51 GB | S:154 L:5 | ThePirateBay
[result] [2] The Bourne Supremacy 2004 1080p BluRay DD+ 5.1 x265-EDGE2020 | 3.91 GB | S:42 L:8 | ThePirateBay
...
[picked] name="The Bourne Supremacy (2004) 1080p BrRip x264 YIFY" dir=/Users/you/Downloads/BT-Spider
[meta] 等待元数据...  peers=7
[progress] 2.1%  32 MB/1.51 GB  ↓ 8.5 MB/s  peers=12  ETA 2m54s
[progress] 5.4%  83 MB/1.51 GB  ↓ 11.2 MB/s  peers=28  ETA 2m10s
...
[done] ✅ The Bourne Supremacy (2004) 1080p BrRip x264 YIFY -> /Users/you/Downloads/BT-Spider
```

**JSON 输出样例（每行一条）：**

```json
{"event":"search","keyword":"Ubuntu 24.04","ts":"2026-04-14T10:22:01+08:00"}
{"event":"result","index":1,"name":"ubuntu-24.04-desktop-amd64.iso","size":"4.7 GB","seeders":523,"leechers":12,"source":"1337x","ts":"..."}
{"event":"progress","percent":"63.2","completed":"3.0 GB","total":"4.7 GB","speed":"15.2 MB/s","peers":42,"eta":"1m52s","ts":"..."}
{"event":"done","name":"ubuntu-24.04-desktop-amd64.iso","dir":"/Users/you/Downloads/BT-Spider","ts":"..."}
```

**所有选项：**

```
--dir <path>          下载目录（默认使用 config.json 或 ~/Downloads/BT-Spider）
--pick <N>            搜索模式下选第 N 个结果（默认 1）
--show <N>            搜索模式下预览前 N 条候选（默认 5）
--json                输出 JSON 每行一条（默认文本）
--interval <dur>      进度输出间隔（默认 2s）
```

**退出码：** `0` 成功 / `1` 失败 / `2` 参数错误 / `130` 用户中断

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

文件保存至 `~/Downloads/BT-Spider/`，可在 `config.json` 或 `--dir` 参数中覆盖。

下载时会自动从 [trackerslist.com](https://trackerslist.com/best.txt) 拉取最新 tracker 列表（每 24 小时刷新），提升连接成功率。
程序启动时会先使用内置 fallback trackers，远端列表在后台异步刷新，不再阻塞启动。

若 2 分钟内未能获取种子元数据（通常是 peer 数不足），任务会标记为失败。

## 配置

可选的 `config.json` 示例：

```json
{
  "download_dir": "/Users/you/Downloads/BT-Spider",
  "max_results": 100,
  "max_conns": 80,
  "listen_port": 0,
  "seed": false,
  "enable_tracker_list": true
}
```

其中 `listen_port: 0` 表示自动选择可用端口，能减少固定端口被占用导致的启动失败。

## 代理

```bash
export HTTPS_PROXY=http://127.0.0.1:7890
./bt-spider           # 或 ./bt-download ...
```

## 关于中文搜索

本项目对 CJK 关键词采用 **bigram（二元语法）** 分词，避免 ASCII 式的空格分词在中文上失效的问题：

```
"谍影重重第二部"  →  [谍影, 影重, 重重, 重第, 第二, 二部]
```

搜索结果标题至少需命中一半以上 bigram 才会保留。纯英文关键词仍按空格分词。

## 项目结构

```
.
├── main.go                   # TUI 主入口（bubbletea）
├── cmd/
│   └── download/
│       └── main.go           # Headless CLI（脚本/AI 调用）
├── tui/
│   └── tui.go                # bubbletea Model/Update/View
├── config/
│   └── config.go             # 配置（下载目录、最大结果数、连接数等）
├── engine/
│   ├── engine.go             # 下载引擎、任务注册表
│   ├── download.go           # 异步下载、状态机、EWMA 速度/ETA
│   └── trackers.go           # Tracker 列表自动更新
├── search/
│   ├── search.go             # Provider 接口、并发搜索、CJK bigram 过滤
│   ├── filter_test.go        # 过滤器单元测试
│   ├── apibay.go             # ThePirateBay（JSON API）
│   ├── btdig.go              # BTDigg（HTML 爬取）
│   ├── bt4g.go               # BT4G（RSS）
│   ├── yts.go                # YTS（JSON API，电影）
│   ├── eztv.go               # EZTV（JSON API，剧集）
│   ├── nyaa.go               # Nyaa（RSS，动漫）
│   └── 1337x.go              # 1337x（HTML 爬取）
└── pkg/
    ├── httputil/
    │   └── client.go         # 共享 HTTP 客户端（代理、UA、超时）
    └── utils/
        └── format.go         # FormatBytes, FormatDuration, ProgressBar
```

## 相关项目

| 项目 | 说明 |
|------|------|
| [BT-Books](https://github.com/huangke19/BT-Books) | 电子书下载工具（Z-Library） |
| [BT-Music](https://github.com/huangke19/BT-Music) | 音乐下载工具（B站 yt-dlp + BT搜索） |

## 许可证

MIT
