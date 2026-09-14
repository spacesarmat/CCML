package audio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestNormalizeEssentiaPerformanceDefaultsAndBounds(t *testing.T) {
	t.Parallel()

	got := normalizeEssentiaPerformance(model.EssentiaPerformance{})
	if got.Mode != "adaptive" || got.Workers != 2 || got.FastSeconds != 120 || got.MinKeyStrength != 0.62 {
		t.Fatalf("defaults = %+v", got)
	}

	got = normalizeEssentiaPerformance(model.EssentiaPerformance{
		Mode: "accurate", Workers: 9, FastSeconds: 500, MinKeyStrength: 2,
	})
	if got.Mode != "accurate" || got.Workers != 4 || got.FastSeconds != 300 || got.MinKeyStrength != 0.95 {
		t.Fatalf("bounded = %+v", got)
	}
}

func TestEssentiaPerformancePersists(t *testing.T) {
	t.Setenv("CCML_ESSENTIA", "")
	t.Setenv("PATH", "")

	appDir := t.TempDir()
	analyzer := NewEssentiaAnalyzer(appDir)
	saved, err := analyzer.ConfigurePerformance(model.EssentiaPerformance{
		Mode: "adaptive", Workers: 3, FastSeconds: 180, MinKeyStrength: 0.70,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Mode != "adaptive" || saved.Workers != 3 || saved.FastSeconds != 180 || saved.MinKeyStrength != 0.70 {
		t.Fatalf("saved = %+v", saved)
	}
	if _, err := os.Stat(filepath.Join(appDir, essentiaPerformanceConfigFileName)); err != nil {
		t.Fatal(err)
	}

	reloaded := NewEssentiaAnalyzer(appDir)
	got := reloaded.Performance()
	if got != saved {
		t.Fatalf("reloaded = %+v, want %+v", got, saved)
	}
}
