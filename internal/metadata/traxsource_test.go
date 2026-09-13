package metadata

import (
	"testing"
)

func TestTraxsourceSearchReleases(t *testing.T) {
	doc := `<html><body><div class="release-grid"><div class="grid-page"><div class="grid-item" data-tid="123"><div class="ellip"><a class="com-title" href="/title/123/release">Release</a></div></div></div></div></body></html>`
	got := traxsourceSearchReleases(doc, "https://www.traxsource.com")
	if len(got) != 1 || got[0] != "https://www.traxsource.com/title/123/release" {
		t.Fatalf("releases = %#v", got)
	}
}

func TestTraxsourceReleaseCandidates(t *testing.T) {
	markup := `<html><head><meta property="og:image" content="https://geo-media.beatport.com/image.jpg"></head><body>
	<h1 class="artists"><a class="com-artists">Artist</a></h1>
	<h1 class="title">Great Release</h1>
	<a class="com-label">House Label</a>
	<div class="cat-rdate">CAT123 | 2026-08-30</div>
	<div class="trklist">
	  <div class="trk-row">
	    <div class="tnum">1</div>
	    <div class="title"><a href="/track/song/555">Song</a><span class="version">Extended Mix</span></div>
	    <div class="artists"><a class="com-artists">Artist</a></div>
	    <div class="genre"><a>Deep House</a></div>
	    <span class="duration">06:32</span>
	  </div>
	</div></body></html>`
	items := traxsourceReleaseCandidates(markup, "https://www.traxsource.com/title/123/great-release")
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	got := items[0]
	if got.Source != "Traxsource" || got.Title != "Song (Extended Mix)" || got.Artist != "Artist" {
		t.Fatalf("unexpected candidate: %+v", got)
	}
	if got.Album != "Great Release" || got.AlbumArtist != "Artist" || got.Genre != "Deep House" || got.Label != "House Label" || got.CatalogNumber != "CAT123" {
		t.Fatalf("unexpected release fields: %+v", got)
	}
	if got.ReleaseDate != "2026-08-30" || got.Year != 2026 || got.DurationMS != 392000 || got.TrackNumber != 1 || got.TrackTotal != 1 {
		t.Fatalf("unexpected numeric fields: %+v", got)
	}
	if got.ExternalID != "555" || got.ArtworkEmbeddable {
		t.Fatalf("unexpected ids/artwork flags: %+v", got)
	}
}
