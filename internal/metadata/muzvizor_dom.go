package metadata

import (
	"context"
	"fmt"
	htmlstd "html"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

const (
	muzvizorGenreCacheTTL         = 15 * time.Minute
	muzvizorMaxGenreFallbackPages = 6
)

var (
	muzvizorDOMDivTag     = regexp.MustCompile(`(?is)</?div\b[^>]*>`)
	muzvizorDOMClassAttr  = regexp.MustCompile(`(?is)\bclass\s*=\s*["']([^"']*)["']`)
	muzvizorDOMStageClass = regexp.MustCompile(`(?i)^track__stage_([a-z0-9_-]+)$`)
	muzvizorDOMGenreHref  = regexp.MustCompile(`(?is)<a\b[^>]*\bhref\s*=\s*["']([^"']+)["']`)
	muzvizorDOMGenrePath  = regexp.MustCompile(`(?i)^/genres/[a-z0-9-]+/?$`)

	muzvizorGenreSearchMu   sync.Mutex
	muzvizorGenreIndexCache = map[string]muzvizorCachedGenreIndex{}
	muzvizorGenrePageCache  = map[string]muzvizorCachedGenrePage{}
)

type muzvizorCachedGenreIndex struct {
	until time.Time
	urls  []string
}

type muzvizorCachedGenrePage struct {
	until time.Time
	items []model.MetadataCandidate
}

// muzvizorRenderedCandidatesFromHTML follows the DOM structure observed on the
// public MUZVIZOR track rows. The bool reports whether rendered row blocks were
// present at all, allowing the caller to distinguish "no row markup" from "rows
// were present, but none matched the query".
func muzvizorRenderedCandidatesFromHTML(doc, sourceURL string, query model.MetadataQuery) ([]model.MetadataCandidate, bool) {
	rows := muzvizorDOMDivBlocksByClass(doc, "track__row_main")
	if len(rows) == 0 {
		return nil, false
	}

	items := make([]model.MetadataCandidate, 0, len(rows))
	for _, row := range rows {
		artist, title := muzvizorDOMTitleArtist(row)
		if artist == "" || title == "" {
			continue
		}
		bpm, ok := muzvizorDOMBPM(row)
		if !ok {
			continue
		}
		key, ok := muzvizorDOMKey(row)
		if !ok {
			continue
		}

		items = append(items, model.MetadataCandidate{
			Source:     "MUZVIZOR",
			SourceKind: ProviderKindDJPool,
			ExternalID: muzvizorExternalID(artist, title, bpm, key),
			SourceURL:  sourceURL,
			Title:      title,
			Artist:     artist,
			Genre:      muzvizorDOMGenre(row),
			Stage:      muzvizorDOMStage(row),
			BPM:        bpm,
			Key:        key,
			KeyScale:   "camelot",
		})
	}
	return muzvizorLimitCandidates(query, items), true
}

func muzvizorDOMTitleArtist(row string) (artist, title string) {
	if title = muzvizorDOMTextByClass(row, "track__title"); title != "" {
		if artist = muzvizorDOMTextByClass(row, "track__artist"); artist != "" {
			return artist, title
		}
	}

	lines := muzvizorDOMColumnLines(row, "track__column_title")
	values := make([]string, 0, len(lines))
	for _, value := range lines {
		value = strings.TrimSpace(value)
		if value == "" || muzvizorNoiseLine(value) {
			continue
		}
		if _, ok := parseMuzvizorBPM(value); ok {
			continue
		}
		if _, ok := parseMuzvizorCamelot(value); ok {
			continue
		}
		values = append(values, value)
	}
	if len(values) >= 2 {
		return values[len(values)-1], values[len(values)-2]
	}

	rowLines := muzvizorVisibleLines(row)
	for index, value := range rowLines {
		if _, ok := parseMuzvizorBPM(value); !ok {
			continue
		}
		if artist, title = muzvizorPreviousArtistTitle(rowLines, index); artist != "" && title != "" {
			return artist, title
		}
	}
	return "", ""
}

func muzvizorDOMTextByClass(row, className string) string {
	blocks := muzvizorDOMDivBlocksByClass(row, className)
	if len(blocks) == 0 {
		return ""
	}
	for _, value := range muzvizorVisibleLines(blocks[0]) {
		value = strings.TrimSpace(value)
		if value != "" && !muzvizorNoiseLine(value) {
			return value
		}
	}
	return ""
}

func muzvizorDOMBPM(row string) (float64, bool) {
	for _, value := range muzvizorDOMColumnLines(row, "track__column_bpm") {
		if bpm, ok := parseMuzvizorBPM(value); ok {
			return bpm, true
		}
	}
	return 0, false
}

func muzvizorDOMKey(row string) (string, bool) {
	for _, value := range muzvizorDOMColumnLines(row, "track__column_key") {
		if key, ok := parseMuzvizorCamelot(value); ok {
			return key, true
		}
	}
	return "", false
}

func muzvizorDOMGenre(row string) string {
	lines := muzvizorDOMColumnLines(row, "track__column_genre")
	parts := make([]string, 0, len(lines))
	seen := map[string]bool{}
	for _, value := range lines {
		value = strings.Trim(strings.TrimSpace(value), ",")
		if value == "" || muzvizorNoiseLine(value) {
			continue
		}
		if _, ok := parseMuzvizorBPM(value); ok {
			continue
		}
		if _, ok := parseMuzvizorCamelot(value); ok {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		parts = append(parts, value)
	}
	return strings.Join(parts, ", ")
}

func muzvizorDOMStage(row string) string {
	for _, match := range muzvizorDOMClassAttr.FindAllStringSubmatch(row, -1) {
		if len(match) < 2 {
			continue
		}
		for _, className := range strings.Fields(match[1]) {
			stageMatch := muzvizorDOMStageClass.FindStringSubmatch(className)
			if len(stageMatch) != 2 {
				continue
			}
			switch strings.ToLower(stageMatch[1]) {
			case "prime":
				return "Prime Time"
			case "warm", "warmup", "warm-up":
				return "Warm Up"
			case "main", "mainstage":
				return "Mainstage"
			case "event":
				return "Event"
			default:
				value := strings.ReplaceAll(stageMatch[1], "_", " ")
				value = strings.ReplaceAll(value, "-", " ")
				value = strings.TrimSpace(value)
				if value == "" {
					return ""
				}
				return strings.ToUpper(value[:1]) + value[1:]
			}
		}
	}
	return ""
}

func muzvizorDOMColumnLines(row, className string) []string {
	blocks := muzvizorDOMDivBlocksByClass(row, className)
	if len(blocks) == 0 {
		return nil
	}
	return muzvizorVisibleLines(blocks[0])
}

func muzvizorDOMDivBlocksByClass(doc, className string) []string {
	tags := muzvizorDOMDivTag.FindAllStringIndex(doc, -1)
	if len(tags) == 0 {
		return nil
	}

	out := make([]string, 0, 8)
	for i, loc := range tags {
		tag := doc[loc[0]:loc[1]]
		if muzvizorDOMClosingDiv(tag) || !muzvizorDOMTagHasClass(tag, className) {
			continue
		}

		depth := 1
		for j := i + 1; j < len(tags); j++ {
			nested := doc[tags[j][0]:tags[j][1]]
			if muzvizorDOMClosingDiv(nested) {
				depth--
			} else {
				depth++
			}
			if depth == 0 {
				out = append(out, doc[loc[0]:tags[j][1]])
				break
			}
		}
	}
	return out
}

func muzvizorDOMClosingDiv(tag string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(tag)), "</div")
}

