package metadata

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/spacesarmat/CCML/internal/model"
)

const maxRankedCandidates = 40

// ScoreCandidate computes a normalized 0..1 match score from the local track
// and a provider candidate. The weights favor title/artist while still using
// album, duration, identifiers, and metadata completeness as tie-breakers.
func ScoreCandidate(query model.MetadataQuery, candidate model.MetadataCandidate) model.MetadataScore {
	title := textSimilarity(query.Title, candidate.Title)
	artist := textSimilarity(query.Artist, candidate.Artist)
	album := 0.0
	if strings.TrimSpace(query.Album) != "" && strings.TrimSpace(candidate.Album) != "" {
		album = textSimilarity(query.Album, candidate.Album)
	}
	duration := durationSimilarity(query.DurationMS, candidate.DurationMS)
	identifier := 0.0
	if strings.TrimSpace(query.ISRC) != "" && strings.TrimSpace(candidate.ISRC) != "" {
		if normalizeIdentifier(query.ISRC) == normalizeIdentifier(candidate.ISRC) {
			identifier = 1
		}
	}
	completeness := candidateCompleteness(candidate)

	// When album/ISRC are unavailable, their weights are redistributed into the
	// strong text match dimensions so sparse providers are not unfairly punished.
	weights := map[string]float64{
		"title": 0.34, "artist": 0.29, "album": 0.12,
		"duration": 0.10, "identifier": 0.12, "completeness": 0.03,
	}
	available := map[string]bool{
		"title":        strings.TrimSpace(query.Title) != "" && strings.TrimSpace(candidate.Title) != "",
		"artist":       strings.TrimSpace(query.Artist) != "" && strings.TrimSpace(candidate.Artist) != "",
		"album":        strings.TrimSpace(query.Album) != "" && strings.TrimSpace(candidate.Album) != "",
		"duration":     query.DurationMS > 0 && candidate.DurationMS > 0,
		"identifier":   strings.TrimSpace(query.ISRC) != "" && strings.TrimSpace(candidate.ISRC) != "",
		"completeness": true,
	}
	values := map[string]float64{
		"title": title, "artist": artist, "album": album,
		"duration": duration, "identifier": identifier, "completeness": completeness,
	}

	var weighted, totalWeight float64
	for key, weight := range weights {
		if !available[key] {
			continue
		}
		weighted += values[key] * weight
		totalWeight += weight
	}
	if totalWeight == 0 {
		totalWeight = 1
	}
	total := clamp01(weighted / totalWeight)

	// Exact ISRC is a very strong signal. Keep text mismatches visible, but do
	// not let minor punctuation/version differences bury an identifier match.
	if identifier == 1 && total < 0.92 {
		total = 0.92
	}

	return model.MetadataScore{
		Title: title, Artist: artist, Album: album, Duration: duration,
		Identifier: identifier, Completeness: completeness, Total: total,
	}
}

func rankCandidates(query model.MetadataQuery, items []model.MetadataCandidate) []model.MetadataCandidate {
	for i := range items {
		items[i].Score = ScoreCandidate(query, items[i])
		items[i].Confidence = items[i].Score.Total
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Confidence == items[j].Confidence {
			return candidateCompleteness(items[i]) > candidateCompleteness(items[j])
		}
		return items[i].Confidence > items[j].Confidence
	})
	items = dedupeCandidates(items)
	if len(items) > maxRankedCandidates {
		items = items[:maxRankedCandidates]
	}
	return items
}

