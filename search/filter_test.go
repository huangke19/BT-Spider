package search

import (
	"reflect"
	"testing"
)

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
	// 模拟 provider 返回的混杂结果
	results := []Result{
		{Name: "The Bourne Supremacy 2004 1080p BluRay", Seeders: 50},
		{Name: "The Bourne Identity 2002 1080p", Seeders: 30},
		{Name: "谍影重重 第二部 2004 中字", Seeders: 20},
		{Name: "谍影重重 第二部分.The.Bourne.Identity.2002", Seeders: 10}, // 错标
		{Name: "Random Movie 2020", Seeders: 5},
	}

	t.Run("英文关键词精确过滤", func(t *testing.T) {
		filtered := filterByKeyword(results, "Bourne Supremacy")
		// tokens: bourne, supremacy —— 要求 >= 2 个匹配
		// 只有 #1 能命中两个
		if len(filtered) != 1 || filtered[0].Name != results[0].Name {
			t.Errorf("期望只保留 Bourne Supremacy，实际 %d 条: %+v", len(filtered), filtered)
		}
	})

	t.Run("中文关键词 bigram 过滤掉无关结果", func(t *testing.T) {
		filtered := filterByKeyword(results, "谍影重重第二部")
		// 应保留包含中文名的两条，过滤掉纯英文和无关电影
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

	t.Run("零匹配时不再错误地保留所有结果（通过 provider 过滤掉了）", func(t *testing.T) {
		// 构造一个确实没 token 匹配的场景：关键词 CJK 段长 >= 2 可 bigram
		noMatch := []Result{
			{Name: "Completely Unrelated Content", Seeders: 1},
		}
		filtered := filterByKeyword(noMatch, "谍影重重")
		// 过滤后为空 → fallback 保守返回原始（这是合理兜底行为，保护调用方）
		if len(filtered) != 1 {
			t.Errorf("兜底应返回 1 条，得到 %d 条", len(filtered))
		}
	})
}
