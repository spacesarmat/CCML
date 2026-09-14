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

const bananaStreetBaseURL = "https://bananastreet.ru"

var (
	bananaStreetScriptStyle = regexp.MustCompile(`(?is)<(?:script|style)\b[^>]*>.*?</(?:script|style)>`)
	bananaStreetBreakTag    = regexp.MustCompile(`(?is)<br\s*/?>|</(?:div|p|li|tr|td|th|span|a|h[1-6]|section|article|button)>`)
	bananaStreetAnyTag      = regexp.MustCompile(`(?is)<[^>]+>`)
	bananaStreetSpace       = regexp.MustCompile(`\s+`)
)

// BananaStreetProvider reads metadata exposed by Bananastreet's public search
// page. The confirmed public cards expose Artist, Title and Genre; BPM/Key are
// intentionally left empty unless the public site starts exposing them in a
// separately verified contract.
//
// The provider does not authenticate, stream, download audio, or use hidden
// application endpoints.
type BananaStreetProvider struct {
	client    *http.Client
	baseURL   string
	userAgent string
}

func NewBananaStreetProvider(userAgent string) *BananaStreetProvider {
	return newBananaStreetProviderWithBaseURL(bananaStreetBaseURL, userAgent)
}

func newBananaStreetProviderWithBaseURL(baseURL, userAgent string) *BananaStreetProvider {
	return &BananaStreetProvider{
		client:    &http.Client{Timeout: 15 * time.Second},
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		userAgent: strings.TrimSpace(userAgent),
	}
}

func (p *BananaStreetProvider) Name() string { return "Bananastreet" }
func (p *BananaStreetProvider) Kind() string { return ProviderKindDJPool }

func (p *BananaStreetProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	term := bananaStreetSearchTerm(query)
	if term == "" {
		return nil, fmt.Errorf("artist or title is required")
	}

	target := bananaStreetSearchURL(p.baseURL, term)
	bananaStreetDebugf("search start artist=%q title=%q term=%q url=%s", query.Artist, query.Title, term, target)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("create Bananastreet request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	if p.userAgent != "" {
		req.Header.Set("User-Agent", p.userAgent)
	} else {
		req.Header.Set("User-Agent", "CCML metadata client")
	}

	bananaStreetDebugf("HTTP GET %s", target)
	body, err := fetchProviderBytes(ctx, p.client, req, p.Name(), 2)
	if err != nil {
		bananaStreetDebugf("HTTP FAIL url=%s error=%v", target, err)
		return nil, err
	}
	bananaStreetDebugf("HTTP OK url=%s bytes=%d", target, len(body))

	doc := string(body)
	bananaStreetDebugDocument(target, query, doc)
	if bananaStreetDebugEnabled() {
		raw := bananaStreetCandidatesFromHTML(doc, target, model.MetadataQuery{})
		bananaStreetDebugCandidates("raw", query, raw)
	}
	items := bananaStreetCandidatesFromHTML(doc, target, query)
	bananaStreetDebugCandidates("matched", query, items)
	sort.SliceStable(items, func(i, j int) bool {
		return bananaStreetQueryFit(query, items[i]) > bananaStreetQueryFit(query, items[j])
	})
	if len(items) > 12 {
		items = items[:12]
	}
	bananaStreetDebugf("search finished matches=%d", len(items))
	return items, nil
}

func bananaStreetSearchTerm(query model.MetadataQuery) string {
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

func bananaStreetSearchURL(baseURL, term string) string {
	encoded := url.QueryEscape(strings.TrimSpace(term))
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	return strings.TrimRight(baseURL, "/") + "/search?q=" + encoded
}

func bananaStreetCandidatesFromHTML(doc, sourceURL string, query model.MetadataQuery) []model.MetadataCandidate {
	lines := bananaStreetVisibleLines(doc)
	if len(lines) == 0 {
		return nil
	}

	out := make([]model.MetadataCandidate, 0, 8)
	seen := map[string]bool{}
	add := func(item model.MetadataCandidate) {
		if bananaStreetQueryFit(query, item) < 0 {
			return
		}
		key := normalizeText(item.Artist) + "\x00" + normalizeText(item.Title) + "\x00" + normalizeText(item.Genre)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, item)
	}

	// The public search page can render compact track results as one identity
	// line: "Artist - Title". The following line may be an uploader/author and
	// must not be treated as the full track artist. Try every separator and keep
	// only the split that conservatively matches the lookup query.
	if strings.TrimSpace(query.Artist) != "" && strings.TrimSpace(query.Title) != "" {
		for _, line := range lines {
			if item, ok := bananaStreetCombinedLineCandidate(line, sourceURL, query); ok {
				add(item)
			}
		}
	}

	// Older/public release cards are rendered in the observed order:
	// Title, Artist, like count, comment count, Genre, stream count.
	// Numeric counters are deliberately ignored; they are only structural
	// anchors so navigation text is not mistaken for metadata.
	for i := 0; i+5 < len(lines); i++ {
		title := strings.TrimSpace(lines[i])
		artist := strings.TrimSpace(lines[i+1])
		if bananaStreetNoiseLine(title) || bananaStreetNoiseLine(artist) {
			continue
		}
		if !bananaStreetCountLine(lines[i+2]) || !bananaStreetCountLine(lines[i+3]) {
			continue
		}
		genre := strings.TrimSpace(lines[i+4])
		if !looksLikeBananaStreetGenre(genre) {
			continue
		}
		if !bananaStreetCountLine(lines[i+5]) {
			continue
		}

		add(model.MetadataCandidate{
			Source:     "Bananastreet",
			SourceKind: ProviderKindDJPool,
			ExternalID: bananaStreetExternalID(artist, title, genre),
			SourceURL:  sourceURL,
			Title:      title,
			Artist:     artist,
			Genre:      genre,
		})
	}
	return out
}

