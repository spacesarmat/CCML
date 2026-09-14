package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

const mixcloudSearchFixture = `{
  "data": [
    {
      "key": "/club-uploader/vadim-adamov-hardphol-mvrgo-u-tebya-odnoy/",
      "name": "Vadim Adamov, Hardphol, MVRGØ - У тебя одной",
      "url": "https://www.mixcloud.com/club-uploader/vadim-adamov-hardphol-mvrgo-u-tebya-odnoy/",
      "user": {
        "name": "Club Uploader",
        "username": "club-uploader",
        "key": "/club-uploader/"
      },
      "tags": [
        {"name": "House", "key": "/discover/house/"},
        {"name": "Promo", "key": "/discover/promo/"}
      ]
    },
    {
      "key": "/other/something-else/",
      "name": "Completely Different Show",
      "url": "https://www.mixcloud.com/other/something-else/",
      "user": {
        "name": "Someone Else",
        "username": "someone-else",
        "key": "/someone-else/"
      },
      "tags": [
        {"name": "Techno", "key": "/discover/techno/"}
      ]
    }
  ]
}`

func TestMixcloudProviderKind(t *testing.T) {
	t.Parallel()

	provider := newMixcloudProviderWithBaseURL("https://example.test", "CCML test")
	if got := provider.Name(); got != "Mixcloud" {
		t.Fatalf("Name() = %q", got)
	}
	if got := provider.Kind(); got != ProviderKindDJPool {
		t.Fatalf("Kind() = %q, want %q", got, ProviderKindDJPool)
	}
}

func TestMixcloudParsesCombinedCloudcastIdentity(t *testing.T) {
	t.Parallel()

	items, err := mixcloudCandidatesFromJSON(
		[]byte(mixcloudSearchFixture),
		"https://api.mixcloud.com/search/?q=test&type=cloudcast",
		model.MetadataQuery{
			Artist: "Vadim Adamov, Hardphol, Mvrgø",
			Title:  "У Тебя Одной",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("candidates = %d, want 1: %+v", len(items), items)
	}
	got := items[0]
	if got.Artist != "Vadim Adamov, Hardphol, MVRGØ" || got.Title != "У тебя одной" {
		t.Fatalf("unexpected identity: %+v", got)
	}
	if got.Genre != "House" {
		t.Fatalf("genre = %q, want House", got.Genre)
	}
	if got.ExternalID != "cloudcast:/club-uploader/vadim-adamov-hardphol-mvrgo-u-tebya-odnoy/" {
		t.Fatalf("external ID = %q", got.ExternalID)
	}
	if got.BPM != 0 || got.Key != "" || got.KeyScale != "" {
		t.Fatalf("Mixcloud does not supply confirmed BPM/Key metadata: %+v", got)
	}
}

func TestMixcloudUsesUploaderOnlyWhenItMatchesArtist(t *testing.T) {
	t.Parallel()

	body := []byte(`{
	  "data": [{
	    "key": "/artist-one/track-name/",
	    "name": "Track Name",
	    "url": "https://www.mixcloud.com/artist-one/track-name/",
	    "user": {"name": "Artist One", "username": "artist-one"},
	    "tags": [{"name": "Tech House"}]
	  }]
	}`)
	items, err := mixcloudCandidatesFromJSON(
		body,
		"https://api.mixcloud.com/search/?q=test&type=cloudcast",
		model.MetadataQuery{Artist: "Artist One", Title: "Track Name"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected candidates: %+v", items)
	}
	if items[0].Artist != "Artist One" || items[0].Title != "Track Name" || items[0].Genre != "Tech House" {
		t.Fatalf("unexpected candidate: %+v", items[0])
	}
}

func TestMixcloudRejectsUploaderMismatch(t *testing.T) {
	t.Parallel()

	body := []byte(`{
	  "data": [{
	    "key": "/radio/show/",
	    "name": "Track Name",
	    "url": "https://www.mixcloud.com/radio/show/",
	    "user": {"name": "Unrelated Radio Station", "username": "radio"},
	    "tags": [{"name": "House"}]
	  }]
	}`)
	items, err := mixcloudCandidatesFromJSON(
		body,
		"https://api.mixcloud.com/search/?q=test&type=cloudcast",
		model.MetadataQuery{Artist: "Artist One", Title: "Track Name"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("uploader mismatch must be rejected: %+v", items)
	}
}

func TestMixcloudSearchUsesOfficialPublicAPI(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotQ string
	var gotType string
	var gotLimit string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQ = r.URL.Query().Get("q")
		gotType = r.URL.Query().Get("type")
		gotLimit = r.URL.Query().Get("limit")
		if r.Header.Get("Authorization") != "" {
			t.Fatal("read-only Mixcloud search must not send Authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mixcloudSearchFixture))
	}))
	defer server.Close()

	provider := newMixcloudProviderWithBaseURL(server.URL, "CCML test")
	items, err := provider.Search(context.Background(), model.MetadataQuery{
		Artist: "Vadim Adamov, Hardphol, Mvrgø",
		Title:  "У Тебя Одной",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/search/" {
		t.Fatalf("path = %q, want /search/", gotPath)
	}
	if gotQ != "Vadim Adamov, Hardphol, Mvrgø - У Тебя Одной" {
		t.Fatalf("q = %q", gotQ)
	}
	if gotType != "cloudcast" {
		t.Fatalf("type = %q, want cloudcast", gotType)
	}
	if gotLimit != "20" {
		t.Fatalf("limit = %q, want 20", gotLimit)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected search result: %+v", items)
	}
}

func TestMixcloudGenreTagsStayConservative(t *testing.T) {
	t.Parallel()

	got := mixcloudGenreFromTags([]mixcloudTag{
		{Name: "House"},
		{Name: "Promo"},
		{Name: "Techno"},
		{Name: "Episode 475"},
		{Name: "house"},
	})
	if got != "House, Techno" {
		t.Fatalf("genre = %q, want House, Techno", got)
	}
}
