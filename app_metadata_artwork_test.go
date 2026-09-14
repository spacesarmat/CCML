package main

import (
	"errors"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestIsArtworkFetchError(t *testing.T) {
	positive := []string{
		`download artwork: HTTP 404 Not Found`,
		`download artwork: HTTP 403 Forbidden`,
		`download artwork: HTTP 503 Service Unavailable`,
		`download artwork: Get "https://i.discogs.com/example.jpg": dial tcp: lookup i.discogs.com: no such host`,
		`download artwork: Get "https://coverartarchive.org/example": context deadline exceeded`,
		`parse artwork URL: parse "::": missing protocol scheme`,
		`unsupported artwork URL scheme "ftp"`,
		`artwork URL has no host`,
		`read artwork response: unexpected EOF`,
		`close artwork response: connection reset by peer`,
		`artwork exceeds 15 MiB`,
		`cover image exceeds 15 MiB`,
		`cover image is empty`,
		`unsupported cover image type "image/webp"; use JPEG or PNG`,
	}
	for _, value := range positive {
		if !isArtworkFetchError(errors.New(value)) {
			t.Fatalf("expected artwork error classification for %q", value)
		}
	}

	negative := []string{
		`write metadata: permission denied`,
		`track 7 not found`,
		`metadata candidate has no applicable fields`,
		`update track tags: database is locked`,
	}
	for _, value := range negative {
		if isArtworkFetchError(errors.New(value)) {
			t.Fatalf("unexpected artwork error classification for %q", value)
		}
	}

	if isArtworkFetchError(nil) {
		t.Fatal("nil error must not be classified as artwork fetch error")
	}
}

func TestEnrichmentArtworkOptionsProvideTrustedFallbacks(t *testing.T) {
	primary := model.MetadataCandidate{
		Source:            "CCML Merge",
		Confidence:        0.97,
		ArtworkURL:        "https://coverartarchive.org/release/example/front-1200",
		ArtworkWidth:      1200,
		ArtworkHeight:     1200,
		ArtworkEmbeddable: true,
	}
	ranked := []model.MetadataCandidate{
		{
			Source:            "Spotify",
			Confidence:        0.96,
			MatchClass:        "exact",
			ArtworkURL:        "https://i.scdn.co/image/good",
			ArtworkWidth:      1000,
			ArtworkHeight:     1000,
			ArtworkEmbeddable: true,
		},
		{
			Source:            "MusicBrainz",
			Confidence:        0.95,
			MatchClass:        "high",
			ArtworkURL:        "https://coverartarchive.org/release/example/front-1200",
			ArtworkWidth:      1200,
			ArtworkHeight:     1200,
			ArtworkEmbeddable: true,
		},
		{
			Source:            "Deezer",
			Confidence:        0.90,
			MatchClass:        "high",
			ArtworkURL:        "https://e-cdns-images.dzcdn.net/images/cover/good/1000x1000.jpg",
			ArtworkWidth:      1000,
			ArtworkHeight:     1000,
			ArtworkEmbeddable: true,
		},
		{
			Source:            "Rejected",
			Confidence:        0.99,
			MatchClass:        "rejected",
			ArtworkURL:        "https://example.invalid/wrong.jpg",
			ArtworkEmbeddable: true,
		},
		{
			Source:            "Below threshold",
			Confidence:        0.80,
			MatchClass:        "high",
			ArtworkURL:        "https://example.invalid/low.jpg",
			ArtworkEmbeddable: true,
		},
	}

	got := enrichmentArtworkOptions(primary, ranked, 0.86)
	if len(got) != 3 {
		t.Fatalf("artwork options = %d, want 3: %+v", len(got), got)
	}
	if got[0].Source != "CCML Merge" || got[1].Source != "Spotify" || got[2].Source != "Deezer" {
		t.Fatalf("unexpected artwork fallback order: %+v", got)
	}
}

func TestEnrichmentArtworkOptionsUseProviderWhenMergeHasNoArtwork(t *testing.T) {
	primary := model.MetadataCandidate{Source: "CCML Merge", Confidence: 0.95}
	ranked := []model.MetadataCandidate{
		{
			Source:            "Deezer",
			Confidence:        0.90,
			MatchClass:        "high",
			ArtworkURL:        "https://e-cdns-images.dzcdn.net/images/cover/good/1000x1000.jpg",
			ArtworkEmbeddable: true,
		},
	}

	got := enrichmentArtworkOptions(primary, ranked, 0.86)
	if len(got) != 1 || got[0].Source != "Deezer" {
		t.Fatalf("unexpected artwork fallback options: %+v", got)
	}
}
