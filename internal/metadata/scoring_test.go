package metadata

import (
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestScoreCandidatePrefersExactMetadata(t *testing.T) {
	q := model.MetadataQuery{Title: "Around the World", Artist: "Daft Punk", Album: "Homework", DurationMS: 429533}
	exact := model.MetadataCandidate{Title: "Around the World", Artist: "Daft Punk", Album: "Homework", DurationMS: 429000}
	weak := model.MetadataCandidate{Title: "Around The World - Remastered", Artist: "Daft Punk", Album: "Homework", DurationMS: 435000}
	if ScoreCandidate(q, exact).Total <= ScoreCandidate(q, weak).Total {
		t.Fatal("exact candidate should rank higher")
	}
}

func TestScoreCandidateExactISRCIsStrongSignal(t *testing.T) {
	q := model.MetadataQuery{Title: "Song", Artist: "Artist", ISRC: "GB-A1B-23-12345"}
	candidate := model.MetadataCandidate{Title: "Song (Radio Edit)", Artist: "Artist", ISRC: "GBA1B2312345"}
	if got := ScoreCandidate(q, candidate).Total; got < 0.92 {
		t.Fatalf("exact ISRC score = %.3f, want >= 0.92", got)
	}
}

func TestBuildSuggestedUsesEmbeddableArtworkOnly(t *testing.T) {
	items := []model.MetadataCandidate{
		{Source: "Spotify", Title: "Song", Artist: "Artist", Confidence: .98, ArtworkURL: "https://example/spotify.jpg", ArtworkWidth: 1000, ArtworkHeight: 1000, ArtworkEmbeddable: false},
		{Source: "MusicBrainz", Title: "Song", Artist: "Artist", Confidence: .95, ArtworkURL: "https://example/caa.jpg", ArtworkWidth: 1200, ArtworkHeight: 1200, ArtworkEmbeddable: true},
	}
	got := buildSuggested(items)
	if got.ArtworkURL != "https://example/caa.jpg" || !got.ArtworkEmbeddable {
		t.Fatalf("unexpected suggested artwork: %+v", got)
	}
}
