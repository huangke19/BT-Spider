package search

import (
	"strings"
	"unicode"

	"github.com/huangke/bt-spider/config"
	"github.com/huangke/bt-spider/pkg/logger"
)

// NLPResolve 自然语言电影搜索 pipeline：
//
//	预处理: 意图剥离 + 中文数字规范化
//	解析链: alias → tmdb → groq （由 DefaultResolverChain 构造）
//
// 新增解析层：在 search/resolver.go 里实现 Resolver，加入 DefaultResolverChain。
func NLPResolve(raw string, cfg *config.Config) (MovieResolution, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return MovieResolution{}, false
	}

	logger.Info("nlp resolve start", "input", raw)

	// 预处理：剥离意图词 + 中文序号规范化
	cleaned := stripMovieIntent(raw)
	cleaned = normalizeChineseNumbers(cleaned)
	logger.Debug("nlp preprocess", "input", raw, "cleaned", cleaned)

	chain := DefaultResolverChain(cfg)

	// 先用预处理后的输入尝试整条链
	if r, ok := chain.Resolve(cleaned); ok {
		return r, true
	}
	// 预处理可能过度剥离，alias 层再用原始输入兜底一次
	if cleaned != raw {
		if r, ok := (aliasResolver{}).Resolve(raw); ok {
			logger.Info("resolver hit", "resolver", "alias", "input", raw, "query", r.Query, "note", "raw fallback")
			return r, true
		}
	}

	logger.Error("nlp resolve failed", "input", raw, "cleaned", cleaned)
	return MovieResolution{}, false
}

// stripMovieIntent 剥离"帮我下载"等意图前缀和"高清的"等尾缀修饰词。
func stripMovieIntent(s string) string {
	s = strings.TrimSpace(s)

	prefixes := []string{
		"帮我下载", "帮我搜索", "帮我搜", "帮我找",
		"给我下载", "给我找", "给我搜",
		"我想看", "我要看", "我想下载", "我要下载",
		"下载一下", "搜索一下", "搜一下", "搜一搜", "找一下", "找一找",
		"下载", "搜索", "搜",
	}

	sRunes := []rune(s)
	sLower := strings.ToLower(s)
	for _, p := range prefixes {
		if strings.HasPrefix(sLower, p) {
			pRunes := []rune(p)
			if len(sRunes) > len(pRunes) {
				s = strings.TrimSpace(string(sRunes[len(pRunes):]))
				sRunes = []rune(s)
				sLower = strings.ToLower(s)
			}
			break
		}
	}

	// 剥离尾缀修饰词（从最长到最短，避免子串误匹配）
	suffixes := []string{"高清蓝光版", "蓝光版", "高清版", "完整版", "完整的", "高清的", "要高清"}
	sRunes = []rune(s)
	sLower = strings.ToLower(s)
	for _, suf := range suffixes {
		if strings.HasSuffix(sLower, suf) {
			sufRunes := []rune(suf)
			if len(sRunes) > len(sufRunes) {
				s = strings.TrimSpace(string(sRunes[:len(sRunes)-len(sufRunes)]))
			}
			break
		}
	}

	return strings.TrimSpace(s)
}

// normalizeChineseNumbers 将 "第X部/集/季" 中的中文数字转为阿拉伯数字，
// 方便后续别名匹配（如"第二部" → "2"）。
func normalizeChineseNumbers(s string) string {
	cnToArabic := map[rune]string{
		'一': "1", '二': "2", '三': "3", '四': "4", '五': "5",
		'六': "6", '七': "7", '八': "8", '九': "9", '十': "10",
	}

	runes := []rune(s)
	var out []rune
	i := 0
	for i < len(runes) {
		r := runes[i]

		// 匹配 "第X部/集/季" 模式（X 为中文数字或阿拉伯数字）
		if r == '第' && i+1 < len(runes) {
			next := runes[i+1]
			var numStr string
			if d, ok := cnToArabic[next]; ok {
				numStr = d
			} else if unicode.IsDigit(next) {
				numStr = string(next)
			}
			if numStr != "" {
				out = append(out, []rune(numStr)...)
				i += 2
				// 跳过后面的量词
				if i < len(runes) {
					switch runes[i] {
					case '部', '集', '季':
						i++
					}
				}
				continue
			}
		}

		out = append(out, r)
		i++
	}
	return string(out)
}
