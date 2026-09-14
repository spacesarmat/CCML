package metadata

import (
	"context"
	"crypto/sha1"
	"fmt"
	htmlstd "html"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

const jesteiBaseURL = "https://jesteipool.ru"

var (
	jesteiScriptStyle = regexp.MustCompile(`(?is)<(?:script|style)\b[^>]*>.*?</(?:script|style)>`)
	jesteiBreakTag    = regexp.MustCompile(`(?is)<br\s*/?>|</(?:div|p|li|tr|td|th|span|a|h[1-6]|section|article|button)>`)
	jesteiAnyTag      = regexp.MustCompile(`(?is)<[^>]+>`)
	jesteiSpace       = regexp.MustCompile(`\s+`)
	jesteiTitleTag    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	jesteiOGTitle     = regexp.MustCompile(`(?is)<meta[^>]+(?:property|name)\s*=\s*["']og:title["'][^>]+content\s*=\s*["']([^"']+)["']`)
	jesteiTrackPath   = regexp.MustCompile(`(?i)/track/([0-9]+)`)
	jesteiCamelotKey  = regexp.MustCompile(`^(?:[1-9]|1[0-2])[AB]$`)
)

type JesteiProvider struct {
	client    *http.Client
	baseURL   string
	userAgent string
}

func NewJesteiProvider(userAgent string) *JesteiProvider {
	return newJesteiProviderWithBaseURL(jesteiBaseURL, userAgent)
}

func newJesteiProviderWithBaseURL(baseURL, userAgent string) *JesteiProvider {
	return &JesteiProvider{
		client:    &http.Client{Timeout: 12 * time.Second},
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		userAgent: strings.TrimSpace(userAgent),
	}
}

func (p *JesteiProvider) Name() string { return "Jestei Pool" }
func (p *JesteiProvider) Kind() string { return ProviderKindDJPool }

func (p *JesteiProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	term := jesteiSearchTerm(query)
	if term == "" {
		return nil, fmt.Errorf("artist or title is required")
	}

	searchURL := jesteiSearchURL(p.baseURL, term)
	jesteiDebugf("search start artist=%q title=%q term=%q url=%s", query.Artist, query.Title, term, searchURL)

	searchBody, err := p.fetchHTML(ctx, searchURL)
	if err != nil {
		jesteiDebugf("search page FAIL url=%s error=%v", searchURL, err)
		return nil, err
	}
	jesteiDebugf("search page OK url=%s bytes=%d", searchURL, len(searchBody))

	trackURLs := jesteiTrackURLsFromHTML(string(searchBody), p.baseURL)
	jesteiDebugf("search page trackURLs=%d urls=%q", len(trackURLs), trackURLs)
	if len(trackURLs) > 6 {
		trackURLs = trackURLs[:6]
	}

	items := make([]model.MetadataCandidate, 0, len(trackURLs))
	seen := map[string]bool{}
	for _, trackURL := range trackURLs {
		if err := ctx.Err(); err != nil {
			return items, err
		}

		body, fetchErr := p.fetchHTML(ctx, trackURL)
		if fetchErr != nil {
			jesteiDebugf("track FAIL url=%s error=%v", trackURL, fetchErr)
			continue
		}
		jesteiDebugf("track OK url=%s bytes=%d", trackURL, len(body))

		item, ok := jesteiCandidateFromTrackHTML(string(body), trackURL, query)
		if !ok {
			jesteiDebugf("track rejected url=%s", trackURL)
			continue
		}
		fit := jesteiQueryFit(query, item)
		jesteiDebugf(
			"track candidate artist=%q title=%q genre=%q bpm=%.3f key=%q stage=%q fit=%.3f url=%s",
			item.Artist,
			item.Title,
			item.Genre,
			item.BPM,
			item.Key,
			item.Stage,
			fit,
			trackURL,
		)

		key := normalizeText(item.Artist) + "\x00" + normalizeText(item.Title) + "\x00" + item.ExternalID
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		return jesteiQueryFit(query, items[i]) > jesteiQueryFit(query, items[j])
	})
	if len(items) > 12 {
		items = items[:12]
	}
	jesteiDebugf("search finished matches=%d", len(items))
	return items, nil
}

