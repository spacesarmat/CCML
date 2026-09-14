package main

import (
	"errors"
	"testing"
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
