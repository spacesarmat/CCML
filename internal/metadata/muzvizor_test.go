package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

const muzvizorFixture = `
<!doctype html>
<html><body>
<div class="track">
  <span>Coming Back (Extended Mix)</span>
  <span>I Gemin, Secret Atelier</span>
  <span>120</span>
  <span>2A</span>
  <span>Deep</span>
</div>
<div class="track">
  <span>GAZ (VTARYAK, RAZNOST REMIX)</span>
  <span>Zivert</span>
  <span>132</span>
  <span>2A</span>
  <span>Baile Funk,</span>
  <span>Pop</span>
</div>
<div class="track">
  <span>Москва (Nei Blend)</span>
  <span>Винтаж, DJ Smash</span>
  <span>TOP 100</span>
  <span>140</span>
  <span>11A</span>
  <span>House,</span>
  <span>Pop</span>
</div>
<div class="track">
  <span>Completely Different Song</span>
  <span>Someone Else</span>
  <span>128</span>
  <span>7B</span>
  <span>House</span>
</div>
</body></html>`

func TestMuzvizorProviderKind(t *testing.T) {
	t.Parallel()
	if got := NewMuzvizorProvider("test").Kind(); got != ProviderKindDJPool {
		t.Fatalf("Kind() = %q, want %q", got, ProviderKindDJPool)
	}
}

func TestMuzvizorCandidatesParseDJFields(t *testing.T) {
	t.Parallel()

	items := muzvizorCandidatesFromHTML(muzvizorFixture, "https://muzvizor.com/tracks?search=coming", model.MetadataQuery{
		Artist: "I Gemin, Secret Atelier",
		Title:  "Coming Back (Extended Mix)",
	})
	if len(items) != 1 {
		t.Fatalf("candidates = %d, want 1: %+v", len(items), items)
	}
	got := items[0]
	if got.Source != "MUZVIZOR" || got.SourceKind != ProviderKindDJPool {
		t.Fatalf("unexpected source: %+v", got)
	}
	if got.Title != "Coming Back (Extended Mix)" || got.Artist != "I Gemin, Secret Atelier" {
		t.Fatalf("unexpected identity: %+v", got)
	}
	if got.BPM != 120 || got.Key != "2A" || got.KeyScale != "camelot" {
		t.Fatalf("unexpected DJ fields: %+v", got)
	}
	if got.Genre != "Deep" {
		t.Fatalf("genre = %q, want Deep", got.Genre)
	}
	if got.ExternalID == "" || got.SourceURL == "" {
		t.Fatalf("missing stable source data: %+v", got)
	}
}

func TestMuzvizorParserRejectsUnrelatedTracks(t *testing.T) {
	t.Parallel()

	items := muzvizorCandidatesFromHTML(muzvizorFixture, "https://muzvizor.com/tracks", model.MetadataQuery{
		Artist: "No Such Artist",
		Title:  "No Such Track",
	})
	if len(items) != 0 {
		t.Fatalf("unexpected unrelated candidates: %+v", items)
	}
}

func TestMuzvizorSearchUsesConfirmedPublicTrackQuery(t *testing.T) {
	t.Parallel()

	var requestedPath string
	var requestedQuery string
	var legacySearch string
	var legacyQ string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedQuery = r.URL.Query().Get("query")
		legacySearch = r.URL.Query().Get("search")
		legacyQ = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(muzvizorFixture))
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
	if requestedPath != "/tracks" {
		t.Fatalf("path = %q, want /tracks", requestedPath)
	}
	if requestedQuery != "Винтаж, DJ Smash - Москва (Nei Blend)" {
		t.Fatalf("query = %q", requestedQuery)
	}
	if legacySearch != "" || legacyQ != "" {
		t.Fatalf("legacy guessed query parameters must not be used: search=%q q=%q", legacySearch, legacyQ)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected result count: %d: %+v", len(items), items)
	}
	if items[0].Title != "Москва (Nei Blend)" || items[0].Artist != "Винтаж, DJ Smash" ||
		items[0].BPM != 140 || items[0].Key != "11A" || items[0].Genre != "House, Pop" {
		t.Fatalf("unexpected result: %+v", items[0])
	}
}

func TestMuzvizorBaseTitleFallbackCanMatchVersionedPoolTrack(t *testing.T) {
	t.Parallel()

	items := muzvizorCandidatesFromHTML(muzvizorFixture, "https://muzvizor.com/tracks", model.MetadataQuery{
		Artist: "I Gemin, Secret Atelier",
		Title:  "Coming Back",
	})
	if len(items) != 1 {
		t.Fatalf("base-title query should match versioned pool track: %+v", items)
	}
}
