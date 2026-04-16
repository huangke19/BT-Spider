package search

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// IsCJK 判断字符是否为 CJK（中日韩）字符
func IsCJK(r rune) bool {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF:
		return true
	case r >= 0x3400 && r <= 0x4DBF:
		return true
	case r >= 0x3040 && r <= 0x309F:
		return true
	case r >= 0x30A0 && r <= 0x30FF:
		return true
	case r >= 0xAC00 && r <= 0xD7AF:
		return true
	}
	return false
}

// ContainsCJK 检查字符串是否包含 CJK 字符
func ContainsCJK(s string) bool {
	for _, r := range s {
		if IsCJK(r) {
			return true
		}
	}
	return false
}

// SplitComparableTokens 将字符串按字母/数字边界拆分为可比较的 token 列表
func SplitComparableTokens(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	if len(parts) == 0 {
		return nil
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = NormalizeComparableToken(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// NormalizeComparableToken 规范化单个 token（小写 + 去重音符号）
func NormalizeComparableToken(token string) string {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return ""
	}
	decomposed := norm.NFD.String(token)
	var b strings.Builder
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// TrimLeadingArticle 去除 token 列表开头的冠词
func TrimLeadingArticle(tokens []string) []string {
	if len(tokens) == 0 {
		return tokens
	}
	switch tokens[0] {
	case "the", "a", "an":
		return tokens[1:]
	default:
		return tokens
	}
}

// IsStrictYearToken 判断 token 是否为年份（1xxx / 2xxx 四位数）
func IsStrictYearToken(tok string) bool {
	if len(tok) != 4 {
		return false
	}
	if tok[0] != '1' && tok[0] != '2' {
		return false
	}
	for i := 1; i < 4; i++ {
		if tok[i] < '0' || tok[i] > '9' {
			return false
		}
	}
	return true
}

// IsStrict1080Token 判断 token 是否为 1080p
func IsStrict1080Token(tok string) bool {
	switch tok {
	case "1080p", "1920x1080":
		return true
	default:
		return false
	}
}

// IsStrictForbiddenResolutionToken 判断 token 是否为禁止的分辨率
func IsStrictForbiddenResolutionToken(tok string) bool {
	switch tok {
	case "720p", "1080i", "2160p", "4k", "2k", "1440p", "uhd", "480p", "360p":
		return true
	default:
		return false
	}
}

// ParseMovieTitleYear 从关键词中解析出电影标题、年份和 1080p 标记
func ParseMovieTitleYear(keyword string) (titleKey, year string, has1080, ok bool) {
	tokens := SplitComparableTokens(strings.ToLower(strings.TrimSpace(keyword)))
	if len(tokens) == 0 {
		return "", "", false, false
	}

	filtered := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		switch {
		case IsStrictYearToken(tok):
			year = tok
			continue
		case IsStrict1080Token(tok):
			has1080 = true
			continue
		default:
			filtered = append(filtered, tok)
		}
	}

	filtered = TrimLeadingArticle(filtered)
	if len(filtered) == 0 {
		return "", "", has1080, false
	}

	titleKey = strings.Join(filtered, " ")
	if ContainsCJK(titleKey) {
		return "", "", has1080, false
	}
	return titleKey, year, has1080, true
}
