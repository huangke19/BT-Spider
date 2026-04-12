package httputil

import (
	"net/http"
	"time"
)

// DefaultUA is the default User-Agent string used for HTTP requests.
const DefaultUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// DefaultTimeout is the default HTTP client timeout.
const DefaultTimeout = 15 * time.Second

// NewClient creates an *http.Client with proxy support and the given timeout.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	}
}
