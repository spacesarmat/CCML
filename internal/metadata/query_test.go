package metadata

import (
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestQueryFromTrackFallsBackToFilename(t *testing.T) {
	q := QueryFromTrack(model.Track{FileName: "01 - Daft Punk - One More Time.flac", DurationMS: 320000}, "")
	if q.Artist != "Daft Punk" || q.Title != "One More Time" {
		t.Fatalf("unexpected query: %+v", q)
	}
}

func TestQueryFromTrackPreservesTags(t *testing.T) {
	q := QueryFromTrack(model.Track{FileName: "wrong.mp3", Artist: "Tagged Artist", Title: "Tagged Title", Album: "Album"}, "USAAA2600001")
	if q.Artist != "Tagged Artist" || q.Title != "Tagged Title" || q.Album != "Album" || q.ISRC != "USAAA2600001" {
		t.Fatalf("unexpected query: %+v", q)
	}
}
