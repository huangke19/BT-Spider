package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/huangke/bt-spider/app"
	"github.com/huangke/bt-spider/pkg/utils"
)

const version = "0.6.0"

// --- 消息类型 ---

type tickMsg time.Time

type searchDoneMsg struct {
	keyword string
	results []app.SearchResult
	err     error
}

type resolveDoneMsg struct {
	resolved app.MovieResolution
	ok       bool
	original string
}

type statusMsg struct {
	text  string
	isErr bool
}

type engineEventMsg struct {
	event app.EngineEvent
}

type searchStreamStartMsg struct {
	ch         <-chan app.SearchUpdate
	cancel     context.CancelFunc
	keyword    string
	generation int
}

type searchStreamUpdateMsg struct {
	update     app.SearchUpdate
	generation int
}

type searchStreamDoneMsg struct {
	generation int
}

type sizeResolvedMsg struct {
	index int
	size  string
}

// --- 样式 ---

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("230"))

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			MarginTop(1)

	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	errStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	okStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	infoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("110"))

	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			MarginTop(1)

	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229"))

	selectedMarkerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))

	barDone = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	barTodo = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	seedHotStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("48"))
	seedHighStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	seedMediumStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))
	seedLowStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("179"))

	chromeStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("117"))

	metaLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	valueStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229"))

	accentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("81"))

	statusBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 1)
)

// --- Model ---

type Model struct {
	app     *app.App
	input   textinput.Model
	results []app.SearchResult
	status  string
	isErr   bool
	width   int
	height  int
	doctor  app.DoctorReport
	// 环境自检详情面板
	showDoctor bool
	// 当前搜索结果选中项
	selectedResult int
	// 最近一次拉到的下载快照（View 期间复用）
	snapshots []app.DownloadSnapshot

	// 流式搜索
	searchCancel     context.CancelFunc      // 当前搜索的 cancel
	searchGeneration int                     // 每次新搜索 +1，防止旧 update 污染
	currentSearchCh  <-chan app.SearchUpdate // 当前搜索的 channel
	resolvingSize    map[int]bool            // 正在 resolving 的结果索引
	currentKeyword   string
	currentProvider  string
	searchInFlight   bool
	lastResolved     string
}

// New 创建一个初始化好的 Model
func New(a *app.App) Model {
	ti := textinput.New()
	ti.Placeholder = "直接输入片名 / search <关键词> / doctor / <序号> / magnet:... / c <序号> / q"
	ti.Prompt = "bt> "
	ti.CharLimit = 4096
	ti.Focus()

	report := a.DoctorReport()

	return Model{
		app:           a,
		input:         ti,
		status:        "可输入关键词开始搜索，doctor 查看启动自检详情",
		doctor:        report,
		resolvingSize: map[int]bool{},
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tickCmd(), eventCmd(m.app))
}

