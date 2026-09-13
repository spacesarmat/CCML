package metadata

import (
	"context"
	"errors"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

type fakeProvider struct {
	name  string
	items []model.MetadataCandidate
	err   error
}

func (p fakeProvider) Name() string { return p.name }
func (p fakeProvider) Search(context.Context, model.MetadataQuery) ([]model.MetadataCandidate, error) {
	return p.items, p.err
}

func TestServiceKeepsHealthyProviderWhenAnotherFails(t *testing.T) {
	service := NewService(
		fakeProvider{name: "broken", err: errors.New("temporary failure")},
		fakeProvider{name: "healthy", items: []model.MetadataCandidate{{Source: "healthy", ExternalID: "1", Title: "Song", Artist: "Artist"}}},
	)
	result, err := service.Search(context.Background(), model.MetadataQuery{Title: "Song", Artist: "Artist"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("candidates = %d, want 1", len(result.Candidates))
	}
	if len(result.ProviderReports) != 2 {
		t.Fatalf("provider reports = %d, want 2", len(result.ProviderReports))
	}
}

func TestServiceReturnsDiagnosticsWhenAllProvidersFail(t *testing.T) {
	service := NewService(fakeProvider{name: "broken", err: errors.New("failure")})
	result, err := service.Search(context.Background(), model.MetadataQuery{Title: "Song"})
	if err != nil {
		t.Fatalf("Search() should return diagnostics without fatal error, got %v", err)
	}
	if len(result.Candidates) != 0 || len(result.ProviderReports) != 1 || result.ProviderReports[0].Status != "error" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestValidateProvidersReturnsPerProviderHealth(t *testing.T) {
	service := NewService(
		fakeProvider{name: "ok", items: []model.MetadataCandidate{{Title: "One More Time", Artist: "Daft Punk"}}},
		fakeProvider{name: "empty"},
		fakeProvider{name: "broken", err: errors.New("boom")},
	)
	reports := service.ValidateProviders(context.Background())
	if len(reports) != 3 {
		t.Fatalf("reports = %d, want 3", len(reports))
	}
	status := map[string]string{}
	for _, report := range reports {
		status[report.Name] = report.Status
	}
	if status["ok"] != "ok" || status["empty"] != "empty" || status["broken"] != "error" {
		t.Fatalf("unexpected statuses: %+v", status)
	}
}
