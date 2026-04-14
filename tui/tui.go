package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/huangke/bt-spider/engine"
	"github.com/huangke/bt-spider/pkg/utils"
	"github.com/huangke/bt-spider/search"
)

const version = "0.6.0"

// --- 消息类型 ---

type tickMsg time.Time

type searchDoneMsg struct {
	keyword string
	results []search.Result
	err     error
}

type statusMsg struct {
	text  string
	isErr bool
}

// --- 样式 ---

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1)

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			MarginTop(1)

	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	errStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	okStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			MarginTop(1)

	barDone = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	barTodo = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
)

// --- Model ---

type Model struct {
	engine  *engine.Engine
	input   textinput.Model
	results []search.Result
	status  string
	isErr   bool
	width   int
	height  int
	// 最近一次拉到的下载快照（View 期间复用）
	snapshots []engine.DownloadSnapshot
}

// New 创建一个初始化好的 Model
func New(eng *engine.Engine) Model {
	ti := textinput.New()
	ti.Placeholder = "search <关键词>  |  <序号>  |  magnet:...  |  c <序号> 取消  |  q 退出"
	ti.Prompt = "bt> "
	ti.CharLimit = 4096
	ti.Focus()

	return Model{
		engine: eng,
		input:  ti,
		status: fmt.Sprintf("下载目录: %s", eng.Config().DownloadDir),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tickCmd())
}

// --- Update ---

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 6

	case tickMsg:
		m.snapshots = m.engine.ListDownloads()
		return m, tickCmd()

	case searchDoneMsg:
		if msg.err != nil {
			m.results = nil
			m.status = "搜索失败: " + msg.err.Error()
			m.isErr = true
			return m, nil
		}
		m.results = msg.results
		if len(msg.results) == 0 {
			m.status = fmt.Sprintf("「%s」没有找到有做种的结果", msg.keyword)
			m.isErr = true
		} else {
			m.status = fmt.Sprintf("找到 %d 个结果，输入序号下载", len(msg.results))
			m.isErr = false
		}
		return m, nil

	case statusMsg:
		m.status = msg.text
		m.isErr = msg.isErr
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			return m.handleCommand()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// handleCommand 解析当前输入并分发
func (m Model) handleCommand() (tea.Model, tea.Cmd) {
	raw := strings.TrimSpace(m.input.Value())
	if raw == "" {
		return m, nil
	}
	m.input.SetValue("")

	lower := strings.ToLower(raw)

	switch {
	case lower == "q" || lower == "quit" || lower == "exit":
		return m, tea.Quit

	case lower == "clear":
		n := m.engine.ClearFinished()
		return m, statusCmd(fmt.Sprintf("已清理 %d 个已结束任务", n), false)

	case strings.HasPrefix(lower, "search "):
		keyword := strings.TrimSpace(raw[7:])
		if keyword == "" {
			return m, statusCmd("请输入搜索关键词", true)
		}
		m.status = fmt.Sprintf("搜索中: %s ...", keyword)
		m.isErr = false
		return m, searchCmd(keyword)

	case strings.HasPrefix(lower, "c "):
		numStr := strings.TrimSpace(raw[2:])
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return m, statusCmd("用法: c <下载序号>", true)
		}
		if num < 1 || num > len(m.snapshots) {
			return m, statusCmd("下载序号超出范围", true)
		}
		id := m.snapshots[num-1].ID
		if m.engine.RemoveDownload(id) {
			return m, statusCmd(fmt.Sprintf("已取消任务 #%d", num), false)
		}
		return m, statusCmd("取消失败", true)

	case strings.HasPrefix(raw, "magnet:"):
		return m, addMagnetCmd(m.engine, raw, "")

	default:
		// 尝试解析为序号
		num, err := strconv.Atoi(raw)
		if err != nil {
			return m, statusCmd("未知命令：search <关键词> / 序号 / magnet: / c <序号> / q", true)
		}
		if num < 1 || num > len(m.results) {
			return m, statusCmd("搜索结果序号超出范围", true)
		}
		r := m.results[num-1]
		return m, addMagnetCmd(m.engine, r.Magnet, r.Name)
	}
}

// --- Commands (tea.Cmd 工厂) ---

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func searchCmd(keyword string) tea.Cmd {
	return func() tea.Msg {
		results, err := search.Search(keyword, search.DefaultProviders())
		return searchDoneMsg{keyword: keyword, results: results, err: err}
	}
}

