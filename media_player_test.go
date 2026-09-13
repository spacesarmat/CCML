package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
	"github.com/spacesarmat/CCML/internal/store"
)

func TestPrepareTrackStreamsMP3WithoutFFmpeg(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "track.mp3")
	if err := os.WriteFile(path, []byte("fake mp3 payload"), 0o600); err != nil {
		t.Fatal(err)
	}

	db, err := store.Open(filepath.Join(dir, "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id, err := db.UpsertTrack(ctx, model.Track{
		Path:       path,
		FileName:   filepath.Base(path),
		Extension:  ".mp3",
		DurationMS: 12345,
	})
	if err != nil {
		t.Fatal(err)
	}

	cacheDir := filepath.Join(dir, "cache")
	if err := os.MkdirAll(filepath.Join(cacheDir, "artwork"), 0o700); err != nil {
		t.Fatal(err)
	}
	service := &mediaService{
		store:    db,
		cacheDir: cacheDir,
		baseURL:  "http://127.0.0.1:12345/ccml-media/token",
	}

	media, err := service.PrepareTrack(ctx, id)
	if err != nil {
		t.Fatalf("PrepareTrack() error = %v", err)
	}
	if media.IsPreview {
		t.Fatal("MP3 should use the direct stream before a compatibility fallback is needed")
	}
	if !strings.Contains(media.AudioURL, "/track/") || !strings.Contains(media.AudioURL, "?v=") {
		t.Fatalf("unexpected direct audio URL %q", media.AudioURL)
	}
	if media.DurationMS != 12345 {
		t.Fatalf("DurationMS = %d, want 12345", media.DurationMS)
	}
}

func TestMediaServerSupportsByteRanges(t *testing.T) {
	dir := t.TempDir()
	audioDir := filepath.Join(dir, "audio")
	if err := os.MkdirAll(audioDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(audioDir, "preview.mp3"), []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}

	service := &mediaService{cacheDir: dir, token: "test-token"}
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/ccml-media/test-token/audio/preview.mp3", nil)
	request.Header.Set("Range", "bytes=2-5")
	response := httptest.NewRecorder()
	service.ServeHTTP(response, request)

	result := response.Result()
	defer result.Body.Close()
	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want %d", result.StatusCode, http.StatusPartialContent)
	}
	if string(body) != "2345" {
		t.Fatalf("body = %q, want %q", string(body), "2345")
	}
	if got := result.Header.Get("Content-Range"); got != "bytes 2-5/10" {
		t.Fatalf("Content-Range = %q", got)
	}
}
