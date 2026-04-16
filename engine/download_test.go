package engine

import (
	"sync"
	"testing"
	"time"
)

// --- Mock TorrentHandle ---

type mockHandle struct {
	mu        sync.Mutex
	completed int64
	uploaded  int64
	peers     int
	dropped   bool
}

func (m *mockHandle) BytesCompleted() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.completed
}

func (m *mockHandle) ActivePeers() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.peers
}

func (m *mockHandle) BytesUploaded() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.uploaded
}

func (m *mockHandle) Drop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dropped = true
}

func (m *mockHandle) setCompleted(n int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completed = n
}

func (m *mockHandle) setUploaded(n int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.uploaded = n
}

// --- Helper ---

func newTestDownload(h TorrentHandle, policy DownloadPolicy) (*Download, chan Event) {
	events := make(chan Event, 64)
	d := &Download{
		ID:        "test-id",
		Magnet:    "magnet:?xt=urn:btih:abc",
		handle:    h,
		CreatedAt: time.Now(),
		name:      "test-file",
		totalSize: 1000,
		state:     StateDownloading,
		policy:    policy,
		onEvent: func(ev Event) {
			select {
			case events <- ev:
			default:
			}
		},
	}
	return d, events
}

func drainEvent(ch chan Event, timeout time.Duration) (Event, bool) {
	select {
	case ev := <-ch:
		return ev, true
	case <-time.After(timeout):
		return Event{}, false
	}
}

// --- Tests ---

func TestDownload_InitialState(t *testing.T) {
	h := &mockHandle{}
	d, _ := newTestDownload(h, DownloadPolicy{})
	if d.State() != StateDownloading {
		t.Fatalf("expected StateDownloading, got %v", d.State())
	}
}

func TestDownload_Snapshot(t *testing.T) {
	h := &mockHandle{completed: 500, peers: 3, uploaded: 100}
	d, _ := newTestDownload(h, DownloadPolicy{})

	snap := d.Snapshot()
	if snap.ID != "test-id" {
		t.Fatalf("expected ID test-id, got %s", snap.ID)
	}
	if snap.Completed != 500 {
		t.Fatalf("expected completed 500, got %d", snap.Completed)
	}
	if snap.Peers != 3 {
		t.Fatalf("expected 3 peers, got %d", snap.Peers)
	}
	if snap.Total != 1000 {
		t.Fatalf("expected total 1000, got %d", snap.Total)
	}
}

func TestDownload_Cancel(t *testing.T) {
	h := &mockHandle{}
	d, events := newTestDownload(h, DownloadPolicy{})

	d.Cancel()

	if d.State() != StateCanceled {
		t.Fatalf("expected StateCanceled, got %v", d.State())
	}
	if !h.dropped {
		t.Fatal("expected handle Drop() to be called")
	}

	ev, ok := drainEvent(events, time.Second)
	if !ok {
		t.Fatal("expected EventCanceled")
	}
	if ev.Type != EventCanceled {
		t.Fatalf("expected EventCanceled, got %v", ev.Type)
	}
}

func TestDownload_CancelIdempotent(t *testing.T) {
	h := &mockHandle{}
	d, events := newTestDownload(h, DownloadPolicy{})

	d.Cancel()
	d.Cancel() // 二次取消不应 panic

	if d.State() != StateCanceled {
		t.Fatalf("expected StateCanceled, got %v", d.State())
	}
	// 只应有一个事件
	drainEvent(events, 100*time.Millisecond)
	if _, ok := drainEvent(events, 100*time.Millisecond); ok {
		t.Fatal("expected only one cancel event")
	}
}

func TestDownload_SetFailed(t *testing.T) {
	h := &mockHandle{}
	d, events := newTestDownload(h, DownloadPolicy{})

	d.setFailed("timeout")

	if d.State() != StateFailed {
		t.Fatalf("expected StateFailed, got %v", d.State())
	}

	snap := d.Snapshot()
	if snap.Err != "timeout" {
		t.Fatalf("expected err 'timeout', got %q", snap.Err)
	}

	ev, ok := drainEvent(events, time.Second)
	if !ok {
		t.Fatal("expected EventFailed")
	}
	if ev.Type != EventFailed {
		t.Fatalf("expected EventFailed, got %v", ev.Type)
	}
	if ev.Detail != "timeout" {
		t.Fatalf("expected detail 'timeout', got %q", ev.Detail)
	}
}

