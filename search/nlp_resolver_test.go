package search

import (
	"testing"

	"github.com/huangke/bt-spider/config"
)

func TestStripMovieIntent(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"帮我下载星际穿越", "星际穿越"},
		{"帮我搜索 The Dark Knight", "The Dark Knight"},
		{"我想看 这个杀手不太冷 高清的", "这个杀手不太冷"},
		{"下载 盗梦空间 完整版", "盗梦空间"},
		{"搜一下黑暗骑士", "黑暗骑士"},
		{"黑暗骑士", "黑暗骑士"}, // 无前缀不变
		{"  帮我下载  美国队长第二部  ", "美国队长第二部"},
		{"下载一下星际穿越蓝光版", "星际穿越"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := stripMovieIntent(c.in)
			if got != c.want {
				t.Errorf("stripMovieIntent(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestNormalizeChineseNumbers(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"美国队长第二部", "美国队长2"},
		{"谍影重重第三部", "谍影重重3"},
		{"复仇者联盟第四部", "复仇者联盟4"},
		{"第一季", "1"},
		{"第10季", "10季"},   // 当前行为：仅匹配中文数字，阿拉伯数字不被剥离量词
		{"没有第X的文本", "没有第X的文本"}, // 非数字后缀不变
		{"第", "第"},          // 孤立的"第"不变
		{"星际穿越", "星际穿越"},    // 无"第X部"不变
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := normalizeChineseNumbers(c.in)
			if got != c.want {
				t.Errorf("normalizeChineseNumbers(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestNLPResolveL1L2 测试 L1 预处理 + L2 本地别名路径（不依赖网络）。
// L3/L4 因需要真实 API key 和外部服务，在本测试里只验证空 key 时被正确跳过。
func TestNLPResolveL1L2(t *testing.T) {
	emptyCfg := &config.Config{} // 无 TMDB / Groq key

	cases := []struct {
		name    string
		input   string
		wantOK  bool
		wantSub string // 结果 query 中应包含的子串
	}{
		{"意图前缀 + 中文别名", "帮我下载星际穿越", true, "Interstellar"},
		{"中文数字序号 → 别名", "我想看复仇者联盟第二部", true, "Avengers: Age of Ultron"},
		{"别名含重音字符", "这个杀手不太冷", true, "Léon"},
		{"严格电影格式直通", "Inception 2010 1080P", true, "inception 2010"}, // 严格路径全小写
		{"无法识别（无 API key）", "一部没人听过的小电影", false, ""},
		{"空输入", "", false, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := NLPResolve(c.input, emptyCfg)
			if ok != c.wantOK {
				t.Fatalf("NLPResolve(%q) ok = %v, want %v (got=%+v)", c.input, ok, c.wantOK, got)
			}
			if c.wantOK && c.wantSub != "" {
				if !contains(got.Query, c.wantSub) {
					t.Errorf("NLPResolve(%q) query = %q, want contains %q", c.input, got.Query, c.wantSub)
				}
			}
		})
	}
}

// 小工具：避免从 strings 导入，减少噪音
func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
