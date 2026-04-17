package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/huangke/bt-spider/search"
)

type fakeProvider struct {
	name    string
	delay   time.Duration
	results []search.Result
	err     error
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) Search(keyword string, page int) ([]search.Result, error) {
	time.Sleep(f.delay)
	return f.results, f.err
}

func TestSearchStreamOrdering(t *testing.T) {
	CacheInvalidate("kw")
	providers := []search.Provider{
		&fakeProvider{name: "fast", delay: 50 * time.Millisecond, results: []search.Result{{Name: "A", Size: "1.0 GB", Seeders: 10, InfoHash: "h1"}}},
		&fakeProvider{name: "slow", delay: 300 * time.Millisecond, results: []search.Result{{Name: "B", Size: "1.2 GB", Seeders: 20, InfoHash: "h2"}}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := SearchStream(ctx, "kw", providers, time.Second)
	var updates []StreamUpdate
	for u := range ch {
		updates = append(updates, u)
	}
	if len(updates) < 2 {
		t.Fatalf("expected >=2 updates, got %d", len(updates))
	}
	if len(updates[0].Results) != 1 || updates[0].Results[0].Name != "A" {
		t.Fatalf("expected first streamed result to be A, got %+v", updates[0].Results)
	}
	if len(updates[len(updates)-2].Results) != 2 || updates[len(updates)-2].Results[0].Name != "B" {
		t.Fatalf("expected later higher-seeder result to move to front, got %+v", updates[len(updates)-2].Results)
	}
	if !updates[len(updates)-1].Done {
		t.Fatal("last update should be Done")
	}
}
