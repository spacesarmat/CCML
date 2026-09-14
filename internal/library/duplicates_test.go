package library

import (
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestFindDuplicatesExactMetadataAndDuration(t *testing.T) {
	t.Parallel()

	tracks := []model.Track{
		{ID: 1, Artist: "Daft Punk", Title: "Around the World", DurationMS: 429_000},
		{ID: 2, Artist: "daft punk", Title: "Around The World", DurationMS: 430_200},
		{ID: 3, Artist: "Daft Punk", Title: "Around the World", DurationMS: 450_000},
	}

	groups := FindDuplicates(tracks, 2_000)
	if len(groups) != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", len(groups))
	}
	if groups[0].MatchClass != "metadata" {
		t.Fatalf("match class = %q, want metadata", groups[0].MatchClass)
	}
	if got := len(groups[0].Tracks); got != 2 {
		t.Fatalf("expected 2 tracks in group, got %d", got)
	}
}

func TestFindDuplicatesExactISRCOverridesMetadataDifferences(t *testing.T) {
	t.Parallel()

	tracks := []model.Track{
		{ID: 10, Artist: "Artist A", Title: "Track Name", ISRC: "US-ABC-24-12345", DurationMS: 210_000},
		{ID: 11, Artist: "Artist A feat. B", Title: "Track Name (Radio Edit)", ISRC: "USABC2412345", DurationMS: 198_000},
	}

	groups := FindDuplicates(tracks, 2_000)
	if len(groups) != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", len(groups))
	}
	if groups[0].MatchClass != "isrc" {
		t.Fatalf("match class = %q, want isrc", groups[0].MatchClass)
	}
	if groups[0].SharedISRC != "USABC2412345" {
		t.Fatalf("shared ISRC = %q", groups[0].SharedISRC)
	}
	if groups[0].Confidence != 1 {
		t.Fatalf("confidence = %v, want 1", groups[0].Confidence)
	}
}

func TestFindDuplicatesVersionNormalizedPossibleGroup(t *testing.T) {
	t.Parallel()

	tracks := []model.Track{
		{ID: 20, Artist: "Asake", Title: "Amen (Clean)", DurationMS: 180_000},
		{ID: 21, Artist: "Asake", Title: "Amen (Intro Edit Clean)", DurationMS: 181_200},
	}

	groups := FindDuplicates(tracks, 2_000)
	if len(groups) != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", len(groups))
	}
	if groups[0].MatchClass != "possible" {
		t.Fatalf("match class = %q, want possible", groups[0].MatchClass)
	}
	if groups[0].Confidence >= 0.9 {
		t.Fatalf("possible confidence = %v, expected below 0.9", groups[0].Confidence)
	}
}

func TestFindDuplicatesDoesNotMergeFarDurationVariants(t *testing.T) {
	t.Parallel()

	tracks := []model.Track{
		{ID: 30, Artist: "Artist", Title: "Song (Radio Edit)", DurationMS: 180_000},
		{ID: 31, Artist: "Artist", Title: "Song (Extended Mix)", DurationMS: 310_000},
	}

	groups := FindDuplicates(tracks, 2_000)
	if len(groups) != 0 {
		t.Fatalf("expected no duplicate group, got %d", len(groups))
	}
}

func TestNormalizeISRCRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	if got := normalizeISRC("US-ABC-24-12345"); got != "USABC2412345" {
		t.Fatalf("normalized ISRC = %q", got)
	}
	if got := normalizeISRC("short"); got != "" {
		t.Fatalf("malformed ISRC normalized to %q, want empty", got)
	}
}
