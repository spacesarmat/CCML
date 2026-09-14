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
// and a provider candidate. Besides ordinary text similarity it explicitly
// models mix/version semantics so a Radio Edit, Remix or Live recording cannot
// outrank the requested Original merely because the artist/title words match.
func ScoreCandidate(query model.MetadataQuery, candidate model.MetadataCandidate) model.MetadataScore {
	title := textSimilarity(stripVersionText(query.Title), stripVersionText(candidate.Title))
	artist := artistSimilarity(query.Artist, candidate.Artist)
	album := 0.0
	if strings.TrimSpace(query.Album) != "" && strings.TrimSpace(candidate.Album) != "" {
		album = textSimilarity(query.Album, candidate.Album)
	}
	version := versionSimilarity(query.Title, candidate.Title)
	duration := durationSimilarity(query.DurationMS, candidate.DurationMS)
	identifier := 0.0
	if strings.TrimSpace(query.ISRC) != "" && strings.TrimSpace(candidate.ISRC) != "" {
		if normalizeIdentifier(query.ISRC) == normalizeIdentifier(candidate.ISRC) {
			identifier = 1
		}
	}
	completeness := candidateCompleteness(candidate)

	weights := map[string]float64{
		"title": 0.29, "artist": 0.24, "album": 0.09, "version": 0.12,
		"duration": 0.11, "identifier": 0.13, "completeness": 0.02,
	}
	available := map[string]bool{
		"title":        strings.TrimSpace(query.Title) != "" && strings.TrimSpace(candidate.Title) != "",
		"artist":       strings.TrimSpace(query.Artist) != "" && strings.TrimSpace(candidate.Artist) != "",
		"album":        strings.TrimSpace(query.Album) != "" && strings.TrimSpace(candidate.Album) != "",
		"version":      strings.TrimSpace(query.Title) != "" && strings.TrimSpace(candidate.Title) != "",
		"duration":     query.DurationMS > 0 && candidate.DurationMS > 0,
		"identifier":   strings.TrimSpace(query.ISRC) != "" && strings.TrimSpace(candidate.ISRC) != "",
		"completeness": true,
	}
	values := map[string]float64{
		"title": title, "artist": artist, "album": album, "version": version,
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

	// Version mismatches are dangerous for a tagger: a 3:20 Radio Edit should
	// not be auto-applied to a 6:40 Extended Mix even when the textual stem is
	// identical. Exact ISRC remains authoritative and bypasses these caps.
	if identifier != 1 {
		switch {
		case version <= 0.05:
			total *= 0.52
		case version < 0.5:
			total *= 0.72
		case version < 0.8:
			total *= 0.88
		}
		if duration == 0 && query.DurationMS > 0 && candidate.DurationMS > 0 {
			total *= 0.78
		}
	}

	// Exact ISRC is the strongest catalog signal. Keep obvious text errors
	// visible in the breakdown, but do not bury an exact identifier match.
	if identifier == 1 && total < 0.96 {
		total = 0.96
	}

	return model.MetadataScore{
		Title: title, Artist: artist, Album: album, Version: version, Duration: duration,
		Identifier: identifier, Completeness: completeness, Total: clamp01(total),
	}
}

func rankCandidates(query model.MetadataQuery, items []model.MetadataCandidate) []model.MetadataCandidate {
	for i := range items {
		items[i].Score = ScoreCandidate(query, items[i])
		items[i].Confidence = items[i].Score.Total
		items[i].MatchIssues = candidateIssues(query, items[i])
		items[i].MatchClass = matchClass(items[i])
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

func matchClass(candidate model.MetadataCandidate) string {
	if candidate.Score.Identifier == 1 && candidate.Confidence >= 0.95 {
		return "exact"
	}
	switch {
	case candidate.Confidence >= 0.94 && candidate.Score.Title >= 0.94 && candidate.Score.Artist >= 0.90 && candidate.Score.Version >= 0.90:
		return "exact"
	case candidate.Confidence >= 0.84:
		return "high"
	case candidate.Confidence >= 0.70:
		return "medium"
	case candidate.Confidence >= 0.55:
		return "low"
	default:
		return "rejected"
	}
}

func candidateIssues(query model.MetadataQuery, candidate model.MetadataCandidate) []string {
	issues := make([]string, 0, 3)
	if strings.TrimSpace(query.Title) != "" && strings.TrimSpace(candidate.Title) != "" {
		v := versionSimilarity(query.Title, candidate.Title)
		if v < 0.5 {
			issues = append(issues, "version_mismatch")
		}
	}
	if query.DurationMS > 0 && candidate.DurationMS > 0 {
		delta := int64(math.Abs(float64(query.DurationMS - candidate.DurationMS)))
		if delta > 10000 {
			issues = append(issues, "duration_mismatch")
		}
	}
	if strings.TrimSpace(query.ISRC) != "" && strings.TrimSpace(candidate.ISRC) != "" && normalizeIdentifier(query.ISRC) != normalizeIdentifier(candidate.ISRC) {
		issues = append(issues, "isrc_mismatch")
	}
	return issues
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
	items = trustedCandidates(items)
	if len(items) == 0 {
		return model.MetadataCandidate{}
	}
	base := items[0]
	base.Source = "CCML Merge"
	base.ExternalID = ""
	base.SourceURL = ""

	pickString := func(field string, get func(model.MetadataCandidate) string) string {
		bestValue := ""
		bestScore := -1.0
		for _, item := range items {
			v := strings.TrimSpace(get(item))
			if v == "" {
				continue
			}
			score := item.Confidence + sourceFieldBonus(field, item.Source)
			if score > bestScore {
				bestScore = score
				bestValue = v
			}
		}
		return bestValue
	}
	pickInt := func(field string, get func(model.MetadataCandidate) int) int {
		bestValue := 0
		bestScore := -1.0
		for _, item := range items {
			v := get(item)
			if v <= 0 {
				continue
			}
			score := item.Confidence + sourceFieldBonus(field, item.Source)
			if score > bestScore {
				bestScore = score
				bestValue = v
			}
		}
		return bestValue
	}
	pickFloat := func(field string, get func(model.MetadataCandidate) float64) float64 {
		bestValue := 0.0
		bestScore := -1.0
		for _, item := range items {
			v := get(item)
			if v <= 0 {
				continue
			}
			score := item.Confidence + sourceFieldBonus(field, item.Source)
			if score > bestScore {
				bestScore = score
				bestValue = v
			}
		}
		return bestValue
	}

	base.AlbumArtist = pickString("albumArtist", func(c model.MetadataCandidate) string { return c.AlbumArtist })
	base.ReleaseDate = pickString("releaseDate", func(c model.MetadataCandidate) string { return c.ReleaseDate })
	base.Genre = pickString("genre", func(c model.MetadataCandidate) string { return c.Genre })
	base.Label = pickString("label", func(c model.MetadataCandidate) string { return c.Label })
	base.CatalogNumber = pickString("catalogNumber", func(c model.MetadataCandidate) string { return c.CatalogNumber })
	base.ISRC = pickString("isrc", func(c model.MetadataCandidate) string { return c.ISRC })
	base.TrackNumber = pickInt("trackNumber", func(c model.MetadataCandidate) int { return c.TrackNumber })
	base.TrackTotal = pickInt("trackTotal", func(c model.MetadataCandidate) int { return c.TrackTotal })
	base.DiscNumber = pickInt("discNumber", func(c model.MetadataCandidate) int { return c.DiscNumber })
	base.DiscTotal = pickInt("discTotal", func(c model.MetadataCandidate) int { return c.DiscTotal })
	base.BPM = pickFloat("bpm", func(c model.MetadataCandidate) float64 { return c.BPM })
	base.Key = pickString("key", func(c model.MetadataCandidate) string { return c.Key })
	base.KeyScale = pickString("keyScale", func(c model.MetadataCandidate) string { return c.KeyScale })
	if base.Year == 0 {
		base.Year = pickInt("year", func(c model.MetadataCandidate) int { return c.Year })
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
	items = trustedCandidates(items)
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
		{"key", func(c model.MetadataCandidate) string { return c.Key }},
		{"keyScale", func(c model.MetadataCandidate) string { return c.KeyScale }},
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
	type floatField struct {
		name string
		get  func(model.MetadataCandidate) float64
	}
	floatFields := []floatField{
		{"bpm", func(c model.MetadataCandidate) float64 { return c.BPM }},
	}

	options := make([]model.MetadataFieldOption, 0, len(items)*8)
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
		for _, field := range floatFields {
			value := field.get(item)
			if value <= 0 {
				continue
			}
			key := field.name + "\x00" + strconv.FormatFloat(value, 'f', 3, 64)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			options = append(options, model.MetadataFieldOption{Field: field.name, Decimal: value, Source: item.Source, ExternalID: item.ExternalID, Confidence: item.Confidence})
		}
	}
	return options
}

type versionSignature map[string]struct{}

var versionRules = []struct {
	name  string
	terms []string
}{
	{"extended", []string{"extended", "extended mix", "extended version", "club mix", "club version", "12 inch", "12inch"}},
	{"radio", []string{"radio edit", "radio mix", "radio version", "single edit", "single version"}},
	{"remix", []string{"remix", "rmx", "rework", "bootleg", "mashup"}},
	{"remaster", []string{"remaster", "remastered", "remastered version", "digital remaster"}},
	{"live", []string{"live", "live version", "concert", "live at"}},
	{"instrumental", []string{"instrumental", "instrumental version"}},
	{"acoustic", []string{"acoustic", "unplugged"}},
	{"acapella", []string{"acapella", "acappella", "a cappella", "a capella"}},
	{"dub", []string{"dub", "dub mix", "dub version"}},
	{"edit", []string{"edit", "short edit", "video edit"}},
	{"clean", []string{"clean", "clean edit", "clean mix", "clean version"}},
	{"dirty", []string{"dirty", "dirty edit", "dirty mix", "dirty version", "explicit", "explicit version"}},
	{"intro", []string{"intro", "intro edit", "intro mix", "intro version"}},
	{"outro", []string{"outro", "outro edit", "outro mix", "outro version"}},
}

func versionSignatureFor(title string) versionSignature {
	norm := " " + normalizeText(title) + " "
	sig := versionSignature{}
	for _, rule := range versionRules {
		for _, term := range rule.terms {
			needle := " " + normalizeText(term) + " "
			if strings.Contains(norm, needle) {
				sig[rule.name] = struct{}{}
				break
			}
		}
	}
	// "Original Mix" explicitly describes the unmodified/original version.
	if strings.Contains(norm, " original mix ") || strings.Contains(norm, " original version ") {
		for k := range sig {
			if k == "edit" {
				delete(sig, k)
			}
		}
		sig["original"] = struct{}{}
	}
	return sig
}

func versionSimilarity(a, b string) float64 {
	sa := versionSignatureFor(a)
	sb := versionSignatureFor(b)
	if len(sa) == 0 && len(sb) == 0 {
		return 1
	}
	if _, ok := sa["original"]; ok && len(sa) == 1 && len(sb) == 0 {
		return 0.96
	}
	if _, ok := sb["original"]; ok && len(sb) == 1 && len(sa) == 0 {
		return 0.96
	}
	if len(sa) == 0 || len(sb) == 0 {
		return 0.28
	}
	common := 0
	for key := range sa {
		if _, ok := sb[key]; ok {
			common++
		}
	}
	if common == 0 {
		return 0
	}
	union := len(sa) + len(sb) - common
	return float64(common) / float64(union)
}

func stripVersionText(value string) string {
	result := strings.ToLower(value)
	terms := []string{"original version", "original mix"}
	for _, rule := range versionRules {
		terms = append(terms, rule.terms...)
	}
	sort.SliceStable(terms, func(i, j int) bool { return len(terms[i]) > len(terms[j]) })
	for _, term := range terms {
		result = strings.ReplaceAll(result, term, " ")
	}
	return strings.Join(strings.Fields(result), " ")
}

func artistSimilarity(a, b string) float64 {
	clean := func(value string) string {
		v := strings.ToLower(value)
		for _, marker := range []string{" feat. ", " feat ", " featuring ", " ft. ", " ft "} {
			v = strings.ReplaceAll(v, marker, " & ")
		}
		return v
	}
	return textSimilarity(clean(a), clean(b))
}

func sourceFieldBonus(field, source string) float64 {
	source = strings.ToLower(strings.TrimSpace(source))
	bonuses := map[string]map[string]float64{
		"genre":         {"discogs": 0.08, "traxsource": 0.09, "muzvizor": 0.10, "remixpool": 0.10, "bananastreet": 0.08, "yandex music": 0.03},
		"label":         {"discogs": 0.10, "traxsource": 0.10, "musicbrainz": 0.04},
		"catalogNumber": {"discogs": 0.12, "traxsource": 0.12, "musicbrainz": 0.03},
		"isrc":          {"musicbrainz": 0.10, "spotify": 0.10, "deezer": 0.08, "apple music": 0.08},
		"releaseDate":   {"musicbrainz": 0.08, "discogs": 0.07, "traxsource": 0.06, "apple music": 0.05},
		"year":          {"musicbrainz": 0.06, "discogs": 0.06, "traxsource": 0.05},
		"trackNumber":   {"musicbrainz": 0.05, "spotify": 0.05, "apple music": 0.05, "deezer": 0.04},
		"trackTotal":    {"musicbrainz": 0.05, "spotify": 0.05, "apple music": 0.05, "deezer": 0.04},
		"discNumber":    {"musicbrainz": 0.05, "spotify": 0.05, "apple music": 0.05},
		"discTotal":     {"musicbrainz": 0.05, "spotify": 0.05, "apple music": 0.05},
		"albumArtist":   {"musicbrainz": 0.05, "discogs": 0.04, "spotify": 0.04},
		"bpm":           {"muzvizor": 0.12, "remixpool": 0.12},
		"key":           {"muzvizor": 0.12, "remixpool": 0.12},
		"keyScale":      {"muzvizor": 0.12, "remixpool": 0.12},
	}
	if bySource, ok := bonuses[field]; ok {
		return bySource[source]
	}
	return 0
}

func trustedCandidates(items []model.MetadataCandidate) []model.MetadataCandidate {
	trusted := make([]model.MetadataCandidate, 0, len(items))
	for _, item := range items {
		if item.MatchClass != "rejected" {
			trusted = append(trusted, item)
		}
	}
	if len(trusted) == 0 {
		return items
	}
	return trusted
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
