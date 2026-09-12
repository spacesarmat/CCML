package audio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/your-github/ccml/internal/model"
)

// EssentiaAnalyzer is an optional adapter for essentia_streaming_extractor_music.
type EssentiaAnalyzer struct {
	path string
}

// NewEssentiaAnalyzer discovers Essentia from CCML_ESSENTIA or PATH.
func NewEssentiaAnalyzer() *EssentiaAnalyzer {
	path := strings.TrimSpace(os.Getenv("CCML_ESSENTIA"))
	if path == "" {
		if discovered, err := exec.LookPath("essentia_streaming_extractor_music"); err == nil {
			path = discovered
		}
	}
	return &EssentiaAnalyzer{path: path}
}

// Available reports whether Essentia can be invoked.
func (a *EssentiaAnalyzer) Available() bool { return a.path != "" }

// Path returns the discovered executable path.
func (a *EssentiaAnalyzer) Path() string { return a.path }

// Analyze extracts BPM and key using Essentia's music extractor JSON output.
func (a *EssentiaAnalyzer) Analyze(ctx context.Context, input string) (result model.BPMKey, resultErr error) {
	if !a.Available() {
		return model.BPMKey{}, errors.New("Essentia is not installed; set CCML_ESSENTIA or install essentia_streaming_extractor_music")
	}

	temp, err := os.CreateTemp("", "ccml-essentia-*.json")
	if err != nil {
		return model.BPMKey{}, fmt.Errorf("create Essentia result file: %w", err)
	}
	resultPath := temp.Name()
	if err := temp.Close(); err != nil {
		return model.BPMKey{}, fmt.Errorf("close Essentia result file: %w", err)
	}
	defer func() {
		if err := os.Remove(resultPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove Essentia result file: %w", err))
		}
	}()

	cmd := exec.CommandContext(ctx, a.path, input, resultPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return model.BPMKey{}, fmt.Errorf("run Essentia for %q: %w: %s", filepath.Base(input), err, tail(string(output), 3000))
	}

	data, err := os.ReadFile(resultPath)
	if err != nil {
		return model.BPMKey{}, fmt.Errorf("read Essentia JSON: %w", err)
	}
	var parsed struct {
		Rhythm struct {
			BPM float64 `json:"bpm"`
		} `json:"rhythm"`
		Tonal struct {
			KeyKey      string  `json:"key_key"`
			KeyScale    string  `json:"key_scale"`
			KeyStrength float64 `json:"key_strength"`
		} `json:"tonal"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return model.BPMKey{}, fmt.Errorf("decode Essentia JSON: %w", err)
	}
	result = model.BPMKey{
		BPM:      parsed.Rhythm.BPM,
		Key:      parsed.Tonal.KeyKey,
		Scale:    parsed.Tonal.KeyScale,
		Strength: parsed.Tonal.KeyStrength,
	}
	return result, nil
}
