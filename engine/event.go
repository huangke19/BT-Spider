package engine

import "fmt"

// EventType 下载状态变更事件类型
type EventType int

const (
	EventMetaReceived   EventType = iota + 1 // 元数据获取完成，开始下载
	EventDownloadDone                        // 下载完成（不做种 或 全部文件就绪）
	EventSeedingStarted                      // 进入做种状态
	EventSeedingStopped                      // 做种结束（达到分享率或时间限制）
	EventFailed                              // 下载失败
	EventCanceled                            // 用户取消
)

func (t EventType) String() string {
	switch t {
	case EventMetaReceived:
		return "元数据就绪"
	case EventDownloadDone:
		return "下载完成"
	case EventSeedingStarted:
		return "开始做种"
	case EventSeedingStopped:
		return "做种结束"
	case EventFailed:
		return "下载失败"
	case EventCanceled:
		return "已取消"
	}
	return "未知事件"
}

// Event 下载引擎发出的离散状态变更事件。
type Event struct {
	Type       EventType
	DownloadID string
	Name       string
	Detail     string // 可选：补充说明
}

func (e Event) String() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Name, e.Type, e.Detail)
	}
	return fmt.Sprintf("[%s] %s", e.Name, e.Type)
}
