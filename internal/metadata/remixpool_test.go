package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

const remixpoolFixture = `
<!doctype html>
<html>
<body>
  <h2>Новинки</h2>
  <div>Записи</div><div>Скорость</div><div>Тональность</div><div>Жанр</div>
  <div class="track">
    <span>That Is House</span>
    <span>Simon Fava</span>
    <span>122</span>
    <span>5A</span>
    <span>Хаус</span>
  </div>
  <div class="track">
    <span>Closer (Sarah Feliz Remix)</span>
    <span>Ne-Yo</span>
    <span>128</span>
    <span>9A</span>
    <span>Мумбатон</span>
  </div>
  <div class="track">
    <span>Roman (Intro Edit)</span>
    <span>NEWLIGHTCHILD</span>
    <span>70.5</span>
    <span>9A</span>
    <span>Русское</span>
  </div>
</body>
</html>`

func TestRemixPoolProviderKind(t *testing.T) {
	p := NewRemixPoolProvider("test")
	if got := p.Name(); got != "RemixPool" {
		t.Fatalf("Name() = %q", got)
	}
	if got := p.Kind(); got != ProviderKindDJPool {
		t.Fatalf("Kind() = %q, want %q", got, ProviderKindDJPool)
	}
}

func TestRemixPoolCandidatesParseDJFields(t *testing.T) {
	items := remixpoolCandidatesFromHTML(
		remixpoolFixture,
		"https://remixpool.ru/new-releases/",
		model.MetadataQuery{Artist: "Ne-Yo", Title: "Closer (Sarah Feliz Remix)"},
	)
	if len(items) != 1 {
		t.Fatalf("got %d candidates, want 1: %+v", len(items), items)
	}
	got := items[0]
	if got.Artist != "Ne-Yo" || got.Title != "Closer (Sarah Feliz Remix)" {
		t.Fatalf("unexpected artist/title: %+v", got)
	}
	if got.BPM != 128 || got.Key != "9A" || got.KeyScale != "camelot" {
		t.Fatalf("unexpected BPM/key: %+v", got)
	}
	if got.Genre != "Мумбатон" {
		t.Fatalf("genre = %q", got.Genre)
	}
	if got.SourceKind != ProviderKindDJPool {
		t.Fatalf("source kind = %q", got.SourceKind)
	}
}

func TestRemixPoolParserRejectsUnrelatedTracks(t *testing.T) {
	items := remixpoolCandidatesFromHTML(
		remixpoolFixture,
		"https://remixpool.ru/new-releases/",
		model.MetadataQuery{Artist: "Zivert", Title: "GAZ"},
	)
	if len(items) != 0 {
		t.Fatalf("expected no unrelated matches, got %+v", items)
	}
}

func TestRemixPoolBaseTitleFallbackCanMatchVersionedTrack(t *testing.T) {
	items := remixpoolCandidatesFromHTML(
		remixpoolFixture,
		"https://remixpool.ru/new-releases/",
		model.MetadataQuery{Artist: "NEWLIGHTCHILD", Title: "Roman"},
	)
	if len(items) != 1 {
		t.Fatalf("base title query should match versioned pool title: %+v", items)
	}
	if items[0].BPM != 70.5 || items[0].Key != "9A" {
		t.Fatalf("unexpected candidate: %+v", items[0])
	}
}

func TestRemixPoolSearchUsesConfirmedPublicNewReleasesPage(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(remixpoolFixture))
	}))
	defer server.Close()

	provider := newRemixPoolProviderWithBaseURL(server.URL, "CCML test")
	items, err := provider.Search(context.Background(), model.MetadataQuery{
		Artist: "Simon Fava",
		Title:  "That Is House",
	})
	if err != nil {
		t.Fatal(err)
	}
	if requestedPath != "/new-releases/" {
		t.Fatalf("path = %q, want /new-releases/", requestedPath)
	}
	if len(items) != 1 || items[0].BPM != 122 || items[0].Key != "5A" {
		t.Fatalf("unexpected search result: %+v", items)
	}
}
