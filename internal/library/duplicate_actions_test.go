package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

type fakeDuplicateActionStore struct {
	tracks      map[int64]model.Track
	roots       []model.LibraryRoot
	deleteError error
	deleted     []int64
}

func (f *fakeDuplicateActionStore) TrackByID(_ context.Context, id int64) (model.Track, error) {
	track, ok := f.tracks[id]
	if !ok {
		return model.Track{}, errors.New("not found")
	}
	return track, nil
}

func (f *fakeDuplicateActionStore) AllTracks(_ context.Context) ([]model.Track, error) {
	out := make([]model.Track, 0, len(f.tracks))
	for _, track := range f.tracks {
		out = append(out, track)
	}
	return out, nil
}

func (f *fakeDuplicateActionStore) ListLibraryRoots(_ context.Context) ([]model.LibraryRoot, error) {
	return f.roots, nil
}

func (f *fakeDuplicateActionStore) DeleteTracksByID(_ context.Context, ids []int64) error {
	if f.deleteError != nil {
		return f.deleteError
	}
	f.deleted = append([]int64(nil), ids...)
	for _, id := range ids {
		delete(f.tracks, id)
	}
	return nil
}

func TestQuarantineDuplicateTracksMovesAndRemovesIndexRows(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sourceDir := filepath.Join(root, "library")
	quarantineParent := filepath.Join(root, "outside")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(quarantineParent, 0o755); err != nil {
		t.Fatal(err)
	}

	firstPath := filepath.Join(sourceDir, "track-a.mp3")
	secondPath := filepath.Join(sourceDir, "track-b.mp3")
	if err := os.WriteFile(firstPath, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := &fakeDuplicateActionStore{
		tracks: map[int64]model.Track{
			1: {ID: 1, Path: firstPath, FileName: "track-a.mp3", Artist: "Artist", Title: "Song", DurationMS: 180_000},
			2: {ID: 2, Path: secondPath, FileName: "track-b.mp3", Artist: "Artist", Title: "Song", DurationMS: 180_500},
		},
		roots: []model.LibraryRoot{{Path: sourceDir}},
	}

	result, err := QuarantineDuplicateTracks(context.Background(), store, []int64{2}, quarantineParent)
	if err != nil {
		t.Fatal(err)
	}
	if result.Completed != 1 || len(result.Paths) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := os.Stat(secondPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(result.Paths[0]); err != nil {
		t.Fatalf("quarantined file missing: %v", err)
	}
	if _, ok := store.tracks[2]; ok {
		t.Fatal("track 2 still indexed")
	}
	if _, ok := store.tracks[1]; !ok {
		t.Fatal("track 1 should remain indexed")
	}
}

func TestQuarantineRejectsDestinationInsideLibrary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	firstPath := filepath.Join(root, "a.mp3")
	secondPath := filepath.Join(root, "b.mp3")
	if err := os.WriteFile(firstPath, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := &fakeDuplicateActionStore{
		tracks: map[int64]model.Track{
			1: {ID: 1, Path: firstPath, FileName: "a.mp3", Artist: "Artist", Title: "Song", DurationMS: 180_000},
			2: {ID: 2, Path: secondPath, FileName: "b.mp3", Artist: "Artist", Title: "Song", DurationMS: 180_500},
		},
		roots: []model.LibraryRoot{{Path: root}},
	}

	_, err := QuarantineDuplicateTracks(context.Background(), store, []int64{2}, root)
	if err == nil {
		t.Fatal("expected quarantine-inside-library error")
	}
	if _, err := os.Stat(secondPath); err != nil {
		t.Fatalf("source changed despite rejection: %v", err)
	}
}

func TestDuplicateActionRejectsRemovingWholeGroup(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	firstPath := filepath.Join(root, "a.mp3")
	secondPath := filepath.Join(root, "b.mp3")
	if err := os.WriteFile(firstPath, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := &fakeDuplicateActionStore{
		tracks: map[int64]model.Track{
			1: {ID: 1, Path: firstPath, FileName: "a.mp3", Artist: "Artist", Title: "Song", DurationMS: 180_000},
			2: {ID: 2, Path: secondPath, FileName: "b.mp3", Artist: "Artist", Title: "Song", DurationMS: 180_500},
		},
	}

	_, err := DeleteDuplicateTracks(context.Background(), store, []int64{1, 2}, "DELETE")
	if err == nil {
		t.Fatal("expected keep-one-file safety error")
	}
	if _, err := os.Stat(firstPath); err != nil {
		t.Fatalf("first source changed: %v", err)
	}
	if _, err := os.Stat(secondPath); err != nil {
		t.Fatalf("second source changed: %v", err)
	}
}

func TestDeleteDuplicateTracksRestoresFilesWhenDatabaseFails(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	firstPath := filepath.Join(root, "a.mp3")
	secondPath := filepath.Join(root, "b.mp3")
	if err := os.WriteFile(firstPath, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := &fakeDuplicateActionStore{
		tracks: map[int64]model.Track{
			1: {ID: 1, Path: firstPath, FileName: "a.mp3", Artist: "Artist", Title: "Song", DurationMS: 180_000},
			2: {ID: 2, Path: secondPath, FileName: "b.mp3", Artist: "Artist", Title: "Song", DurationMS: 180_500},
		},
		deleteError: errors.New("database unavailable"),
	}

	_, err := DeleteDuplicateTracks(context.Background(), store, []int64{2}, "DELETE")
	if err == nil {
		t.Fatal("expected database error")
	}
	if _, err := os.Stat(secondPath); err != nil {
		t.Fatalf("source was not restored: %v", err)
	}
}

func TestDeleteDuplicateTracksRequiresLiteralConfirmation(t *testing.T) {
	t.Parallel()

	store := &fakeDuplicateActionStore{tracks: map[int64]model.Track{}}
	_, err := DeleteDuplicateTracks(context.Background(), store, []int64{1}, "delete")
	if err == nil {
		t.Fatal("expected confirmation error")
	}
}
