# BT-Spider

Personal BT download tool with multi-source search, ebook download, and Telegram Bot.

## Features

- Magnet link download with real-time progress (speed, ETA, peers)
- 6 search sources aggregated: ApiBay, BTDigg, BT4G, YTS, EZTV, Nyaa
- Concurrent search across all sources, auto-dedup by info_hash
- Ebook search & download via Z-Library (zlib CLI)
- Telegram Bot: remote search, download, status, cancel
- Public tracker list auto-refresh (24h)
- Proxy support (HTTP_PROXY / HTTPS_PROXY)
- User permission control (Telegram whitelist)

## Quick Start

### Build

```bash
go build -o bt-spider .
```

### CLI Mode

```bash
# Default download dir: ~/Downloads/BT-Spider/
./bt-spider

# Custom download dir
./bt-spider /path/to/download
```

Interactive commands:

```
bt> search matrix          # Search torrents
bt> 1                      # Download result #1
bt> magnet:?xt=urn:btih:...  # Download magnet link
bt> book 三毛 撒哈拉的故事    # Search ebooks
bt> quit                   # Exit
```

### Bot Mode

```bash
# Via config file
cp config.example.json config.json
# Edit config.json, set telegram_bot_token
./bt-spider --bot

# Or via environment variable
export BT_TELEGRAM_BOT_TOKEN="your-token"
./bt-spider --bot
```

Bot commands:

| Command | Description |
|---------|-------------|
| `/search <keyword>` | Search torrents |
| `/s <keyword>` | Search (short) |
| `/status` | Download status |
| `/cancel` | Cancel downloads |
| `/help` | Help |

Send keywords directly to search, send magnet links to download.

### Proxy

```bash
export HTTPS_PROXY=http://127.0.0.1:7890
export HTTP_PROXY=http://127.0.0.1:7890
./bt-spider
```

## Configuration

See [config.example.json](config.example.json):

| Field | Default | Description |
|-------|---------|-------------|
| `download_dir` | `~/Downloads/BT-Spider/` | Download directory |
| `listen_addr` | `:6881` | BT listen address |
| `max_conns` | `80` | Max connections per torrent |
| `seed` | `false` | Seed after download |
| `enable_tracker_list` | `true` | Auto-fetch public trackers |
| `telegram_bot_token` | `""` | Telegram Bot token |
| `allowed_user_ids` | `[]` | Telegram user ID whitelist (empty = no restriction) |

## Search Sources

| Source | Type | Interface | Notes |
|--------|------|-----------|-------|
| ApiBay (TPB) | General | JSON API | Auto-filters irrelevant results |
| BTDigg | General (DHT) | HTML scraping | No exact seed count |
| BT4G | General | RSS | Cloudflare protected |
| YTS | Movies | JSON API | High quality movie torrents |
| EZTV | TV Shows | JSON API | Client-side keyword filter |
| Nyaa | Anime | RSS | Japanese/anime content |

## Ebook Download

Requires [heartleo/zlib](https://github.com/heartleo/zlib) CLI:

```bash
# Install
GOPATH=/tmp/gopath go install github.com/heartleo/zlib/cmd/zlib@latest
cp /tmp/gopath/bin/zlib ~/bin/zlib

# Login (requires proxy)
HTTPS_PROXY=http://127.0.0.1:7890 ~/bin/zlib login --email your@email.com --password yourpass
```

Ebooks are saved to `~/Documents/Books/` by default.

## Project Structure

```
.
├── main.go                # CLI entry point, interactive REPL, bot startup
├── config/
│   └── config.go          # Configuration loading, defaults, env overrides
├── engine/
│   ├── engine.go          # BT download engine (sync download with progress)
│   ├── download.go        # Async download task (for bot mode)
│   └── trackers.go        # Public tracker list auto-refresh (24h)
├── search/
│   ├── search.go          # Provider interface, concurrent search, dedup, DefaultProviders()
│   ├── apibay.go          # ThePirateBay provider (JSON API)
│   ├── btdig.go           # BTDigg provider (HTML scraping)
│   ├── bt4g.go            # BT4G provider (RSS)
│   ├── yts.go             # YTS provider (JSON API, movies)
│   ├── eztv.go            # EZTV provider (JSON API, TV shows)
│   ├── nyaa.go            # Nyaa provider (RSS, anime)
│   ├── book.go            # BookProvider interface, SearchBooks aggregation
│   ├── zlib.go            # Z-Library ebook search/download (via zlib CLI)
│   ├── zlibrary.go        # Z-Library web search links
│   └── annasarchive.go    # Anna's Archive web search links
├── bot/
│   └── bot.go             # Telegram Bot (search, download, progress, file send)
├── pkg/
│   ├── utils/
│   │   └── format.go      # FormatBytes, FormatDuration, ProgressBar, Truncate
│   └── httputil/
│       └── client.go      # Shared HTTP client factory (proxy, UA, timeout)
├── config.example.json
├── .gitignore
├── LICENSE
└── README.md
```

## Architecture

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
│     search (Provider)   │  Concurrent search, dedup, sort
│  ApiBay│BtDig│BT4G│...  │
└────────────┬────────────┘
             │
             ▼
┌─────────────────────────┐
│     engine (Engine)     │  Torrent download, progress, trackers
│  AddMagnet│AddMagnetAsync│
└────────────┬────────────┘
             │
             ▼
┌─────────────────────────┐
│   pkg/httputil + utils  │  Shared HTTP client, formatting
└─────────────────────────┘
```

## Tech Stack

- [anacrolix/torrent](https://github.com/anacrolix/torrent) - BT protocol
- [telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) - Telegram Bot SDK
- [heartleo/zlib](https://github.com/heartleo/zlib) - Z-Library CLI

## License

MIT
