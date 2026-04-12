package bot

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/huangke/bt-spider/config"
	"github.com/huangke/bt-spider/engine"
	"github.com/huangke/bt-spider/search"
)

// Bot Telegram 机器人
type Bot struct {
	api     *tgbotapi.BotAPI
	cfg     *config.Config
	eng     *engine.Engine
	results sync.Map // chatID -> []search.Result（缓存搜索结果）
	downloads sync.Map // chatID_msgID -> *engine.Download（活跃下载）
}

// New 创建 Telegram Bot
func New(cfg *config.Config, eng *engine.Engine) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		return nil, fmt.Errorf("创建 Telegram Bot 失败: %w", err)
	}

	log.Printf("🤖 Telegram Bot 已连接: @%s", api.Self.UserName)

	return &Bot{
		api: api,
		cfg: cfg,
		eng: eng,
	}, nil
}

// Run 启动 Bot 消息循环
func (b *Bot) Run() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			b.handleCallback(update.CallbackQuery)
			continue
		}

		if update.Message == nil {
			continue
		}

		// 权限检查
		if !b.cfg.IsAllowedUser(update.Message.From.ID) {
			b.reply(update.Message.Chat.ID, "⛔ 无权限使用此 Bot")
			continue
		}

		b.handleMessage(update.Message)
	}
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	text := strings.TrimSpace(msg.Text)

	switch {
	case text == "/start" || text == "/help":
		b.handleHelp(msg.Chat.ID)

	case strings.HasPrefix(text, "/search ") || strings.HasPrefix(text, "/s "):
		parts := strings.SplitN(text, " ", 2)
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			b.reply(msg.Chat.ID, "用法: /search <关键词>")
			return
		}
		b.handleSearch(msg.Chat.ID, strings.TrimSpace(parts[1]))

	case text == "/status":
		b.handleStatus(msg.Chat.ID)

	case text == "/cancel":
		b.handleCancel(msg.Chat.ID)

	case strings.HasPrefix(text, "magnet:"):
		b.handleMagnet(msg.Chat.ID, text)

	default:
		// 如果不是命令，当作搜索关键词
		if text != "" && !strings.HasPrefix(text, "/") {
			b.handleSearch(msg.Chat.ID, text)
		}
	}
}

func (b *Bot) handleHelp(chatID int64) {
	help := `🕷 *BT\-Spider Bot*

*命令:*
/search <关键词> \- 搜索磁力链接
/status \- 查看下载状态
/cancel \- 取消当前下载
/help \- 显示帮助

直接发送关键词也可以搜索
直接发送磁力链接可以开始下载`

	msg := tgbotapi.NewMessage(chatID, help)
	msg.ParseMode = "MarkdownV2"
	b.api.Send(msg)
}

func (b *Bot) handleSearch(chatID int64, keyword string) {
	// 发送搜索中提示
	waitMsg := b.reply(chatID, fmt.Sprintf("🔍 搜索: %s ...", keyword))

	providers := []search.Provider{
		search.NewApiBay(),
		search.NewBtDig(),
		search.NewBT4G(),
		search.NewYTS(),
		search.NewEZTV(),
		search.NewNyaa(),
	}

	results, err := search.Search(keyword, providers)
	if err != nil {
		b.editMessage(chatID, waitMsg, fmt.Sprintf("❌ 搜索失败: %v", err))
		return
	}

	if len(results) == 0 {
		b.editMessage(chatID, waitMsg, "未找到有做种的结果")
		return
	}

	// 最多 15 条
	limit := 15
	if len(results) < limit {
		limit = len(results)
	}
	results = results[:limit]

	// 缓存结果
	b.results.Store(chatID, results)

	// 构建消息和按钮
	var text strings.Builder
	text.WriteString(fmt.Sprintf("🔍 搜索 \"%s\" 找到 %d 个结果:\n\n", keyword, len(results)))

	var rows [][]tgbotapi.InlineKeyboardButton
	for i, r := range results {
		seedIcon := "🟢"
		if r.Seeders < 5 {
			seedIcon = "🟡"
		}
		text.WriteString(fmt.Sprintf("%d. %s\n   %s | %s Seeders: %d | %s\n\n",
			i+1, r.Name, r.Size, seedIcon, r.Seeders, r.Source))

		// 每行一个按钮
		btn := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("⬇️ %d. %s (%s)", i+1, truncate(r.Name, 30), r.Size),
			fmt.Sprintf("dl:%d", i),
		)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.editMessageWithKeyboard(chatID, waitMsg, text.String(), keyboard)
}

