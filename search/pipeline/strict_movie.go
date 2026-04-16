package pipeline

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/huangke/bt-spider/search"
)

type strictMovieQuery struct {
	titleKey string
	year     string
}

var strictMovieAlternateTitles = map[string][]string{
	"leon":                  {"leon the professional"},
	"leon the professional": {"leon"},
}

func parseStrictMovieQuery(keyword string) (strictMovieQuery, bool) {
	titleKey, year, has1080, ok := search.ParseMovieTitleYear(keyword)
	if !ok || year == "" || !has1080 {
		return strictMovieQuery{}, false
	}
	return strictMovieQuery{titleKey: titleKey, year: year}, true
}

func finalizeStrictMovieResults(allResults []search.Result, query strictMovieQuery) []search.Result {
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
		result search.Result
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

	out := make([]search.Result, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.result)
	}
	return out
}

func strictMovieMatchesTitle(name string, query strictMovieQuery) bool {
	titleKey := strictMovieTitleKey(name, query.year)
	if titleKey == query.titleKey {
		return true
	}
	for _, alt := range strictMovieAlternateTitles[query.titleKey] {
		if titleKey == alt {
			return true
		}
	}
	for _, alt := range strictMovieAlternateTitles[titleKey] {
		if alt == query.titleKey {
			return true
		}
	}
	return false
}

func strictMovieTitleKey(name, year string) string {
	tokens := search.SplitComparableTokens(strings.ToLower(name))
	if len(tokens) == 0 {
		return ""
	}

	cut := len(tokens)
	for i, tok := range tokens {
		if tok == year || search.IsStrict1080Token(tok) || search.IsStrictForbiddenResolutionToken(tok) {
			cut = i
			break
		}
	}

	tokens = tokens[:cut]
	tokens = search.TrimLeadingArticle(tokens)
	if len(tokens) == 0 {
		return ""
	}
	return strings.Join(tokens, " ")
}

func strictMovieHas1080P(name string) bool {
	tokens := search.SplitComparableTokens(strings.ToLower(name))
	for _, tok := range tokens {
		if tok == "1080p" || tok == "1920x1080" {
			return true
		}
	}
	return false
}

func strictMovieHasForbiddenResolution(name string) bool {
	tokens := search.SplitComparableTokens(strings.ToLower(name))
	for _, tok := range tokens {
		if search.IsStrictForbiddenResolutionToken(tok) {
			return true
		}
	}
	return false
}

func scoreStrictMovieResult(r search.Result) int {
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
	case "torrentkitty":
		return 3
	case "bt4g":
		return 2
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
