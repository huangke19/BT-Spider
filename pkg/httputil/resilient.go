package httputil

import (
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// --- HTTP Error ---

// HTTPError 表示非 2xx 的 HTTP 响应。
type HTTPError struct {
	StatusCode int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d", e.StatusCode)
}

// IsClientError 判断是否为 4xx 客户端错误（不应重试）。
func IsClientError(err error) bool {
	if he, ok := err.(*HTTPError); ok {
		return he.StatusCode >= 400 && he.StatusCode < 500
	}
	return false
}

// --- Circuit Breaker ---

type breakerState int

const (
	breakerClosed   breakerState = iota // 正常放行
	breakerOpen                         // 熔断，拒绝请求
	breakerHalfOpen                     // 试探性放行一次
)

type hostBreaker struct {
	mu           sync.Mutex
	state        breakerState
	failures     int
	maxFailures  int
	lastFailure  time.Time
	resetTimeout time.Duration
}

func (b *hostBreaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case breakerClosed:
		return true
	case breakerOpen:
		if time.Since(b.lastFailure) > b.resetTimeout {
			b.state = breakerHalfOpen
			return true
		}
		return false
	case breakerHalfOpen:
		return true
	}
	return true
}

func (b *hostBreaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.state = breakerClosed
}

func (b *hostBreaker) recordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	b.lastFailure = time.Now()
	if b.failures >= b.maxFailures {
		b.state = breakerOpen
	}
}

// --- Resilient Client ---

// ResilientClient 封装 HTTP 请求 + 自动重试 + 熔断器。
type ResilientClient struct {
	client       *http.Client
	ua           string
	maxBody      int64
	maxRetries   int
	baseDelay    time.Duration
	maxFailures  int
	resetTimeout time.Duration

	mu       sync.Mutex
	breakers map[string]*hostBreaker
}

// Option 配置 ResilientClient 的选项。
type Option func(*ResilientClient)

func WithTimeout(d time.Duration) Option {
	return func(c *ResilientClient) { c.client.Timeout = d }
}

func WithUA(ua string) Option {
	return func(c *ResilientClient) { c.ua = ua }
}

func WithMaxBody(n int64) Option {
	return func(c *ResilientClient) { c.maxBody = n }
}

func WithRetry(maxRetries int, baseDelay time.Duration) Option {
	return func(c *ResilientClient) {
		c.maxRetries = maxRetries
		c.baseDelay = baseDelay
	}
}

func WithCircuitBreaker(maxFailures int, resetTimeout time.Duration) Option {
	return func(c *ResilientClient) {
		c.maxFailures = maxFailures
		c.resetTimeout = resetTimeout
	}
}

// NewResilientClient 创建带重试和熔断的 HTTP 客户端。
// 默认：2 次重试、500ms 退避、5 次连续失败触发熔断、30s 后半开。
func NewResilientClient(opts ...Option) *ResilientClient {
	c := &ResilientClient{
		client: &http.Client{
			Timeout:   DefaultTimeout,
			Transport: SharedTransport(),
		},
		ua:           DefaultUA,
		maxBody:      2 << 20, // 2 MB
		maxRetries:   2,
		baseDelay:    500 * time.Millisecond,
		maxFailures:  5,
		resetTimeout: 30 * time.Second,
		breakers:     make(map[string]*hostBreaker),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *ResilientClient) getBreaker(rawURL string) *hostBreaker {
	u, err := url.Parse(rawURL)
	if err != nil {
		return &hostBreaker{maxFailures: c.maxFailures, resetTimeout: c.resetTimeout}
	}
	host := u.Host

	c.mu.Lock()
	defer c.mu.Unlock()
	b, ok := c.breakers[host]
	if !ok {
		b = &hostBreaker{maxFailures: c.maxFailures, resetTimeout: c.resetTimeout}
		c.breakers[host] = b
	}
	return b
}

// Get 执行 GET 请求，自动设置 UA、重试、熔断。
func (c *ResilientClient) Get(rawURL string) ([]byte, error) {
	return c.GetWithHeaders(rawURL, nil)
}

// GetWithHeaders 执行带自定义头的 GET 请求。
func (c *ResilientClient) GetWithHeaders(rawURL string, headers map[string]string) ([]byte, error) {
	breaker := c.getBreaker(rawURL)

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.baseDelay * time.Duration(1<<(attempt-1))
			jitter := time.Duration(rand.Int64N(int64(delay/4) + 1))
			time.Sleep(delay + jitter)
		}

		if !breaker.allow() {
			return nil, fmt.Errorf("熔断器已开启: %s", rawURL)
		}

		body, err := c.doGet(rawURL, headers)
		if err == nil {
			breaker.recordSuccess()
			return body, nil
		}

		lastErr = err
		breaker.recordFailure()

		if IsClientError(err) {
			return nil, err
		}
	}

	return nil, lastErr
}

func (c *ResilientClient) doGet(rawURL string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", c.ua)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, &HTTPError{StatusCode: resp.StatusCode}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBody))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	return body, nil
}