func dedupeCandidates(items []model.MetadataCandidate) []model.MetadataCandidate {
	seen := make(map[string]struct{}, len(items))
	out := make([]model.MetadataCandidate, 0, len(items))
	for _, item := range items {
		key := normalizeText(item.Artist) + "\x00" + normalizeText(item.Title) + "\x00" + normalizeText(item.Album) + "\x00" + normalizeIdentifier(item.ISRC)
		if key == "\x00\x00\x00" {
			key = item.Source + "\x00" + item.ExternalID
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func buildSuggested(items []model.MetadataCandidate) model.MetadataCandidate {
	if len(items) == 0 {
		return model.MetadataCandidate{}
	}
	base := items[0]
	base.Source = "CCML Merge"
	base.ExternalID = ""
	base.SourceURL = ""

	pickString := func(get func(model.MetadataCandidate) string) string {
		for _, item := range items {
			if v := strings.TrimSpace(get(item)); v != "" {
				return v
			}
		}
		return ""
	}
	pickInt := func(get func(model.MetadataCandidate) int) int {
		for _, item := range items {
			if v := get(item); v > 0 {
				return v
			}
		}
		return 0
	}

	base.AlbumArtist = pickString(func(c model.MetadataCandidate) string { return c.AlbumArtist })
	base.ReleaseDate = pickString(func(c model.MetadataCandidate) string { return c.ReleaseDate })
	base.Genre = pickString(func(c model.MetadataCandidate) string { return c.Genre })
	base.Label = pickString(func(c model.MetadataCandidate) string { return c.Label })
	base.CatalogNumber = pickString(func(c model.MetadataCandidate) string { return c.CatalogNumber })
	base.ISRC = pickString(func(c model.MetadataCandidate) string { return c.ISRC })
	base.TrackNumber = pickInt(func(c model.MetadataCandidate) int { return c.TrackNumber })
	base.TrackTotal = pickInt(func(c model.MetadataCandidate) int { return c.TrackTotal })
	base.DiscNumber = pickInt(func(c model.MetadataCandidate) int { return c.DiscNumber })
	base.DiscTotal = pickInt(func(c model.MetadataCandidate) int { return c.DiscTotal })
	if base.Year == 0 {
		base.Year = pickInt(func(c model.MetadataCandidate) int { return c.Year })
	}

	// Artwork may only be auto-selected from providers that explicitly allow
	// CCML to download/embed it. Prefer the largest known image.
	bestArea := -1
	base.ArtworkURL = ""
	base.ArtworkWidth = 0
	base.ArtworkHeight = 0
	base.ArtworkEmbeddable = false
	minimumArtworkConfidence := items[0].Confidence - 0.05
	if minimumArtworkConfidence < 0.75 {
		minimumArtworkConfidence = 0.75
	}
	for _, item := range items {
		if item.Confidence < minimumArtworkConfidence || !item.ArtworkEmbeddable || strings.TrimSpace(item.ArtworkURL) == "" {
			continue
		}
		area := item.ArtworkWidth * item.ArtworkHeight
		if area == 0 {
			area = 1
		}
		if area > bestArea {
			bestArea = area
			base.ArtworkURL = item.ArtworkURL
			base.ArtworkWidth = item.ArtworkWidth
			base.ArtworkHeight = item.ArtworkHeight
			base.ArtworkEmbeddable = true
		}
	}
	return base
}

func buildFieldOptions(items []model.MetadataCandidate) []model.MetadataFieldOption {
	type strField struct {
		name string
		get  func(model.MetadataCandidate) string
	}
	stringFields := []strField{
		{"title", func(c model.MetadataCandidate) string { return c.Title }},
		{"artist", func(c model.MetadataCandidate) string { return c.Artist }},
		{"album", func(c model.MetadataCandidate) string { return c.Album }},
		{"albumArtist", func(c model.MetadataCandidate) string { return c.AlbumArtist }},
		{"releaseDate", func(c model.MetadataCandidate) string { return c.ReleaseDate }},
		{"genre", func(c model.MetadataCandidate) string { return c.Genre }},
		{"label", func(c model.MetadataCandidate) string { return c.Label }},
		{"catalogNumber", func(c model.MetadataCandidate) string { return c.CatalogNumber }},
		{"isrc", func(c model.MetadataCandidate) string { return c.ISRC }},
	}
	type intField struct {
		name string
		get  func(model.MetadataCandidate) int
	}
	intFields := []intField{
		{"year", func(c model.MetadataCandidate) int { return c.Year }},
		{"trackNumber", func(c model.MetadataCandidate) int { return c.TrackNumber }},
		{"trackTotal", func(c model.MetadataCandidate) int { return c.TrackTotal }},
		{"discNumber", func(c model.MetadataCandidate) int { return c.DiscNumber }},
		{"discTotal", func(c model.MetadataCandidate) int { return c.DiscTotal }},
	}

	options := make([]model.MetadataFieldOption, 0, len(items)*5)
	seen := map[string]struct{}{}
	for _, item := range items {
		for _, field := range stringFields {
			value := strings.TrimSpace(field.get(item))
			if value == "" {
				continue
			}
			key := field.name + "\x00" + normalizeText(value)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			options = append(options, model.MetadataFieldOption{Field: field.name, Value: value, Source: item.Source, ExternalID: item.ExternalID, Confidence: item.Confidence})
		}
		for _, field := range intFields {
			value := field.get(item)
			if value <= 0 {
				continue
			}
			key := field.name + "\x00" + strconv.Itoa(value)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			options = append(options, model.MetadataFieldOption{Field: field.name, Number: value, Source: item.Source, ExternalID: item.ExternalID, Confidence: item.Confidence})
		}
	}
	return options
}

func textSimilarity(a, b string) float64 {
	a = normalizeText(a)
	b = normalizeText(b)
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	// Sørensen–Dice on rune bigrams is tolerant of punctuation and small edits.
	ab := bigrams(a)
	bb := bigrams(b)
	if len(ab) == 0 || len(bb) == 0 {
		return 0
	}
	counts := make(map[string]int, len(ab))
	for _, x := range ab {
		counts[x]++
	}
	common := 0
	for _, x := range bb {
		if counts[x] > 0 {
			common++
			counts[x]--
		}
	}
	return clamp01(2 * float64(common) / float64(len(ab)+len(bb)))
}

func durationSimilarity(a, b int64) float64 {
	if a <= 0 || b <= 0 {
		return 0
	}
	delta := math.Abs(float64(a - b))
	switch {
	case delta <= 1000:
		return 1
	case delta <= 2500:
		return 0.9
	case delta <= 5000:
		return 0.7
	case delta <= 10000:
		return 0.35
	default:
		return 0
	}
}

func candidateCompleteness(c model.MetadataCandidate) float64 {
	values := []bool{
		strings.TrimSpace(c.Title) != "", strings.TrimSpace(c.Artist) != "",
		strings.TrimSpace(c.Album) != "", strings.TrimSpace(c.AlbumArtist) != "",
		strings.TrimSpace(c.Genre) != "", strings.TrimSpace(c.Label) != "",
		strings.TrimSpace(c.CatalogNumber) != "", strings.TrimSpace(c.ISRC) != "",
		c.Year > 0, c.TrackNumber > 0, c.DurationMS > 0,
	}
	count := 0
	for _, ok := range values {
		if ok {
			count++
		}
	}
	return float64(count) / float64(len(values))
}

func normalizeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastSpace := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func normalizeIdentifier(value string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func bigrams(value string) []string {
	runes := []rune(value)
	if len(runes) == 1 {
		return []string{string(runes)}
	}
	out := make([]string, 0, len(runes)-1)
	for i := 0; i+1 < len(runes); i++ {
		out = append(out, string(runes[i:i+2]))
	}
	return out
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

// metadataSimilarity is kept for provider adapters that only expose the core
// artist/title/duration tuple; the aggregation layer performs the final score.
func metadataSimilarity(query model.MetadataQuery, artist, title string, duration int64) float64 {
	return ScoreCandidate(query, model.MetadataCandidate{Artist: artist, Title: title, DurationMS: duration}).Total
}
