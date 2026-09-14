package metadata

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

const (
	EnrichmentSearchAuto = "auto"
	EnrichmentSearchFast = "fast"
	EnrichmentSearchFull = "full"

	fastProviderTimeout = 10 * time.Second
	slowProviderTimeout = 25 * time.Second
)

// EnrichmentSearchDiagnostics describes how much of the provider pipeline was
// needed for one automatic-enrichment lookup.
type EnrichmentSearchDiagnostics struct {
	Mode               string
	DurationMS         int64
	ProvidersResponded int
	ProvidersSkipped   int
	EarlyStopped       bool
}

// NormalizeEnrichmentSearchMode converts missing/unknown values to the safe
// default used by jobs created before Stage 8.1B.
func NormalizeEnrichmentSearchMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case EnrichmentSearchFast:
		return EnrichmentSearchFast
	case EnrichmentSearchFull:
		return EnrichmentSearchFull
	default:
		return EnrichmentSearchAuto
	}
}

// SearchEnrichment is optimized for automatic batch enrichment.
//
// If the exact local title produces no usable candidate, recognized trailing
// version qualifiers are removed for one fallback provider pass.
func (s *Service) SearchEnrichment(
	ctx context.Context,
	query model.MetadataQuery,
	mode string,
	minimumConfidence float64,
) (model.MetadataLookupResult, EnrichmentSearchDiagnostics, error) {
	primary, primaryDiagnostics, err := s.searchEnrichmentOnce(ctx, query, mode, minimumConfidence)
	if err != nil || !metadataLookupNeedsTitleFallback(primary) {
		return primary, primaryDiagnostics, err
	}

	fallbackQuery, ok := titleFallbackQuery(query)
	if !ok {
		return primary, primaryDiagnostics, nil
	}

	fallback, fallbackDiagnostics, fallbackErr := s.searchEnrichmentOnce(ctx, fallbackQuery, mode, minimumConfidence)
	fallbackDiagnostics.DurationMS += primaryDiagnostics.DurationMS
	if primaryDiagnostics.ProvidersResponded > fallbackDiagnostics.ProvidersResponded {
		fallbackDiagnostics.ProvidersResponded = primaryDiagnostics.ProvidersResponded
	}
	if primaryDiagnostics.ProvidersSkipped < fallbackDiagnostics.ProvidersSkipped {
		fallbackDiagnostics.ProvidersSkipped = primaryDiagnostics.ProvidersSkipped
	}
	fallbackDiagnostics.EarlyStopped = primaryDiagnostics.EarlyStopped || fallbackDiagnostics.EarlyStopped

	if fallbackErr != nil {
		if ctx.Err() != nil {
			return fallback, fallbackDiagnostics, fallbackErr
		}
		return primary, primaryDiagnostics, nil
	}
	if len(fallback.Candidates) == 0 {
		return primary, primaryDiagnostics, nil
	}
	return markTitleFallbackResult(query.Title, fallback), fallbackDiagnostics, nil
}

// searchEnrichmentOnce performs one provider pass for the exact supplied query.
//
// Full preserves the old behavior and waits for every configured provider.
// Fast queries only the providers that do not have CCML-side serialized request
// pacing. Auto queries those providers first and only escalates to deferred
// providers when the fast group cannot produce an exact high-confidence match.
//
// Provider-specific rate limiters remain authoritative inside each adapter.
func (s *Service) searchEnrichmentOnce(
	ctx context.Context,
	query model.MetadataQuery,
	mode string,
	minimumConfidence float64,
) (model.MetadataLookupResult, EnrichmentSearchDiagnostics, error) {
	started := time.Now()
	mode = NormalizeEnrichmentSearchMode(mode)
	diagnostics := EnrichmentSearchDiagnostics{Mode: mode}

	finish := func(result model.MetadataLookupResult, responded int, early bool, err error) (model.MetadataLookupResult, EnrichmentSearchDiagnostics, error) {
		diagnostics.DurationMS = time.Since(started).Milliseconds()
		diagnostics.ProvidersResponded = responded
		diagnostics.ProvidersSkipped = len(s.providers) - responded
		if diagnostics.ProvidersSkipped < 0 {
			diagnostics.ProvidersSkipped = 0
		}
		diagnostics.EarlyStopped = early
		return result, diagnostics, err
	}

	if len(s.providers) == 0 {
		return finish(model.MetadataLookupResult{}, 0, false, fmt.Errorf("no metadata providers configured"))
	}

	if mode == EnrichmentSearchFull {
		result, err := s.searchOnce(ctx, query)
		return finish(result, len(result.ProviderReports), false, err)
	}

	fast, deferred := partitionEnrichmentProviders(s.providers)
	threshold := earlyStopThreshold(minimumConfidence)

	if mode == EnrichmentSearchFast {
		chosen := fast
		// A user may intentionally enable only MusicBrainz/Discogs/etc. In that
		// configuration Fast remains useful by querying the available set with a
		// shorter deadline and exact-match early stopping.
		if len(chosen) == 0 {
			chosen = s.providers
		}
		result, responded, early, err := searchProviderSet(
			ctx, query, chosen, fastProviderTimeout, model.MetadataLookupResult{}, threshold,
		)
		return finish(result, responded, early, err)
	}

	// Auto: fast group first.
	result := model.MetadataLookupResult{}
	responded := 0
	if len(fast) > 0 {
		var early bool
		var err error
		result, responded, early, err = searchProviderSet(
			ctx, query, fast, fastProviderTimeout, result, threshold,
		)
		if err != nil {
			return finish(result, responded, early, err)
		}
		if early || autoResultIsFinal(query, result, threshold) || len(deferred) == 0 {
			return finish(result, responded, early, nil)
		}
	}

	// No exact answer from the fast group: escalate only now.
	if len(deferred) == 0 {
		deferred = s.providers
	}
	result, slowResponded, early, err := searchProviderSet(
		ctx, query, deferred, slowProviderTimeout, result, threshold,
	)
	responded += slowResponded
	return finish(result, responded, early, err)
}

