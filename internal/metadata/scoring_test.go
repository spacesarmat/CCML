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

func TestScoreCandidatePenalizesWrongVersion(t *testing.T) {
	q := model.MetadataQuery{Title: "Finally", Artist: "Kings Of Tomorrow", DurationMS: 472000}
	original := model.MetadataCandidate{Title: "Finally (Original Mix)", Artist: "Kings Of Tomorrow", DurationMS: 471500}
	radio := model.MetadataCandidate{Title: "Finally (Radio Edit)", Artist: "Kings Of Tomorrow", DurationMS: 215000}

	originalScore := ScoreCandidate(q, original)
	radioScore := ScoreCandidate(q, radio)
	if originalScore.Version <= radioScore.Version {
		t.Fatalf("original version score %.3f should be above radio %.3f", originalScore.Version, radioScore.Version)
	}
	if originalScore.Total <= radioScore.Total {
		t.Fatalf("original total %.3f should be above radio %.3f", originalScore.Total, radioScore.Total)
	}
}

func TestScoreCandidateMatchesRequestedExtendedMix(t *testing.T) {
	q := model.MetadataQuery{Title: "Music Sounds Better With You (Extended Mix)", Artist: "Stardust", DurationMS: 392000}
	extended := model.MetadataCandidate{Title: "Music Sounds Better With You - Extended Mix", Artist: "Stardust", DurationMS: 391500}
	plain := model.MetadataCandidate{Title: "Music Sounds Better With You", Artist: "Stardust", DurationMS: 245000}
	if ScoreCandidate(q, extended).Total <= ScoreCandidate(q, plain).Total {
		t.Fatal("requested extended mix should outrank plain version")
	}
}

func TestRankCandidatesClassifiesVersionMismatch(t *testing.T) {
	q := model.MetadataQuery{Title: "Song", Artist: "Artist", DurationMS: 300000}
	items := rankCandidates(q, []model.MetadataCandidate{{Title: "Song (Live)", Artist: "Artist", DurationMS: 300000}})
	if len(items) != 1 {
		t.Fatalf("got %d candidates", len(items))
	}
	if items[0].MatchClass == "exact" || items[0].MatchClass == "high" {
		t.Fatalf("wrong-version candidate classified too strongly: %s %.3f", items[0].MatchClass, items[0].Confidence)
	}
	found := false
	for _, issue := range items[0].MatchIssues {
		if issue == "version_mismatch" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected version_mismatch issue: %+v", items[0].MatchIssues)
	}
}

func TestBuildSuggestedPrefersCatalogSourceForCatalogNumber(t *testing.T) {
	items := []model.MetadataCandidate{
		{Source: "Deezer", Title: "Song", Artist: "Artist", CatalogNumber: "DEEZER-1", Confidence: .94, MatchClass: "high"},
		{Source: "Discogs", Title: "Song", Artist: "Artist", CatalogNumber: "CAT-123", Confidence: .90, MatchClass: "high"},
	}
	got := buildSuggested(items)
	if got.CatalogNumber != "CAT-123" {
		t.Fatalf("catalog number = %q, want Discogs value", got.CatalogNumber)
	}
}
