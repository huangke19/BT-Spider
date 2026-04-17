package pipeline

import (
	"testing"
	"time"

	"github.com/huangke/bt-spider/search"
)

func TestCacheGetPut(t *testing.T) {
	c := &searchCache{entries: make(map[string]*cacheEntry)}
	results := []search.Result{{Name: "A", Seeders: 10}}

	c.Put("Ubuntu", results)
	got, ok := c.Get("ubuntu  ")
	if !ok || len(got) != 1 || got[0].Name != "A" {
		t.Fatalf("expected cache hit, got %v %v", got, ok)
	}
}

func TestCacheExpiry(t *testing.T) {
	c := &searchCache{entries: make(map[string]*cacheEntry)}
	c.entries["k"] = &cacheEntry{
		results: []search.Result{{Name: "x"}},
		expiry:  time.Now().Add(-time.Second),
	}
	c.order = []string{"k"}
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected expired")
	}
}

func TestCacheEviction(t *testing.T) {
	c := &searchCache{entries: make(map[string]*cacheEntry)}
	for i := 0; i < SearchCacheMaxEntries+10; i++ {
		c.Put(string(rune('a'+i%26))+string(rune('0'+i/26)), []search.Result{{Name: "x"}})
	}
	if len(c.entries) > SearchCacheMaxEntries {
		t.Fatalf("cache exceeded max: %d", len(c.entries))
	}
}