type enrichmentProviderResponse struct {
	provider string
	kind     string
	items    []model.MetadataCandidate
	err      error
	duration time.Duration
}

func searchProviderSet(
	ctx context.Context,
	query model.MetadataQuery,
	providers []Provider,
	timeout time.Duration,
	seed model.MetadataLookupResult,
	earlyThreshold float64,
) (model.MetadataLookupResult, int, bool, error) {
	if len(providers) == 0 {
		return finalizeEnrichmentLookup(query, seed), 0, false, nil
	}

	groupCtx, cancelGroup := context.WithCancel(ctx)
	defer cancelGroup()

	responses := make(chan enrichmentProviderResponse, len(providers))
	for _, provider := range providers {
		provider := provider
		go func() {
			started := time.Now()
			providerCtx, cancel := context.WithTimeout(groupCtx, timeout)
			items, err := provider.Search(providerCtx, query)
			cancel()
			items = stampProviderCandidates(provider, items)
			responses <- enrichmentProviderResponse{
				provider: provider.Name(),
				kind:     providerKind(provider),
				items:    items,
				err:      err,
				duration: time.Since(started),
			}
		}()
	}

	result := seed
	responded := 0
	remaining := len(providers)

	for remaining > 0 {
		select {
		case <-ctx.Done():
			return finalizeEnrichmentLookup(query, result), responded, false, ctx.Err()

		case response := <-responses:
			responded++
			remaining--

			report := model.MetadataProviderReport{
				Name:       response.provider,
				Kind:       response.kind,
				Candidates: len(response.items),
				DurationMS: response.duration.Milliseconds(),
			}
			if response.err != nil {
				// A provider cancelled because another provider already supplied
				// an exact match must not become a warning on the winning result.
				if groupCtx.Err() == nil {
					report.Status = "error"
					report.Error = response.err.Error()
					report.Retryable = isRetryableMetadataError(response.err)
					result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", response.provider, response.err))
					result.ProviderReports = append(result.ProviderReports, report)
				}
			} else if len(response.items) == 0 {
				report.Status = "empty"
				result.ProviderReports = append(result.ProviderReports, report)
			} else {
				report.Status = "ok"
				result.ProviderReports = append(result.ProviderReports, report)
				result.Candidates = append(result.Candidates, response.items...)
			}

			if remaining > 0 && earlyThreshold > 0 && autoResultIsFinal(query, result, earlyThreshold) {
				cancelGroup()
				return finalizeEnrichmentLookup(query, result), responded, true, nil
			}
		}
	}

	return finalizeEnrichmentLookup(query, result), responded, false, nil
}

func finalizeEnrichmentLookup(query model.MetadataQuery, result model.MetadataLookupResult) model.MetadataLookupResult {
	sort.Slice(result.ProviderReports, func(i, j int) bool {
		return result.ProviderReports[i].Name < result.ProviderReports[j].Name
	})
	result.Candidates = rankCandidates(query, result.Candidates)
	result.Suggested = buildSuggested(result.Candidates)
	result.FieldOptions = buildFieldOptions(result.Candidates)
	return result
}

func autoResultIsFinal(query model.MetadataQuery, result model.MetadataLookupResult, threshold float64) bool {
	if len(result.Candidates) == 0 {
		return false
	}
	ranked := rankCandidates(query, append([]model.MetadataCandidate(nil), result.Candidates...))
	if len(ranked) == 0 {
		return false
	}
	top := ranked[0]
	if top.Score.Identifier == 1 && top.Confidence >= 0.95 {
		return true
	}
	return top.MatchClass == "exact" && top.Confidence >= threshold
}

func earlyStopThreshold(minimumConfidence float64) float64 {
	if minimumConfidence < 0.94 {
		return 0.94
	}
	if minimumConfidence > 1 {
		return 1
	}
	return minimumConfidence
}

// Deferred providers have explicit request pacing, serialized fallback work,
// higher failure latency, or a combination of those traits. They remain fully
// available in Auto fallback and Full mode.
func partitionEnrichmentProviders(providers []Provider) (fast []Provider, deferred []Provider) {
	for _, provider := range providers {
		switch strings.ToLower(strings.TrimSpace(provider.Name())) {
		case "musicbrainz", "apple itunes", "theaudiodb", "discogs", "traxsource", "muzvizor":
			deferred = append(deferred, provider)
		default:
			fast = append(fast, provider)
		}
	}
	return fast, deferred
}
