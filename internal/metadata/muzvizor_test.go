package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestMuzvizorSearchUsesPublicTrackQuery(t *testing.T) {
	t.Parallel()

	var requestedPath string
	var requestedSearch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedSearch = r.URL.Query().Get("search")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(muzvizorFixture))
	}))
	defer server.Close()

	provider := newMuzvizorProviderWithBaseURL(server.URL, "CCML test")
	items, err := provider.Search(context.Background(), model.MetadataQuery{
		Artist: "Zivert",
		Title:  "GAZ (VTARYAK, RAZNOST REMIX)",
	})
	if err != nil {
		t.Fatal(err)
	}
	if requestedPath != "/tracks" {
		t.Fatalf("path = %q, want /tracks", requestedPath)
	}
	if !strings.Contains(strings.ToLower(requestedSearch), "zivert") ||
		!strings.Contains(strings.ToLower(requestedSearch), "gaz") {
		t.Fatalf("search term = %q", requestedSearch)
	}
	if len(items) != 1 || items[0].BPM != 132 || items[0].Key != "2A" {
		t.Fatalf("unexpected result: %+v", items)
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
