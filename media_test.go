package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBrowserFriendlyAudio(t *testing.T) {
	for _, ext := range []string{".mp3", ".m4a", ".aac", ".wav", ".flac"} {
		if !browserFriendlyAudio(ext) {
			t.Fatalf("expected %s to stream directly", ext)
		}
	}
	for _, ext := range []string{".ogg", ".aiff", ".aif"} {
		if browserFriendlyAudio(ext) {
			t.Fatalf("expected %s to use a compatibility preview", ext)
		}
	}
}

func TestFileCacheKeyChangesWithFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "track.wav")
	if err := os.WriteFile(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := fileCacheKey(path, "test-v1")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("two-two"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := fileCacheKey(path, "test-v1")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("cache key did not change after file content/size changed")
	}
}

func TestIsWithinMediaCache(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "audio", "preview.mp3")
	outside := filepath.Join(filepath.Dir(root), "outside.mp3")
	if !isWithin(inside, root) {
		t.Fatal("expected cache child to be accepted")
	}
	if isWithin(outside, root) {
		t.Fatal("expected path outside cache to be rejected")
	}
}