func bananaStreetCombinedLineCandidate(line, sourceURL string, query model.MetadataQuery) (model.MetadataCandidate, bool) {
	line = strings.TrimSpace(line)
	if line == "" || !strings.Contains(line, " - ") {
		return model.MetadataCandidate{}, false
	}

	bestScore := -1.0
	best := model.MetadataCandidate{}
	searchFrom := 0
	for {
		relative := strings.Index(line[searchFrom:], " - ")
		if relative < 0 {
			break
		}
		index := searchFrom + relative
		artist := strings.TrimSpace(line[:index])
		title := strings.TrimSpace(line[index+3:])
		searchFrom = index + 3
		if artist == "" || title == "" || bananaStreetNoiseLine(artist) || bananaStreetNoiseLine(title) {
			continue
		}

		item := model.MetadataCandidate{
			Source:     "Bananastreet",
			SourceKind: ProviderKindDJPool,
			ExternalID: bananaStreetExternalID(artist, title, ""),
			SourceURL:  sourceURL,
			Title:      title,
			Artist:     artist,
		}
		score := bananaStreetQueryFit(query, item)
		if score < 0 {
			continue
		}

		// Compact search rows have fewer structural anchors than release cards,
		// so require a stronger identity fit before accepting them.
		titleFit := textSimilarity(query.Title, title)
		if queryBase := strings.TrimSpace(stripVersionText(query.Title)); queryBase != "" {
			candidateBase := strings.TrimSpace(stripVersionText(title))
			if candidateBase == "" {
				candidateBase = title
			}
			if baseFit := textSimilarity(queryBase, candidateBase); baseFit > titleFit {
				titleFit = baseFit
			}
		}
		artistFit := textSimilarity(query.Artist, artist)
		if titleFit < 0.62 || artistFit < 0.48 {
			continue
		}

		if score > bestScore {
			bestScore = score
			best = item
		}
	}
	if bestScore < 0 {
		return model.MetadataCandidate{}, false
	}
	return best, true
}

func bananaStreetVisibleLines(doc string) []string {
	doc = bananaStreetScriptStyle.ReplaceAllString(doc, " ")
	doc = bananaStreetBreakTag.ReplaceAllString(doc, "\n")
	doc = bananaStreetAnyTag.ReplaceAllString(doc, " ")
	doc = htmlstd.UnescapeString(doc)

	raw := strings.Split(doc, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(bananaStreetSpace.ReplaceAllString(line, " "))
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func bananaStreetCountLine(value string) bool {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\u00a0", "")
	value = strings.ReplaceAll(value, " ", "")
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	number, err := strconv.ParseInt(value, 10, 64)
	return err == nil && number >= 0
}

func bananaStreetNoiseLine(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return true
	}
	switch value {
	case "главное", "новинки", "популярное", "радио", "диджеи", "поиск", "стили",
		"плейлисты", "чарты", "топ 30", "еще", "войти", "смотреть всё", "что послушать",
		"популярно сейчас", "релизы", "треки", "исполнители", "результаты поиска":
		return true
	}
	return false
}

func looksLikeBananaStreetGenre(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len([]rune(value)) > 80 || bananaStreetNoiseLine(value) {
		return false
	}
	if bananaStreetCountLine(value) {
		return false
	}

	known := []string{
		"house", "deep house", "club house", "vocal house", "tech house", "afro house",
		"progressive house", "funky house", "disco house", "organic house", "electro house",
		"melodic house", "techno", "melodic techno", "trance", "progressive", "edm",
		"dance", "pop", "russian pop", "soul pop", "hip-hop", "hip hop", "trap",
		"r&b", "rnb", "soul", "rock", "electronica", "downtempo", "lounge", "ambient",
		"chillout", "dubstep", "brostep", "drum & bass", "dnb", "breakbeat", "hardstyle",
		"nu disco", "disco", "funk", "afrobeats", "baile funk", "mash up", "mashup",
		"indie dance", "open format", "moombahton", "latin", "balearic",
	}
	for _, genre := range known {
		if value == genre || strings.Contains(value, genre) {
			return true
		}
	}
	return false
}

func bananaStreetQueryFit(query model.MetadataQuery, candidate model.MetadataCandidate) float64 {
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

func bananaStreetExternalID(artist, title, genre string) string {
	sum := sha1.Sum([]byte(
		normalizeText(artist) + "\x00" +
			normalizeText(title) + "\x00" +
			normalizeText(genre),
	))
	return fmt.Sprintf("%x", sum[:8])
}
