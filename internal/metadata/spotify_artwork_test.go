package metadata

import "testing"

func TestSelectSpotifyArtworkPrefersLargestValidImage(t *testing.T) {
	t.Parallel()

	url, width, height, embeddable := selectSpotifyArtwork([]spotifyImage{
		{URL: "https://i.scdn.co/image/small", Width: 64, Height: 64},
		{URL: "ftp://i.scdn.co/image/invalid", Width: 1200, Height: 1200},
		{URL: "https://i.scdn.co/image/large", Width: 640, Height: 640},
		{URL: "https://i.scdn.co/image/medium", Width: 300, Height: 300},
	})

	if !embeddable {
		t.Fatal("expected Spotify artwork to be embeddable")
	}
	if url != "https://i.scdn.co/image/large" {
		t.Fatalf("url = %q, want largest valid image", url)
	}
	if width != 640 || height != 640 {
		t.Fatalf("size = %dx%d, want 640x640", width, height)
	}
}

func TestSelectSpotifyArtworkRejectsMissingOrUnsupportedURL(t *testing.T) {
	t.Parallel()

	for _, images := range [][]spotifyImage{
		nil,
		{{URL: "", Width: 640, Height: 640}},
		{{URL: "spotify:image:abc", Width: 640, Height: 640}},
		{{URL: "ftp://example.com/cover.jpg", Width: 640, Height: 640}},
	} {
		url, width, height, embeddable := selectSpotifyArtwork(images)
		if embeddable {
			t.Fatalf("unexpected embeddable artwork: %q %dx%d", url, width, height)
		}
		if url != "" || width != 0 || height != 0 {
			t.Fatalf("unexpected artwork output: %q %dx%d", url, width, height)
		}
	}
}

func TestSelectSpotifyArtworkAcceptsHTTPFallback(t *testing.T) {
	t.Parallel()

	url, width, height, embeddable := selectSpotifyArtwork([]spotifyImage{
		{URL: "http://example.com/cover.jpg", Width: 300, Height: 300},
	})
	if !embeddable || url == "" || width != 300 || height != 300 {
		t.Fatalf("unexpected result: %q %dx%d embeddable=%v", url, width, height, embeddable)
	}
}
