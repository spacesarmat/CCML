package library

import (
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestFindDuplicatesMetadataAndDuration(t *testing.T) {
	t.Parallel()

	tracks := []model.Track{
		{ID: 1, Artist: "Daft Punk", Title: "Around the World", DurationMS: 429_000},
		{ID: 2, Artist: "daft punk", Title: "Around The World", DurationMS: 430_200},
		{ID: 3, Artist: "Daft Punk", Title: "Around the World", DurationMS: 450_000},
		{ID: 4, Artist: "", Title: "Unknown", DurationMS: 100_000},
	}

	groups := FindDuplicates(tracks, 2_000)
	if len(groups) != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", len(groups))
	}
	if got := len(groups[0].Tracks); got != 2 {
		t.Fatalf("expected 2 tracks in group, got %d", got)
	}
}
