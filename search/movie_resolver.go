package search

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

type MovieResolution struct {
	Query   string
	Display string
}

type movieMeta struct {
	Title string
	Year  string
}

var movieAliases = map[string]movieMeta{
	// 用户最常搜的示例：美国队长第二部
	"美国队长第二部":                 {Title: "Captain America: The Winter Soldier", Year: "2014"},
	"美国队长2":                    {Title: "Captain America: The Winter Soldier", Year: "2014"},
	"美国队长：冬日战士":              {Title: "Captain America: The Winter Soldier", Year: "2014"},
	"美国队长冬日战士":                {Title: "Captain America: The Winter Soldier", Year: "2014"},
	"captainamericathewintersoldier": {Title: "Captain America: The Winter Soldier", Year: "2014"},

	// 常见补充
	"美国队长第一部":              {Title: "Captain America: The First Avenger", Year: "2011"},
	"美国队长1":                 {Title: "Captain America: The First Avenger", Year: "2011"},
	"captainamericathefirstavenger": {Title: "Captain America: The First Avenger", Year: "2011"},
	"星际穿越":                   {Title: "Interstellar", Year: "2014"},
	"interstellar":              {Title: "Interstellar", Year: "2014"},
	"盗梦空间":                   {Title: "Inception", Year: "2010"},
	"inception":                 {Title: "Inception", Year: "2010"},
	"黑暗骑士":                   {Title: "The Dark Knight", Year: "2008"},
	"thedarkknight":             {Title: "The Dark Knight", Year: "2008"},
	"黑暗骑士崛起":                 {Title: "The Dark Knight Rises", Year: "2012"},
	"thedarkknightrises":        {Title: "The Dark Knight Rises", Year: "2012"},
	"复仇者联盟第二部":               {Title: "Avengers: Age of Ultron", Year: "2015"},
	"复联2":                      {Title: "Avengers: Age of Ultron", Year: "2015"},
	"avengersageofultron":       {Title: "Avengers: Age of Ultron", Year: "2015"},
	"这个杀手不太冷":                 {Title: "Léon", Year: "1994"},
	"杀手莱昂":                    {Title: "Léon", Year: "1994"},
	"leon":                     {Title: "Léon", Year: "1994"},
	"léon":                     {Title: "Léon", Year: "1994"},
	"leontheprofessional":      {Title: "Léon", Year: "1994"},
}

var movieAliasKeys []string

func init() {
	movieAliasKeys = make([]string, 0, len(movieAliases))
	for k := range movieAliases {
		movieAliasKeys = append(movieAliasKeys, k)
	}
	sort.Slice(movieAliasKeys, func(i, j int) bool {
		if len(movieAliasKeys[i]) != len(movieAliasKeys[j]) {
			return len(movieAliasKeys[i]) > len(movieAliasKeys[j])
		}
		return movieAliasKeys[i] < movieAliasKeys[j]
	})
}

// ResolveMovieSearchInput 把自然语言/模糊电影名转成 strict 搜索 query。
// 返回 true 说明可以直接送进 search.Search。
func ResolveMovieSearchInput(raw string) (MovieResolution, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return MovieResolution{}, false
	}

	// 1) 已经是严格格式，直接接受
	if query, ok := normalizeStrictMovieQuery(raw); ok {
		return MovieResolution{
			Query:   query,
			Display: "已识别为精确电影搜索: " + query,
		}, true
	}

	// 2) 先做规范化，再走别名命中
	norm := normalizeMovieKey(raw)
	if meta, ok := lookupMovieMeta(norm); ok {
		query := formatMovieQuery(meta.Title, meta.Year) + " 1080P"
		return MovieResolution{
			Query:   query,
			Display: "已解析为: " + query,
		}, true
	}

	// 3) 如果拆得出英文片名 + 年份/1080P，也尝试补齐
	titleKey, year, has1080, ok := parseMovieTitleYear(raw)
	if ok {
		if meta, ok := lookupMovieMeta(normalizeMovieKey(titleKey)); ok {
			query := formatMovieQuery(meta.Title, meta.Year) + " 1080P"
			return MovieResolution{
				Query:   query,
				Display: "已解析为: " + query,
			}, true
		}

		if year != "" {
			query := formatMovieQuery(titleKey, year)
			if !has1080 {
				query += " 1080P"
			}
			return MovieResolution{
				Query:   query,
				Display: "已识别为: " + query,
			}, true
		}
	}

	return MovieResolution{}, false
}

func lookupMovieMeta(norm string) (movieMeta, bool) {
	if norm == "" {
		return movieMeta{}, false
	}
	normRunes := []rune(norm)
	for _, key := range movieAliasKeys {
		keyRunes := []rune(key)
		// 要求较短一侧至少 3 个字符才允许子串匹配，
		// 避免"2"这种碎片命中任何含 2 的别名（如 "黑客帝国2" 被误判成 "美国队长2"）。
		shortLen := len(normRunes)
		if len(keyRunes) < shortLen {
			shortLen = len(keyRunes)
		}
		if shortLen < 3 {
			continue
		}
		if strings.Contains(norm, key) || strings.Contains(key, norm) {
			return movieAliases[key], true
		}
	}
	return movieMeta{}, false
}

func normalizeStrictMovieQuery(raw string) (string, bool) {
	titleKey, year, has1080, ok := parseMovieTitleYear(raw)
	if !ok || year == "" || !has1080 {
		return "", false
	}
	return formatMovieQuery(titleKey, year) + " 1080P", true
}

func formatMovieQuery(title, year string) string {
	title = strings.TrimSpace(title)
	year = strings.TrimSpace(year)
	if title == "" {
		return ""
	}
	if year == "" {
		return title
	}
	return fmt.Sprintf("%s %s", title, year)
}

func normalizeMovieKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
		case isCJK(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}
