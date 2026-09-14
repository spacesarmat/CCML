package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

const bananaStreetFixture = `
<!doctype html>
<html><body>
<nav>
  <a>Главное</a><a>Новинки</a><a>Поиск</a><a>Стили</a>
</nav>
<section>
  <article>
    <a href="/113101-vadim-adamov-restless">Restless</a>
    <a href="/vadimadamov">Vadim Adamov</a>
    <span>12</span>
    <span>1</span>
    <a href="/genres/deep-house">deep house</a>
    <span>1 269</span>
  </article>
  <article>
    <a href="/113102-vintage-dj-smash-moskva-nei-blend">Москва (Nei Blend)</a>
    <a href="/vintage-dj-smash">Винтаж, DJ Smash</a>
    <span>9</span>
    <span>0</span>
    <a href="/genres/pop">pop</a>
    <span>521</span>
  </article>
  <article>
    <a href="/113103-other-track">Completely Different Song</a>
    <a href="/someone">Someone Else</a>
    <span>3</span>
    <span>0</span>
    <a href="/genres/house">house</a>
    <span>777</span>
  </article>
</section>
</body></html>`

func TestBananaStreetCountLineSupportsSpacesAndNBSP(t *testing.T) {
	t.Parallel()

	cases := []string{"0", "12", "1 269", "1\u00a0269"}
	for _, value := range cases {
		if !bananaStreetCountLine(value) {
			t.Fatalf("expected count line for %q", value)
		}
	}
	for _, value := range []string{"", "12 likes", "-1", "12.5"} {
		if bananaStreetCountLine(value) {
			t.Fatalf("unexpected count line for %q", value)
		}
	}
}

func TestBananaStreetProviderKind(t *testing.T) {
	t.Parallel()

	provider := newBananaStreetProviderWithBaseURL("https://example.test", "CCML test")
	if got := provider.Name(); got != "Bananastreet" {
		t.Fatalf("Name() = %q", got)
	}
	if got := provider.Kind(); got != ProviderKindDJPool {
		t.Fatalf("Kind() = %q, want %q", got, ProviderKindDJPool)
	}
}

func TestBananaStreetCandidatesParsePublicFields(t *testing.T) {
	t.Parallel()

	items := bananaStreetCandidatesFromHTML(
		bananaStreetFixture,
		"https://bananastreet.ru/search?q=test",
		model.MetadataQuery{Artist: "Винтаж, DJ Smash", Title: "Москва (Nei Blend)"},
	)
	if len(items) != 1 {
		t.Fatalf("candidates = %d, want 1: %+v", len(items), items)
	}
	got := items[0]
	if got.Artist != "Винтаж, DJ Smash" || got.Title != "Москва (Nei Blend)" {
		t.Fatalf("unexpected identity: %+v", got)
	}
	if got.Genre != "pop" {
		t.Fatalf("genre = %q, want pop", got.Genre)
	}
	if got.BPM != 0 || got.Key != "" || got.KeyScale != "" {
		t.Fatalf("unconfirmed BPM/Key must remain empty: %+v", got)
	}
	if got.SourceKind != ProviderKindDJPool {
		t.Fatalf("source kind = %q", got.SourceKind)
	}
}

func TestBananaStreetParsesObservedCompactSearchResult(t *testing.T) {
	t.Parallel()

	const fixture = `
	<html><body>
	  <div>Happy Deny</div>
	  <div>•</div>
	  <div>Rodionov1977</div>
	  <button>Слушать</button>
	  <a href="/track/example">Vadim Adamov, Hardphol, MVRGØ - У тебя одной</a>
	  <a href="/user/vadim-adamov">Vadim Adamov</a>
	  <button>Слушать</button>
	  <div>Hosted by SKWIIK - Sibanium Radio EP074</div>
	  <div>PROGRAMIQA Radio</div>
	</body></html>`

	items := bananaStreetCandidatesFromHTML(
		fixture,
		"https://bananastreet.ru/search?q=test",
		model.MetadataQuery{
			Artist: "Vadim Adamov, Hardphol, Mvrgø",
			Title:  "У Тебя Одной",
		},
	)
	if len(items) != 1 {
		t.Fatalf("compact search candidate = %+v", items)
	}
	got := items[0]
	if got.Artist != "Vadim Adamov, Hardphol, MVRGØ" || got.Title != "У тебя одной" {
		t.Fatalf("unexpected identity: %+v", got)
	}
	if got.Genre != "" || got.BPM != 0 || got.Key != "" {
		t.Fatalf("unconfirmed compact-row fields must stay empty: %+v", got)
	}
}

