package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestTraxsourceBridgeCandidates(t *testing.T) {
	body := []byte(`{"status":"success","data":{"tracks":[{"track_id":"14660000","title":"Magic Carpet","version":"Extended Mix","duration":"6:48","artists":[{"name":"Alex Galvan"}],"label":{"name":"Test Label"},"genre":{"name":"Deep House"},"release_date":"2026-08-01","url":"https://www.traxsource.com/track/14660000/magic-carpet","artwork_url":"https://example.test/cover.jpg"}]}}`)
	items, err := parseTraxsourceBridgeCandidates(body)
	if err != nil {
		t.Fatalf("parse fallback: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	got := items[0]
	if got.Source != "Traxsource" || got.ExternalID != "14660000" || got.Title != "Magic Carpet (Extended Mix)" || got.Artist != "Alex Galvan" {
		t.Fatalf("unexpected candidate: %+v", got)
	}
	if got.Label != "Test Label" || got.Genre != "Deep House" || got.DurationMS != 408000 || got.Year != 2026 {
		t.Fatalf("unexpected fallback fields: %+v", got)
	}
}

func TestTraxsourceFallsBackAfterCloudflareForbidden(t *testing.T) {
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<html><title>Just a moment...</title>Cloudflare human verification</html>"))
	}))
	defer direct.Close()

	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "secret" {
			t.Fatalf("missing fallback API key")
		}
		if r.URL.Query().Get("term") != "Fallback Artist Fallback Song" {
			t.Fatalf("term = %q", r.URL.Query().Get("term"))
		}
		if r.URL.Query().Get("type") != "tracks" {
			t.Fatalf("type = %q", r.URL.Query().Get("type"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"tracks":[{"track_id":"55","title":"Fallback Song","artists":[{"name":"Fallback Artist"}],"duration":"4:12","url":"https://www.traxsource.com/track/55/fallback-song"}]}}`))
	}))
	defer bridge.Close()

	provider := NewTraxsourceProviderWithFallback("CCML-test", "secret")
	provider.baseURL = direct.URL
	provider.bridgeURL = bridge.URL
	provider.client = &http.Client{Timeout: time.Second}

	items, err := provider.Search(context.Background(), model.MetadataQuery{Artist: "Fallback Artist", Title: "Fallback Song"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(items) != 1 || items[0].ExternalID != "55" {
		t.Fatalf("unexpected fallback result: %+v", items)
	}
}
