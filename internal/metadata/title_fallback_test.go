package metadata

import (
	"context"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

type fallbackQueryProvider struct {
	name    string
	queries []string
}

func (p *fallbackQueryProvider) Name() string {
	if p.name != "" {
		return p.name
	}
	return "FallbackMock"
}

func (p *fallbackQueryProvider) Search(_ context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	p.queries = append(p.queries, query.Title)
	if query.Title != "Track Name" {
		return nil, nil
	}
	return []model.MetadataCandidate{{
		Source:        p.Name(),
		ExternalID:    "1",
		Title:         "Track Name",
		Artist:        "Artist",
		Album:         "Release",
		DurationMS:    360_000,
		ReleaseDate:   "2026-01-02",
		Label:         "Label",
		CatalogNumber: "CAT001",
	}}, nil
}

type primaryHitProvider struct {
	queries []string
}

func (p *primaryHitProvider) Name() string { return "PrimaryHit" }

func (p *primaryHitProvider) Search(_ context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	p.queries = append(p.queries, query.Title)
	return []model.MetadataCandidate{{
		Source:     p.Name(),
		ExternalID: "1",
		Title:      query.Title,
		Artist:     query.Artist,
		DurationMS: query.DurationMS,
	}}, nil
}

func TestTitleFallbackQueryExtendedMix(t *testing.T) {
	t.Parallel()

	fallback, ok := titleFallbackQuery(model.MetadataQuery{
		Artist: "Artist",
		Title:  "Track Name (Extended Mix)",
	})
	if !ok {
		t.Fatal("expected fallback query")
	}
	if fallback.Title != "Track Name" {
		t.Fatalf("fallback title = %q", fallback.Title)
	}
}

func TestTitleFallbackQueryKeepsMultipleLocalTags(t *testing.T) {
	t.Parallel()

	base, suffix, ok := splitTitleVersionSuffix("Track Name (Extended Mix) (Dirty)")
	if !ok {
		t.Fatal("expected version suffix")
	}
	if base != "Track Name" {
		t.Fatalf("base = %q", base)
	}
	if suffix != " (Extended Mix) (Dirty)" {
		t.Fatalf("suffix = %q", suffix)
	}
}

func TestTitleFallbackSupportsNamedRemixAndSeparator(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"Track Name (John Doe Remix)": "Track Name",
		"Track Name [Intro Clean]":    "Track Name",
		"Track Name - Radio Edit":     "Track Name",
		"Track Name — Acapella":       "Track Name",
	}
	for input, wanted := range cases {
		base, _, ok := splitTitleVersionSuffix(input)
		if !ok || base != wanted {
			t.Fatalf("splitTitleVersionSuffix(%q) = %q, %v; want %q, true", input, base, ok, wanted)
		}
	}
}

func TestTitleFallbackDoesNotStripOrdinaryBracketText(t *testing.T) {
	t.Parallel()

	if base, suffix, ok := splitTitleVersionSuffix("Track Name (Part 2)"); ok {
		t.Fatalf("unexpected fallback: base=%q suffix=%q", base, suffix)
	}
	if base, suffix, ok := splitTitleVersionSuffix("Dirty Dancing"); ok {
		t.Fatalf("unexpected fallback: base=%q suffix=%q", base, suffix)
	}
}

func TestPreserveLocalVersionTitleReplacesProviderVersion(t *testing.T) {
	t.Parallel()

	got := PreserveLocalVersionTitle(
		"Track Name (Extended Mix) (Dirty)",
		"Track Name (Original Mix)",
	)
	if got != "Track Name (Extended Mix) (Dirty)" {
		t.Fatalf("preserved title = %q", got)
	}
}

func TestPreserveLocalVersionTitleDoesNotTouchUnrelatedCandidate(t *testing.T) {
	t.Parallel()

	got := PreserveLocalVersionTitle("Track Name (Extended Mix)", "Completely Different")
	if got != "Completely Different" {
		t.Fatalf("unrelated provider title changed to %q", got)
	}
}

func TestServiceRetriesWithoutVersionSuffixAndRestoresIt(t *testing.T) {
	t.Parallel()

	provider := &fallbackQueryProvider{}
	service := NewService(provider)

	result, err := service.Search(context.Background(), model.MetadataQuery{
		Title:      "Track Name (Extended Mix)",
		Artist:     "Artist",
		DurationMS: 360_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.queries) != 2 {
		t.Fatalf("provider calls = %d, want 2 (%v)", len(provider.queries), provider.queries)
	}
	if provider.queries[0] != "Track Name (Extended Mix)" || provider.queries[1] != "Track Name" {
		t.Fatalf("queries = %#v", provider.queries)
	}
	if len(result.Candidates) == 0 {
		t.Fatal("expected fallback candidate")
	}
	if result.Candidates[0].Title != "Track Name (Extended Mix)" {
		t.Fatalf("restored candidate title = %q", result.Candidates[0].Title)
	}
	if !containsString(result.Candidates[0].MatchIssues, titleFallbackIssue) {
		t.Fatalf("fallback issue missing: %#v", result.Candidates[0].MatchIssues)
	}
}

func TestServiceDoesNotFallbackWhenPrimarySearchSucceeds(t *testing.T) {
	t.Parallel()

	provider := &primaryHitProvider{}
	service := NewService(provider)

	result, err := service.Search(context.Background(), model.MetadataQuery{
		Title:      "Track Name (Extended Mix)",
		Artist:     "Artist",
		DurationMS: 360_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) == 0 {
		t.Fatal("expected primary candidate")
	}
	if len(provider.queries) != 1 {
		t.Fatalf("provider calls = %d, want 1", len(provider.queries))
	}
}

func TestEnrichmentRetriesWithoutVersionSuffix(t *testing.T) {
	t.Parallel()

	provider := &fallbackQueryProvider{name: "Fast Fallback Mock"}
	service := NewService(provider)

	result, _, err := service.SearchEnrichment(
		context.Background(),
		model.MetadataQuery{
			Title:      "Track Name (Dirty)",
			Artist:     "Artist",
			DurationMS: 360_000,
		},
		EnrichmentSearchAuto,
		0.86,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.queries) != 2 {
		t.Fatalf("provider calls = %d, want 2 (%v)", len(provider.queries), provider.queries)
	}
	if len(result.Candidates) == 0 || result.Candidates[0].Title != "Track Name (Dirty)" {
		t.Fatalf("unexpected fallback result: %+v", result.Candidates)
	}
}

func TestCleanAndDirtyAreDifferentVersions(t *testing.T) {
	t.Parallel()

	if got := versionSimilarity("Track Name (Clean)", "Track Name (Dirty)"); got != 0 {
		t.Fatalf("clean/dirty version similarity = %v, want 0", got)
	}
}
