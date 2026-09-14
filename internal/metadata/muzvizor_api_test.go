package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

const muzvizorAPIFixture = `{
  "count": 1,
  "results": [
    {
      "id": 3812,
      "title": "Москва (Nei Blend)",
      "artists": [
        {"name": "Винтаж"},
        {"name": "DJ Smash"}
      ],
      "bpm": 140,
      "key": {"name": "11A"},
      "genres": [
        {"name": "Pop"},
        {"name": "House"}
      ],
      "stage": {"slug": "prime"}
    }
  ]
}`

func TestMuzvizorAPIURLEscapesQuerySeparators(t *testing.T) {
	t.Parallel()

	got := muzvizorAPIURL(
		"https://muzvizor.com",
		"Simon & Garfunkel - Bridge Over Troubled Water",
	)
	want := "https://muzvizor.com/api/v1/tracks/?query=Simon%20%26%20Garfunkel%20-%20Bridge%20Over%20Troubled%20Water"
	if got != want {
		t.Fatalf("API URL = %q, want %q", got, want)
	}
}

func TestMuzvizorAPIURLUsesConfirmedPublicEndpoint(t *testing.T) {
	t.Parallel()

	got := muzvizorAPIURL(
		"https://muzvizor.com",
		"Винтаж, DJ Smash - Москва (Nei Blend)",
	)
	want := "https://muzvizor.com/api/v1/tracks/?query=%D0%92%D0%B8%D0%BD%D1%82%D0%B0%D0%B6%2C%20DJ%20Smash%20-%20%D0%9C%D0%BE%D1%81%D0%BA%D0%B2%D0%B0%20%28Nei%20Blend%29"
	if got != want {
		t.Fatalf("API URL = %q, want %q", got, want)
	}
}

func TestMuzvizorAPIEnvelopeIsNotTrackCandidate(t *testing.T) {
	t.Parallel()

	var root any
	if err := json.Unmarshal([]byte(muzvizorAPIFixture), &root); err != nil {
		t.Fatal(err)
	}
	envelope, ok := root.(map[string]any)
	if !ok {
		t.Fatalf("unexpected root type %T", root)
	}
	if item, ok := muzvizorAPICandidateFromObject(envelope, "https://muzvizor.com/tracks"); ok {
		t.Fatalf("API envelope must not become a track candidate: %+v", item)
	}
}

