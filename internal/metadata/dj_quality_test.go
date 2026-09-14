package metadata

import (
	"sort"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestDJPoolEvidenceSurvivesUntilVisibleDedupe(t *testing.T) {
	t.Parallel()

	query := model.MetadataQuery{Artist: "Artist One", Title: "Track Name"}
	input := []model.MetadataCandidate{
		{Source: "MUZVIZOR", SourceKind: ProviderKindDJPool, ExternalID: "m1", Artist: "Artist One", Title: "Track Name", BPM: 128},
		{Source: "Jestei Pool", SourceKind: ProviderKindDJPool, ExternalID: "j1", Artist: "Artist One", Title: "Track Name", BPM: 128},
		{Source: "RemixPool", SourceKind: ProviderKindDJPool, ExternalID: "r1", Artist: "Artist One", Title: "Track Name", BPM: 126},
	}

	evidence := rankCandidateEvidence(query, input)
	if len(evidence) != 3 {
		t.Fatalf("evidence = %d, want 3", len(evidence))
	}
	visible := dedupeRankedCandidates(evidence)
	if len(visible) != 1 {
		t.Fatalf("visible candidates = %d, want 1: %+v", len(visible), visible)
	}
}

func TestDJPoolVisibleDedupeKeepsVersionsSeparate(t *testing.T) {
	t.Parallel()

	a := model.MetadataCandidate{
		SourceKind: ProviderKindDJPool,
		Artist:     "Artist One",
		Title:      "Track Name (Extended Mix)",
	}
	b := model.MetadataCandidate{
		SourceKind: ProviderKindDJPool,
		Artist:     "Artist One",
		Title:      "Track Name (Radio Edit)",
	}
	if candidateDedupeKey(a) == candidateDedupeKey(b) {
		t.Fatal("different track versions must not share a DJ-pool dedupe key")
	}
}

func TestDJFieldOptionsAggregateCrossProviderConsensus(t *testing.T) {
	t.Parallel()

	items := []model.MetadataCandidate{
		{Source: "Jestei Pool", ExternalID: "j", Confidence: 0.92, MatchClass: "high", BPM: 128, Key: "8A", KeyScale: "camelot", Genre: "House, Techno"},
		{Source: "MUZVIZOR", ExternalID: "m", Confidence: 0.91, MatchClass: "high", BPM: 128.3, Key: "8A", KeyScale: "camelot", Genre: "Techno / House"},
		{Source: "RemixPool", ExternalID: "r", Confidence: 0.95, MatchClass: "exact", BPM: 126, Key: "9A", KeyScale: "camelot", Genre: "House"},
	}
	options := buildFieldOptions(items)

	var bpmConsensus, bpmConflict *model.MetadataFieldOption
	for i := range options {
		option := &options[i]
		if option.Field != "bpm" {
			continue
		}
		if option.Decimal > 127.5 && option.Decimal < 128.6 {
			bpmConsensus = option
		}
		if option.Decimal > 125.5 && option.Decimal < 126.5 {
			bpmConflict = option
		}
	}
	if bpmConsensus == nil || bpmConflict == nil {
		t.Fatalf("expected consensus and conflict BPM options: %+v", options)
	}
	if bpmConsensus.Support != 2 {
		t.Fatalf("BPM consensus support = %d, want 2", bpmConsensus.Support)
	}
	gotSources := append([]string(nil), bpmConsensus.Sources...)
	sort.Strings(gotSources)
	if len(gotSources) != 2 || gotSources[0] != "Jestei Pool" || gotSources[1] != "MUZVIZOR" {
		t.Fatalf("BPM consensus sources = %+v", gotSources)
	}
	if bpmConsensus.Quality <= bpmConflict.Quality {
		t.Fatalf("two-source BPM consensus should outrank single-source conflict: consensus=%.3f conflict=%.3f", bpmConsensus.Quality, bpmConflict.Quality)
	}

	var genreConsensus *model.MetadataFieldOption
	for i := range options {
		option := &options[i]
		if option.Field == "genre" && option.Support == 2 {
			genreConsensus = option
			break
		}
	}
	if genreConsensus == nil {
		t.Fatalf("genre order normalization did not aggregate consensus: %+v", options)
	}
}

func TestDJMergePrefersReliableGenreSource(t *testing.T) {
	t.Parallel()

	items := []model.MetadataCandidate{
		{
			Source:     "Mixcloud",
			SourceKind: ProviderKindDJPool,
			ExternalID: "mix",
			Artist:     "Artist One",
			Title:      "Track Name",
			Genre:      "House",
			Confidence: 0.99,
			MatchClass: "exact",
		},
		{
			Source:     "Jestei Pool",
			SourceKind: ProviderKindDJPool,
			ExternalID: "jes",
			Artist:     "Artist One",
			Title:      "Track Name",
			Genre:      "Tech House",
			Confidence: 0.93,
			MatchClass: "high",
		},
	}
	suggested := buildSuggested(items)
	if suggested.Genre != "Tech House" {
		t.Fatalf("genre = %q, want Jestei reliability to beat weak show-tag source", suggested.Genre)
	}
}

func TestFieldConsensusBonusIsBounded(t *testing.T) {
	t.Parallel()

	if got := fieldConsensusBonus(1); got != 0 {
		t.Fatalf("support=1 bonus = %v", got)
	}
	if got := fieldConsensusBonus(2); got != 0.04 {
		t.Fatalf("support=2 bonus = %v", got)
	}
	if got := fieldConsensusBonus(20); got != 0.12 {
		t.Fatalf("support cap = %v", got)
	}
}
