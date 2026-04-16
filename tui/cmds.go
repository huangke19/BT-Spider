package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/huangke/bt-spider/app"
)

// 本文件集中 bubbletea 的 tea.Cmd 工厂，隔离「Model 逻辑」和「异步副作用」。
// 所有 Cmd 都只调用 app.App，不直接依赖 engine / search。

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func searchCmd(a *app.App, keyword string) tea.Cmd {
	return func() tea.Msg {
		results, err := a.Search(keyword)
		return searchDoneMsg{keyword: keyword, results: results, err: err}
	}
}

func addMagnetCmd(a *app.App, magnet, name string) tea.Cmd {
	return func() tea.Msg {
		if err := a.AddMagnet(magnet); err != nil {
			return statusMsg{text: "添加下载失败: " + err.Error(), isErr: true}
		}
		hint := name
		if hint == "" {
			hint = "新任务"
		}
		return statusMsg{text: "已加入下载队列: " + hint, isErr: false}
	}
}

func resolveCmd(a *app.App, raw string) tea.Cmd {
	return func() tea.Msg {
		resolved, ok := a.ResolveNLP(raw)
		return resolveDoneMsg{resolved: resolved, ok: ok, original: raw}
	}
}

func statusCmd(text string, isErr bool) tea.Cmd {
	return func() tea.Msg {
		return statusMsg{text: text, isErr: isErr}
	}
}
