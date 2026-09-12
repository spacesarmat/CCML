// Package metadata normalizes searches across external metadata providers.
package metadata

import (
	"context"
	"fmt"
	"sync"

	"github.com/your-github/ccml/internal/model"
)

// Provider is implemented by an external metadata service.
type Provider interface {
	Name() string
	Search(context.Context, model.MetadataQuery) ([]model.MetadataCandidate, error)
}

// Service searches several independent providers.
type Service struct {
	providers []Provider
}

// NewService creates a metadata aggregation service.
func NewService(providers ...Provider) *Service {
	return &Service{providers: providers}
}

// ProviderNames returns the configured provider names for diagnostics/UI.
func (s *Service) ProviderNames() []string {
	names := make([]string, 0, len(s.providers))
	for _, provider := range s.providers {
		names = append(names, provider.Name())
	}
	return names
}

// Search queries all configured providers concurrently. Provider failures are
// returned as warnings so a healthy service can still provide useful results.
func (s *Service) Search(ctx context.Context, query model.MetadataQuery) (model.MetadataLookupResult, error) {
	if len(s.providers) == 0 {
		return model.MetadataLookupResult{}, fmt.Errorf("no metadata providers configured")
	}

	type response struct {
		provider string
		items    []model.MetadataCandidate
		err      error
	}
	responses := make(chan response, len(s.providers))
	var wg sync.WaitGroup
	for _, provider := range s.providers {
		provider := provider
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := provider.Search(ctx, query)
			responses <- response{provider: provider.Name(), items: items, err: err}
		}()
	}
	wg.Wait()
	close(responses)

	result := model.MetadataLookupResult{}
	failed := 0
	for response := range responses {
		if response.err != nil {
			failed++
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", response.provider, response.err))
			continue
		}
		result.Candidates = append(result.Candidates, response.items...)
	}
	if failed == len(s.providers) {
		return result, fmt.Errorf("all metadata providers failed")
	}
	return result, nil
}
