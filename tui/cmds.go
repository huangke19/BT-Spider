package tui

import (
	"context"
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

func eventCmd(a *app.App) tea.Cmd {
	return func() tea.Msg {
		ev, ok := a.WaitEvent()
		if !ok {
			return nil
		}
		return engineEventMsg{event: ev}
	}
}

// searchStreamStartCmd 发起一次流式搜索；返回的 msg 里携带 channel 和 cancel。
// Model 收到后应立即调用 drainStreamCmd 消费。
func searchStreamStartCmd(a *app.App, keyword string, generation int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		return searchStreamStartMsg{
			ch:         a.SearchStream(ctx, keyword, 0),
			cancel:     cancel,
			keyword:    keyword,
			generation: generation,
		}
	}
}

// drainStreamCmd 从 channel 拿一条 update。Model 收到后再调一次直到 Done。
func drainStreamCmd(ch <-chan app.SearchUpdate, generation int) tea.Cmd {
	return func() tea.Msg {
		upd, ok := <-ch
		if !ok {
			return searchStreamDoneMsg{generation: generation}
		}
		return searchStreamUpdateMsg{update: upd, generation: generation}
	}
}

// resolveSizeCmd 对单条结果的 size 按需补全。
func resolveSizeCmd(a *app.App, index int, magnet string) tea.Cmd {
	return func() tea.Msg {
		size := a.ResolveSizeOne(magnet, 4*time.Second)
		if size == "" {
			size = "未知"
		}
		return sizeResolvedMsg{index: index, size: size}
	}
}
