package metadata

import (
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestBananaStreetDebugDocumentStatsExposeIdentity(t *testing.T) {
	t.Parallel()

	stats := bananaStreetDebugDocumentStatsFor(
		model.MetadataQuery{
			Artist: "Винтаж, DJ Smash",
			Title:  "Москва (Nei Blend)",
		},
		bananaStreetFixture,
	)
	if stats.VisibleLines == 0 {
		t.Fatal("expected visible lines")
	}
	if !stats.ContainsArtist || !stats.ContainsTitle {
		t.Fatalf("identity should be visible: %+v", stats)
	}
	if stats.Preview == "" {
		t.Fatal("expected compact preview")
	}
}

func TestBananaStreetDebugDocumentStatsDetectShellWithoutTrack(t *testing.T) {
	t.Parallel()

	stats := bananaStreetDebugDocumentStatsFor(
		model.MetadataQuery{
			Artist: "Vadim Adamov, Hardphol, Mvrgø",
			Title:  "У Тебя Одной",
		},
		`<html><body><div id="app">Поиск музыки</div></body></html>`,
	)
	if stats.ContainsArtist || stats.ContainsTitle {
		t.Fatalf("shell must not contain requested identity: %+v", stats)
	}
}
