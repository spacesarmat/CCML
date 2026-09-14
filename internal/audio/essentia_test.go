package audio

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEssentiaAnalyzerPersistsConfiguredPath(t *testing.T) {
	t.Setenv("CCML_ESSENTIA", "")
	t.Setenv("PATH", "")

	appDir := t.TempDir()
	name := "essentia_streaming_extractor_music"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	executable := filepath.Join(appDir, name)
	if err := os.WriteFile(executable, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}

	analyzer := NewEssentiaAnalyzer(appDir)
	if analyzer.Available() {
		t.Fatalf("unexpected automatic Essentia path: %q", analyzer.Path())
	}
	if err := analyzer.Configure(executable); err != nil {
		t.Fatal(err)
	}
	if !analyzer.Available() {
		t.Fatal("configured Essentia should be available")
	}
	if analyzer.Source() != "configured" {
		t.Fatalf("source = %q, want configured", analyzer.Source())
	}

	reloaded := NewEssentiaAnalyzer(appDir)
	if reloaded.Source() != "configured" {
		t.Fatalf("reloaded source = %q, want configured", reloaded.Source())
	}
	want, err := filepath.Abs(executable)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Path() != want {
		t.Fatalf("path = %q, want %q", reloaded.Path(), want)
	}

	if err := reloaded.ClearConfiguredPath(); err != nil {
		t.Fatal(err)
	}
	if reloaded.Available() {
		t.Fatalf("automatic discovery should be empty after reset, got %q", reloaded.Path())
	}
	if _, err := os.Stat(filepath.Join(appDir, essentiaConfigFileName)); !os.IsNotExist(err) {
		t.Fatalf("configured path file was not removed: %v", err)
	}
}

func TestEssentiaAnalyzerFallsBackToEnvironment(t *testing.T) {
	t.Setenv("PATH", "")

	appDir := t.TempDir()
	name := "essentia_streaming_extractor_music"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	executable := filepath.Join(appDir, name)
	if err := os.WriteFile(executable, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CCML_ESSENTIA", executable)

	analyzer := NewEssentiaAnalyzer(appDir)
	if analyzer.Source() != "environment" {
		t.Fatalf("source = %q, want environment", analyzer.Source())
	}
	if analyzer.Path() == "" {
		t.Fatal("environment path was not discovered")
	}
}

func TestEssentiaConfigureRejectsMissingPath(t *testing.T) {
	t.Setenv("CCML_ESSENTIA", "")
	t.Setenv("PATH", "")

	analyzer := NewEssentiaAnalyzer(t.TempDir())
	if err := analyzer.Configure(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("Configure must reject a missing executable")
	}
}
