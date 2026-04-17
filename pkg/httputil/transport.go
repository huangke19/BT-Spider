package httputil

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

var (
	sharedTransportOnce sync.Once
	sharedTransport     *http.Transport
)

// SharedTransport 返回全局复用的 *http.Transport。
// 所有 provider 的 HTTP 客户端共用它，实现 keep-alive 连接复用 + HTTP/2。
func SharedTransport() *http.Transport {
	sharedTransportOnce.Do(func() {
		sharedTransport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   3 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   8,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   3 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	})
	return sharedTransport
}

// Preheat 对常用 provider 的域名做一次 HEAD 请求，预热 TLS + DNS。
// 应在进程启动时、搜索相关代码运行前调用。失败静默（不影响主流程）。
func Preheat(ctx context.Context, hosts []string) {
	if len(hosts) == 0 {
		return
	}
	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: SharedTransport(),
	}
	var wg sync.WaitGroup
	for _, h := range hosts {
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			req, err := http.NewRequestWithContext(ctx, http.MethodHead, host, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", DefaultUA)
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			_ = resp.Body.Close()
		}(h)
	}
	wg.Wait()
}

// DefaultPreheatHosts 预热的默认 provider 域名列表。
func DefaultPreheatHosts() []string {
	return []string{
		"https://apibay.org",
		"https://bt4gprx.com",
		"https://www.torrentkitty.tv",
	}
}
