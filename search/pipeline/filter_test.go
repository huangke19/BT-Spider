package pipeline

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/huangke/bt-spider/search"
)

type stubProvider struct {
	name    string
	delay   time.Duration
	results []search.Result
	err     error
}

func (s stubProvider) Name() string { return s.name }

func (s stubProvider) Search(keyword string, page int) ([]search.Result, error) {
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	return s.results, s.err
}

func TestTokenize(t *testing.T) {
	cases := []struct {
		name    string
		keyword string
		want    []string
	}{
		{
			name:    "纯中文，单段",
			keyword: "谍影重重第二部",
			want:    []string{"谍影", "影重", "重重", "重第", "第二", "二部"},
		},
		{
			name:    "纯中文，含空格分段",
			keyword: "谍影重重 第二部",
			want:    []string{"谍影", "影重", "重重", "第二", "二部"},
		},
		{
			name:    "纯英文",
			keyword: "The Bourne Supremacy 2004",
			want:    []string{"the", "bourne", "supremacy", "2004"},
		},
		{
			name:    "短词过滤",
			keyword: "a b cd efg",
			want:    []string{"efg"},
		},
		{
			name:    "中英混合",
			keyword: "Bourne 谍影重重",
			want:    []string{"bourne", "谍影", "影重", "重重"},
		},
		{
			name:    "单字中文",
			keyword: "爱",
			want:    []string{"爱"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := tokenize(c.keyword)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("tokenize(%q)\n  got:  %v\n  want: %v", c.keyword, got, c.want)
			}
		})
	}
}

func TestFilterByKeyword(t *testing.T) {
	results := []search.Result{
		{Name: "The Bourne Supremacy 2004 1080p BluRay", Seeders: 50},
		{Name: "The Bourne Identity 2002 1080p", Seeders: 30},
		{Name: "谍影重重 第二部 2004 中字", Seeders: 20},
		{Name: "谍影重重 第二部分.The.Bourne.Identity.2002", Seeders: 10},
		{Name: "Random Movie 2020", Seeders: 5},
	}

	t.Run("英文关键词精确过滤", func(t *testing.T) {
		filtered := filterByKeyword(results, "Bourne Supremacy")
		if len(filtered) != 1 || filtered[0].Name != results[0].Name {
			t.Errorf("期望只保留 Bourne Supremacy，实际 %d 条: %+v", len(filtered), filtered)
		}
	})

	t.Run("中文关键词 bigram 过滤掉无关结果", func(t *testing.T) {
		filtered := filterByKeyword(results, "谍影重重第二部")
		names := make(map[string]bool)
		for _, r := range filtered {
			names[r.Name] = true
		}
		if names["Random Movie 2020"] {
			t.Errorf("不应保留无关结果 Random Movie 2020，得到: %+v", filtered)
		}
		if names["The Bourne Supremacy 2004 1080p BluRay"] {
			t.Errorf("不应保留纯英文无中文名结果，得到: %+v", filtered)
		}
		if !names["谍影重重 第二部 2004 中字"] {
			t.Errorf("应保留真正的谍影重重第二部，得到: %+v", filtered)
		}
	})

	t.Run("零匹配时兜底返回原始", func(t *testing.T) {
		noMatch := []search.Result{
			{Name: "Completely Unrelated Content", Seeders: 1},
		}
		filtered := filterByKeyword(noMatch, "谍影重重")
		if len(filtered) != 1 {
			t.Errorf("兜底应返回 1 条，得到 %d 条", len(filtered))
		}
	})
}

func TestSearchWithTimeoutReturnsPartialResults(t *testing.T) {
	providers := []search.Provider{
		stubProvider{
			name: "fast",
			results: []search.Result{
				{Name: "The Bourne Supremacy 2004", Seeders: 10, InfoHash: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
			},
		},
		stubProvider{
			name:  "slow",
			delay: 200 * time.Millisecond,
			results: []search.Result{
				{Name: "Slow Result", Seeders: 20, InfoHash: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"},
			},
		},
	}

	start := time.Now()
	results, err := SearchWithTimeout("Bourne Supremacy", providers, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if time.Since(start) > 150*time.Millisecond {
		t.Fatalf("search did not return early on timeout")
	}
	if len(results) != 1 || results[0].Name != "The Bourne Supremacy 2004" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestSearchWithTimeoutReportsTimeoutWithoutResults(t *testing.T) {
	providers := []search.Provider{
		stubProvider{name: "slow-a", delay: 100 * time.Millisecond},
		stubProvider{name: "slow-b", delay: 100 * time.Millisecond},
	}

	_, err := SearchWithTimeout("Bourne", providers, 20*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if got := err.Error(); !strings.Contains(got, "搜索超时") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
}

func TestFinalizeStrictMovieResultsAcceptsLeonAlternateTitles(t *testing.T) {
	query, ok := parseStrictMovieQuery("Léon 1994 1080P")
	if !ok {
		t.Fatal("expected strict query parse to succeed")
	}

	results := []search.Result{
		{Name: "Leon.1994.1080p.BluRay.x264", Seeders: 10, Source: "1337x", InfoHash: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{Name: "Leon.The.Professional.1994.1080p.BluRay.x265", Seeders: 8, Source: "ThePirateBay", InfoHash: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"},
		{Name: "Leon.1994.720p.BluRay.x264", Seeders: 50, Source: "1337x", InfoHash: "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"},
	}

	filtered := finalizeStrictMovieResults(results, query)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 leon matches, got %d: %+v", len(filtered), filtered)
	}

	names := []string{filtered[0].Name, filtered[1].Name}
	if !strings.Contains(strings.Join(names, "|"), "Leon.1994.1080p.BluRay.x264") {
		t.Fatalf("expected direct Leon title to remain, got %+v", names)
	}
	if !strings.Contains(strings.Join(names, "|"), "Leon.The.Professional.1994.1080p.BluRay.x265") {
		t.Fatalf("expected alternate Leon title to remain, got %+v", names)
	}
}
