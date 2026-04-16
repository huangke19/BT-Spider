package query

import (
	"strings"
	"unicode"

	"github.com/huangke/bt-spider/config"
	"github.com/huangke/bt-spider/pkg/logger"
	"github.com/huangke/bt-spider/search"
)

// NLPResolve 自然语言电影搜索 pipeline
func NLPResolve(raw string, cfg *config.Config) (search.MovieResolution, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return search.MovieResolution{}, false
	}

	logger.Info("nlp resolve start", "input", raw)

	cleaned := stripMovieIntent(raw)
	cleaned = normalizeChineseNumbers(cleaned)
	logger.Debug("nlp preprocess", "input", raw, "cleaned", cleaned)

	chain := DefaultResolverChain(cfg)

	if r, ok := chain.Resolve(cleaned); ok {
		return r, true
	}
	if cleaned != raw {
		if r, ok := (aliasResolver{}).Resolve(raw); ok {
			logger.Info("resolver hit", "resolver", "alias", "input", raw, "query", r.Query, "note", "raw fallback")
			return r, true
		}
	}

	logger.Error("nlp resolve failed", "input", raw, "cleaned", cleaned)
	return search.MovieResolution{}, false
}

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