func (p *JesteiProvider) fetchHTML(ctx context.Context, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("create Jestei Pool request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Referer", p.baseURL+"/home")
	if p.userAgent != "" {
		req.Header.Set("User-Agent", p.userAgent)
	} else {
		req.Header.Set("User-Agent", "CCML metadata client")
	}

	jesteiDebugf("HTTP GET %s", target)
	body, err := fetchProviderBytes(ctx, p.client, req, p.Name(), 1)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func jesteiSearchTerm(query model.MetadataQuery) string {
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

func jesteiSearchURL(baseURL, term string) string {
	encoded := url.QueryEscape(strings.TrimSpace(term))
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	return strings.TrimRight(baseURL, "/") + "/search?q=" + encoded
}

func jesteiTrackURLsFromHTML(doc, baseURL string) []string {
	matches := jesteiTrackPath.FindAllStringSubmatch(doc, -1)
	out := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		id := strings.TrimSpace(match[1])
		if id == "" {
			continue
		}
		target := strings.TrimRight(baseURL, "/") + "/track/" + id
		if seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}
	return out
}

func jesteiCandidateFromTrackHTML(doc, sourceURL string, query model.MetadataQuery) (model.MetadataCandidate, bool) {
	lines := jesteiVisibleLines(doc)
	if len(lines) == 0 {
		return model.MetadataCandidate{}, false
	}

	artist, title := jesteiIdentity(doc, lines, query)
	if artist == "" || title == "" {
		jesteiDebugf("identity miss url=%s titleTag=%q preview=%q", sourceURL, jesteiHTMLTitle(doc), jesteiPreview(lines))
		return model.MetadataCandidate{}, false
	}

	item := model.MetadataCandidate{
		Source:     "Jestei Pool",
		SourceKind: ProviderKindDJPool,
		ExternalID: jesteiExternalID(sourceURL, artist, title),
		SourceURL:  sourceURL,
		Title:      title,
		Artist:     artist,
		Genre:      jesteiGenres(lines),
		Stage:      jesteiStage(lines),
	}
	if bpm, ok := jesteiBPM(lines); ok {
		item.BPM = bpm
	}
	if key, ok := jesteiKey(lines); ok {
		item.Key = key
		item.KeyScale = "camelot"
	}

	if jesteiQueryFit(query, item) < 0 {
		return model.MetadataCandidate{}, false
	}
	return item, true
}

func jesteiIdentity(doc string, lines []string, query model.MetadataQuery) (string, string) {
	titleTag := jesteiHTMLTitle(doc)
	titleTag = jesteiStripSiteSuffix(titleTag)
	if artist, title, ok := jesteiSplitIdentity(titleTag, query); ok {
		return artist, title
	}

	bestScore := -1.0
	bestArtist := ""
	bestTitle := ""
	for i := 0; i < len(lines); i++ {
		artistCandidate := strings.TrimSpace(lines[i])
		if artistCandidate == "" || len([]rune(artistCandidate)) > 160 {
			continue
		}
		for j := i + 1; j < len(lines) && j <= i+4; j++ {
			titleCandidate := strings.TrimSpace(lines[j])
			if titleCandidate == "" || len([]rune(titleCandidate)) > 220 {
				continue
			}
			item := model.MetadataCandidate{Artist: artistCandidate, Title: titleCandidate}
			score := jesteiQueryFit(query, item)
			if score < 0 {
				continue
			}
			if artistSimilarity(query.Artist, artistCandidate) < 0.62 {
				continue
			}
			titleFit := textSimilarity(query.Title, titleCandidate)
			if base := strings.TrimSpace(stripVersionText(query.Title)); base != "" {
				candidateBase := strings.TrimSpace(stripVersionText(titleCandidate))
				if candidateBase == "" {
					candidateBase = titleCandidate
				}
				if baseFit := textSimilarity(base, candidateBase); baseFit > titleFit {
					titleFit = baseFit
				}
			}
			if titleFit < 0.70 {
				continue
			}
			if score > bestScore {
				bestScore = score
				bestArtist = artistCandidate
				bestTitle = titleCandidate
			}
		}
	}
	return bestArtist, bestTitle
}

func jesteiHTMLTitle(doc string) string {
	if match := jesteiOGTitle.FindStringSubmatch(doc); len(match) > 1 {
		return strings.TrimSpace(htmlstd.UnescapeString(jesteiAnyTag.ReplaceAllString(match[1], " ")))
	}
	if match := jesteiTitleTag.FindStringSubmatch(doc); len(match) > 1 {
		return strings.TrimSpace(htmlstd.UnescapeString(jesteiAnyTag.ReplaceAllString(match[1], " ")))
	}
	return ""
}

func jesteiStripSiteSuffix(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	for _, suffix := range []string{
		" | jestei pool",
		" — jestei pool",
		" - jestei pool",
		" | jesteipool",
		" — jesteipool",
		" - jesteipool",
	} {
		if strings.HasSuffix(lower, suffix) {
			return strings.TrimSpace(value[:len(value)-len(suffix)])
		}
	}
	return value
}

func jesteiSplitIdentity(value string, query model.MetadataQuery) (string, string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || !strings.Contains(value, " - ") {
		return "", "", false
	}

	bestScore := -1.0
	bestArtist := ""
	bestTitle := ""
	searchFrom := 0
	for {
		relative := strings.Index(value[searchFrom:], " - ")
		if relative < 0 {
			break
		}
		index := searchFrom + relative
		artist := strings.TrimSpace(value[:index])
		title := strings.TrimSpace(value[index+3:])
		searchFrom = index + 3
		if artist == "" || title == "" {
			continue
		}

		item := model.MetadataCandidate{Artist: artist, Title: title}
		score := jesteiQueryFit(query, item)
		if score < 0 {
			continue
		}

		artistFit := artistSimilarity(query.Artist, artist)
		titleFit := textSimilarity(query.Title, title)
		if base := strings.TrimSpace(stripVersionText(query.Title)); base != "" {
			titleBase := strings.TrimSpace(stripVersionText(title))
			if titleBase == "" {
				titleBase = title
			}
			if baseFit := textSimilarity(base, titleBase); baseFit > titleFit {
				titleFit = baseFit
			}
		}
		if artistFit < 0.62 || titleFit < 0.70 {
			continue
		}
		if score > bestScore {
			bestScore = score
			bestArtist = artist
			bestTitle = title
		}
	}
	if bestScore < 0 {
		return "", "", false
	}
	return bestArtist, bestTitle, true
}

func jesteiGenres(lines []string) string {
	start := -1
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), "это жанр трека") {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}

	out := make([]string, 0, 4)
	seen := map[string]bool{}
	for i := start; i < len(lines) && i < start+14; i++ {
		lower := strings.ToLower(strings.TrimSpace(lines[i]))
		if strings.Contains(lower, "это маркировки") {
			break
		}
		if !jesteiLooksLikeGenre(lines[i]) {
			continue
		}
		key := normalizeText(lines[i])
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, strings.TrimSpace(lines[i]))
	}
	return strings.Join(out, ", ")
}

