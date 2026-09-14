package metadata

import (
	"context"
	"crypto/sha1"
	"fmt"
	htmlstd "html"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

const muzvizorBaseURL = "https://muzvizor.com"

var (
	muzvizorScriptStyle = regexp.MustCompile(`(?is)<(?:script|style)\b[^>]*>.*?</(?:script|style)>`)
	muzvizorBreakTag    = regexp.MustCompile(`(?is)<br\s*/?>|</(?:div|p|li|tr|td|th|span|a|h[1-6]|section|article)>`)
	muzvizorAnyTag      = regexp.MustCompile(`(?is)<[^>]+>`)
	muzvizorSpace       = regexp.MustCompile(`\s+`)
	muzvizorCamelotKey  = regexp.MustCompile(`^(?:[1-9]|1[0-2])[AB]$`)
	muzvizorNumber      = regexp.MustCompile(`^\d{1,4}$`)
	muzvizorRussianDate = regexp.MustCompile(`(?i)^\d{1,2}\s+(?:января|февраля|марта|апреля|мая|июня|июля|августа|сентября|октября|ноября|декабря)(?:\s+\d{4})?$`)
)

// MuzvizorProvider reads public track metadata exposed by MUZVIZOR.
//
// It intentionally does not authenticate, download audio, or attempt to bypass
// subscription/anti-bot controls. Only metadata visible through public pages is
// consumed.
type MuzvizorProvider struct {
	client    *http.Client
	baseURL   string
	userAgent string
}

func NewMuzvizorProvider(userAgent string) *MuzvizorProvider {
	return newMuzvizorProviderWithBaseURL(muzvizorBaseURL, userAgent)
}

func newMuzvizorProviderWithBaseURL(baseURL, userAgent string) *MuzvizorProvider {
	return &MuzvizorProvider{
		client:    &http.Client{Timeout: 15 * time.Second},
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		userAgent: strings.TrimSpace(userAgent),
	}
}

func (p *MuzvizorProvider) Name() string { return "MUZVIZOR" }
func (p *MuzvizorProvider) Kind() string { return ProviderKindDJPool }

func (p *MuzvizorProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	term := muzvizorSearchTerm(query)
	if term == "" {
		return nil, fmt.Errorf("artist or title is required")
	}

	// First try the confirmed public search route. MUZVIZOR can return only a
	// JavaScript shell to a backend HTTP client even though the browser later
	// renders track rows. If no rows are available, use public genre pages.
	searchURL := muzvizorSearchURL(p.baseURL, term)

	var directErr error
	doc, err := p.fetchHTML(ctx, searchURL)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		directErr = err
	} else if collected := muzvizorCandidatesFromHTML(doc, searchURL, query); len(collected) > 0 {
		return muzvizorLimitCandidates(query, collected), nil
	}

	fallback, fallbackErr := p.searchPublicGenrePages(ctx, query)
	if len(fallback) > 0 {
		return fallback, nil
	}
	if err := muzvizorFallbackError(directErr, fallbackErr); err != nil {
		return nil, err
	}
	return nil, nil
}

func muzvizorSearchTerm(query model.MetadataQuery) string {
	artist := strings.TrimSpace(query.Artist)
	title := strings.TrimSpace(query.Title)
	switch {
	case artist != "" && title != "":
		return artist + " - " + title
	case artist != "":
		return artist
	default:
		return title
	}
}

func muzvizorSearchURL(baseURL, term string) string {
	// MUZVIZOR's browser search uses encodeURIComponent-style escaping.
	// url.Values.Encode would turn spaces into '+', while the site-generated
	// public URL uses %20. Keep the backend request byte-compatible with it.
	return strings.TrimRight(baseURL, "/") + "/tracks?query=" + url.PathEscape(strings.TrimSpace(term))
}

func (p *MuzvizorProvider) fetchHTML(ctx context.Context, target string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", fmt.Errorf("create MUZVIZOR request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.7")
	req.Header.Set("Referer", p.baseURL+"/tracks")
	if p.userAgent != "" {
		req.Header.Set("User-Agent", p.userAgent)
	} else {
		req.Header.Set("User-Agent", "CCML metadata client")
	}

	body, err := fetchProviderBytes(ctx, p.client, req, p.Name(), 1)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func muzvizorCandidatesFromHTML(doc, sourceURL string, query model.MetadataQuery) []model.MetadataCandidate {
	if items, rowsFound := muzvizorRenderedCandidatesFromHTML(doc, sourceURL, query); rowsFound && len(items) > 0 {
		return items
	}
	// A rendered row can contain decorative/badge text that makes a strict DOM
	// extraction fail even though the visible track sequence is complete. Do not
	// let the mere presence of track__row_main suppress the proven text parser.
	lines := muzvizorVisibleLines(doc)
	if len(lines) == 0 {
		return nil
	}

	type ranked struct {
		item model.MetadataCandidate
		fit  float64
	}
	var matches []ranked

	for i := 0; i < len(lines); i++ {
		bpm, ok := parseMuzvizorBPM(lines[i])
		if !ok {
			continue
		}

		keyIndex := -1
		key := ""
		for j := i + 1; j <= i+2 && j < len(lines); j++ {
			if parsed, ok := parseMuzvizorCamelot(lines[j]); ok {
				keyIndex = j
				key = parsed
				break
			}
			if !muzvizorNoiseLine(lines[j]) {
				break
			}
		}
		if keyIndex < 0 {
			continue
		}

		artist, title := muzvizorPreviousArtistTitle(lines, i)
		if title == "" || artist == "" {
			continue
		}

		genreParts := make([]string, 0, 2)
		for j := keyIndex + 1; j < len(lines) && j <= keyIndex+3; j++ {
			if !looksLikeMuzvizorGenre(lines[j]) {
				break
			}
			part := strings.Trim(strings.TrimSpace(lines[j]), ",")
			if part != "" {
				genreParts = append(genreParts, part)
			}
		}

		item := model.MetadataCandidate{
			Source:     "MUZVIZOR",
			SourceKind: ProviderKindDJPool,
			ExternalID: muzvizorExternalID(artist, title, bpm, key),
			SourceURL:  sourceURL,
			Title:      title,
			Artist:     artist,
			Genre:      strings.Join(genreParts, ", "),
			BPM:        bpm,
			Key:        key,
			KeyScale:   "camelot",
		}
		fit := muzvizorQueryFit(query, item)
		if fit < 0 {
			continue
		}
		matches = append(matches, ranked{item: item, fit: fit})
	}

	sort.SliceStable(matches, func(i, j int) bool { return matches[i].fit > matches[j].fit })
	out := make([]model.MetadataCandidate, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		key := normalizeText(match.item.Artist) + "\x00" + normalizeText(match.item.Title) +
			"\x00" + formatMuzvizorBPM(match.item.BPM) + "\x00" + match.item.Key
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, match.item)
	}
	return out
}