func muzvizorDOMTagHasClass(tag, className string) bool {
	match := muzvizorDOMClassAttr.FindStringSubmatch(tag)
	if len(match) < 2 {
		return false
	}
	for _, current := range strings.Fields(match[1]) {
		if current == className {
			return true
		}
	}
	return false
}

// searchPublicGenrePages is a public-page fallback for the case where
// /tracks?query= returns only a JavaScript shell to the Go HTTP client.
// It discovers genre URLs only from the public /genres page, then reads their
// server-rendered track rows. No hidden API or authenticated endpoint is used.
func (p *MuzvizorProvider) searchPublicGenrePages(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	muzvizorGenreSearchMu.Lock()
	defer muzvizorGenreSearchMu.Unlock()

	genreURLs := p.muzvizorPublicGenreURLs(ctx)
	var firstErr error
	successfulPages := 0

	for index, genreURL := range genreURLs {
		if index >= muzvizorMaxGenreFallbackPages {
			break
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		items, err := p.muzvizorPublicGenrePageCandidates(ctx, genreURL)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		successfulPages++
		if matches := muzvizorLimitCandidates(query, items); len(matches) > 0 {
			return matches, nil
		}
	}

	if successfulPages == 0 && firstErr != nil {
		return nil, firstErr
	}
	return nil, nil
}

func (p *MuzvizorProvider) muzvizorPublicGenreURLs(ctx context.Context) []string {
	now := time.Now()
	cacheKey := strings.TrimRight(p.baseURL, "/")
	if cached, ok := muzvizorGenreIndexCache[cacheKey]; ok && now.Before(cached.until) {
		return append([]string(nil), cached.urls...)
	}

	var values []string
	if doc, err := p.fetchHTML(ctx, cacheKey+"/genres"); err == nil {
		values = muzvizorGenreURLsFromHTML(doc, cacheKey)
	}

	// These two public pages are independently confirmed and cover the reported
	// regression track. They also keep the fallback useful if /genres markup
	// changes temporarily.
	values = ensureMuzvizorGenreURL(values, cacheKey+"/genres/house")
	values = ensureMuzvizorGenreURL(values, cacheKey+"/genres/pop")
	muzvizorPrioritizeGenreURLs(values)

	muzvizorGenreIndexCache[cacheKey] = muzvizorCachedGenreIndex{
		until: now.Add(muzvizorGenreCacheTTL),
		urls:  append([]string(nil), values...),
	}
	return append([]string(nil), values...)
}

func (p *MuzvizorProvider) muzvizorPublicGenrePageCandidates(ctx context.Context, target string) ([]model.MetadataCandidate, error) {
	now := time.Now()
	if cached, ok := muzvizorGenrePageCache[target]; ok && now.Before(cached.until) {
		return cached.items, nil
	}

	doc, err := p.fetchHTML(ctx, target)
	if err != nil {
		return nil, err
	}
	items := muzvizorCandidatesFromHTML(doc, target, model.MetadataQuery{})
	muzvizorGenrePageCache[target] = muzvizorCachedGenrePage{
		until: now.Add(muzvizorGenreCacheTTL),
		items: items,
	}
	return items, nil
}

func muzvizorGenreURLsFromHTML(doc, baseURL string) []string {
	matches := muzvizorDOMGenreHref.FindAllStringSubmatch(doc, -1)
	out := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		rawHref := htmlstd.UnescapeString(strings.TrimSpace(match[1]))
		parsed, err := url.Parse(rawHref)
		if err != nil || !muzvizorDOMGenrePath.MatchString(parsed.Path) {
			continue
		}
		target := strings.TrimRight(baseURL, "/") + strings.TrimRight(parsed.Path, "/")
		if seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}
	return out
}

