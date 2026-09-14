package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestITunesProviderParsesSong(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"trackId":42,"trackName":"One More Time","artistName":"Daft Punk","collectionName":"Discovery","releaseDate":"2000-11-30T12:00:00Z","trackTimeMillis":320000,"primaryGenreName":"Dance","trackNumber":1,"trackCount":14,"discNumber":1,"discCount":1,"artworkUrl100":"https://example.test/100x100bb.jpg","trackViewUrl":"https://music.apple.com/test"}]}`))
	}))
	defer server.Close()

	provider := NewITunesProvider("US")
	provider.baseURL = server.URL
	provider.minInterval = 0
	provider.client = &http.Client{Timeout: time.Second}
	items, err := provider.Search(context.Background(), model.MetadataQuery{Artist: "Daft Punk", Title: "One More Time"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Album != "Discovery" || items[0].ReleaseDate != "2000-11-30" || items[0].TrackTotal != 14 {
		t.Fatalf("unexpected candidate: %+v", items[0])
	}
	if !items[0].ArtworkEmbeddable {
		t.Fatal("Apple iTunes artwork should be available for explicit embedding")
	}
	if items[0].ArtworkURL != "https://example.test/1200x1200bb.jpg" {
		t.Fatalf("ArtworkURL = %q", items[0].ArtworkURL)
	}
}
