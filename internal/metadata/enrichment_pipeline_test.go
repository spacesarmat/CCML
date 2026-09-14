package metadata

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

type pipelineProvider struct {
	name      string
	candidate model.MetadataCandidate
	delay     time.Duration
	calls     atomic.Int32
}

func (p *pipelineProvider) Name() string { return p.name }

func (p *pipelineProvider) Search(ctx context.Context, _ model.MetadataQuery) ([]model.MetadataCandidate, error) {
	p.calls.Add(1)
	if p.delay > 0 {
		timer := time.NewTimer(p.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	if p.candidate.Title == "" && p.candidate.Artist == "" {
		return nil, nil
	}
	item := p.candidate
	if item.Source == "" {
		item.Source = p.name
	}
	return []model.MetadataCandidate{item}, nil
}

func TestSearchEnrichmentAutoStopsBeforeDeferredProviders(t *testing.T) {
	fast := &pipelineProvider{
		name: "Deezer",
		candidate: model.MetadataCandidate{
			Source: "Deezer", Title: "One More Time", Artist: "Daft Punk", DurationMS: 320000,
		},
	}
	deferred := &pipelineProvider{
		name: "MusicBrainz",
		candidate: model.MetadataCandidate{
			Source: "MusicBrainz", Title: "One More Time", Artist: "Daft Punk", DurationMS: 320000,
		},
	}

	service := NewService(fast, deferred)
	result, diag, err := service.SearchEnrichment(
		context.Background(),
		model.MetadataQuery{Title: "One More Time", Artist: "Daft Punk", DurationMS: 320000},
		EnrichmentSearchAuto,
		0.86,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) == 0 || result.Candidates[0].MatchClass != "exact" {
		t.Fatalf("expected exact fast result, got %+v", result.Candidates)
	}
	if got := deferred.calls.Load(); got != 0 {
		t.Fatalf("deferred provider calls = %d, want 0", got)
	}
	if diag.ProvidersSkipped != 1 {
		t.Fatalf("providers skipped = %d, want 1", diag.ProvidersSkipped)
	}
}

func TestSearchEnrichmentAutoEscalatesWhenFastResultIsWeak(t *testing.T) {
	fast := &pipelineProvider{
		name: "Deezer",
		candidate: model.MetadataCandidate{
			Source: "Deezer", Title: "Different Song", Artist: "Different Artist", DurationMS: 100000,
		},
	}
	deferred := &pipelineProvider{
		name: "MusicBrainz",
		candidate: model.MetadataCandidate{
			Source: "MusicBrainz", Title: "One More Time", Artist: "Daft Punk", DurationMS: 320000,
		},
	}

	service := NewService(fast, deferred)
	result, diag, err := service.SearchEnrichment(
		context.Background(),
		model.MetadataQuery{Title: "One More Time", Artist: "Daft Punk", DurationMS: 320000},
		EnrichmentSearchAuto,
		0.86,
	)
	if err != nil {
		t.Fatal(err)
	}
	if deferred.calls.Load() != 1 {
		t.Fatalf("deferred provider was not queried")
	}
	if len(result.Candidates) == 0 || result.Candidates[0].Source != "MusicBrainz" {
		t.Fatalf("unexpected ranked result: %+v", result.Candidates)
	}
	if diag.ProvidersResponded != 2 {
		t.Fatalf("providers responded = %d, want 2", diag.ProvidersResponded)
	}
}

func TestSearchEnrichmentFastAvoidsDeferredProviders(t *testing.T) {
	fast := &pipelineProvider{
		name: "Deezer",
		candidate: model.MetadataCandidate{
			Source: "Deezer", Title: "Song", Artist: "Artist",
		},
	}
	deferred := &pipelineProvider{
		name: "Discogs",
		candidate: model.MetadataCandidate{
			Source: "Discogs", Title: "Song", Artist: "Artist",
		},
	}

	service := NewService(fast, deferred)
	_, diag, err := service.SearchEnrichment(
		context.Background(),
		model.MetadataQuery{Title: "Song", Artist: "Artist"},
		EnrichmentSearchFast,
		0.86,
	)
	if err != nil {
		t.Fatal(err)
	}
	if deferred.calls.Load() != 0 {
		t.Fatalf("deferred provider calls = %d, want 0", deferred.calls.Load())
	}
	if diag.Mode != EnrichmentSearchFast || diag.ProvidersSkipped != 1 {
		t.Fatalf("unexpected diagnostics: %+v", diag)
	}
}

func TestPartitionEnrichmentProvidersDefersMuzvizor(t *testing.T) {
	t.Parallel()

	deezer := &pipelineProvider{name: "Deezer"}
	muzvizor := &pipelineProvider{name: "MUZVIZOR"}

	fast, deferred := partitionEnrichmentProviders([]Provider{deezer, muzvizor})
	if len(fast) != 1 || fast[0].Name() != "Deezer" {
		t.Fatalf("fast providers = %+v, want Deezer only", providerNamesForTest(fast))
	}
	if len(deferred) != 1 || deferred[0].Name() != "MUZVIZOR" {
		t.Fatalf("deferred providers = %+v, want MUZVIZOR only", providerNamesForTest(deferred))
	}
}

func TestSearchEnrichmentAutoStopsBeforeDeferredMuzvizor(t *testing.T) {
	fast := &pipelineProvider{
		name: "Deezer",
		candidate: model.MetadataCandidate{
			Source: "Deezer", Title: "One More Time", Artist: "Daft Punk", DurationMS: 320000,
		},
	}
	muzvizor := &pipelineProvider{
		name: "MUZVIZOR",
		candidate: model.MetadataCandidate{
			Source: "MUZVIZOR", Title: "One More Time", Artist: "Daft Punk", DurationMS: 320000,
		},
	}

	service := NewService(fast, muzvizor)
	_, diag, err := service.SearchEnrichment(
		context.Background(),
		model.MetadataQuery{Title: "One More Time", Artist: "Daft Punk", DurationMS: 320000},
		EnrichmentSearchAuto,
		0.86,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := muzvizor.calls.Load(); got != 0 {
		t.Fatalf("MUZVIZOR calls = %d, want 0 after exact fast result", got)
	}
	if diag.ProvidersSkipped != 1 {
		t.Fatalf("providers skipped = %d, want 1", diag.ProvidersSkipped)
	}
}

func providerNamesForTest(providers []Provider) []string {
	names := make([]string, 0, len(providers))
	for _, provider := range providers {
		names = append(names, provider.Name())
	}
	return names
}

func TestSearchEnrichmentFullQueriesEveryProvider(t *testing.T) {
	fast := &pipelineProvider{
		name:      "Deezer",
		candidate: model.MetadataCandidate{Source: "Deezer", Title: "Song", Artist: "Artist"},
	}
	deferred := &pipelineProvider{
		name:      "MusicBrainz",
		candidate: model.MetadataCandidate{Source: "MusicBrainz", Title: "Song", Artist: "Artist"},
	}

	service := NewService(fast, deferred)
	_, diag, err := service.SearchEnrichment(
		context.Background(),
		model.MetadataQuery{Title: "Song", Artist: "Artist"},
		EnrichmentSearchFull,
		0.86,
	)
	if err != nil {
		t.Fatal(err)
	}
	if fast.calls.Load() != 1 || deferred.calls.Load() != 1 {
		t.Fatalf("full mode calls: fast=%d deferred=%d", fast.calls.Load(), deferred.calls.Load())
	}
	if diag.ProvidersResponded != 2 || diag.ProvidersSkipped != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diag)
	}
}
