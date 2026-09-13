package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestYandexMusicProviderSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "OAuth secret-token" {
			t.Fatalf("authorization = %q", got)
		}
		if got := r.Header.Get("Accept-Language"); got != "ru" {
			t.Fatalf("accept-language = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"result":{"tracks":{"results":[{
				"id":"42","title":"Song","version":"Original Mix","durationMs":321000,
				"coverUri":"avatars.yandex.net/get-music-content/123/abc/%%",
				"artists":[{"name":"Artist"}],
				"albums":[{"id":100,"title":"Album","trackCount":10,"genre":"house","year":2026,
					"releaseDate":"2026-05-01","trackPosition":{"volume":1,"index":3},
					"artists":[{"name":"Artist"}],"labels":[{"name":"Label"}]}]
			}]}}
		}`))
	}))
	defer server.Close()

	provider := NewYandexMusicProvider("secret-token", "ru", "CCML-test")
	provider.baseURL = server.URL
	items, err := provider.Search(context.Background(), model.MetadataQuery{Artist: "Artist", Title: "Song"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	got := items[0]
	if got.Source != "Yandex Music" || got.Title != "Song (Original Mix)" || got.Artist != "Artist" {
		t.Fatalf("unexpected candidate: %+v", got)
	}
	if got.Album != "Album" || got.Label != "Label" || got.Genre != "house" || got.TrackNumber != 3 || got.TrackTotal != 10 || got.DiscNumber != 1 {
		t.Fatalf("unexpected release metadata: %+v", got)
	}
	if got.ReleaseDate != "2026-05-01" || got.Year != 2026 || got.DurationMS != 321000 {
		t.Fatalf("unexpected date/duration: %+v", got)
	}
	if got.ArtworkURL != "https://avatars.yandex.net/get-music-content/123/abc/1000x1000" || got.ArtworkEmbeddable {
		t.Fatalf("unexpected artwork: %+v", got)
	}
	if got.SourceURL != "https://music.yandex.ru/album/100/track/42" {
		t.Fatalf("source url = %q", got.SourceURL)
	}
}