func ensureMuzvizorGenreURL(values []string, target string) []string {
	for _, value := range values {
		if value == target {
			return values
		}
	}
	return append(values, target)
}

func muzvizorPrioritizeGenreURLs(values []string) {
	priority := map[string]int{
		"/genres/house":       0,
		"/genres/pop":         1,
		"/genres/hip-hop":     2,
		"/genres/open-format": 3,
		"/genres/club":        4,
		"/genres/rave":        5,
		"/genres/bass":        6,
		"/genres/breakbeat":   7,
		"/genres/baile-funk":  8,
	}
	sort.SliceStable(values, func(i, j int) bool {
		pi, pj := 100, 100
		if parsed, err := url.Parse(values[i]); err == nil {
			if value, ok := priority[strings.TrimRight(parsed.Path, "/")]; ok {
				pi = value
			}
		}
		if parsed, err := url.Parse(values[j]); err == nil {
			if value, ok := priority[strings.TrimRight(parsed.Path, "/")]; ok {
				pj = value
			}
		}
		if pi == pj {
			return values[i] < values[j]
		}
		return pi < pj
	})
}

func muzvizorLimitCandidates(query model.MetadataQuery, items []model.MetadataCandidate) []model.MetadataCandidate {
	matches := make([]model.MetadataCandidate, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		if muzvizorQueryFit(query, item) < 0 {
			continue
		}
		key := normalizeText(item.Artist) + "\x00" + normalizeText(item.Title) +
			"\x00" + formatMuzvizorBPM(item.BPM) + "\x00" + strings.ToUpper(strings.TrimSpace(item.Key))
		if seen[key] {
			continue
		}
		seen[key] = true
		matches = append(matches, item)
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return muzvizorQueryFit(query, matches[i]) > muzvizorQueryFit(query, matches[j])
	})
	if len(matches) > 12 {
		matches = matches[:12]
	}
	return matches
}

func muzvizorFallbackError(directErr, fallbackErr error) error {
	switch {
	case fallbackErr == nil:
		return nil
	case directErr == nil:
		return fallbackErr
	default:
		return fmt.Errorf("MUZVIZOR public search failed (%v); genre fallback failed: %w", directErr, fallbackErr)
	}
}
