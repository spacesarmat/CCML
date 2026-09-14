package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tommyo123/mtag"
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

func TestSelectArtworkPrefersFrontCover(t *testing.T) {
	images := []mtag.Picture{
		{Type: mtag.PictureOther, MIME: "image/png", Data: []byte("fallback")},
		{Type: mtag.PictureCoverFront, MIME: "image/jpeg", Data: []byte("front")},
	}
	picture, ok := selectArtwork(images)
	if !ok {
		t.Fatal("expected artwork")
	}
	if picture.Type != mtag.PictureCoverFront || string(picture.Data) != "front" {
		t.Fatalf("selected picture = type %d data %q", picture.Type, string(picture.Data))
	}
}

func TestSelectArtworkFallsBackToEmbeddedImage(t *testing.T) {
	images := []mtag.Picture{
		{Type: mtag.PictureOther, MIME: "image/jpeg", Data: []byte("cover")},
	}
	picture, ok := selectArtwork(images)
	if !ok {
		t.Fatal("expected fallback artwork")
	}
	if string(picture.Data) != "cover" {
		t.Fatalf("selected data = %q", string(picture.Data))
	}
}

func TestExtensionForImageUsesPayloadSignature(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	if got := extensionForImage("image/jpeg", png); got != ".png" {
		t.Fatalf("extensionForImage() = %q, want .png", got)
	}
}
