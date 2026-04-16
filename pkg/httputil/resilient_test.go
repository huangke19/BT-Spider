package httputil

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestResilientClient_RetryOnServerError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewResilientClient(WithRetry(2, 10*time.Millisecond))
	body, err := c.Get(srv.URL)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("expected 'ok', got %q", string(body))
	}
	if calls.Load() != 3 {
		t.Fatalf("expected 3 calls, got %d", calls.Load())
	}
}

func TestResilientClient_NoRetryOn4xx(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewResilientClient(WithRetry(2, 10*time.Millisecond))
	_, err := c.Get(srv.URL)
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if !IsClientError(err) {
		t.Fatalf("expected client error, got: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 call (no retry on 4xx), got %d", calls.Load())
	}
}

func TestResilientClient_CircuitBreaker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewResilientClient(
		WithRetry(0, 10*time.Millisecond),
		WithCircuitBreaker(3, 50*time.Millisecond),
	)

	// Trip the breaker: 3 failures
	for i := 0; i < 3; i++ {
		c.Get(srv.URL)
	}

	// Next call should be rejected by circuit breaker
	_, err := c.Get(srv.URL)
	if err == nil {
		t.Fatal("expected circuit breaker error")
	}
	if !contains(err.Error(), "熔断器已开启") {
		t.Fatalf("expected circuit breaker message, got: %v", err)
	}

	// Wait for reset timeout, then should allow again
	time.Sleep(60 * time.Millisecond)
	_, err = c.Get(srv.URL)
	// Still fails (server returns 500) but breaker allowed the request
	if err != nil && contains(err.Error(), "熔断器已开启") {
		t.Fatal("circuit breaker should have reset to half-open")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
