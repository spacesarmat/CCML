package library

import (
	"sort"
	"strings"
	"unicode"

	"github.com/your-github/ccml/internal/model"
)

// FindDuplicates groups tracks by normalized artist/title and near-equal duration.
// Fingerprinting can later be added without changing the UI contract.
func FindDuplicates(tracks []model.Track, toleranceMS int64) []model.DuplicateGroup {
	byMetadata := make(map[string][]model.Track)
	for _, track := range tracks {
		artist := normalizeText(track.Artist)
		title := normalizeText(track.Title)
		if artist == "" || title == "" || track.DurationMS <= 0 {
			continue
		}
		key := artist + "\x00" + title
		byMetadata[key] = append(byMetadata[key], track)
	}

	groups := make([]model.DuplicateGroup, 0)
	for _, candidates := range byMetadata {
		if len(candidates) < 2 {
			continue
		}
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].DurationMS < candidates[j].DurationMS })
		for start := 0; start < len(candidates); {
			end := start + 1
			for end < len(candidates) && candidates[end].DurationMS-candidates[start].DurationMS <= toleranceMS {
				end++
			}
			if end-start >= 2 {
				cluster := append([]model.Track(nil), candidates[start:end]...)
				groups = append(groups, model.DuplicateGroup{
					Artist:     cluster[0].Artist,
					Title:      cluster[0].Title,
					DurationMS: medianDuration(cluster),
					Tracks:     cluster,
				})
			}
			start = end
		}
	}

	sort.Slice(groups, func(i, j int) bool {
		left := strings.ToLower(groups[i].Artist + "\x00" + groups[i].Title)
		right := strings.ToLower(groups[j].Artist + "\x00" + groups[j].Title)
		return left < right
	})
	return groups
}

func normalizeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	builder.Grow(len(value))
	previousSpace := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			previousSpace = false
			continue
		}
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			if !previousSpace && builder.Len() > 0 {
				builder.WriteByte(' ')
				previousSpace = true
			}
		}
	}
	return strings.TrimSpace(builder.String())
}

func medianDuration(tracks []model.Track) int64 {
	values := make([]int64, len(tracks))
	for i, track := range tracks {
		values[i] = track.DurationMS
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values[len(values)/2]
}