func TestDownload_WatchLifecycle_NoSeed(t *testing.T) {
	h := &mockHandle{completed: 0}
	d, events := newTestDownload(h, DownloadPolicy{Seed: false})

	go d.watchLifecycle()

	// 模拟下载完成
	time.Sleep(200 * time.Millisecond)
	h.setCompleted(1000)

	ev, ok := drainEvent(events, 3*time.Second)
	if !ok {
		t.Fatal("expected EventDownloadDone")
	}
	if ev.Type != EventDownloadDone {
		t.Fatalf("expected EventDownloadDone, got %v", ev.Type)
	}
	if d.State() != StateDone {
		t.Fatalf("expected StateDone, got %v", d.State())
	}
}

func TestDownload_WatchLifecycle_SeedThenRatioLimit(t *testing.T) {
	h := &mockHandle{completed: 0}
	d, events := newTestDownload(h, DownloadPolicy{
		Seed:           true,
		SeedRatioLimit: 1.0,
	})

	go d.watchLifecycle()

	// 模拟下载完成
	time.Sleep(200 * time.Millisecond)
	h.setCompleted(1000)

	// 应该先收到 EventSeedingStarted
	ev, ok := drainEvent(events, 3*time.Second)
	if !ok {
		t.Fatal("expected EventSeedingStarted")
	}
	if ev.Type != EventSeedingStarted {
		t.Fatalf("expected EventSeedingStarted, got %v", ev.Type)
	}

	// 模拟上传到 ratio >= 1.0
	time.Sleep(200 * time.Millisecond)
	h.setUploaded(1000)

	ev, ok = drainEvent(events, 3*time.Second)
	if !ok {
		t.Fatal("expected EventSeedingStopped")
	}
	if ev.Type != EventSeedingStopped {
		t.Fatalf("expected EventSeedingStopped, got %v", ev.Type)
	}
	if d.State() != StateDone {
		t.Fatalf("expected StateDone, got %v", d.State())
	}
	if !h.dropped {
		t.Fatal("expected handle Drop() after seed done")
	}
}

func TestDownload_WatchLifecycle_SeedTimeLimit(t *testing.T) {
	h := &mockHandle{completed: 0}
	d, events := newTestDownload(h, DownloadPolicy{
		Seed:          true,
		SeedTimeLimit: 1 * time.Second,
	})

	go d.watchLifecycle()

	// 模拟下载完成
	time.Sleep(200 * time.Millisecond)
	h.setCompleted(1000)

	// EventSeedingStarted
	ev, ok := drainEvent(events, 3*time.Second)
	if !ok {
		t.Fatal("expected EventSeedingStarted")
	}
	if ev.Type != EventSeedingStarted {
		t.Fatalf("expected EventSeedingStarted, got %v", ev.Type)
	}

	// 等待做种时间到
	ev, ok = drainEvent(events, 5*time.Second)
	if !ok {
		t.Fatal("expected EventSeedingStopped after time limit")
	}
	if ev.Type != EventSeedingStopped {
		t.Fatalf("expected EventSeedingStopped, got %v", ev.Type)
	}
	if d.State() != StateDone {
		t.Fatalf("expected StateDone, got %v", d.State())
	}
}

func TestDownload_CancelDuringDownload_StopsWatcher(t *testing.T) {
	h := &mockHandle{completed: 0}
	d, events := newTestDownload(h, DownloadPolicy{Seed: false})

	go d.watchLifecycle()

	time.Sleep(200 * time.Millisecond)
	d.Cancel()

	ev, ok := drainEvent(events, time.Second)
	if !ok {
		t.Fatal("expected EventCanceled")
	}
	if ev.Type != EventCanceled {
		t.Fatalf("expected EventCanceled, got %v", ev.Type)
	}

	// watchLifecycle 应自行退出，不再产生事件
	h.setCompleted(1000)
	time.Sleep(2 * time.Second)
	if _, ok := drainEvent(events, 500*time.Millisecond); ok {
		t.Fatal("expected no more events after cancel")
	}
}