func (b *Bot) handleCallback(cb *tgbotapi.CallbackQuery) {
	// 权限检查
	if !b.cfg.IsAllowedUser(cb.From.ID) {
		callback := tgbotapi.NewCallback(cb.ID, "⛔ 无权限")
		b.api.Request(callback)
		return
	}

	chatID := cb.Message.Chat.ID

	if !strings.HasPrefix(cb.Data, "dl:") {
		return
	}

	// 解析下载序号
	var idx int
	fmt.Sscanf(cb.Data, "dl:%d", &idx)

	// 获取缓存的搜索结果
	val, ok := b.results.Load(chatID)
	if !ok {
		callback := tgbotapi.NewCallback(cb.ID, "搜索结果已过期，请重新搜索")
		b.api.Request(callback)
		return
	}
	results := val.([]search.Result)
	if idx < 0 || idx >= len(results) {
		callback := tgbotapi.NewCallback(cb.ID, "无效选择")
		b.api.Request(callback)
		return
	}

	result := results[idx]

	// 回应 callback
	callback := tgbotapi.NewCallback(cb.ID, fmt.Sprintf("开始下载: %s", truncate(result.Name, 40)))
	b.api.Request(callback)

	// 开始下载
	b.startDownload(chatID, result)
}

func (b *Bot) startDownload(chatID int64, result search.Result) {
	statusMsg := b.reply(chatID, fmt.Sprintf("⏳ 开始下载: %s\n正在获取元数据...", result.Name))

	dl, err := b.eng.AddMagnetAsync(result.Magnet)
	if err != nil {
		b.editMessage(chatID, statusMsg, fmt.Sprintf("❌ 下载失败: %v", err))
		return
	}

	// 存储下载任务
	dlKey := fmt.Sprintf("%d_%d", chatID, statusMsg)
	b.downloads.Store(dlKey, dl)

	// 启动进度更新
	go b.trackProgress(chatID, statusMsg, dl, result.Name)
}

