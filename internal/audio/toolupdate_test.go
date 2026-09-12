package audio

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReleasePlan(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		goos     string
		goarch   string
		provider string
		mode     toolReleaseMode
		ffmpeg   string
		ffprobe  string
		archive  string
		wantErr  bool
	}{
		{name: "windows amd64", goos: "windows", goarch: "amd64", provider: toolBuildProviderBtbN, mode: toolReleaseZip, archive: "ffmpeg-n9.0-latest-win64-gpl-9.0.zip"},
		{name: "windows arm64", goos: "windows", goarch: "arm64", provider: toolBuildProviderBtbN, mode: toolReleaseZip, archive: "ffmpeg-n9.0-latest-winarm64-gpl-9.0.zip"},
		{name: "mac intel", goos: "darwin", goarch: "amd64", provider: toolBuildProviderShaka, mode: toolReleaseDirect, ffmpeg: "ffmpeg-osx-x64", ffprobe: "ffprobe-osx-x64"},
		{name: "mac apple silicon", goos: "darwin", goarch: "arm64", provider: toolBuildProviderShaka, mode: toolReleaseDirect, ffmpeg: "ffmpeg-osx-arm64", ffprobe: "ffprobe-osx-arm64"},
		{name: "linux unsupported", goos: "linux", goarch: "amd64", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			plan, err := releasePlan(tt.goos, tt.goarch)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("releasePlan: %v", err)
			}
			if plan.Provider != tt.provider || plan.Mode != tt.mode || plan.FFmpegAsset != tt.ffmpeg || plan.FFprobeAsset != tt.ffprobe || plan.ArchiveAsset != tt.archive {
				t.Fatalf("unexpected plan: %#v", plan)
			}
		})
	}
}

func TestResolveRelease(t *testing.T) {
	t.Parallel()
	plan, err := releasePlan("windows", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	release := githubRelease{TagName: "latest", Assets: []githubReleaseAsset{{
		Name: plan.ArchiveAsset, BrowserDownloadURL: "https://example.invalid/ffmpeg.zip",
		Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 123,
	}}}
	resolved, err := resolveRelease(plan, release)
	if err != nil {
		t.Fatalf("resolveRelease: %v", err)
	}
	if resolved.UpdateID != toolBuildProviderBtbN+":"+release.Assets[0].Digest {
		t.Fatalf("unexpected update ID %q", resolved.UpdateID)
	}
}

func TestShouldAutoCheckSeedsBundledToolchain(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		t.Skip("managed FFmpeg is only enabled on Windows/macOS")
	}
	root := t.TempDir()
	updater := &ToolUpdater{
		root:  root,
		tools: newToolchain("ffmpeg", "ffprobe", "n9.0.1", "bundle-id", toolSourceBundled),
	}
	due, err := updater.ShouldAutoCheck()
	if err != nil {
		t.Fatalf("ShouldAutoCheck: %v", err)
	}
	if due {
		t.Fatal("fresh bundled toolchain should not update immediately")
	}
	state, err := updater.readUpdateState()
	if err != nil {
		t.Fatalf("readUpdateState: %v", err)
	}
	if state.LastCheck.IsZero() {
		t.Fatal("expected seeded last check timestamp")
	}
}

func TestDownloadAssetVerifiesSHA256(t *testing.T) {
	t.Parallel()
	content := []byte("ccml-test-ffmpeg")
	hash := sha256.Sum256(content)
	digest := "sha256:" + hex.EncodeToString(hash[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(content); err != nil {
			t.Errorf("write test response: %v", err)
		}
	}))
	defer server.Close()

	updater := &ToolUpdater{client: server.Client()}
	destination := filepath.Join(t.TempDir(), "ffmpeg")
	asset := githubReleaseAsset{
		Name:               "ffmpeg-test",
		BrowserDownloadURL: server.URL,
		Digest:             digest,
		Size:               int64(len(content)),
	}
	if err := updater.downloadAsset(context.Background(), asset, destination); err != nil {
		t.Fatalf("downloadAsset: %v", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read downloaded asset: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("downloaded content %q, want %q", got, content)
	}
}

func TestDownloadAssetRejectsBadSHA256(t *testing.T) {
	t.Parallel()
	content := []byte("bad-digest-test")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write(content); err != nil {
			t.Errorf("write test response: %v", err)
		}
	}))
	defer server.Close()

	updater := &ToolUpdater{client: server.Client()}
	asset := githubReleaseAsset{
		Name:               "ffmpeg-test",
		BrowserDownloadURL: server.URL,
		Digest:             "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		Size:               int64(len(content)),
	}
	if err := updater.downloadAsset(context.Background(), asset, filepath.Join(t.TempDir(), "ffmpeg")); err == nil {
		t.Fatal("expected SHA-256 mismatch")
	}
}

func TestExtractWindowsTools(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	archive := filepath.Join(dir, "ffmpeg.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(file)
	for name, content := range map[string]string{
		"ffmpeg-build/bin/ffmpeg.exe":  "ffmpeg-bytes",
		"ffmpeg-build/bin/ffprobe.exe": "ffprobe-bytes",
	} {
		entry, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	ffmpeg := filepath.Join(dir, "ffmpeg.exe")
	ffprobe := filepath.Join(dir, "ffprobe.exe")
	if err := extractWindowsTools(archive, ffmpeg, ffprobe); err != nil {
		t.Fatalf("extractWindowsTools: %v", err)
	}
	for path, want := range map[string]string{ffmpeg: "ffmpeg-bytes", ffprobe: "ffprobe-bytes"} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Fatalf("%s = %q, want %q", path, got, want)
		}
	}
}

func TestCommandListContains(t *testing.T) {
	t.Parallel()
	output := ` T.. loudnorm A->A EBU R128 loudness normalization
 A.. alimiter A->A Audio lookahead limiter`
	if !commandListContains(output, "loudnorm") {
		t.Fatal("loudnorm not found")
	}
	if commandListContains(output, "rubberband") {
		t.Fatal("unexpected rubberband match")
	}
}