func jesteiLooksLikeGenre(value string) bool {
	normalized := normalizeText(value)
	if normalized == "" {
		return false
	}
	known := []string{
		"afro house", "bass house", "bassline", "deep house", "disco house", "future house",
		"house", "melodic house", "mid tempo", "rave", "slap house", "tech house", "techno",
		"breakbeat", "drum bass", "drum and bass", "drum & bass", "dubstep", "future bass",
		"trap", "uk garage", "baile funk", "hip hop", "hip-hop", "phonk", "r&b", "rnb",
		"twerk", "amapiano", "arabic", "ethnic", "reggae", "hyperpop", "indie",
		"indie electronic", "jazz", "romantic", "lyric", "pop", "dance", "electronica",
		"nu disco", "disco", "funk", "latin",
	}
	for _, genre := range known {
		if normalized == normalizeText(genre) {
			return true
		}
	}
	return false
}

func jesteiBPM(lines []string) (float64, bool) {
	for i, line := range lines {
		if !strings.Contains(strings.ToLower(line), "количество ударов в минуту") {
			continue
		}
		for j := i - 1; j >= 0 && j >= i-4; j-- {
			raw := strings.ReplaceAll(strings.TrimSpace(lines[j]), ",", ".")
			value, err := strconv.ParseFloat(raw, 64)
			if err == nil && value >= 20 && value <= 300 {
				return value, true
			}
		}
	}
	return 0, false
}

func jesteiKey(lines []string) (string, bool) {
	found := ""
	for _, line := range lines {
		value := strings.ToUpper(strings.TrimSpace(line))
		if !jesteiCamelotKey.MatchString(value) {
			continue
		}
		if found != "" && found != value {
			return "", false
		}
		found = value
	}
	return found, found != ""
}

func jesteiStage(lines []string) string {
	for i, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		if !strings.Contains(lower, "для какой части ночи") {
			continue
		}
		for j := i + 1; j < len(lines) && j <= i+5; j++ {
			if stage := jesteiNormalizeStage(lines[j]); stage != "" {
				return stage
			}
		}
	}
	return ""
}

func jesteiNormalizeStage(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "primetime", "prime time", "праймтайм", "прайм-тайм":
		return "Prime Time"
	case "opening", "open", "открытие":
		return "Opening"
	case "closing", "close", "закрытие":
		return "Closing"
	default:
		return ""
	}
}

func jesteiVisibleLines(doc string) []string {
	doc = jesteiScriptStyle.ReplaceAllString(doc, " ")
	doc = jesteiBreakTag.ReplaceAllString(doc, "\n")
	doc = jesteiAnyTag.ReplaceAllString(doc, " ")
	doc = htmlstd.UnescapeString(doc)

	raw := strings.Split(doc, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(jesteiSpace.ReplaceAllString(line, " "))
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func jesteiQueryFit(query model.MetadataQuery, candidate model.MetadataCandidate) float64 {
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
		if titleFit < 0.54 {
			return -1
		}
	}

	artistFit := 1.0
	if artistQuery != "" {
		artistFit = artistSimilarity(artistQuery, candidate.Artist)
		if artistFit < 0.38 {
			return -1
		}
	}
	return titleFit*0.72 + artistFit*0.28
}

func jesteiExternalID(sourceURL, artist, title string) string {
	if match := jesteiTrackPath.FindStringSubmatch(sourceURL); len(match) > 1 {
		return "track:" + match[1]
	}
	sum := sha1.Sum([]byte(normalizeText(artist) + "\x00" + normalizeText(title)))
	return fmt.Sprintf("%x", sum[:8])
}

func jesteiPreview(lines []string) string {
	limit := len(lines)
	if limit > 14 {
		limit = 14
	}
	return strings.Join(lines[:limit], " | ")
}

func jesteiDebugEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("CCML_JESTEI_DEBUG")))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func jesteiDebugf(format string, args ...any) {
	if !jesteiDebugEnabled() {
		return
	}
	log.Printf("[JESTEI DEBUG] "+format, args...)
}