// --- Update ---

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 6

	case tickMsg:
		m.snapshots = m.app.ListDownloads()
		return m, tickCmd()

	case engineEventMsg:
		m.snapshots = m.app.ListDownloads()
		ev := msg.event
		switch ev.Type {
		case app.EventFailed:
			m.status = ev.String()
			m.isErr = true
		case app.EventCanceled:
			// 取消事件由用户触发，不覆盖已有 status
		default:
			m.status = ev.String()
			m.isErr = false
		}
		return m, eventCmd(m.app)

	case resolveDoneMsg:
		if !msg.ok {
			m.status = fmt.Sprintf("无法识别「%s」，请尝试 search <关键词>", msg.original)
			m.isErr = true
			return m, nil
		}
		m.lastResolved = msg.resolved.Display
		if equalKeyword(msg.resolved.Query, msg.original) {
			// NLP 解析结果与原词相同，不重新搜索（原词搜索已在途中）
			return m, nil
		}
		// 切换到精确查询
		if m.searchCancel != nil {
			m.searchCancel()
		}
		m.searchGeneration++
		m.status = "切换到精确查询: " + msg.resolved.Display
		m.isErr = false
		return m, searchStreamStartCmd(m.app, msg.resolved.Query, m.searchGeneration)

	case searchDoneMsg:
		m.searchInFlight = false
		if msg.err != nil {
			m.results = nil
			m.selectedResult = 0
			m.status = "搜索失败: " + msg.err.Error()
			m.isErr = true
			return m, nil
		}
		m.results = msg.results
		m.ensureSelectedResult()
		if len(msg.results) == 0 {
			m.status = fmt.Sprintf("「%s」没有找到有做种的结果", msg.keyword)
			m.isErr = true
		} else {
			m.status = fmt.Sprintf("找到 %d 个结果，输入序号下载", len(msg.results))
			m.isErr = false
		}
		return m, nil

	case searchStreamStartMsg:
		if m.searchCancel != nil {
			m.searchCancel() // 取消旧搜索
		}
		m.searchCancel = msg.cancel
		m.currentSearchCh = msg.ch
		m.currentKeyword = msg.keyword
		m.currentProvider = ""
		m.searchInFlight = true
		m.selectedResult = 0
		m.status = "搜索中（流式）..."
		m.isErr = false
		return m, drainStreamCmd(msg.ch, msg.generation)

	case searchStreamUpdateMsg:
		if msg.generation != m.searchGeneration {
			return m, nil // 旧的 update 丢弃
		}
		u := msg.update
		if u.Err != nil {
			m.currentProvider = u.Provider
			m.status = "provider 错误: " + u.Provider + " " + u.Err.Error()
			m.isErr = true
		} else {
			m.results = u.Results
			m.ensureSelectedResult()
			m.resolvingSize = map[int]bool{}
			if len(u.Results) > 0 && u.Provider != "" && !u.Done {
				m.currentProvider = u.Provider
				m.status = "来自 " + u.Provider + "（" + strconv.Itoa(len(u.Results)) + "）"
				m.isErr = false
			}
		}
		if u.Done {
			m.searchCancel = nil
			m.searchInFlight = false
			if len(m.results) == 0 {
				m.status = "搜索完成，但没有找到可下载结果"
				m.isErr = true
			} else {
				m.status = "搜索完成，共 " + strconv.Itoa(len(m.results)) + " 条"
				m.isErr = false
			}
			return m, nil
		}
		return m, drainStreamCmd(m.currentSearchCh, msg.generation)

	case searchStreamDoneMsg:
		if msg.generation == m.searchGeneration {
			m.searchCancel = nil
			m.searchInFlight = false
		}
		return m, nil

	case sizeResolvedMsg:
		if msg.index >= 0 && msg.index < len(m.results) {
			m.results[msg.index].Size = msg.size
		}
		delete(m.resolvingSize, msg.index)
		return m, nil

	case statusMsg:
		m.status = msg.text
		m.isErr = msg.isErr
		return m, nil

	case tea.KeyMsg:
		if strings.TrimSpace(m.input.Value()) == "" && len(m.results) > 0 {
			switch msg.Type {
			case tea.KeyUp:
				m.moveSelectedResult(-1)
				return m, nil
			case tea.KeyDown:
				m.moveSelectedResult(1)
				return m, nil
			case tea.KeyPgUp:
				m.moveSelectedResult(-10)
				return m, nil
			case tea.KeyPgDown:
				m.moveSelectedResult(10)
				return m, nil
			case tea.KeyHome:
				m.selectedResult = 0
				return m, nil
			case tea.KeyEnd:
				m.selectedResult = len(m.results) - 1
				return m, nil
			}
		}
		switch msg.Type {
		case tea.KeyEsc:
			if m.showDoctor {
				m.showDoctor = false
				m.status = "已隐藏环境自检详情"
				m.isErr = false
				return m, nil
			}
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
		return m.downloadSelectedResult()
	}
	m.input.SetValue("")

	lower := strings.ToLower(raw)

	switch {
	case lower == "q" || lower == "quit" || lower == "exit":
		return m, tea.Quit

	case lower == "doctor":
		m.doctor = m.app.DoctorReport()
		m.showDoctor = true
		return m, statusCmd(m.doctor.Summary(), false)

	case lower == "doctor hide" || lower == "doctor close":
		m.showDoctor = false
		return m, statusCmd("已隐藏环境自检详情", false)

	case lower == "clear":
		n := m.app.ClearFinished()
		return m, statusCmd(fmt.Sprintf("已清理 %d 个已结束任务", n), false)

	case strings.HasPrefix(lower, "search "):
		keyword := strings.TrimSpace(raw[7:])
		if keyword == "" {
			return m, statusCmd("请输入搜索关键词", true)
		}
		m.currentKeyword = keyword
		m.lastResolved = ""
		m.status = fmt.Sprintf("搜索中: %s ...", keyword)
		m.isErr = false
		m.searchGeneration++
		return m, searchStreamStartCmd(m.app, keyword, m.searchGeneration)

	case strings.HasPrefix(lower, "movie "):
		query := strings.TrimSpace(raw[6:])
		if query == "" {
			return m, statusCmd("请输入电影名称", true)
		}
		if resolved, ok := m.app.ResolveLocal(query); ok {
			m.currentKeyword = resolved.Query
			m.lastResolved = resolved.Display
			m.status = resolved.Display + " ..."
			m.isErr = false
			m.searchGeneration++
			return m, searchStreamStartCmd(m.app, resolved.Query, m.searchGeneration)
		}
		return m, statusCmd("无法识别电影名，试试：movie Inception 2010 1080P", true)

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
		if m.app.CancelDownload(id) {
			return m, statusCmd(fmt.Sprintf("已取消任务 #%d", num), false)
		}
		return m, statusCmd("取消失败", true)

	case strings.HasPrefix(raw, "magnet:"):
		return m, addMagnetCmd(m.app, raw, "")

	default:
		// 尝试解析为序号
		num, err := strconv.Atoi(raw)
		if err != nil {
			// 先试本地快速解析（无网络延迟）
			if resolved, ok := m.app.ResolveLocal(raw); ok {
				m.currentKeyword = resolved.Query
				m.lastResolved = resolved.Display
				m.status = resolved.Display + " ..."
				m.isErr = false
				m.searchGeneration++
				return m, searchStreamStartCmd(m.app, resolved.Query, m.searchGeneration)
			}
			// 本地不认识 → 先用原词发起流式搜索（投机搜索，决策 D2），同时走 NLP
			m.currentKeyword = raw
			m.status = "正在识别: " + raw + " ..."
			m.isErr = false
			m.searchGeneration++
			originalGen := m.searchGeneration
			return m, tea.Batch(
				searchStreamStartCmd(m.app, raw, originalGen),
				resolveCmd(m.app, raw),
			)
		}
		if num < 1 || num > len(m.results) {
			return m, statusCmd("搜索结果序号超出范围", true)
		}
		m.selectedResult = num - 1
		return m.downloadResultAt(num - 1)
	}
}

