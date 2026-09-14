package metadata

import (
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

const (
	// ProviderKindCatalog is the default for normal release/catalog services.
	ProviderKindCatalog = "catalog"
	// ProviderKindDJPool identifies DJ-oriented pools and mix/catalog sources.
	ProviderKindDJPool = "dj_pool"
)

// providerKinder is intentionally optional so every existing provider remains
// source-compatible. New DJ-pool adapters only need to implement Kind().
type providerKinder interface {
	Kind() string
}

func providerKind(provider Provider) string {
	if provider == nil {
		return ProviderKindCatalog
	}
	if typed, ok := provider.(providerKinder); ok {
		switch strings.ToLower(strings.TrimSpace(typed.Kind())) {
		case ProviderKindDJPool:
			return ProviderKindDJPool
		case ProviderKindCatalog:
			return ProviderKindCatalog
		}
	}
	return ProviderKindCatalog
}

func stampProviderCandidates(provider Provider, items []model.MetadataCandidate) []model.MetadataCandidate {
	kind := providerKind(provider)
	name := ""
	if provider != nil {
		name = provider.Name()
	}
	for i := range items {
		if strings.TrimSpace(items[i].Source) == "" {
			items[i].Source = name
		}
		if strings.TrimSpace(items[i].SourceKind) == "" {
			items[i].SourceKind = kind
		}
	}
	return items
}
