package metadata

import (
	"math"
	"sort"
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

// rankCandidateEvidence scores and sorts every provider candidate without
// collapsing cross-provider duplicates. Stage 18.6 keeps this evidence long
// enough to calculate field consensus, then deduplicates only the visible
// candidate list.
func rankCandidateEvidence(query model.MetadataQuery, items []model.MetadataCandidate) []model.MetadataCandidate {
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
	return items
}

func dedupeRankedCandidates(items []model.MetadataCandidate) []model.MetadataCandidate {
	items = dedupeCandidates(items)
	if len(items) > maxRankedCandidates {
		items = items[:maxRankedCandidates]
	}
	return items
}

func candidateDedupeKey(item model.MetadataCandidate) string {
	artist := normalizeText(item.Artist)
	title := normalizeText(item.Title)

	// DJ pools often omit or disagree on album-level fields. For the visible
	// result list, Artist + full versioned Title is the stable identity. The
	// full title is intentionally retained so Extended/Radio/Remix variants
	// never collapse into one row.
	if item.SourceKind == ProviderKindDJPool && artist != "" && title != "" {
		return "dj_pool\x00" + artist + "\x00" + title
	}

	key := artist + "\x00" + title + "\x00" + normalizeText(item.Album) + "\x00" + normalizeIdentifier(item.ISRC)
	if key == "\x00\x00\x00" {
		return item.Source + "\x00" + item.ExternalID
	}
	return key
}

type fieldOptionAccumulator struct {
	option   model.MetadataFieldOption
	sources  map[string]struct{}
	bestBase float64
}

func buildQualityFieldOptions(items []model.MetadataCandidate) []model.MetadataFieldOption {
	accumulators := make([]fieldOptionAccumulator, 0, len(items)*8)

	add := func(option model.MetadataFieldOption) {
		baseQuality := option.Confidence + sourceFieldBonus(option.Field, option.Source)
		index := -1
		for i := range accumulators {
			if fieldOptionsEquivalent(accumulators[i].option, option) {
				index = i
				break
			}
		}

		if index < 0 {
			sources := map[string]struct{}{}
			if source := strings.TrimSpace(option.Source); source != "" {
				sources[source] = struct{}{}
			}
			accumulators = append(accumulators, fieldOptionAccumulator{
				option:   option,
				sources:  sources,
				bestBase: baseQuality,
			})
			return
		}

		acc := &accumulators[index]
		if source := strings.TrimSpace(option.Source); source != "" {
			acc.sources[source] = struct{}{}
		}
		if option.Confidence > acc.option.Confidence {
			acc.option.Confidence = option.Confidence
		}
		if baseQuality > acc.bestBase {
			acc.bestBase = baseQuality
			acc.option.Source = option.Source
			acc.option.ExternalID = option.ExternalID
			if option.Value != "" {
				acc.option.Value = option.Value
			}
			if option.Number > 0 {
				acc.option.Number = option.Number
			}
			if option.Decimal > 0 {
				acc.option.Decimal = option.Decimal
			}
		}
	}

	for _, item := range items {
		stringFields := []struct {
			name  string
			value string
		}{
			{"title", item.Title},
			{"artist", item.Artist},
			{"album", item.Album},
			{"albumArtist", item.AlbumArtist},
			{"releaseDate", item.ReleaseDate},
			{"genre", item.Genre},
			{"label", item.Label},
			{"catalogNumber", item.CatalogNumber},
			{"isrc", item.ISRC},
			{"key", item.Key},
			{"keyScale", item.KeyScale},
		}
		for _, field := range stringFields {
			value := strings.TrimSpace(field.value)
			if value == "" {
				continue
			}
			add(model.MetadataFieldOption{
				Field:      field.name,
				Value:      value,
				Source:     item.Source,
				ExternalID: item.ExternalID,
				Confidence: item.Confidence,
			})
		}

		intFields := []struct {
			name  string
			value int
		}{
			{"year", item.Year},
			{"trackNumber", item.TrackNumber},
			{"trackTotal", item.TrackTotal},
			{"discNumber", item.DiscNumber},
			{"discTotal", item.DiscTotal},
		}
		for _, field := range intFields {
			if field.value <= 0 {
				continue
			}
			add(model.MetadataFieldOption{
				Field:      field.name,
				Number:     field.value,
				Source:     item.Source,
				ExternalID: item.ExternalID,
				Confidence: item.Confidence,
			})
		}

		if item.BPM > 0 {
			add(model.MetadataFieldOption{
				Field:      "bpm",
				Decimal:    item.BPM,
				Source:     item.Source,
				ExternalID: item.ExternalID,
				Confidence: item.Confidence,
			})
		}
	}

	options := make([]model.MetadataFieldOption, 0, len(accumulators))
	for _, acc := range accumulators {
		sources := make([]string, 0, len(acc.sources))
		for source := range acc.sources {
			sources = append(sources, source)
		}
		sort.Strings(sources)

		option := acc.option
		option.Support = len(sources)
		if option.Support == 0 {
			option.Support = 1
		}
		option.Sources = sources
		option.Quality = acc.bestBase + fieldConsensusBonus(option.Support)
		options = append(options, option)
	}

	sort.SliceStable(options, func(i, j int) bool {
		if options[i].Field != options[j].Field {
			return options[i].Field < options[j].Field
		}
		if options[i].Quality != options[j].Quality {
			return options[i].Quality > options[j].Quality
		}
		if options[i].Support != options[j].Support {
			return options[i].Support > options[j].Support
		}
		return options[i].Confidence > options[j].Confidence
	})
	return options
}

func fieldOptionsEquivalent(a, b model.MetadataFieldOption) bool {
	if a.Field != b.Field {
		return false
	}
	if a.Field == "bpm" {
		return a.Decimal > 0 && b.Decimal > 0 && math.Abs(a.Decimal-b.Decimal) <= 0.5
	}
	if a.Number > 0 || b.Number > 0 {
		return a.Number == b.Number
	}
	return canonicalFieldString(a.Field, a.Value) == canonicalFieldString(b.Field, b.Value)
}

func canonicalFieldString(field, value string) string {
	value = strings.TrimSpace(value)
	if field != "genre" {
		return normalizeText(value)
	}

	// Genre order differs between pools ("House, Techno" vs "Techno / House").
	// Treat the same set as one consensus value without inventing mappings.
	replacer := strings.NewReplacer(";", ",", "/", ",", "|", ",")
	parts := strings.Split(replacer.Replace(value), ",")
	normalized := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		key := normalizeText(part)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, key)
	}
	sort.Strings(normalized)
	return strings.Join(normalized, "|")
}

func fieldConsensusBonus(support int) float64 {
	if support <= 1 {
		return 0
	}
	bonus := 0.04 * float64(support-1)
	if bonus > 0.12 {
		bonus = 0.12
	}
	return bonus
}

func bestFieldOption(options []model.MetadataFieldOption, field string) (model.MetadataFieldOption, bool) {
	best := model.MetadataFieldOption{}
	found := false
	for _, option := range options {
		if option.Field != field {
			continue
		}
		if !found || option.Quality > best.Quality ||
			(option.Quality == best.Quality && option.Support > best.Support) ||
			(option.Quality == best.Quality && option.Support == best.Support && option.Confidence > best.Confidence) {
			best = option
			found = true
		}
	}
	return best, found
}