// --- View ---

func (m Model) View() string {
	if m.width <= 0 {
		m.width = 120
	}
	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	if m.showDoctor {
		b.WriteString(renderDoctorPanel(m.doctor))
		b.WriteString("\n")
	}

	b.WriteString(m.renderWorkspace())
	b.WriteString("\n")
	b.WriteString(m.renderDownloadsPanel())

	// 状态行
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")

	// 输入行
	b.WriteString(m.input.View())
	b.WriteString("\n")

	// 提示
	b.WriteString(hintStyle.Render("快捷: ↑/↓/PgUp/PgDn 选结果  •  Enter 下载选中  •  search/movie 直接搜索  •  c <n> 取消  •  clear 清理结束任务  •  doctor 面板  •  q 退出"))

	return b.String()
}

func (m Model) renderHeader() string {
	var body strings.Builder
	body.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render("BT-Spider"),
		dimStyle.Render("  v"+version),
	))
	body.WriteString("\n")
	body.WriteString(metaLabelStyle.Render("下载目录  "))
	body.WriteString(dimStyle.Render(m.app.DownloadDir()))
	body.WriteString("\n")
	body.WriteString(renderDoctorSummary(m.doctor))
	return chromeStyle.Width(maxInt(40, m.width-2)).Render(body.String())
}

func (m Model) renderWorkspace() string {
	if m.width >= 120 {
		leftWidth := int(float64(m.width) * 0.62)
		if leftWidth < 68 {
			leftWidth = 68
		}
		rightWidth := m.width - leftWidth - 1
		if rightWidth < 32 {
			rightWidth = 32
			leftWidth = m.width - rightWidth - 1
		}
		left := m.renderResultsPanel(leftWidth)
		right := lipgloss.JoinVertical(lipgloss.Left,
			m.renderInspectorPanel(rightWidth),
			m.renderSearchOverviewPanel(rightWidth),
		)
		return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderSearchOverviewPanel(m.width),
		m.renderResultsPanel(m.width),
		m.renderInspectorPanel(m.width),
	)
}

