package metadata

import (
	"context"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

type djPoolFoundationProvider struct{}

func (djPoolFoundationProvider) Name() string { return "Foundation Pool" }
func (djPoolFoundationProvider) Kind() string { return ProviderKindDJPool }
func (djPoolFoundationProvider) Search(context.Context, model.MetadataQuery) ([]model.MetadataCandidate, error) {
	return []model.MetadataCandidate{{
		Title:      "Foundation Track",
		Artist:     "Foundation Artist",
		Genre:      "House",
		BPM:        126.5,
		Key:        "6A",
		KeyScale:   "camelot",
		DurationMS: 300_000,
	}}, nil
}

func TestDJPoolProviderKindIsStampedOnCandidates(t *testing.T) {
	t.Parallel()

	service := NewService(djPoolFoundationProvider{})
	result, err := service.Search(context.Background(), model.MetadataQuery{
		Title:      "Foundation Track",
		Artist:     "Foundation Artist",
		DurationMS: 300_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) == 0 {
		t.Fatal("expected candidate")
	}
	if got := result.Candidates[0].SourceKind; got != ProviderKindDJPool {
		t.Fatalf("source kind = %q, want %q", got, ProviderKindDJPool)
	}
	if len(result.ProviderReports) != 1 || result.ProviderReports[0].Kind != ProviderKindDJPool {
		t.Fatalf("provider report kind missing: %+v", result.ProviderReports)
	}
}

func TestSuggestedAndFieldOptionsCarryDJFields(t *testing.T) {
	t.Parallel()

	items := []model.MetadataCandidate{
		{
			Source: "Pool A", SourceKind: ProviderKindDJPool,
			Title: "Track", Artist: "Artist", BPM: 126.5, Key: "6A", KeyScale: "camelot",
			Confidence: 0.94, MatchClass: "exact",
		},
		{
			Source: "Pool B", SourceKind: ProviderKindDJPool,
			Title: "Track", Artist: "Artist", BPM: 128, Key: "Am", KeyScale: "minor",
			Confidence: 0.82, MatchClass: "high",
		},
	}

	suggested := buildSuggested(items)
	if suggested.BPM != 126.5 || suggested.Key != "6A" || suggested.KeyScale != "camelot" {
		t.Fatalf("unexpected suggestion: BPM=%v key=%q scale=%q", suggested.BPM, suggested.Key, suggested.KeyScale)
	}

	options := buildFieldOptions(items)
	var bpmSeen, keySeen bool
	for _, option := range options {
		if option.Field == "bpm" && option.Decimal == 126.5 {
			bpmSeen = true
		}
		if option.Field == "key" && option.Value == "6A" {
			keySeen = true
		}
	}
	if !bpmSeen || !keySeen {
		t.Fatalf("DJ metadata field options missing: %+v", options)
	}
}

type normalFoundationProvider struct{}

func (normalFoundationProvider) Name() string { return "Normal Mock" }
func (normalFoundationProvider) Search(context.Context, model.MetadataQuery) ([]model.MetadataCandidate, error) {
	return nil, nil
}

func TestNormalProviderDefaultsToCatalogKind(t *testing.T) {
	t.Parallel()

	provider := normalFoundationProvider{}
	items := stampProviderCandidates(provider, []model.MetadataCandidate{{Title: "Track"}})
	if len(items) != 1 || items[0].SourceKind != ProviderKindCatalog {
		t.Fatalf("unexpected default kind: %+v", items)
	}
}