func TestBananaStreetCombinedLineChoosesBestSeparator(t *testing.T) {
	t.Parallel()

	item, ok := bananaStreetCombinedLineCandidate(
		"Artist One - Track Name - Club Edit",
		"https://bananastreet.ru/search?q=test",
		model.MetadataQuery{Artist: "Artist One", Title: "Track Name - Club Edit"},
	)
	if !ok {
		t.Fatal("expected combined-line candidate")
	}
	if item.Artist != "Artist One" || item.Title != "Track Name - Club Edit" {
		t.Fatalf("unexpected split: %+v", item)
	}
}

func TestBananaStreetParserRejectsUnrelatedTracks(t *testing.T) {
	t.Parallel()

	items := bananaStreetCandidatesFromHTML(
		bananaStreetFixture,
		"https://bananastreet.ru/search?q=test",
		model.MetadataQuery{Artist: "No Such Artist", Title: "No Such Title"},
	)
	if len(items) != 0 {
		t.Fatalf("unexpected candidates: %+v", items)
	}
}

func TestBananaStreetBaseTitleFallbackCanMatchVersionedTrack(t *testing.T) {
	t.Parallel()

	query := model.MetadataQuery{
		Artist: "Vadim Adamov",
		Title:  "Restless (Extended Mix)",
	}
	items := bananaStreetCandidatesFromHTML(
		bananaStreetFixture,
		"https://bananastreet.ru/search?q=test",
		query,
	)
	if len(items) != 1 {
		t.Fatalf("base-title similarity should match versioned query: %+v", items)
	}
	if items[0].Title != "Restless" {
		t.Fatalf("title = %q", items[0].Title)
	}
}

func TestBananaStreetSearchURLEscapesQuerySeparators(t *testing.T) {
	t.Parallel()

	got := bananaStreetSearchURL(
		"https://bananastreet.ru",
		"Simon & Garfunkel - Bridge Over Troubled Water",
	)
	want := "https://bananastreet.ru/search?q=Simon%20%26%20Garfunkel%20-%20Bridge%20Over%20Troubled%20Water"
	if got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}

func TestBananaStreetSearchUsesBrowserCompatiblePublicSearchPage(t *testing.T) {
	t.Parallel()

	const expectedQuery = "Винтаж, DJ Smash - Москва (Nei Blend)"
	const expectedRawQuery = "q=%D0%92%D0%B8%D0%BD%D1%82%D0%B0%D0%B6%2C%20DJ%20Smash%20-%20%D0%9C%D0%BE%D1%81%D0%BA%D0%B2%D0%B0%20%28Nei%20Blend%29"

	var requestedPath string
	var requestedQuery string
	var requestedRawQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedQuery = r.URL.Query().Get("q")
		requestedRawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(bananaStreetFixture))
	}))
	defer server.Close()

	provider := newBananaStreetProviderWithBaseURL(server.URL, "CCML test")
	items, err := provider.Search(context.Background(), model.MetadataQuery{
		Artist: "Винтаж, DJ Smash",
		Title:  "Москва (Nei Blend)",
	})
	if err != nil {
		t.Fatal(err)
	}
	if requestedPath != "/search" {
		t.Fatalf("path = %q, want /search", requestedPath)
	}
	if requestedQuery != expectedQuery {
		t.Fatalf("q = %q, want %q", requestedQuery, expectedQuery)
	}
	if requestedRawQuery != expectedRawQuery {
		t.Fatalf("raw query = %q, want %q", requestedRawQuery, expectedRawQuery)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected search result: %+v", items)
	}
}