func (m Model) renderResultsPanel(width int) string {
	var body strings.Builder
	body.WriteString(panelTitleStyle.Render("搜索结果"))
	body.WriteString("\n")
	body.WriteString(dimStyle.Render(m.resultsSummary()))
	body.WriteString("\n\n")
	if len(m.results) == 0 {
		body.WriteString(dimStyle.Render("尚未搜索。直接输入片名、关键词、`search <关键词>` 或 `movie <片名>` 即可开始。"))
		return panelStyle.Width(width).Render(body.String())
	}

	limit := 10
	if len(m.results) < limit {
		limit = len(m.results)
	}
	start := resultWindowStart(m.selectedResult, len(m.results), limit)
	end := start + limit
	if end > len(m.results) {
		end = len(m.results)
	}
	sizeColWidth, seedColWidth, sourceColWidth := resultColumnWidths(m.results)
	contentWidth := width - panelStyle.GetHorizontalFrameSize()
	if contentWidth < 24 {
		contentWidth = 24
	}
	rowWidth := contentWidth - 2 // 左侧给选中标记留位置
	if rowWidth < 22 {
		rowWidth = 22
	}
	prefixColWidth := maxInt(runewidth.StringWidth(fmt.Sprintf("[%d]", len(m.results))), runewidth.StringWidth("[00]"))
	fixedWidth := prefixColWidth + sizeColWidth + seedColWidth + sourceColWidth + 7
	titleColWidth := rowWidth - fixedWidth
	if titleColWidth < 12 {
		titleColWidth = 12
	}

	headerPrefix := padLeftDisplay("#", prefixColWidth)
	headerTitle := padDisplay("标题", titleColWidth)
	headerSize := padLeftDisplay("大小", sizeColWidth)
	headerSeed := padDisplay("热度", seedColWidth)
	headerSource := padDisplay("来源", sourceColWidth)
	body.WriteString(dimStyle.Render("  "))
	body.WriteString(dimStyle.Render(headerPrefix))
	body.WriteString(dimStyle.Render(" "))
	body.WriteString(dimStyle.Render(headerTitle))
	body.WriteString(dimStyle.Render("  "))
	body.WriteString(dimStyle.Render(headerSize))
	body.WriteString(dimStyle.Render("  "))
	body.WriteString(dimStyle.Render(headerSeed))
	body.WriteString(dimStyle.Render("  "))
	body.WriteString(dimStyle.Render(headerSource))
	body.WriteString("\n")
	body.WriteString(dimStyle.Render("  " + strings.Repeat("─", maxInt(16, rowWidth))))
	body.WriteString("\n")

	for i := start; i < end; i++ {
		r := m.results[i]
		prefixText := padLeftDisplay(fmt.Sprintf("[%d]", i+1), prefixColWidth)
		titleText := padDisplay(truncate(r.Name, titleColWidth), titleColWidth)
		sizeText := padLeftDisplay(r.Size, sizeColWidth)
		seedText := renderSeederCell(r.Seeders, r.Leechers, seedColWidth)
		sourceText := dimStyle.Render(padDisplay(shortSourceLabel(r.Source), sourceColWidth))

		if i == m.selectedResult {
			body.WriteString(selectedMarkerStyle.Render("› "))
			body.WriteString(selectedStyle.Render(prefixText))
			body.WriteString(" ")
			body.WriteString(selectedStyle.Render(titleText))
		} else {
			body.WriteString("  ")
			body.WriteString(dimStyle.Render(prefixText))
			body.WriteString(" ")
			body.WriteString(titleText)
		}
		body.WriteString("  ")
		body.WriteString(sizeText)
		body.WriteString("  ")
		body.WriteString(seedText)
		body.WriteString("  ")
		body.WriteString(sourceText)
		body.WriteString("\n")
	}
	if len(m.results) > limit {
		body.WriteString("\n")
		body.WriteString(dimStyle.Render(fmt.Sprintf("显示 %d-%d / %d", start+1, end, len(m.results))))
	}
	return panelStyle.Width(width).Render(body.String())
}