func muzvizorVisibleLines(doc string) []string {
	doc = muzvizorScriptStyle.ReplaceAllString(doc, " ")
	doc = muzvizorBreakTag.ReplaceAllString(doc, "\n")
	doc = muzvizorAnyTag.ReplaceAllString(doc, " ")
	doc = htmlstd.UnescapeString(doc)

	raw := strings.Split(doc, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(muzvizorSpace.ReplaceAllString(line, " "))
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func parseMuzvizorBPM(raw string) (float64, bool) {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), ",", ".")
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 20 || value > 300 {
		return 0, false
	}
	return value, true
}

func parseMuzvizorCamelot(raw string) (string, bool) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if !muzvizorCamelotKey.MatchString(value) {
		return "", false
	}
	return value, true
}

func muzvizorPreviousArtistTitle(lines []string, bpmIndex int) (artist, title string) {
	values := make([]string, 0, 2)
	for i := bpmIndex - 1; i >= 0 && len(values) < 2; i-- {
		value := strings.TrimSpace(lines[i])
		if muzvizorNoiseLine(value) {
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
	if len(values) < 2 {
		return "", ""
	}
	return values[0], values[1]
}

func muzvizorNoiseLine(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return true
	}
	if muzvizorRussianDate.MatchString(normalized) {
		return true
	}
	switch normalized {
	case "сегодня", "вчера", "top 100", "топ 100", "топ★100",
		"загрузка треков...", "загрузка треков…", "перейти в поиск",
		"добавить в избранные", "добавить в избранные треки":
		return true
	}
	if strings.HasPrefix(normalized, "показаны ") ||
		strings.HasPrefix(normalized, "топ-") ||
		strings.HasPrefix(normalized, "авторизуйтесь") {
		return true
	}
	if muzvizorNumber.MatchString(normalized) {
		// Standalone track position/ranking numbers are not Artist/Title.
		return true
	}
	return false
}

func looksLikeMuzvizorGenre(value string) bool {
	normalized := strings.ToLower(strings.Trim(strings.TrimSpace(value), ","))
	if normalized == "" || len(normalized) > 60 {
		return false
	}
	known := []string{
		"pop", "hip-hop", "hip hop", "house", "deep", "deep house", "bass", "bass house",
		"big room", "rave", "dnb", "drum", "drum'n'bass", "drum & bass", "baile funk",
		"disco", "disco house", "funk", "jersey", "jersey club", "afro", "afro house",
		"tech", "tech house", "techno", "club", "trap", "breakbeat", "breaks", "uk",
		"2 step", "2step", "indie dance", "open format", "moombahton", "rock", "rnb",
		"r&b", "soul", "afrobeats", "amapiano", "electronic", "halfstep", "future beats",
		"nu disco", "organic", "progressive", "instrumental", "trance", "dubstep", "underground",
	}
	for _, item := range known {
		if normalized == item || strings.Contains(normalized, item+",") || strings.Contains(normalized, ", "+item) {
			return true
		}
	}
	return false
}

func muzvizorQueryFit(query model.MetadataQuery, candidate model.MetadataCandidate) float64 {
	titleQuery := strings.TrimSpace(query.Title)
	artistQuery := strings.TrimSpace(query.Artist)

	titleFit := 1.0
	if titleQuery != "" {
		titleFit = textSimilarity(titleQuery, candidate.Title)
		if queryBase := strings.TrimSpace(stripVersionText(titleQuery)); queryBase != "" {
			candidateBase := strings.TrimSpace(stripVersionText(candidate.Title))
			if candidateBase == "" {
				candidateBase = candidate.Title
			}
			if baseFit := textSimilarity(queryBase, candidateBase); baseFit > titleFit {
				titleFit = baseFit
			}
		}
		if titleFit < 0.48 {
			return -1
		}
	}

	artistFit := 1.0
	if artistQuery != "" {
		artistFit = textSimilarity(artistQuery, candidate.Artist)
		if artistFit < 0.30 {
			return -1
		}
	}

	return titleFit*0.72 + artistFit*0.28
}

func muzvizorExternalID(artist, title string, bpm float64, key string) string {
	sum := sha1.Sum([]byte(
		normalizeText(artist) + "\x00" +
			normalizeText(title) + "\x00" +
			formatMuzvizorBPM(bpm) + "\x00" +
			strings.ToUpper(strings.TrimSpace(key)),
	))
	return fmt.Sprintf("%x", sum[:8])
}

func formatMuzvizorBPM(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
