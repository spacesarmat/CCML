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
	"sync"

	"github.com/spacesarmat/CCML/internal/model"
)

const essentiaConfigFileName = "essentia.json"

type essentiaConfig struct {
	Path string `json:"path"`
}

// EssentiaAnalyzer is an optional adapter for essentia_streaming_extractor_music.
//
// CCML keeps Essentia as an external command-line dependency. A user-selected
// executable path is persisted inside the CCML config directory; environment
// and PATH discovery remain available as fallbacks.
type EssentiaAnalyzer struct {
	mu         sync.RWMutex
	path       string
	source     string
	configPath string
}

// NewEssentiaAnalyzer discovers Essentia from a saved CCML path, CCML_ESSENTIA,
// or PATH. The optional appDir keeps older call sites source-compatible.
func NewEssentiaAnalyzer(appDir ...string) *EssentiaAnalyzer {
	a := &EssentiaAnalyzer{}
	if len(appDir) > 0 && strings.TrimSpace(appDir[0]) != "" {
		a.configPath = filepath.Join(appDir[0], essentiaConfigFileName)
	}
	a.Refresh()
	return a
}

// Refresh repeats configured/environment/PATH discovery.
func (a *EssentiaAnalyzer) Refresh() {
	if a == nil {
		return
	}
	path, source := discoverEssentia(a.configPath)
	a.mu.Lock()
	a.path = path
	a.source = source
	a.mu.Unlock()
}

// Available reports whether Essentia can be invoked.
func (a *EssentiaAnalyzer) Available() bool {
	return a != nil && a.Path() != ""
}

// Path returns the active executable path.
func (a *EssentiaAnalyzer) Path() string {
	if a == nil {
		return ""
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.path
}

// Source reports how the active executable was discovered.
func (a *EssentiaAnalyzer) Source() string {
	if a == nil {
		return ""
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.source
}

// Configure stores an explicit Essentia extractor path for future launches.
func (a *EssentiaAnalyzer) Configure(path string) error {
	if a == nil {
		return errors.New("Essentia analyzer is not available")
	}
	path = strings.TrimSpace(path)
	if err := validateEssentiaPath(path); err != nil {
		return err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve Essentia path: %w", err)
	}
	if a.configPath != "" {
		payload, err := json.MarshalIndent(essentiaConfig{Path: absolute}, "", "  ")
		if err != nil {
			return fmt.Errorf("encode Essentia config: %w", err)
		}
		if err := os.WriteFile(a.configPath, payload, 0o600); err != nil {
			return fmt.Errorf("save Essentia config: %w", err)
		}
	}
	a.mu.Lock()
	a.path = absolute
	a.source = "configured"
	a.mu.Unlock()
	return nil
}

// ClearConfiguredPath removes the saved override and restores automatic discovery.
func (a *EssentiaAnalyzer) ClearConfiguredPath() error {
	if a == nil {
		return errors.New("Essentia analyzer is not available")
	}
	if a.configPath != "" {
		if err := os.Remove(a.configPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove Essentia config: %w", err)
		}
	}
	a.Refresh()
	return nil
}

func discoverEssentia(configPath string) (string, string) {
	if strings.TrimSpace(configPath) != "" {
		if data, err := os.ReadFile(configPath); err == nil {
			var config essentiaConfig
			if json.Unmarshal(data, &config) == nil && validateEssentiaPath(config.Path) == nil {
				if absolute, absErr := filepath.Abs(strings.TrimSpace(config.Path)); absErr == nil {
					return absolute, "configured"
				}
			}
		}
	}

	if path := strings.TrimSpace(os.Getenv("CCML_ESSENTIA")); path != "" {
		if validateEssentiaPath(path) == nil {
			if absolute, err := filepath.Abs(path); err == nil {
				return absolute, "environment"
			}
		}
	}

	if discovered, err := exec.LookPath("essentia_streaming_extractor_music"); err == nil {
		return discovered, "PATH"
	}
	if discovered, err := exec.LookPath("essentia_streaming_extractor_music.exe"); err == nil {
		return discovered, "PATH"
	}
	return "", ""
}

func validateEssentiaPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("Essentia executable path is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("Essentia executable is not accessible: %w", err)
	}
	if info.IsDir() {
		return errors.New("Essentia executable path points to a directory")
	}
	return nil
}

// Analyze extracts BPM and key using Essentia's music extractor JSON output.
func (a *EssentiaAnalyzer) Analyze(ctx context.Context, input string) (result model.BPMKey, resultErr error) {
	path := a.Path()
	if path == "" {
		return model.BPMKey{}, errors.New("Essentia is not configured; choose essentia_streaming_extractor_music in Settings, set CCML_ESSENTIA, or install it on PATH")
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

	cmd := exec.CommandContext(ctx, path, input, resultPath)
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