func (m Model) renderInspectorPanel(width int) string {
	var body strings.Builder
	body.WriteString(panelTitleStyle.Render("当前选中"))
	body.WriteString("\n")
	if len(m.results) == 0 {
		body.WriteString(dimStyle.Render("还没有可选资源。搜索完成后，这里会显示当前条目的来源、体积和下载提示。"))
		return panelStyle.Width(width).Render(body.String())
	}
	m.ensureSelectedResult()
	r := m.results[m.selectedResult]
	lines := []string{
		fmt.Sprintf("%s %s", metaLabelStyle.Render("条目"), valueStyle.Render(fmt.Sprintf("#%d", m.selectedResult+1))),
		fmt.Sprintf("%s %s", metaLabelStyle.Render("标题"), truncate(r.Name, maxInt(16, width-8))),
		fmt.Sprintf("%s %s", metaLabelStyle.Render("来源"), r.Source),
		fmt.Sprintf("%s %s", metaLabelStyle.Render("大小"), r.Size),
	}
	if r.Seeders >= 0 {
		lines = append(lines, fmt.Sprintf("%s %d / %d", metaLabelStyle.Render("做种/下载"), r.Seeders, r.Leechers))
	}
	if r.Size == "未知" && m.resolvingSize[m.selectedResult] {
		lines = append(lines, warnStyle.Render("大小补全中，下载后会按需拉取元数据"))
	}
	lines = append(lines, dimStyle.Render("空输入按 Enter 直接下载当前选中项"))
	body.WriteString(strings.Join(lines, "\n"))
	return panelStyle.Width(width).Render(body.String())
}

func (m Model) renderSearchOverviewPanel(width int) string {
	var body strings.Builder
	body.WriteString(panelTitleStyle.Render("搜索概览"))
	body.WriteString("\n")
	keyword := "尚未发起搜索"
	if strings.TrimSpace(m.currentKeyword) != "" {
		keyword = m.currentKeyword
	}
	state := "待命"
	switch {
	case m.searchInFlight:
		state = accentStyle.Render("流式搜索中")
	case len(m.results) > 0:
		state = okStyle.Render("结果已就绪")
	case strings.TrimSpace(m.currentKeyword) != "":
		state = dimStyle.Render("已结束，无结果")
	}
	provider := "—"
	if strings.TrimSpace(m.currentProvider) != "" {
		provider = m.currentProvider
	}
	body.WriteString(fmt.Sprintf("%s %s\n", metaLabelStyle.Render("关键词"), truncate(keyword, maxInt(16, width-8))))
	body.WriteString(fmt.Sprintf("%s %s\n", metaLabelStyle.Render("状态"), state))
	body.WriteString(fmt.Sprintf("%s %s\n", metaLabelStyle.Render("最近来源"), provider))
	body.WriteString(fmt.Sprintf("%s %s\n", metaLabelStyle.Render("候选数"), valueStyle.Render(strconv.Itoa(len(m.results)))))
	if strings.TrimSpace(m.lastResolved) != "" {
		body.WriteString(fmt.Sprintf("%s %s\n", metaLabelStyle.Render("识别结果"), truncate(m.lastResolved, maxInt(16, width-10))))
	}
	body.WriteString(fmt.Sprintf("%s %s", metaLabelStyle.Render("下载任务"), renderDownloadCounts(m.snapshots)))
	return panelStyle.Width(width).Render(strings.TrimRight(body.String(), "\n"))
}

func (m Model) renderDownloadsPanel() string {
	width := m.width
	if width <= 0 {
		width = 120
	}
	var body strings.Builder
	body.WriteString(panelTitleStyle.Render("下载任务"))
	body.WriteString("\n")
	body.WriteString(dimStyle.Render(renderDownloadHeadline(m.snapshots)))
	body.WriteString("\n\n")
	if len(m.snapshots) == 0 {
		body.WriteString(dimStyle.Render("暂无下载任务。选中搜索结果后按 Enter，即可把磁力加入下载队列。"))
		return panelStyle.Width(width).Render(body.String())
	}
	barWidth := width - 34
	if barWidth < 16 {
		barWidth = 16
	}
	for i, s := range m.snapshots {
		if i > 0 {
			body.WriteString("\n")
		}
		body.WriteString(renderDownload(i+1, s, barWidth))
	}
	return panelStyle.Width(width).Render(body.String())
}

func (m Model) renderStatusBar() string {
	line := m.status
	if line == "" {
		line = "准备就绪"
	}
	if m.isErr {
		return statusBoxStyle.Width(maxInt(40, m.width-2)).Render(errStyle.Render("✖ " + line))
	}
	return statusBoxStyle.Width(maxInt(40, m.width-2)).Render(okStyle.Render("• " + line))
}

