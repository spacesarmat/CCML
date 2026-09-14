package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

const muzvizorRenderedRowFixture = `
<!doctype html>
<html><body>
<div class="track__row track__row_main">
  <div class="track__column track__column_stage">
    <div class="track__stage track__stage_prime"></div>
  </div>
  <div class="track__column track__column_control">
    <button>Play</button>
    <button>Download</button>
  </div>
  <div class="track__column track__column_title">
    <div class="track__title">Москва (Nei Blend)</div>
    <div class="track__artist">Винтаж, DJ Smash</div>
    <span>TOP 100</span>
  </div>
  <div class="track__column track__column_bpm"><span>140</span></div>
  <div class="track__column track__column_key"><span>11A</span></div>
  <div class="track__column track__column_genre">
    <span>House,</span>
    <span>Pop</span>
  </div>
</div>
</body></html>`

func TestMuzvizorRenderedDOMParsesColumnsAndStage(t *testing.T) {
	t.Parallel()

	items, rowsFound := muzvizorRenderedCandidatesFromHTML(
		muzvizorRenderedRowFixture,
		"https://muzvizor.com/genres/house",
		model.MetadataQuery{Artist: "Винтаж, DJ Smash", Title: "Москва (Nei Blend)"},
	)
	if !rowsFound {
		t.Fatal("expected rendered MUZVIZOR row markup")
	}
	if len(items) != 1 {
		t.Fatalf("candidates = %d, want 1: %+v", len(items), items)
	}
	got := items[0]
	if got.Title != "Москва (Nei Blend)" || got.Artist != "Винтаж, DJ Smash" {
		t.Fatalf("unexpected identity: %+v", got)
	}
	if got.BPM != 140 || got.Key != "11A" || got.KeyScale != "camelot" {
		t.Fatalf("unexpected BPM/key: %+v", got)
	}
	if got.Genre != "House, Pop" {
		t.Fatalf("genre = %q, want House, Pop", got.Genre)
	}
	if got.Stage != "Prime Time" {
		t.Fatalf("stage = %q, want Prime Time", got.Stage)
	}
}

func TestMuzvizorRenderedDOMRejectsUnrelatedTrack(t *testing.T) {
	t.Parallel()

	items, rowsFound := muzvizorRenderedCandidatesFromHTML(
		muzvizorRenderedRowFixture,
		"https://muzvizor.com/genres/house",
		model.MetadataQuery{Artist: "Someone Else", Title: "Different Track"},
	)
	if !rowsFound {
		t.Fatal("expected rendered row markup")
	}
	if len(items) != 0 {
		t.Fatalf("unexpected unrelated candidates: %+v", items)
	}
}

func TestMuzvizorSearchFallsBackFromJSShellToPublicGenrePage(t *testing.T) {
	t.Parallel()

	requested := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested[r.URL.Path]++
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		switch r.URL.Path {
		case "/tracks":
			// The browser can render rows later with JavaScript, but the Go
			// backend sees only this page shell.
			_, _ = w.Write([]byte(`<html><body><div id="app"></div></body></html>`))
		case "/genres":
			_, _ = w.Write([]byte(`<html><body>
				<a href="/genres/pop">Pop</a>
				<a href="/genres/house">House</a>
			</body></html>`))
		case "/genres/house":
			_, _ = w.Write([]byte(muzvizorRenderedRowFixture))
		case "/genres/pop":
			_, _ = w.Write([]byte(`<html><body></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := newMuzvizorProviderWithBaseURL(server.URL, "CCML test")
	items, err := provider.Search(context.Background(), model.MetadataQuery{
		Artist: "Винтаж, DJ Smash",
		Title:  "Москва (Nei Blend)",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected fallback result count: %d: %+v", len(items), items)
	}
	got := items[0]
	if got.BPM != 140 || got.Key != "11A" || got.Genre != "House, Pop" || got.Stage != "Prime Time" {
		t.Fatalf("unexpected fallback result: %+v", got)
	}
	if requested["/tracks"] != 1 || requested["/genres"] != 1 || requested["/genres/house"] != 1 {
		t.Fatalf("unexpected request sequence: %+v", requested)
	}
	if requested["/genres/pop"] != 0 {
		t.Fatalf("fallback should stop after House match: %+v", requested)
	}
}

func TestMuzvizorGenreFallbackCachesPublicPages(t *testing.T) {
	t.Parallel()

	requested := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested[r.URL.Path]++
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/tracks":
			_, _ = w.Write([]byte(`<html><body><div id="app"></div></body></html>`))
		case "/genres":
			_, _ = w.Write([]byte(`<a href="/genres/house">House</a>`))
		case "/genres/house":
			_, _ = w.Write([]byte(muzvizorRenderedRowFixture))
		case "/genres/pop":
			_, _ = w.Write([]byte(`<html><body></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := newMuzvizorProviderWithBaseURL(server.URL, "CCML test")
	query := model.MetadataQuery{Artist: "Винтаж, DJ Smash", Title: "Москва (Nei Blend)"}

	for i := 0; i < 2; i++ {
		items, err := provider.Search(context.Background(), query)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 {
			t.Fatalf("search %d: unexpected result: %+v", i+1, items)
		}
	}

	if requested["/genres"] != 1 || requested["/genres/house"] != 1 {
		t.Fatalf("genre index/page should be cached: %+v", requested)
	}
}
