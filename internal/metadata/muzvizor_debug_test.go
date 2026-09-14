package metadata

import (
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestMuzvizorDebugDocumentStatsExposeRenderedShape(t *testing.T) {
	t.Parallel()

	stats := muzvizorDebugDocumentStatsFor(
		model.MetadataQuery{
			Artist: "Винтаж, DJ Smash",
			Title:  "Москва (Nei Blend)",
		},
		muzvizorRenderedRowFixture,
	)
	if stats.Rows != 1 {
		t.Fatalf("rows = %d, want 1", stats.Rows)
	}
	if stats.TitleColumns != 1 || stats.BPMColumns != 1 || stats.KeyColumns != 1 || stats.GenreColumns != 1 {
		t.Fatalf("unexpected column stats: %+v", stats)
	}
	if !stats.ContainsArtist || !stats.ContainsTitle {
		t.Fatalf("query identity should be visible: %+v", stats)
	}
	if stats.Preview == "" {
		t.Fatal("expected a compact debug preview")
	}
}

func TestMuzvizorDebugDocumentStatsDetectJSShell(t *testing.T) {
	t.Parallel()

	stats := muzvizorDebugDocumentStatsFor(
		model.MetadataQuery{
			Artist: "Винтаж, DJ Smash",
			Title:  "Москва (Nei Blend)",
		},
		`<html><body><div id="app">Загрузка треков...</div></body></html>`,
	)
	if stats.Rows != 0 || stats.TitleColumns != 0 {
		t.Fatalf("JavaScript shell must not report track rows: %+v", stats)
	}
	if stats.ContainsArtist || stats.ContainsTitle {
		t.Fatalf("JavaScript shell must not contain the query identity: %+v", stats)
	}
}
