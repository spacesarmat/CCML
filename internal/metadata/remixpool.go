package metadata

import (
	"context"
	"crypto/sha1"
	"fmt"
	htmlstd "html"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

const remixpoolBaseURL = "https://remixpool.ru"

var (
	remixpoolScriptStyle = regexp.MustCompile(`(?is)<(?:script|style)\b[^>]*>.*?</(?:script|style)>`)
	remixpoolBreakTag    = regexp.MustCompile(`(?is)<br\s*/?>|</(?:div|p|li|tr|td|th|span|a|h[1-6]|section|article|button)>`)
	remixpoolAnyTag      = regexp.MustCompile(`(?is)<[^>]+>`)
	remixpoolSpace       = regexp.MustCompile(`\s+`)
	remixpoolCamelotKey  = regexp.MustCompile(`^(?:[1-9]|1[0-2])[AB]$`)
)

// RemixPoolProvider reads metadata visible on RemixPool's public new-releases
// page. It does not authenticate, stream, download audio, or attempt to bypass
// subscription controls.
type RemixPoolProvider struct {
	client    *http.Client
	baseURL   string
	userAgent string
}

func NewRemixPoolProvider(userAgent string) *RemixPoolProvider {
	return newRemixPoolProviderWithBaseURL(remixpoolBaseURL, userAgent)
}

func newRemixPoolProviderWithBaseURL(baseURL, userAgent string) *RemixPoolProvider {
	return &RemixPoolProvider{
		client:    &http.Client{Timeout: 15 * time.Second},
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		userAgent: strings.TrimSpace(userAgent),
	}
}

func (p *RemixPoolProvider) Name() string { return "RemixPool" }
func (p *RemixPoolProvider) Kind() string { return ProviderKindDJPool }

func (p *RemixPoolProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	if strings.TrimSpace(query.Title) == "" && strings.TrimSpace(query.Artist) == "" {
		return nil, fmt.Errorf("artist or title is required")
	}

	target := p.baseURL + "/new-releases/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("create RemixPool request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.7")
	if p.userAgent != "" {
		req.Header.Set("User-Agent", p.userAgent)
	} else {
		req.Header.Set("User-Agent", "CCML metadata client")
	}

	body, err := fetchProviderBytes(ctx, p.client, req, p.Name(), 2)
	if err != nil {
		return nil, err
	}

	items := remixpoolCandidatesFromHTML(string(body), target, query)
	sort.SliceStable(items, func(i, j int) bool {
		return remixpoolQueryFit(query, items[i]) > remixpoolQueryFit(query, items[j])
	})
	if len(items) > 12 {
		items = items[:12]
	}
	return items, nil
}

func remixpoolCandidatesFromHTML(doc, sourceURL string, query model.MetadataQuery) []model.MetadataCandidate {
	lines := remixpoolVisibleLines(doc)
	if len(lines) == 0 {
		return nil
	}

	out := make([]model.MetadataCandidate, 0, 8)
	seen := map[string]bool{}

	for i := 0; i < len(lines); i++ {
		bpm, ok := parseRemixPoolBPM(lines[i])
		if !ok {
			continue
		}
		if i < 2 || i+1 >= len(lines) {
			continue
		}

		key, ok := parseRemixPoolCamelot(lines[i+1])
		if !ok {
			continue
		}

		title := strings.TrimSpace(lines[i-2])
		artist := strings.TrimSpace(lines[i-1])
		if remixpoolNoiseLine(title) || remixpoolNoiseLine(artist) {
			continue
		}

		genre := ""
		if i+2 < len(lines) && looksLikeRemixPoolGenre(lines[i+2]) {
			genre = strings.TrimSpace(lines[i+2])
		}

		item := model.MetadataCandidate{
			Source:     "RemixPool",
			SourceKind: ProviderKindDJPool,
			ExternalID: remixpoolExternalID(artist, title, bpm, key),
			SourceURL:  sourceURL,
			Title:      title,
			Artist:     artist,
			Genre:      genre,
			BPM:        bpm,
			Key:        key,
			KeyScale:   "camelot",
		}
		if remixpoolQueryFit(query, item) < 0 {
			continue
		}

		dedupeKey := normalizeText(item.Artist) + "\x00" + normalizeText(item.Title) +
			"\x00" + formatRemixPoolBPM(item.BPM) + "\x00" + item.Key
		if seen[dedupeKey] {
			continue
		}
		seen[dedupeKey] = true
		out = append(out, item)
	}
	return out
}

func remixpoolVisibleLines(doc string) []string {
	doc = remixpoolScriptStyle.ReplaceAllString(doc, " ")
	doc = remixpoolBreakTag.ReplaceAllString(doc, "\n")
	doc = remixpoolAnyTag.ReplaceAllString(doc, " ")
	doc = htmlstd.UnescapeString(doc)

	raw := strings.Split(doc, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(remixpoolSpace.ReplaceAllString(line, " "))
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func parseRemixPoolBPM(raw string) (float64, bool) {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), ",", ".")
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 20 || value > 300 {
		return 0, false
	}
	return value, true
}

func parseRemixPoolCamelot(raw string) (string, bool) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if !remixpoolCamelotKey.MatchString(value) {
		return "", false
	}
	return value, true
}

func remixpoolNoiseLine(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return true
	}
	switch value {
	case "новинки", "фильтр", "(не выбрано)", "записи", "скорость", "тональность",
		"жанр", "скачать", "скачать все", "горячие треки", "топ скачиваний",
		"аккаунт", "вход", "управление подпиской", "оформите подписку":
		return true
	}
	if strings.HasPrefix(value, "из ") || strings.HasPrefix(value, "© ") {
		return true
	}
	return false
}

func looksLikeRemixPoolGenre(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > 60 {
		return false
	}
	if remixpoolNoiseLine(value) {
		return false
	}
	if _, ok := parseRemixPoolBPM(value); ok {
		return false
	}
	if _, ok := parseRemixPoolCamelot(value); ok {
		return false
	}

	normalized := strings.ToLower(value)
	known := []string{
		"хаус", "русское", "эдит", "ремикс", "поп", "трэп", "мумбатон", "слив",
		"house", "edit", "remix", "pop", "trap", "moombahton", "hip-hop", "hip hop",
		"afro", "baile funk", "breakbeat", "jersey", "dance", "dnb", "dubstep",
		"techno", "tech house", "deep house", "latin", "r&b", "rnb", "rock",
	}
	for _, item := range known {
		if normalized == item || strings.Contains(normalized, item) {
			return true
		}
	}
	return false
}

func remixpoolQueryFit(query model.MetadataQuery, candidate model.MetadataCandidate) float64 {
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

func remixpoolExternalID(artist, title string, bpm float64, key string) string {
	sum := sha1.Sum([]byte(
		normalizeText(artist) + "\x00" +
			normalizeText(title) + "\x00" +
			formatRemixPoolBPM(bpm) + "\x00" +
			strings.ToUpper(strings.TrimSpace(key)),
	))
	return fmt.Sprintf("%x", sum[:8])
}

func formatRemixPoolBPM(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
