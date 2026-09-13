package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestMetadataLookupCacheLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()

	want := model.MetadataLookupResult{
		Candidates: []model.MetadataCandidate{{Source: "MusicBrainz", Title: "Song", Artist: "Artist", MatchClass: "high", Confidence: .9}},
	}
	if err := db.PutMetadataLookupCache(ctx, "key", want, time.Hour); err != nil {
		t.Fatalf("PutMetadataLookupCache() error = %v", err)
	}
	got, _, ok, err := db.MetadataLookupCache(ctx, "key")
	if err != nil {
		t.Fatalf("MetadataLookupCache() error = %v", err)
	}
	if !ok || len(got.Candidates) != 1 || got.Candidates[0].Title != "Song" {
		t.Fatalf("MetadataLookupCache() = %+v ok=%v", got, ok)
	}
	if err := db.ClearMetadataLookupCache(ctx); err != nil {
		t.Fatalf("ClearMetadataLookupCache() error = %v", err)
	}
	if _, _, ok, err := db.MetadataLookupCache(ctx, "key"); err != nil || ok {
		t.Fatalf("cache after clear ok=%v err=%v", ok, err)
	}
}