func addMagnetCmd(eng *engine.Engine, magnet, name string) tea.Cmd {
	return func() tea.Msg {
		_, err := eng.AddMagnetAsync(magnet)
		if err != nil {
			return statusMsg{text: "添加下载失败: " + err.Error(), isErr: true}
		}
		hint := name
		if hint == "" {
			hint = "新任务"
		}
		return statusMsg{text: "已加入下载队列: " + hint, isErr: false}
	}
}

func statusCmd(text string, isErr bool) tea.Cmd {
	return func() tea.Msg {
		return statusMsg{text: text, isErr: isErr}
	}
}

// --- View ---

func (m Model) View() string {
	var b strings.Builder

	// 标题
	b.WriteString(titleStyle.Render(fmt.Sprintf("🕷  BT-Spider v%s", version)))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("下载目录: " + m.engine.Config().DownloadDir))
	b.WriteString("\n")

	// 搜索结果区
	b.WriteString(sectionStyle.Render("── 搜索结果 ──"))
	b.WriteString("\n")
	if len(m.results) == 0 {
		b.WriteString(dimStyle.Render("  (尚未搜索，输入 search <关键词>)"))
		b.WriteString("\n")
	} else {
		limit := 10
		if len(m.results) < limit {
			limit = len(m.results)
		}
		maxName := m.width - 40
		if maxName < 30 {
			maxName = 30
		}
		for i, r := range m.results[:limit] {
			line := fmt.Sprintf("  [%2d] %-*s  %s  S:%d L:%d %s",
				i+1,
				maxName, truncate(r.Name, maxName),
				pad(r.Size, 10),
				r.Seeders, r.Leechers,
				dimStyle.Render("("+r.Source+")"),
			)
			b.WriteString(line)
			b.WriteString("\n")
		}
		if len(m.results) > limit {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  ... 另有 %d 条已隐藏", len(m.results)-limit)))
			b.WriteString("\n")
		}
	}

	// 下载任务区
	b.WriteString(sectionStyle.Render("── 下载任务 ──"))
	b.WriteString("\n")
	if len(m.snapshots) == 0 {
		b.WriteString(dimStyle.Render("  (暂无下载任务)"))
		b.WriteString("\n")
	} else {
		barWidth := m.width - 40
		if barWidth < 20 {
			barWidth = 20
		}
		for i, s := range m.snapshots {
			b.WriteString(renderDownload(i+1, s, barWidth))
			b.WriteString("\n")
		}
	}

	// 状态行
	b.WriteString("\n")
	statusLine := m.status
	if m.isErr {
		b.WriteString(errStyle.Render("✖ " + statusLine))
	} else {
		b.WriteString(okStyle.Render("• " + statusLine))
	}
	b.WriteString("\n")

	// 输入行
	b.WriteString(m.input.View())
	b.WriteString("\n")

	// 提示
	b.WriteString(hintStyle.Render("快捷: Enter 执行  •  search <kw>  •  数字=下载  •  c <n>=取消  •  clear=清理  •  q=退出"))

	return b.String()
}

// renderDownload 渲染单个下载任务
func renderDownload(idx int, s engine.DownloadSnapshot, barWidth int) string {
	var b strings.Builder

	name := truncate(s.Name, 60)
	header := fmt.Sprintf("  [%d] %s  %s", idx, name, dimStyle.Render("["+s.State.String()+"]"))
	b.WriteString(header)
	b.WriteString("\n")

	switch s.State {
	case engine.StateWaitingMeta:
		b.WriteString(dimStyle.Render("       ⏳ 正在连接 peers、获取元数据..."))
	case engine.StateFailed:
		b.WriteString(errStyle.Render("       ✖ " + s.Err))
	case engine.StateCanceled:
		b.WriteString(dimStyle.Render("       已取消"))
	default:
		// 进度条
		percent := 0.0
		if s.Total > 0 {
			percent = float64(s.Completed) / float64(s.Total) * 100
		}
		bar := renderBar(percent, barWidth)
		eta := "—"
		if s.ETA > 0 {
			eta = utils.FormatDuration(s.ETA)
		}
		if s.State == engine.StateDone {
			eta = "完成"
		}
		b.WriteString(fmt.Sprintf("       %s  %5.1f%%", bar, percent))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("       %s / %s  •  ↓ %s/s  •  peers: %d  •  ETA %s",
			utils.FormatBytes(s.Completed),
			utils.FormatBytes(s.Total),
			utils.FormatBytes(int64(s.Speed)),
			s.Peers,
			eta,
		))
	}
	return b.String()
}

// renderBar 上色进度条
func renderBar(percent float64, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}
	return barDone.Render(strings.Repeat("█", filled)) +
		barTodo.Render(strings.Repeat("░", width-filled))
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
