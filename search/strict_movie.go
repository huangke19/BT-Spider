package search

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

type strictMovieQuery struct {
	titleKey string
	year     string
}

func parseStrictMovieQuery(keyword string) (strictMovieQuery, bool) {
	titleKey, year, has1080, ok := parseMovieTitleYear(keyword)
	if !ok || year == "" || !has1080 {
		return strictMovieQuery{}, false
	}
	return strictMovieQuery{titleKey: titleKey, year: year}, true
}

func parseMovieTitleYear(keyword string) (titleKey, year string, has1080, ok bool) {
	tokens := splitComparableTokens(strings.ToLower(strings.TrimSpace(keyword)))
	if len(tokens) == 0 {
		return "", "", false, false
	}

	filtered := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		switch {
		case isStrictYearToken(tok):
			year = tok
			continue
		case isStrict1080Token(tok):
			has1080 = true
			continue
		default:
			filtered = append(filtered, tok)
		}
	}

	filtered = trimLeadingArticle(filtered)
	if len(filtered) == 0 {
		return "", "", has1080, false
	}

	titleKey = strings.Join(filtered, " ")
	if containsCJK(titleKey) {
		return "", "", has1080, false
	}
	return titleKey, year, has1080, true
}

func finalizeStrictMovieResults(allResults []Result, query strictMovieQuery) []Result {
	allResults = dedup(allResults)

	for i := range allResults {
		name := strings.ToLower(allResults[i].Name)
		if !strictMovieMatchesTitle(name, query) {
			allResults[i].Seeders = 0
			continue
		}
		if !strictMovieHas1080P(name) || strictMovieHasForbiddenResolution(name) {
			allResults[i].Seeders = 0
		}
	}

	var unknownHashes []string
	for _, r := range allResults {
		if r.Seeders == -1 && r.InfoHash != "" {
			unknownHashes = append(unknownHashes, r.InfoHash)
		}
	}
	if len(unknownHashes) > 0 {
		scraped := ScrapeSeeders(unknownHashes, 3*time.Second)
		for i := range allResults {
			if allResults[i].Seeders == -1 {
				if c, ok := scraped[strings.ToUpper(allResults[i].InfoHash)]; ok {
					allResults[i].Seeders = c
				} else {
					allResults[i].Seeders = 0
				}
			}
		}
	}

	type scoredResult struct {
		result Result
		score  int
	}

	candidates := make([]scoredResult, 0, len(allResults))
	for _, r := range allResults {
		if r.Seeders <= 0 {
			continue
		}
		if !strictMovieMatchesTitle(strings.ToLower(r.Name), query) {
			continue
		}
		if !strictMovieHas1080P(strings.ToLower(r.Name)) || strictMovieHasForbiddenResolution(strings.ToLower(r.Name)) {
			continue
		}
		candidates = append(candidates, scoredResult{result: r, score: scoreStrictMovieResult(r)})
	}

	if len(candidates) == 0 {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].result.Seeders != candidates[j].result.Seeders {
			return candidates[i].result.Seeders > candidates[j].result.Seeders
		}
		return sourceTrustScore(candidates[i].result.Source) > sourceTrustScore(candidates[j].result.Source)
	})

	out := make([]Result, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.result)
	}
	return out
}

func strictMovieMatchesTitle(name string, query strictMovieQuery) bool {
	return strictMovieTitleKey(name, query.year) == query.titleKey
}

func strictMovieTitleKey(name, year string) string {
	tokens := splitComparableTokens(strings.ToLower(name))
	if len(tokens) == 0 {
		return ""
	}

	cut := len(tokens)
	for i, tok := range tokens {
		if tok == year || isStrict1080Token(tok) || isStrictForbiddenResolutionToken(tok) {
			cut = i
			break
		}
	}

	tokens = tokens[:cut]
	tokens = trimLeadingArticle(tokens)
	if len(tokens) == 0 {
		return ""
	}
	return strings.Join(tokens, " ")
}

func strictMovieHas1080P(name string) bool {
	tokens := splitComparableTokens(strings.ToLower(name))
	for _, tok := range tokens {
		if tok == "1080p" || tok == "1920x1080" {
			return true
		}
	}
	return false
}

func strictMovieHasForbiddenResolution(name string) bool {
	tokens := splitComparableTokens(strings.ToLower(name))
	for _, tok := range tokens {
		if isStrictForbiddenResolutionToken(tok) {
			return true
		}
	}
	return false
}

func scoreStrictMovieResult(r Result) int {
	score := 80
	score += sourceTrustScore(r.Source) * 3
	score += seederScore(r.Seeders)
	score += sizeSanityScore(r.Size)
	if r.InfoHash != "" {
		score += 2
	}
	return score
}

func sourceTrustScore(source string) int {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "yts":
		return 5
	case "1337x":
		return 4
	case "apibay", "thepiratebay", "tpb":
		return 4
	case "btdigg":
		return 3
	case "torrentkitty":
		return 3
	case "bt4g":
		return 2
	case "eztv", "nyaa":
		return 1
	default:
		return 2
	}
}

func seederScore(seeders int) int {
	switch {
	case seeders >= 300:
		return 6
	case seeders >= 100:
		return 5
	case seeders >= 50:
		return 4
	case seeders >= 20:
		return 3
	case seeders >= 10:
		return 2
	case seeders > 0:
		return 1
	default:
		return 0
	}
}

func sizeSanityScore(size string) int {
	gb, ok := parseSizeToGB(size)
	if !ok {
		return 0
	}
	switch {
	case gb >= 0.8 && gb <= 20:
		return 4
	case gb >= 0.4 && gb < 0.8:
		return 2
	case gb > 20 && gb <= 40:
		return 1
	default:
		return 0
	}
}

func parseSizeToGB(size string) (float64, bool) {
	v := strings.TrimSpace(strings.ToLower(size))
	if v == "" || v == "未知" || v == "-" {
		return 0, false
	}
	fields := strings.Fields(v)
	if len(fields) < 2 {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.TrimSuffix(fields[0], ","), 64)
	if err != nil {
		return 0, false
	}
	unit := strings.TrimSpace(fields[1])
	switch {
	case strings.HasPrefix(unit, "tb"):
		return value * 1024, true
	case strings.HasPrefix(unit, "gb"):
		return value, true
	case strings.HasPrefix(unit, "mb"):
		return value / 1024, true
	case strings.HasPrefix(unit, "kb"):
		return value / (1024 * 1024), true
	default:
		return 0, false
	}
}

func splitComparableTokens(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	})
	if len(parts) == 0 {
		return nil
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func trimLeadingArticle(tokens []string) []string {
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

func isStrictYearToken(tok string) bool {
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

func isStrict1080Token(tok string) bool {
	switch tok {
	case "1080p", "1920x1080":
		return true
	default:
		return false
	}
}

func isStrictForbiddenResolutionToken(tok string) bool {
	switch tok {
	case "720p", "1080i", "2160p", "4k", "2k", "1440p", "uhd", "480p", "360p":
		return true
	default:
		return false
	}
}

func containsCJK(s string) bool {
	for _, r := range s {
		if isCJK(r) {
			return true
		}
	}
	return false
}
