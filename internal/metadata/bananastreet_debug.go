package metadata

import (
	"log"
	"os"
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

type bananaStreetDocumentDebugStats struct {
	Bytes          int
	VisibleLines   int
	ContainsArtist bool
	ContainsTitle  bool
	Preview        string
}

func bananaStreetDebugEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("CCML_BANANASTREET_DEBUG")))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func bananaStreetDebugf(format string, args ...any) {
	if !bananaStreetDebugEnabled() {
		return
	}
	log.Printf("[BANANASTREET DEBUG] "+format, args...)
}

func bananaStreetDebugDocument(target string, query model.MetadataQuery, doc string) {
	if !bananaStreetDebugEnabled() {
		return
	}
	stats := bananaStreetDebugDocumentStatsFor(query, doc)
	bananaStreetDebugf(
		"page url=%s bytes=%d visibleLines=%d containsArtist=%t containsTitle=%t preview=%q",
		target,
		stats.Bytes,
		stats.VisibleLines,
		stats.ContainsArtist,
		stats.ContainsTitle,
		stats.Preview,
	)
	bananaStreetDebugIdentityWindows(query, bananaStreetVisibleLines(doc))
}

func bananaStreetDebugCandidates(label string, query model.MetadataQuery, items []model.MetadataCandidate) {
	if !bananaStreetDebugEnabled() {
		return
	}
	bananaStreetDebugf("%s candidates=%d", label, len(items))
	limit := len(items)
	if limit > 12 {
		limit = 12
	}
	for index := 0; index < limit; index++ {
		item := items[index]
		bananaStreetDebugf(
			"%s candidate[%d] artist=%q title=%q genre=%q fit=%.3f sourceURL=%s",
			label,
			index,
			item.Artist,
			item.Title,
			item.Genre,
			bananaStreetQueryFit(query, item),
			item.SourceURL,
		)
	}
}

func bananaStreetDebugDocumentStatsFor(query model.MetadataQuery, doc string) bananaStreetDocumentDebugStats {
	lines := bananaStreetVisibleLines(doc)
	joined := strings.ToLower(strings.Join(lines, "\n"))
	artist := strings.ToLower(strings.TrimSpace(query.Artist))
	title := strings.ToLower(strings.TrimSpace(query.Title))

	return bananaStreetDocumentDebugStats{
		Bytes:          len(doc),
		VisibleLines:   len(lines),
		ContainsArtist: artist != "" && strings.Contains(joined, artist),
		ContainsTitle:  title != "" && strings.Contains(joined, title),
		Preview:        bananaStreetDebugPreview(lines, artist, title),
	}
}

func bananaStreetDebugPreview(lines []string, artist, title string) string {
	const maxLines = 12
	const maxRunesPerLine = 140

	selected := make([]string, 0, maxLines)
	for _, line := range lines {
		lower := strings.ToLower(line)
		if (artist != "" && strings.Contains(lower, artist)) ||
			(title != "" && strings.Contains(lower, title)) {
			selected = append(selected, bananaStreetDebugTruncate(line, maxRunesPerLine))
			if len(selected) >= maxLines {
				break
			}
		}
	}
	if len(selected) == 0 {
		for _, line := range lines {
			selected = append(selected, bananaStreetDebugTruncate(line, maxRunesPerLine))
			if len(selected) >= maxLines {
				break
			}
		}
	}
	return strings.Join(selected, " | ")
}

func bananaStreetDebugIdentityWindows(query model.MetadataQuery, lines []string) {
	if !bananaStreetDebugEnabled() {
		return
	}
	artist := strings.ToLower(strings.TrimSpace(query.Artist))
	title := strings.ToLower(strings.TrimSpace(query.Title))
	hits := 0
	for index, line := range lines {
		lower := strings.ToLower(line)
		if (artist == "" || !strings.Contains(lower, artist)) &&
			(title == "" || !strings.Contains(lower, title)) {
			continue
		}
		start := index - 4
		if start < 0 {
			start = 0
		}
		end := index + 7
		if end > len(lines) {
			end = len(lines)
		}
		bananaStreetDebugf("identity window line=%d values=%q", index, lines[start:end])
		hits++
		if hits >= 6 {
			break
		}
	}
	if hits == 0 {
		bananaStreetDebugf("identity windows=0")
	}
}

func bananaStreetDebugTruncate(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "..."
}