// renderDownload 渲染单个下载任务
func renderDownload(idx int, s app.DownloadSnapshot, barWidth int) string {
	var b strings.Builder

	name := truncate(s.Name, 60)
	header := fmt.Sprintf("  [%d] %s  %s", idx, name, dimStyle.Render("["+s.State.String()+"]"))
	b.WriteString(header)
	b.WriteString("\n")

	switch s.State {
	case app.StateWaitingMeta:
		b.WriteString(dimStyle.Render("       ⏳ 正在连接 peers、获取元数据..."))
	case app.StateFailed:
		b.WriteString(errStyle.Render("       ✖ " + s.Err))
	case app.StateCanceled:
		b.WriteString(dimStyle.Render("       已取消"))
	case app.StateSeeding:
		b.WriteString(fmt.Sprintf("       已完成下载，正在做种  •  ↑ %s  •  ratio %.2f  •  peers: %d  •  已保种 %s",
			utils.FormatBytes(s.Uploaded),
			s.ShareRatio,
			s.Peers,
			utils.FormatDuration(s.SeedElapsed),
		))
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
		if s.State == app.StateDone {
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

func (m *Model) ensureSelectedResult() {
	if len(m.results) == 0 {
		m.selectedResult = 0
		return
	}
	if m.selectedResult < 0 {
		m.selectedResult = 0
	}
	if m.selectedResult >= len(m.results) {
		m.selectedResult = len(m.results) - 1
	}
}

func (m *Model) moveSelectedResult(delta int) {
	if len(m.results) == 0 {
		m.selectedResult = 0
		return
	}
	m.selectedResult += delta
	if m.selectedResult < 0 {
		m.selectedResult = 0
	}
	if m.selectedResult >= len(m.results) {
		m.selectedResult = len(m.results) - 1
	}
}

func (m Model) downloadSelectedResult() (tea.Model, tea.Cmd) {
	if len(m.results) == 0 {
		return m, nil
	}
	m.ensureSelectedResult()
	return m.downloadResultAt(m.selectedResult)
}

func (m Model) downloadResultAt(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.results) {
		return m, statusCmd("搜索结果序号超出范围", true)
	}
	r := m.results[idx]

	// 按需补全 size（决策 D4）
	var resolveSize tea.Cmd
	if r.Size == "未知" && r.Magnet != "" && !m.resolvingSize[idx] {
		m.resolvingSize[idx] = true
		resolveSize = resolveSizeCmd(m.app, idx, r.Magnet)
	}
	download := addMagnetCmd(m.app, r.Magnet, r.Name)
	if resolveSize != nil {
		return m, tea.Batch(download, resolveSize)
	}
	return m, download
}

func resultWindowStart(selected, total, limit int) int {
	if total <= limit || limit <= 0 {
		return 0
	}
	start := selected - limit/2
	if start < 0 {
		start = 0
	}
	maxStart := total - limit
	if start > maxStart {
		start = maxStart
	}
	return start
}

func renderDoctorSummary(report app.DoctorReport) string {
	ok, info, warn, err := report.Counts()
	switch {
	case err > 0:
		return errStyle.Render(fmt.Sprintf("自检: %d 正常 / %d 提示 / %d 警告 / %d 错误  ·  输入 doctor 查看详情", ok, info, warn, err))
	case warn > 0:
		return warnStyle.Render(fmt.Sprintf("自检: %d 正常 / %d 提示 / %d 警告  ·  输入 doctor 查看详情", ok, info, warn))
	default:
		return infoStyle.Render(fmt.Sprintf("自检: %d 正常 / %d 提示  ·  输入 doctor 查看详情", ok, info))
	}
}

func renderDoctorPanel(report app.DoctorReport) string {
	var b strings.Builder
	b.WriteString(panelTitleStyle.Render("环境检查"))
	b.WriteString("\n")
	for _, check := range report.Checks {
		b.WriteString(fmt.Sprintf("  %s %-8s %s", doctorBadge(check.Status), check.Name, check.Detail))
		b.WriteString("\n")
	}
	b.WriteString(dimStyle.Render("  输入 doctor hide 或按 Esc 可隐藏详情"))
	return panelStyle.Render(b.String())
}

func doctorBadge(status app.DoctorStatus) string {
	switch status {
	case app.DoctorOK:
		return okStyle.Render("OK")
	case app.DoctorInfo:
		return infoStyle.Render("INFO")
	case app.DoctorWarn:
		return warnStyle.Render("WARN")
	case app.DoctorError:
		return errStyle.Render("ERR")
	default:
		return dimStyle.Render("N/A")
	}
}

// truncate 按显示列宽截断字符串（中文等宽字符占 2 列）
func truncate(s string, maxCols int) string {
	if runewidth.StringWidth(s) <= maxCols {
		return s
	}
	if maxCols <= 3 {
		w := 0
		for i, r := range s {
			rw := runewidth.RuneWidth(r)
			if w+rw > maxCols {
				return s[:i]
			}
			w += rw
		}
		return s
	}
	target := maxCols - 3
	w := 0
	var result []rune
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if w+rw > target {
			break
		}
		result = append(result, r)
		w += rw
	}
	return string(result) + "..."
}

func padDisplay(s string, width int) string {
	if runewidth.StringWidth(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-runewidth.StringWidth(s))
}

func padLeftDisplay(s string, width int) string {
	if runewidth.StringWidth(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-runewidth.StringWidth(s)) + s
}

// equalKeyword 比较两个关键词是否等同（忽略大小写和首尾空格）。
func equalKeyword(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func (m Model) resultsSummary() string {
	if strings.TrimSpace(m.currentKeyword) == "" && len(m.results) == 0 {
		return "等待输入"
	}
	if m.searchInFlight {
		return fmt.Sprintf("正在汇聚关键词“%s”的流式结果", m.currentKeyword)
	}
	if len(m.results) == 0 {
		return fmt.Sprintf("“%s” 没有返回可下载条目", m.currentKeyword)
	}
	return fmt.Sprintf("关键词“%s”共返回 %d 条候选", m.currentKeyword, len(m.results))
}

func renderDownloadHeadline(snaps []app.DownloadSnapshot) string {
	if len(snaps) == 0 {
		return "当前没有活跃任务"
	}
	return "任务分布: " + renderDownloadCounts(snaps)
}

func renderDownloadCounts(snaps []app.DownloadSnapshot) string {
	waiting, downloading, seeding, done, failed, canceled := countDownloads(snaps)
	parts := []string{
		fmt.Sprintf("等待 %d", waiting),
		fmt.Sprintf("下载 %d", downloading),
		fmt.Sprintf("做种 %d", seeding),
	}
	if done > 0 {
		parts = append(parts, fmt.Sprintf("完成 %d", done))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("失败 %d", failed))
	}
	if canceled > 0 {
		parts = append(parts, fmt.Sprintf("取消 %d", canceled))
	}
	return strings.Join(parts, "  •  ")
}

func countDownloads(snaps []app.DownloadSnapshot) (waiting, downloading, seeding, done, failed, canceled int) {
	for _, s := range snaps {
		switch s.State {
		case app.StateWaitingMeta:
			waiting++
		case app.StateDownloading:
			downloading++
		case app.StateSeeding:
			seeding++
		case app.StateDone:
			done++
		case app.StateFailed:
			failed++
		case app.StateCanceled:
			canceled++
		}
	}
	return
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func formatSeeders(seeders, leechers int) string {
	if seeders < 0 {
		return "↑— ↓—"
	}
	return fmt.Sprintf("↑%d ↓%d", seeders, leechers)
}

func resultColumnWidths(results []app.SearchResult) (sizeColWidth, seedColWidth, sourceColWidth int) {
	sizeColWidth = runewidth.StringWidth("00.00 GB")
	seedColWidth = runewidth.StringWidth("↑0000 ↓0000")
	sourceColWidth = runewidth.StringWidth("PirateBay")

	for _, r := range results {
		sizeColWidth = maxInt(sizeColWidth, runewidth.StringWidth(r.Size))
		seedColWidth = maxInt(seedColWidth, runewidth.StringWidth(formatSeeders(r.Seeders, r.Leechers)))
		sourceColWidth = maxInt(sourceColWidth, runewidth.StringWidth(shortSourceLabel(r.Source)))
	}

	return sizeColWidth, seedColWidth, sourceColWidth
}

func shortSourceLabel(source string) string {
	switch source {
	case "ThePirateBay":
		return "PirateBay"
	case "TorrentKitty":
		return "Kitty"
	default:
		return source
	}
}

func renderSeederCell(seeders, leechers, width int) string {
	text := padDisplay(formatSeeders(seeders, leechers), width)
	switch {
	case seeders >= 100:
		return seedHotStyle.Render(text)
	case seeders >= 40:
		return seedHighStyle.Render(text)
	case seeders >= 10:
		return seedMediumStyle.Render(text)
	case seeders > 0:
		return seedLowStyle.Render(text)
	default:
		return dimStyle.Render(text)
	}
}
