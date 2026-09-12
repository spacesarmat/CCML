// Package audio contains FFmpeg/ffprobe based audio analysis and processing.
package audio

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	toolSourceEnvironment = "environment"
	toolSourceManaged     = "managed"
	toolSourceBundled     = "bundled"
	toolSourcePath        = "PATH"
)

// Toolchain points to FFmpeg executables used by the backend.
//
// A Toolchain is safe to update while the application is running. The updater
// validates a new pair of binaries before atomically switching to them.
type Toolchain struct {
	mu       sync.RWMutex
	ffmpeg   string
	ffprobe  string
	version  string
	updateID string
	source   string
}

// ToolchainSnapshot is a read-only view of the currently selected tools.
type ToolchainSnapshot struct {
	FFmpeg   string
	FFprobe  string
	Version  string
	UpdateID string
	Source   string
}

// Ready reports whether both FFmpeg and ffprobe are available.
func (t *Toolchain) Ready() bool {
	if t == nil {
		return false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return strings.TrimSpace(t.ffmpeg) != "" && strings.TrimSpace(t.ffprobe) != ""
}

// Snapshot returns the active executable paths and their source.
func (t *Toolchain) Snapshot() ToolchainSnapshot {
	if t == nil {
		return ToolchainSnapshot{}
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return ToolchainSnapshot{
		FFmpeg:   t.ffmpeg,
		FFprobe:  t.ffprobe,
		Version:  t.version,
		UpdateID: t.updateID,
		Source:   t.source,
	}
}

// FFmpegPath returns the active FFmpeg executable path.
func (t *Toolchain) FFmpegPath() string {
	return t.Snapshot().FFmpeg
}

// FFprobePath returns the active ffprobe executable path.
func (t *Toolchain) FFprobePath() string {
	return t.Snapshot().FFprobe
}

// Replace switches the application to a validated FFmpeg/ffprobe pair.
func (t *Toolchain) Replace(ffmpeg, ffprobe, version, updateID, source string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ffmpeg = ffmpeg
	t.ffprobe = ffprobe
	t.version = version
	t.updateID = updateID
	t.source = source
}

// DiscoverToolchain searches explicit environment variables, CCML-managed
// updates, binaries bundled with the application, and finally PATH.
func DiscoverToolchain() *Toolchain {
	if ffmpeg, ffprobe, ok := discoverEnvironmentPair(); ok {
		return newToolchain(ffmpeg, ffprobe, probeToolVersion(ffmpeg), "", toolSourceEnvironment)
	}

	if ffmpeg, ffprobe, version, updateID, ok := discoverManagedPair(); ok {
		return newToolchain(ffmpeg, ffprobe, version, updateID, toolSourceManaged)
	}

	if ffmpeg, ffprobe, version, updateID, ok := discoverBundledPair(); ok {
		return newToolchain(ffmpeg, ffprobe, version, updateID, toolSourceBundled)
	}

	if ffmpeg, err := exec.LookPath("ffmpeg"); err == nil {
		if ffprobe, err := exec.LookPath("ffprobe"); err == nil {
			return newToolchain(ffmpeg, ffprobe, probeToolVersion(ffmpeg), "", toolSourcePath)
		}
	}

	return &Toolchain{}
}

func newToolchain(ffmpeg, ffprobe, version, updateID, source string) *Toolchain {
	return &Toolchain{ffmpeg: ffmpeg, ffprobe: ffprobe, version: version, updateID: updateID, source: source}
}

func discoverEnvironmentPair() (string, string, bool) {
	ffmpeg := strings.TrimSpace(os.Getenv("CCML_FFMPEG"))
	ffprobe := strings.TrimSpace(os.Getenv("CCML_FFPROBE"))
	if ffmpeg == "" || ffprobe == "" {
		return "", "", false
	}
	if !fileExists(ffmpeg) || !fileExists(ffprobe) {
		return "", "", false
	}
	return ffmpeg, ffprobe, true
}

type currentToolState struct {
	Version  string `json:"version"`
	UpdateID string `json:"updateId"`
	Dir      string `json:"dir"`
}

func discoverManagedPair() (string, string, string, string, bool) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", "", "", "", false
	}
	root := filepath.Join(configDir, "CCML", "tools", "ffmpeg")
	payload, err := os.ReadFile(filepath.Join(root, "current.json"))
	if err != nil {
		return "", "", "", "", false
	}
	var state currentToolState
	if err := json.Unmarshal(payload, &state); err != nil || strings.TrimSpace(state.Version) == "" {
		return "", "", "", "", false
	}
	dirName := strings.TrimSpace(state.Dir)
	if dirName == "" {
		dirName = safeVersionDir(state.Version)
	}
	versionDir := filepath.Join(root, filepath.Base(dirName))
	ffmpeg := filepath.Join(versionDir, executableName("ffmpeg"))
	ffprobe := filepath.Join(versionDir, executableName("ffprobe"))
	if !fileExists(ffmpeg) || !fileExists(ffprobe) {
		return "", "", "", "", false
	}
	return ffmpeg, ffprobe, state.Version, state.UpdateID, true
}

func discoverBundledPair() (string, string, string, string, bool) {
	for _, dir := range bundledToolDirectories() {
		ffmpeg := filepath.Join(dir, executableName("ffmpeg"))
		ffprobe := filepath.Join(dir, executableName("ffprobe"))
		if !fileExists(ffmpeg) || !fileExists(ffprobe) {
			continue
		}
		manifest := readBundledManifest(dir)
		version := manifest.Version
		if version == "" {
			version = probeToolVersion(ffmpeg)
		}
		return ffmpeg, ffprobe, version, manifest.UpdateID, true
	}
	return "", "", "", "", false
}

func bundledToolDirectories() []string {
	var dirs []string
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, filepath.Join(cwd, "tools"))
	}
	if executable, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(executable)
		dirs = append(dirs, filepath.Join(exeDir, "tools"))
		if runtime.GOOS == "darwin" {
			dirs = append(dirs, filepath.Clean(filepath.Join(exeDir, "..", "Resources", "tools")))
		}
	}
	return uniqueStrings(dirs)
}

func readBundledManifest(dir string) currentToolState {
	payload, err := os.ReadFile(filepath.Join(dir, "CCML-FFMPEG-MANIFEST.json"))
	if err != nil {
		// Backward-compatible fallback for earlier development bundles.
		legacy, legacyErr := os.ReadFile(filepath.Join(dir, "CCML-FFMPEG-RELEASE.txt"))
		if legacyErr != nil {
			return currentToolState{}
		}
		return currentToolState{Version: strings.TrimSpace(string(legacy))}
	}
	var state currentToolState
	if err := json.Unmarshal(payload, &state); err != nil {
		return currentToolState{}
	}
	return state
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func probeToolVersion(ffmpeg string) string {
	if strings.TrimSpace(ffmpeg) == "" {
		return ""
	}
	cmd := exec.Command(ffmpeg, "-version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(strings.SplitN(string(output), "\n", 2)[0])
	line = strings.TrimPrefix(line, "ffmpeg version ")
	if fields := strings.Fields(line); len(fields) > 0 {
		return fields[0]
	}
	return line
}

func safeVersionDir(version string) string {
	version = strings.TrimSpace(version)
	var b strings.Builder
	for _, r := range version {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = filepath.Clean(value)
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