func TestMuzvizorAPIParsesDJMetadata(t *testing.T) {
	t.Parallel()

	items, err := muzvizorCandidatesFromAPIJSON(
		[]byte(muzvizorAPIFixture),
		"https://muzvizor.com/tracks?query=example",
		model.MetadataQuery{
			Artist: "Винтаж, DJ Smash",
			Title:  "Москва (Nei Blend)",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("candidates = %d, want 1: %+v", len(items), items)
	}
	got := items[0]
	if got.Artist != "Винтаж, DJ Smash" || got.Title != "Москва (Nei Blend)" {
		t.Fatalf("unexpected identity: %+v", got)
	}
	if got.BPM != 140 || got.Key != "11A" || got.KeyScale != "camelot" {
		t.Fatalf("unexpected BPM/key: %+v", got)
	}
	if got.Genre != "Pop, House" {
		t.Fatalf("genre = %q, want Pop, House", got.Genre)
	}
	if got.Stage != "Prime Time" {
		t.Fatalf("stage = %q, want Prime Time", got.Stage)
	}
	if got.ExternalID != "api:3812" {
		t.Fatalf("external ID = %q", got.ExternalID)
	}
}

func TestMuzvizorAPIParserAcceptsAlternatePublicJSONShape(t *testing.T) {
	t.Parallel()

	body := []byte(`{
	  "data": {
	    "items": [{
	      "name": "Москва (Nei Blend)",
	      "artist": "Винтаж, DJ Smash",
	      "tempo": "140",
	      "camelot_key": "11A",
	      "genre": "Pop, House",
	      "stage_name": "prime_time"
	    }]
	  }
	}`)
	items, err := muzvizorCandidatesFromAPIJSON(
		body,
		"https://muzvizor.com/tracks?query=example",
		model.MetadataQuery{Artist: "Винтаж, DJ Smash", Title: "Москва (Nei Blend)"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected candidates: %+v", items)
	}
	got := items[0]
	if got.BPM != 140 || got.Key != "11A" || got.Genre != "Pop, House" || got.Stage != "Prime Time" {
		t.Fatalf("unexpected candidate: %+v", got)
	}
}

func TestMuzvizorAPICollectObjectsStopsAtDepthLimit(t *testing.T) {
	t.Parallel()

	root := map[string]any{}
	current := root
	for depth := 0; depth < muzvizorAPIMaxTraversalDepth+8; depth++ {
		next := map[string]any{}
		current["child"] = next
		current = next
	}
	current["title"] = "Too Deep"

	objects := make([]map[string]any, 0)
	muzvizorAPICollectObjects(root, &objects)
	if got, wantMax := len(objects), muzvizorAPIMaxTraversalDepth+1; got > wantMax {
		t.Fatalf("collected objects = %d, want <= %d", got, wantMax)
	}
	for _, object := range objects {
		if title, _ := object["title"].(string); title == "Too Deep" {
			t.Fatal("collector traversed beyond the configured depth limit")
		}
	}
}

func TestMuzvizorAPIRecursiveValueReadersStopAtDepthLimit(t *testing.T) {
	t.Parallel()

	deep := any("11A")
	for depth := 0; depth < muzvizorAPIMaxTraversalDepth+8; depth++ {
		deep = map[string]any{"value": deep}
	}
	if got := muzvizorAPIText(deep); got != "" {
		t.Fatalf("deep text = %q, want empty after traversal limit", got)
	}
	if bpm, ok := muzvizorAPIBPM(deep); ok || bpm != 0 {
		t.Fatalf("deep BPM = %v, %v; want rejected", bpm, ok)
	}
	if key, ok := muzvizorAPICamelot(deep); ok || key != "" {
		t.Fatalf("deep Camelot = %q, %v; want rejected", key, ok)
	}
}

func TestMuzvizorAPIDeepCandidateIsIgnoredSafely(t *testing.T) {
	t.Parallel()

	candidate := `{"title":"Too Deep","artist":"Artist","bpm":128,"key":"8A"}`
	body := candidate
	for depth := 0; depth < muzvizorAPIMaxTraversalDepth+8; depth++ {
		body = `{"child":` + body + `}`
	}

	items, err := muzvizorCandidatesFromAPIJSON(
		[]byte(body),
		"https://muzvizor.com/tracks?query=example",
		model.MetadataQuery{Artist: "Artist", Title: "Too Deep"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("deep candidate must be ignored, got %+v", items)
	}
}

func TestMuzvizorSearchUsesPublicAPIWithoutBrowserCookies(t *testing.T) {
	t.Parallel()

	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path+"?"+r.URL.RawQuery)
		if r.URL.Path != "/api/v1/tracks/" {
			t.Fatalf("unexpected fallback request: %s", r.URL.String())
		}
		if r.Header.Get("Cookie") != "" {
			t.Fatalf("MUZVIZOR API request must not copy browser cookies")
		}
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("MUZVIZOR API request must not send authorization")
		}
		if !strings.Contains(r.Header.Get("Accept"), "application/json") {
			t.Fatalf("Accept = %q", r.Header.Get("Accept"))
		}
		if got := r.URL.Query().Get("query"); got != "Винтаж, DJ Smash - Москва (Nei Blend)" {
			t.Fatalf("query = %q", got)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(muzvizorAPIFixture))
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
		t.Fatalf("unexpected API search result: %+v", items)
	}
	if len(requests) != 1 {
		t.Fatalf("API match should stop before HTML fallbacks: %+v", requests)
	}
}
