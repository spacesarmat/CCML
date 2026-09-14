package main

import (
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestNormalizeEnrichmentOptionsAlwaysUsesFindMetadataSearch(t *testing.T) {
	t.Parallel()

	for _, inputMode := range []string{"", "auto", "fast", "full", "legacy-value"} {
		opts := model.MetadataEnrichmentOptions{
			MinimumConfidence: 0.86,
			IncludeArtwork:    true,
			OnlyMissing:       true,
			SearchMode:        inputMode,
		}
		if err := normalizeEnrichmentOptions(&opts); err != nil {
			t.Fatalf("normalizeEnrichmentOptions(%q): %v", inputMode, err)
		}
		if opts.SearchMode != metadataSearchModeSameAsFind {
			t.Fatalf("mode %q normalized to %q, want %q", inputMode, opts.SearchMode, metadataSearchModeSameAsFind)
		}
	}
}

func TestNormalizeEnrichmentOptionsStillValidatesConfidence(t *testing.T) {
	t.Parallel()

	opts := model.MetadataEnrichmentOptions{MinimumConfidence: 1.2, SearchMode: "fast"}
	if err := normalizeEnrichmentOptions(&opts); err == nil {
		t.Fatal("expected confidence validation error")
	}
}
