package metadata

import (
	"log"
	"os"
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

type muzvizorDocumentDebugStats struct {
	Bytes          int
	Rows           int
	TitleColumns   int
	BPMColumns     int
	KeyColumns     int
	GenreColumns   int
	VisibleLines   int
	ContainsArtist bool
	ContainsTitle  bool
	Preview        string
}

func muzvizorDebugEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("CCML_MUZVIZOR_DEBUG")))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func muzvizorDebugf(format string, args ...any) {
	if !muzvizorDebugEnabled() {
		return
	}
	log.Printf("[MUZVIZOR DEBUG] "+format, args...)
}

func muzvizorDebugDocument(label, target string, query model.MetadataQuery, doc string) {
	if !muzvizorDebugEnabled() {
		return
	}
	stats := muzvizorDebugDocumentStatsFor(query, doc)
	muzvizorDebugf(
		"%s url=%s bytes=%d rows=%d titleCols=%d bpmCols=%d keyCols=%d genreCols=%d visibleLines=%d containsArtist=%t containsTitle=%t preview=%q",
		label,
		target,
		stats.Bytes,
		stats.Rows,
		stats.TitleColumns,
		stats.BPMColumns,
		stats.KeyColumns,
		stats.GenreColumns,
		stats.VisibleLines,
		stats.ContainsArtist,
		stats.ContainsTitle,
		stats.Preview,
	)
}

func muzvizorDebugCandidates(label string, query model.MetadataQuery, items []model.MetadataCandidate) {
	if !muzvizorDebugEnabled() {
		return
	}
	muzvizorDebugf("%s candidates=%d", label, len(items))
	limit := len(items)
	if limit > 8 {
		limit = 8
	}
	for i := 0; i < limit; i++ {
		item := items[i]
		muzvizorDebugf(
			"%s candidate[%d] artist=%q title=%q bpm=%g key=%q genre=%q stage=%q fit=%.3f sourceURL=%s",
			label,
			i,
			item.Artist,
			item.Title,
			item.BPM,
			item.Key,
			item.Genre,
			item.Stage,
			muzvizorQueryFit(query, item),
			item.SourceURL,
		)
	}
}

func muzvizorDebugDocumentStatsFor(query model.MetadataQuery, doc string) muzvizorDocumentDebugStats {
	lines := muzvizorVisibleLines(doc)
	joined := strings.ToLower(strings.Join(lines, "\n"))
	artist := strings.ToLower(strings.TrimSpace(query.Artist))
	title := strings.ToLower(strings.TrimSpace(query.Title))

	return muzvizorDocumentDebugStats{
		Bytes:          len(doc),
		Rows:           len(muzvizorDOMDivBlocksByClass(doc, "track__row_main")),
		TitleColumns:   len(muzvizorDOMDivBlocksByClass(doc, "track__column_title")),
		BPMColumns:     len(muzvizorDOMDivBlocksByClass(doc, "track__column_bpm")),
		KeyColumns:     len(muzvizorDOMDivBlocksByClass(doc, "track__column_key")),
		GenreColumns:   len(muzvizorDOMDivBlocksByClass(doc, "track__column_genre")),
		VisibleLines:   len(lines),
		ContainsArtist: artist != "" && strings.Contains(joined, artist),
		ContainsTitle:  title != "" && strings.Contains(joined, title),
		Preview:        muzvizorDebugPreview(lines, artist, title),
	}
}

func muzvizorDebugPreview(lines []string, artist, title string) string {
	const maxLines = 12
	const maxRunesPerLine = 140

	selected := make([]string, 0, maxLines)
	for _, line := range lines {
		lower := strings.ToLower(line)
		if (artist != "" && strings.Contains(lower, artist)) ||
			(title != "" && strings.Contains(lower, title)) {
			selected = append(selected, muzvizorDebugTruncate(line, maxRunesPerLine))
			if len(selected) >= maxLines {
				break
			}
		}
	}

	if len(selected) == 0 {
		for _, line := range lines {
			selected = append(selected, muzvizorDebugTruncate(line, maxRunesPerLine))
			if len(selected) >= maxLines {
				break
			}
		}
	}
	return strings.Join(selected, " | ")
}

func muzvizorDebugTruncate(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "..."
}
