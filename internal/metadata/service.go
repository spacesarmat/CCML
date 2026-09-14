// Package metadata normalizes searches across external metadata providers.
package metadata

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
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
// isolated and returned as diagnostics so one unavailable catalog never hides
// healthy results from the other providers.
//
// If the original title yields no usable candidate and ends in recognized
// version qualifiers, CCML automatically retries once without those qualifiers.
func (s *Service) Search(ctx context.Context, query model.MetadataQuery) (model.MetadataLookupResult, error) {
	primary, err := s.searchOnce(ctx, query)
	if err != nil || !metadataLookupNeedsTitleFallback(primary) {
		return primary, err
	}

	fallbackQuery, ok := titleFallbackQuery(query)
	if !ok {
		return primary, nil
	}

	fallback, fallbackErr := s.searchOnce(ctx, fallbackQuery)
	if fallbackErr != nil {
		if ctx.Err() != nil {
			return fallback, fallbackErr
		}
		return primary, nil
	}
	if len(fallback.Candidates) == 0 {
		return primary, nil
	}
	return markTitleFallbackResult(query.Title, fallback), nil
}

func (s *Service) searchOnce(ctx context.Context, query model.MetadataQuery) (model.MetadataLookupResult, error) {
	if len(s.providers) == 0 {
		return model.MetadataLookupResult{}, fmt.Errorf("no metadata providers configured")
	}

	type response struct {
		provider string
		kind     string
		items    []model.MetadataCandidate
		err      error
		duration time.Duration
	}
	responses := make(chan response, len(s.providers))
	var wg sync.WaitGroup
	for _, provider := range s.providers {
		provider := provider
		wg.Add(1)
		go func() {
			defer wg.Done()
			started := time.Now()
			providerCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
			defer cancel()
			items, err := provider.Search(providerCtx, query)
			items = stampProviderCandidates(provider, items)
			responses <- response{provider: provider.Name(), kind: providerKind(provider), items: items, err: err, duration: time.Since(started)}
		}()
	}
	go func() {
		wg.Wait()
		close(responses)
	}()

	result := model.MetadataLookupResult{}
	for response := range responses {
		report := model.MetadataProviderReport{
			Name:       response.provider,
			Kind:       response.kind,
			Candidates: len(response.items),
			DurationMS: response.duration.Milliseconds(),
		}
		if response.err != nil {
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			report.Status = "error"
			report.Error = response.err.Error()
			report.Retryable = isRetryableMetadataError(response.err)
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", response.provider, response.err))
		} else if len(response.items) == 0 {
			report.Status = "empty"
		} else {
			report.Status = "ok"
			result.Candidates = append(result.Candidates, response.items...)
		}
		result.ProviderReports = append(result.ProviderReports, report)
	}

	sort.Slice(result.ProviderReports, func(i, j int) bool {
		return result.ProviderReports[i].Name < result.ProviderReports[j].Name
	})
	evidence := rankCandidateEvidence(query, result.Candidates)
	result.Suggested = buildSuggested(evidence)
	result.FieldOptions = buildFieldOptions(evidence)
	result.Candidates = dedupeRankedCandidates(evidence)
	return result, nil
}

// ValidateProviders performs an explicit connectivity/catalog smoke test against
// every currently configured provider. It is intended for the Settings screen
// and is never run automatically during application startup.
func (s *Service) ValidateProviders(ctx context.Context) []model.MetadataProviderReport {
	if len(s.providers) == 0 {
		return nil
	}
	query := model.MetadataQuery{
		Title:      "One More Time",
		Artist:     "Daft Punk",
		Album:      "Discovery",
		DurationMS: 320000,
	}
	type response struct {
		report model.MetadataProviderReport
	}
	responses := make(chan response, len(s.providers))
	var wg sync.WaitGroup
	for _, provider := range s.providers {
		provider := provider
		wg.Add(1)
		go func() {
			defer wg.Done()
			started := time.Now()
			providerCtx, cancel := context.WithTimeout(ctx, 18*time.Second)
			items, err := provider.Search(providerCtx, query)
			cancel()
			report := model.MetadataProviderReport{
				Name:       provider.Name(),
				Kind:       providerKind(provider),
				Candidates: len(items),
				DurationMS: time.Since(started).Milliseconds(),
			}
			if err != nil {
				report.Status = "error"
				report.Error = err.Error()
				report.Retryable = isRetryableMetadataError(err)
			} else if len(items) == 0 {
				report.Status = "empty"
			} else {
				report.Status = "ok"
			}
			responses <- response{report: report}
		}()
	}
	go func() {
		wg.Wait()
		close(responses)
	}()

	reports := make([]model.MetadataProviderReport, 0, len(s.providers))
	for response := range responses {
		reports = append(reports, response.report)
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].Name < reports[j].Name })
	return reports
}
