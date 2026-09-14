package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

const jesteiTrackFixture = `
<!doctype html>
<html>
<head>
  <title>DJ Smash - Moscow Never Sleeps (P0Lka &amp; 2Yards Remix) | Jestei Pool</title>
</head>
<body>
  <div>Это трек из раздела Record Pool. Он доступен при любом платном тарифе.</div>
  <div>DJ Smash</div>
  <h1>Moscow Never Sleeps (P0Lka &amp; 2Yards Remix)</h1>
  <button>Скачать</button>
  <div>Пляжная вечеринка</div>
  <div>Лучшие треки месяца</div>
  <div>DJ SMASH</div>
  <div>Это жанр трека. Обычно жанры пересекаются, поэтому мы указываем сразу несколько.</div>
  <div>Pop</div>
  <div>Phonk</div>
  <div>Baile Funk</div>
  <div>Это маркировки — они помогут быстро определить, в чем «фишка» трека.</div>
  <div>Русское</div>
  <div>8A</div>
  <div>135</div>
  <div>Количество ударов в минуту</div>
  <div>Подсказка, для какой части ночи подойдет трек: открытие, прайм-тайм или закрытие.</div>
  <div>Primetime</div>
</body>
</html>`

func TestJesteiProviderKind(t *testing.T) {
	t.Parallel()

	provider := newJesteiProviderWithBaseURL("https://example.test", "CCML test")
	if got := provider.Name(); got != "Jestei Pool" {
		t.Fatalf("Name() = %q", got)
	}
	if got := provider.Kind(); got != ProviderKindDJPool {
		t.Fatalf("Kind() = %q, want %q", got, ProviderKindDJPool)
	}
}

func TestJesteiSearchURL(t *testing.T) {
	t.Parallel()

	got := jesteiSearchURL("https://jesteipool.ru", "DJ Smash - Moscow Never Sleeps (P0Lka & 2Yards Remix)")
	want := "https://jesteipool.ru/search?q=DJ%20Smash%20-%20Moscow%20Never%20Sleeps%20%28P0Lka%20%26%202Yards%20Remix%29"
	if got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}

func TestJesteiTrackURLsFromSearchHTML(t *testing.T) {
	t.Parallel()

	doc := `
	  <a href="/track/281016">one</a>
	  <script>window.__x={"/url":"/track/281016"}</script>
	  <a href="https://jesteipool.ru/track/281017">two</a>
	`
	got := jesteiTrackURLsFromHTML(doc, "https://jesteipool.ru")
	if len(got) != 2 {
		t.Fatalf("urls = %+v", got)
	}
	if got[0] != "https://jesteipool.ru/track/281016" || got[1] != "https://jesteipool.ru/track/281017" {
		t.Fatalf("unexpected urls: %+v", got)
	}
}

func TestJesteiParsesPublicTrackMetadata(t *testing.T) {
	t.Parallel()

	item, ok := jesteiCandidateFromTrackHTML(
		jesteiTrackFixture,
		"https://jesteipool.ru/track/281016",
		model.MetadataQuery{
			Artist: "DJ Smash",
			Title:  "Moscow Never Sleeps (P0Lka & 2Yards Remix)",
		},
	)
	if !ok {
		t.Fatal("expected candidate")
	}
	if item.Artist != "DJ Smash" || item.Title != "Moscow Never Sleeps (P0Lka & 2Yards Remix)" {
		t.Fatalf("identity = %+v", item)
	}
	if item.Genre != "Pop, Phonk, Baile Funk" {
		t.Fatalf("genre = %q", item.Genre)
	}
	if item.BPM != 135 {
		t.Fatalf("BPM = %v", item.BPM)
	}
	if item.Key != "8A" || item.KeyScale != "camelot" {
		t.Fatalf("key = %q scale=%q", item.Key, item.KeyScale)
	}
	if item.Stage != "Prime Time" {
		t.Fatalf("stage = %q", item.Stage)
	}
	if item.ExternalID != "track:281016" {
		t.Fatalf("external ID = %q", item.ExternalID)
	}
}

func TestJesteiIdentitySupportsTitleSeparator(t *testing.T) {
	t.Parallel()

	artist, title, ok := jesteiSplitIdentity(
		"Artist One - Track Name - Club Edit",
		model.MetadataQuery{Artist: "Artist One", Title: "Track Name - Club Edit"},
	)
	if !ok {
		t.Fatal("expected split")
	}
	if artist != "Artist One" || title != "Track Name - Club Edit" {
		t.Fatalf("split artist=%q title=%q", artist, title)
	}
}

func TestJesteiSearchFetchesPublicDetailWithoutAuth(t *testing.T) {
	t.Parallel()

	var searchQ string
	var sawAuth bool
	var sawCookie bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			sawAuth = true
		}
		if r.Header.Get("Cookie") != "" {
			sawCookie = true
		}
		switch r.URL.Path {
		case "/search":
			searchQ = r.URL.Query().Get("q")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<a href="/track/281016">DJ Smash</a><a href="/track/999999">Other</a>`))
		case "/track/281016":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(jesteiTrackFixture))
		case "/track/999999":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><head><title>Someone Else - Other Track | Jestei Pool</title></head><body></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := newJesteiProviderWithBaseURL(server.URL, "CCML test")
	items, err := provider.Search(context.Background(), model.MetadataQuery{
		Artist: "DJ Smash",
		Title:  "Moscow Never Sleeps (P0Lka & 2Yards Remix)",
	})
	if err != nil {
		t.Fatal(err)
	}
	if searchQ != "DJ Smash - Moscow Never Sleeps (P0Lka & 2Yards Remix)" {
		t.Fatalf("q = %q", searchQ)
	}
	if sawAuth || sawCookie {
		t.Fatalf("public provider must not send auth/cookies: auth=%v cookie=%v", sawAuth, sawCookie)
	}
	if len(items) != 1 {
		t.Fatalf("items = %+v", items)
	}
}

func TestJesteiKeyRejectsAmbiguousCamelotValues(t *testing.T) {
	t.Parallel()

	if key, ok := jesteiKey([]string{"8A", "9B"}); ok || key != "" {
		t.Fatalf("ambiguous key must be rejected: %q %v", key, ok)
	}
}

func TestJesteiGenresStopAtMarkersSection(t *testing.T) {
	t.Parallel()

	lines := []string{
		"Это жанр трека.",
		"House",
		"Promo",
		"Techno",
		"Это маркировки — они помогут быстро определить, в чем «фишка» трека.",
		"Pop",
	}
	got := jesteiGenres(lines)
	if got != "House, Techno" {
		t.Fatalf("genres = %q", got)
	}
}

func TestJesteiDebugFlag(t *testing.T) {
	t.Setenv("CCML_JESTEI_DEBUG", "1")
	if !jesteiDebugEnabled() {
		t.Fatal("debug should be enabled")
	}
	t.Setenv("CCML_JESTEI_DEBUG", "")
	if jesteiDebugEnabled() {
		t.Fatal("debug should be disabled")
	}
}

func TestJesteiStripSiteSuffix(t *testing.T) {
	t.Parallel()

	got := jesteiStripSiteSuffix("Artist - Title | Jestei Pool")
	if !strings.EqualFold(got, "Artist - Title") {
		t.Fatalf("got %q", got)
	}
}
