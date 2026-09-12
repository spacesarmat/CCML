// Package audio contains FFmpeg/ffprobe based audio analysis and processing.
package audio

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Toolchain points to FFmpeg executables used by the backend.
type Toolchain struct {
	FFmpeg  string
	FFprobe string
}

// Ready reports whether both FFmpeg and ffprobe are available.
func (t *Toolchain) Ready() bool {
	return strings.TrimSpace(t.FFmpeg) != "" && strings.TrimSpace(t.FFprobe) != ""
}

// DiscoverToolchain searches explicit environment variables, a bundled tools
// directory next to the executable, and finally PATH.
func DiscoverToolchain() *Toolchain {
	return &Toolchain{
		FFmpeg:  discoverBinary("CCML_FFMPEG", "ffmpeg"),
		FFprobe: discoverBinary("CCML_FFPROBE", "ffprobe"),
	}
}

func discoverBinary(envName, name string) string {
	if explicit := strings.TrimSpace(os.Getenv(envName)); explicit != "" {
		if fileExists(explicit) {
			return explicit
		}
	}

	exeName := name
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	if executable, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(executable), "tools", exeName)
		if fileExists(candidate) {
			return candidate
		}
	}

	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