func (b *Bot) trackProgress(chatID int64, msgID int, dl *engine.Download, name string) {
	dlKey := fmt.Sprintf("%d_%d", chatID, msgID)
	defer b.downloads.Delete(dlKey)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	lastText := ""
	metadataReceived := false
	var prevCompleted int64
	prevTime := time.Now()
	// 滑动窗口计算平均速度
	type sample struct {
		bytes int64
		time  time.Time
	}
	samples := make([]sample, 0, 10)

	for range ticker.C {
		if dl.IsCanceled() {
			b.editMessage(chatID, msgID, fmt.Sprintf("🚫 已取消: %s", name))
			return
		}

		completed, total, peers, done := dl.Progress()

		if total == 0 {
			if !metadataReceived {
				continue
			}
		} else if !metadataReceived {
			metadataReceived = true
			if dl.Name != "" {
				name = dl.Name
			}
			prevCompleted = completed
			prevTime = time.Now()
		}

		if done {
			text := fmt.Sprintf("✅ 下载完成!\n\n📦 %s\n📁 %s\n💾 %s",
				name, formatBytes(total), b.cfg.DownloadDir)
			b.editMessage(chatID, msgID, text)

			// 发送文件到 Telegram
			b.sendDownloadedFiles(chatID, dl)
			return
		}

		if total > 0 {
			now := time.Now()

			// 更新滑动窗口
			samples = append(samples, sample{bytes: completed, time: now})
			if len(samples) > 10 {
				samples = samples[1:]
			}

			// 计算平均速度
			var avgSpeed float64
			if len(samples) >= 2 {
				first := samples[0]
				last := samples[len(samples)-1]
				dt := last.time.Sub(first.time).Seconds()
				if dt > 0 {
					avgSpeed = float64(last.bytes-first.bytes) / dt
				}
			}
			if avgSpeed <= 0 {
				dt := now.Sub(prevTime).Seconds()
				if dt > 0 {
					avgSpeed = float64(completed-prevCompleted) / dt
				}
			}

			// 计算 ETA
			remaining := total - completed
			eta := "计算中..."
			if avgSpeed > 0 && remaining > 0 {
				secs := float64(remaining) / avgSpeed
				eta = formatDuration(time.Duration(secs) * time.Second)
			}

			prevCompleted = completed
			prevTime = now

			percent := float64(completed) / float64(total) * 100
			bar := progressBar(percent, 20)

			text := fmt.Sprintf("⬇️ 下载中: %s\n\n%s %.1f%%\n%s / %s\n⚡ %s/s | ⏱ ETA %s | 👥 %d peers",
				name, bar, percent,
				formatBytes(completed), formatBytes(total),
				formatBytes(int64(avgSpeed)), eta, peers)

			if text != lastText {
				b.editMessage(chatID, msgID, text)
				lastText = text
			}
		}
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func (b *Bot) handleStatus(chatID int64) {
	var active []string
	b.downloads.Range(func(key, value any) bool {
		dl := value.(*engine.Download)
		if dl.IsCanceled() {
			return true
		}
		completed, total, peers, _ := dl.Progress()
		if total > 0 {
			percent := float64(completed) / float64(total) * 100
			active = append(active, fmt.Sprintf("📦 %s\n   %.1f%% | %s/%s | 👥 %d peers",
				dl.Name, percent, formatBytes(completed), formatBytes(total), peers))
		} else {
			name := dl.Name
			if name == "" {
				name = "获取元数据中..."
			}
			active = append(active, fmt.Sprintf("⏳ %s", name))
		}
		return true
	})

	if len(active) == 0 {
		b.reply(chatID, "没有活跃的下载任务")
		return
	}

	text := fmt.Sprintf("📊 活跃下载 (%d):\n\n%s", len(active), strings.Join(active, "\n\n"))
	b.reply(chatID, text)
}

func (b *Bot) handleCancel(chatID int64) {
	canceled := 0
	b.downloads.Range(func(key, value any) bool {
		k := key.(string)
		if strings.HasPrefix(k, fmt.Sprintf("%d_", chatID)) {
			dl := value.(*engine.Download)
			dl.Cancel()
			canceled++
		}
		return true
	})

	if canceled == 0 {
		b.reply(chatID, "没有可取消的下载任务")
	} else {
		b.reply(chatID, fmt.Sprintf("🚫 已取消 %d 个下载任务", canceled))
	}
}

func (b *Bot) handleMagnet(chatID int64, magnet string) {
	result := search.Result{
		Name:   "磁力链接下载",
		Magnet: magnet,
	}
	b.startDownload(chatID, result)
}

// reply 发送消息并返回消息 ID
func (b *Bot) reply(chatID int64, text string) int {
	msg := tgbotapi.NewMessage(chatID, text)
	sent, err := b.api.Send(msg)
	if err != nil {
		log.Printf("发送消息失败: %v", err)
		return 0
	}
	return sent.MessageID
}

// editMessage 编辑消息
func (b *Bot) editMessage(chatID int64, msgID int, text string) {
	if msgID == 0 {
		return
	}
	edit := tgbotapi.NewEditMessageText(chatID, msgID, text)
	b.api.Send(edit)
}

// editMessageWithKeyboard 编辑消息并附带键盘
func (b *Bot) editMessageWithKeyboard(chatID int64, msgID int, text string, keyboard tgbotapi.InlineKeyboardMarkup) {
	if msgID == 0 {
		return
	}
	edit := tgbotapi.NewEditMessageText(chatID, msgID, text)
	edit.ReplyMarkup = &keyboard
	b.api.Send(edit)
}

// Stop 停止 Bot
func (b *Bot) Stop() {
	b.api.StopReceivingUpdates()
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func progressBar(percent float64, width int) string {
	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}
	bar := make([]rune, width)
	for i := range bar {
		if i < filled {
			bar[i] = '█'
		} else {
			bar[i] = '░'
		}
	}
	return string(bar)
}

// sendDownloadedFiles 将下载完成的文件发送到 Telegram
func (b *Bot) sendDownloadedFiles(chatID int64, dl *engine.Download) {
	// Telegram Bot API 文件大小限制: 50MB
	const maxFileSize int64 = 50 * 1024 * 1024

	files := dl.Torrent.Files()
	for _, f := range files {
		filePath := filepath.Join(b.cfg.DownloadDir, f.Path())

		info, err := os.Stat(filePath)
		if err != nil {
			log.Printf("无法获取文件信息 %s: %v", filePath, err)
			continue
		}

		if info.IsDir() {
			continue
		}

		if info.Size() > maxFileSize {
			b.reply(chatID, fmt.Sprintf("⚠️ 文件过大无法发送 (%.1f MB > 50 MB):\n%s",
				float64(info.Size())/(1024*1024), f.Path()))
			continue
		}

		doc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(filePath))
		doc.Caption = f.Path()
		if _, err := b.api.Send(doc); err != nil {
			log.Printf("发送文件失败 %s: %v", filePath, err)
			b.reply(chatID, fmt.Sprintf("❌ 发送文件失败: %s\n%v", f.Path(), err))
		}
	}
}

func formatBytes(b int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

