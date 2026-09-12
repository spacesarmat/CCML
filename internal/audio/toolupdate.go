package audio

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	shakaReleaseAPI        = "https://api.github.com/repos/shaka-project/static-ffmpeg-binaries/releases/latest"
	btbnReleaseAPI         = "https://api.github.com/repos/BtbN/FFmpeg-Builds/releases/tags/latest"
	toolUpdateInterval     = 7 * 24 * time.Hour
	maxToolDownloadSize    = int64(256 << 20)
	toolHTTPUserAgent      = "CCML/0.2 (+https://github.com/spacesarmat/CCML)"
	toolBuildProviderBtbN  = "BtbN/FFmpeg-Builds"
	toolBuildProviderShaka = "shaka-project/static-ffmpeg-binaries"
)

// ToolUpdateResult describes a completed FFmpeg toolchain update check.
type ToolUpdateResult struct {
	Version     string
	Changed     bool
	FFmpegPath  string
	FFprobePath string
	Source      string
}

// ToolUpdater downloads and validates CCML's managed FFmpeg toolchain.
// Downloads use GitHub release asset SHA-256 digests before activation.
type ToolUpdater struct {
	root        string
	tools       *Toolchain
	client      *http.Client
	apiOverride string
}

// NewToolUpdater creates an updater that stores managed binaries below appDir.
func NewToolUpdater(appDir string, tools *Toolchain) *ToolUpdater {
	return &ToolUpdater{
		root:        filepath.Join(appDir, "tools", "ffmpeg"),
		tools:       tools,
		client:      &http.Client{Timeout: 10 * time.Minute},
		apiOverride: strings.TrimSpace(os.Getenv("CCML_FFMPEG_RELEASE_API")),
	}
}

// AutoUpdateSupported reports whether managed FFmpeg builds are available for
// the current platform/architecture.
func (u *ToolUpdater) AutoUpdateSupported() bool {
	_, err := releasePlan(runtime.GOOS, runtime.GOARCH)
	return err == nil
}

// ShouldAutoCheck reports whether the weekly automatic update check is due.
// Missing tools trigger a check immediately. A freshly bundled toolchain seeds
// the check timestamp so the first application start does not redownload the
// same bootstrap build.
func (u *ToolUpdater) ShouldAutoCheck() (bool, error) {
	if u == nil || !u.AutoUpdateSupported() {
		return false, nil
	}
	if u.tools == nil || !u.tools.Ready() {
		return true, nil
	}
	snapshot := u.tools.Snapshot()
	if snapshot.Source == toolSourceEnvironment {
		return false, nil
	}
	state, err := u.readUpdateState()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("read FFmpeg update state: %w", err)
		}
		if snapshot.Source == toolSourceBundled {
			if err := u.recordCheck(time.Now().UTC()); err != nil {
				return false, err
			}
			return false, nil
		}
		return true, nil
	}
	if state.LastCheck.IsZero() {
		return true, nil
	}
	return time.Since(state.LastCheck) >= toolUpdateInterval, nil
}

// EnsureLatest checks the upstream release and installs it when required.
// If force is false, a recent successful check is reused when tools already
// exist. Explicit CCML_FFMPEG/CCML_FFPROBE overrides are never replaced.
func (u *ToolUpdater) EnsureLatest(ctx context.Context, force bool) (ToolUpdateResult, error) {
	if u == nil || u.tools == nil {
		return ToolUpdateResult{}, errors.New("FFmpeg updater is not initialized")
	}
	plan, err := releasePlan(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return ToolUpdateResult{}, err
	}
	if u.apiOverride != "" {
		plan.APIURL = u.apiOverride
	}

	current := u.tools.Snapshot()
	if current.Source == toolSourceEnvironment {
		return toolUpdateResultFromSnapshot(current, false), nil
	}
	if !force && current.FFmpeg != "" && current.FFprobe != "" {
		due, err := u.ShouldAutoCheck()
		if err != nil {
			return ToolUpdateResult{}, err
		}
		if !due {
			return toolUpdateResultFromSnapshot(current, false), nil
		}
	}

	release, err := u.fetchRelease(ctx, plan.APIURL)
	if err != nil {
		return ToolUpdateResult{}, err
	}
	resolved, err := resolveRelease(plan, release)
	if err != nil {
		return ToolUpdateResult{}, err
	}
	if current.UpdateID != "" && current.UpdateID == resolved.UpdateID && current.FFmpeg != "" && current.FFprobe != "" {
		if err := u.recordCheck(time.Now().UTC()); err != nil {
			return ToolUpdateResult{}, err
		}
		return toolUpdateResultFromSnapshot(current, false), nil
	}

	ffmpegPath, ffprobePath, version, dirName, err := u.installResolvedRelease(ctx, resolved)
	if err != nil {
		return ToolUpdateResult{}, err
	}
	state := currentToolState{Version: version, UpdateID: resolved.UpdateID, Dir: dirName}
	if err := writeJSONAtomic(filepath.Join(u.root, "current.json"), state); err != nil {
		return ToolUpdateResult{}, err
	}
	u.tools.Replace(ffmpegPath, ffprobePath, version, resolved.UpdateID, toolSourceManaged)
	if err := u.recordCheck(time.Now().UTC()); err != nil {
		return ToolUpdateResult{}, err
	}
	return ToolUpdateResult{
		Version: version, Changed: true, FFmpegPath: ffmpegPath, FFprobePath: ffprobePath, Source: toolSourceManaged,
	}, nil
}

func toolUpdateResultFromSnapshot(snapshot ToolchainSnapshot, changed bool) ToolUpdateResult {
	return ToolUpdateResult{
		Version: snapshot.Version, Changed: changed, FFmpegPath: snapshot.FFmpeg,
		FFprobePath: snapshot.FFprobe, Source: snapshot.Source,
	}
}

type toolReleaseMode int

const (
	toolReleaseDirect toolReleaseMode = iota
	toolReleaseZip
)

type toolReleasePlan struct {
	Provider     string
	APIURL       string
	Mode         toolReleaseMode
	FFmpegAsset  string
	FFprobeAsset string
	ArchiveAsset string
}

func releasePlan(goos, goarch string) (toolReleasePlan, error) {
	switch goos {
	case "windows":
		var target string
		switch goarch {
		case "amd64":
			target = "win64"
		case "arm64":
			target = "winarm64"
		default:
			return toolReleasePlan{}, fmt.Errorf("managed FFmpeg is not supported on Windows %s", goarch)
		}
		return toolReleasePlan{
			Provider:     toolBuildProviderBtbN,
			APIURL:       btbnReleaseAPI,
			Mode:         toolReleaseZip,
			ArchiveAsset: fmt.Sprintf("ffmpeg-n9.0-latest-%s-gpl-9.0.zip", target),
		}, nil
	case "darwin":
		var suffix string
		switch goarch {
		case "amd64":
			suffix = "osx-x64"
		case "arm64":
			suffix = "osx-arm64"
		default:
			return toolReleasePlan{}, fmt.Errorf("managed FFmpeg is not supported on macOS %s", goarch)
		}
		return toolReleasePlan{
			Provider:     toolBuildProviderShaka,
			APIURL:       shakaReleaseAPI,
			Mode:         toolReleaseDirect,
			FFmpegAsset:  "ffmpeg-" + suffix,
			FFprobeAsset: "ffprobe-" + suffix,
		}, nil
	default:
		return toolReleasePlan{}, fmt.Errorf("managed FFmpeg is not supported on %s", goos)
	}
}

type githubRelease struct {
	TagName string               `json:"tag_name"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
	Size               int64  `json:"size"`
}

type resolvedToolRelease struct {
	Provider     string
	ReleaseTag   string
	UpdateID     string
	Mode         toolReleaseMode
	FFmpegAsset  githubReleaseAsset
	FFprobeAsset githubReleaseAsset
	ArchiveAsset githubReleaseAsset
}

func (u *ToolUpdater) fetchRelease(ctx context.Context, apiURL string) (release githubRelease, resultErr error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return githubRelease{}, fmt.Errorf("create FFmpeg release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", toolHTTPUserAgent)
	resp, err := u.client.Do(req)
	if err != nil {
		return githubRelease{}, fmt.Errorf("check FFmpeg release: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close FFmpeg release response: %w", closeErr))
		}
	}()
	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			return githubRelease{}, fmt.Errorf("FFmpeg release API returned %s and response body could not be read: %w", resp.Status, readErr)
		}
		return githubRelease{}, fmt.Errorf("FFmpeg release API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	decoder := json.NewDecoder(io.LimitReader(resp.Body, 2<<20))
	if err := decoder.Decode(&release); err != nil {
		return githubRelease{}, fmt.Errorf("decode FFmpeg release response: %w", err)
	}
	if strings.TrimSpace(release.TagName) == "" {
		return githubRelease{}, errors.New("FFmpeg release response has no tag")
	}
	return release, nil
}

func resolveRelease(plan toolReleasePlan, release githubRelease) (resolvedToolRelease, error) {
	resolved := resolvedToolRelease{Provider: plan.Provider, ReleaseTag: release.TagName, Mode: plan.Mode}
	switch plan.Mode {
	case toolReleaseDirect:
		ffmpegAsset, err := findReleaseAsset(release.Assets, plan.FFmpegAsset)
		if err != nil {
			return resolvedToolRelease{}, err
		}
		ffprobeAsset, err := findReleaseAsset(release.Assets, plan.FFprobeAsset)
		if err != nil {
			return resolvedToolRelease{}, err
		}
		resolved.FFmpegAsset = ffmpegAsset
		resolved.FFprobeAsset = ffprobeAsset
		resolved.UpdateID = plan.Provider + ":" + release.TagName + ":" + ffmpegAsset.Digest + ":" + ffprobeAsset.Digest
	case toolReleaseZip:
		archiveAsset, err := findReleaseAsset(release.Assets, plan.ArchiveAsset)
		if err != nil {
			return resolvedToolRelease{}, err
		}
		resolved.ArchiveAsset = archiveAsset
		resolved.UpdateID = plan.Provider + ":" + archiveAsset.Digest
	default:
		return resolvedToolRelease{}, errors.New("unknown FFmpeg release mode")
	}
	return resolved, nil
}

func findReleaseAsset(assets []githubReleaseAsset, name string) (githubReleaseAsset, error) {
	for _, asset := range assets {
		if asset.Name != name {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(asset.Digest), "sha256:") {
			return githubReleaseAsset{}, fmt.Errorf("release asset %s has no SHA-256 digest", name)
		}
		if strings.TrimSpace(asset.BrowserDownloadURL) == "" {
			return githubReleaseAsset{}, fmt.Errorf("release asset %s has no download URL", name)
		}
		return asset, nil
	}
	return githubReleaseAsset{}, fmt.Errorf("FFmpeg release does not contain %s", name)
}

func (u *ToolUpdater) installResolvedRelease(ctx context.Context, release resolvedToolRelease) (ffmpegPath, ffprobePath, version, dirName string, resultErr error) {
	if err := os.MkdirAll(u.root, 0o755); err != nil {
		return "", "", "", "", fmt.Errorf("create FFmpeg root directory: %w", err)
	}
	tempDir, err := os.MkdirTemp(u.root, ".install-*")
	if err != nil {
		return "", "", "", "", fmt.Errorf("create FFmpeg temporary directory: %w", err)
	}
	cleanupTemp := true
	defer func() {
		if !cleanupTemp {
			return
		}
		if removeErr := removeDirectory(tempDir); removeErr != nil {
			resultErr = errors.Join(resultErr, removeErr)
		}
	}()

	ffmpegTemp := filepath.Join(tempDir, executableName("ffmpeg"))
	ffprobeTemp := filepath.Join(tempDir, executableName("ffprobe"))
	switch release.Mode {
	case toolReleaseDirect:
		if err := u.downloadAsset(ctx, release.FFmpegAsset, ffmpegTemp); err != nil {
			return "", "", "", "", fmt.Errorf("download FFmpeg: %w", err)
		}
		if err := u.downloadAsset(ctx, release.FFprobeAsset, ffprobeTemp); err != nil {
			return "", "", "", "", fmt.Errorf("download ffprobe: %w", err)
		}
	case toolReleaseZip:
		archivePath := filepath.Join(tempDir, "ffmpeg.zip")
		if err := u.downloadAsset(ctx, release.ArchiveAsset, archivePath); err != nil {
			return "", "", "", "", fmt.Errorf("download FFmpeg archive: %w", err)
		}
		if err := extractWindowsTools(archivePath, ffmpegTemp, ffprobeTemp); err != nil {
			return "", "", "", "", err
		}
	default:
		return "", "", "", "", errors.New("unknown FFmpeg release mode")
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(ffmpegTemp, 0o755); err != nil {
			return "", "", "", "", fmt.Errorf("make FFmpeg executable: %w", err)
		}
		if err := os.Chmod(ffprobeTemp, 0o755); err != nil {
			return "", "", "", "", fmt.Errorf("make ffprobe executable: %w", err)
		}
	}
	if err := validateToolPair(ctx, ffmpegTemp, ffprobeTemp); err != nil {
		return "", "", "", "", fmt.Errorf("validate downloaded FFmpeg tools: %w", err)
	}

	version = probeToolVersion(ffmpegTemp)
	if version == "" {
		version = release.ReleaseTag
	}
	dirName = updateDirectoryName(release.UpdateID)
	versionDir := filepath.Join(u.root, dirName)
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		return "", "", "", "", fmt.Errorf("create FFmpeg version directory: %w", err)
	}
	ffmpegPath = filepath.Join(versionDir, executableName("ffmpeg"))
	ffprobePath = filepath.Join(versionDir, executableName("ffprobe"))
	if err := replaceDownloadedFile(ffmpegTemp, ffmpegPath); err != nil {
		return "", "", "", "", err
	}
	if err := replaceDownloadedFile(ffprobeTemp, ffprobePath); err != nil {
		return "", "", "", "", err
	}

	releaseRecord := struct {
		Version     string `json:"version"`
		UpdateID    string `json:"updateId"`
		ReleaseTag  string `json:"releaseTag"`
		InstalledAt string `json:"installedAt"`
		Provider    string `json:"provider"`
	}{
		Version: version, UpdateID: release.UpdateID, ReleaseTag: release.ReleaseTag,
		InstalledAt: time.Now().UTC().Format(time.RFC3339Nano), Provider: release.Provider,
	}
	if err := writeJSONAtomic(filepath.Join(versionDir, "release.json"), releaseRecord); err != nil {
		return "", "", "", "", err
	}
	if err := removeDirectory(tempDir); err != nil {
		return "", "", "", "", err
	}
	cleanupTemp = false
	return ffmpegPath, ffprobePath, version, dirName, nil
}

func (u *ToolUpdater) downloadAsset(ctx context.Context, asset githubReleaseAsset, destination string) (resultErr error) {
	if asset.Size <= 0 || asset.Size > maxToolDownloadSize {
		return fmt.Errorf("unexpected asset size %d bytes", asset.Size)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("User-Agent", toolHTTPUserAgent)
	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", asset.Name, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close download response for %s: %w", asset.Name, closeErr))
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %s", resp.Status)
	}

	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("create downloaded tool: %w", err)
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(resp.Body, maxToolDownloadSize+1))
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("save downloaded tool: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close downloaded tool: %w", closeErr)
	}
	if written > maxToolDownloadSize {
		return fmt.Errorf("download exceeded %d bytes", maxToolDownloadSize)
	}
	if asset.Size > 0 && written != asset.Size {
		return fmt.Errorf("download size mismatch: got %d, expected %d", written, asset.Size)
	}

	expected := strings.TrimPrefix(strings.ToLower(asset.Digest), "sha256:")
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("SHA-256 mismatch: got %s, expected %s", actual, expected)
	}
	return nil
}

func extractWindowsTools(archivePath, ffmpegDestination, ffprobeDestination string) (resultErr error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open FFmpeg archive: %w", err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close FFmpeg archive: %w", closeErr))
		}
	}()

	foundFFmpeg := false
	foundFFprobe := false
	for _, file := range reader.File {
		name := strings.ToLower(filepath.ToSlash(file.Name))
		var destination string
		switch {
		case strings.HasSuffix(name, "/bin/ffmpeg.exe"):
			destination = ffmpegDestination
			foundFFmpeg = true
		case strings.HasSuffix(name, "/bin/ffprobe.exe"):
			destination = ffprobeDestination
			foundFFprobe = true
		default:
			continue
		}
		if err := extractZipFile(file, destination); err != nil {
			return err
		}
	}
	if !foundFFmpeg || !foundFFprobe {
		return fmt.Errorf("FFmpeg archive is missing required binaries (ffmpeg=%t ffprobe=%t)", foundFFmpeg, foundFFprobe)
	}
	return nil
}

func extractZipFile(file *zip.File, destination string) (resultErr error) {
	if int64(file.UncompressedSize64) > maxToolDownloadSize {
		return fmt.Errorf("archive entry %s exceeds %d bytes", file.Name, maxToolDownloadSize)
	}
	reader, err := file.Open()
	if err != nil {
		return fmt.Errorf("open %s in archive: %w", file.Name, err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close %s in archive: %w", file.Name, closeErr))
		}
	}()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("create extracted tool: %w", err)
	}
	written, copyErr := io.Copy(out, io.LimitReader(reader, maxToolDownloadSize+1))
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("extract %s: %w", file.Name, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close extracted tool: %w", closeErr)
	}
	if written > maxToolDownloadSize {
		return fmt.Errorf("archive entry %s exceeded %d bytes", file.Name, maxToolDownloadSize)
	}
	return nil
}

func removeDirectory(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove directory %q: %w", path, err)
	}
	return nil
}

func validateToolPair(parent context.Context, ffmpeg, ffprobe string) error {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	if output, err := exec.CommandContext(ctx, ffprobe, "-version").CombinedOutput(); err != nil {
		return fmt.Errorf("ffprobe -version: %w: %s", err, strings.TrimSpace(string(output)))
	}
	filters, err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-filters").CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg -filters: %w: %s", err, strings.TrimSpace(string(filters)))
	}
	filterText := string(filters)
	for _, required := range []string{"loudnorm", "adeclip", "mcompand", "alimiter"} {
		if !commandListContains(filterText, required) {
			return fmt.Errorf("FFmpeg build is missing required filter %q", required)
		}
	}
	encoders, err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-encoders").CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg -encoders: %w: %s", err, strings.TrimSpace(string(encoders)))
	}
	encoderText := string(encoders)
	for _, required := range []string{"libmp3lame", "libopus", "aac", "flac", "pcm_s24le", "pcm_s24be"} {
		if !commandListContains(encoderText, required) {
			return fmt.Errorf("FFmpeg build is missing required encoder %q", required)
		}
	}
	return nil
}

func commandListContains(output, name string) bool {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		for _, field := range fields {
			if field == name {
				return true
			}
		}
	}
	return false
}

func updateDirectoryName(updateID string) string {
	hash := sha256.Sum256([]byte(updateID))
	return "build-" + hex.EncodeToString(hash[:8])
}

func replaceDownloadedFile(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create tool directory: %w", err)
	}
	if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove old tool %q: %w", destination, err)
	}
	if err := os.Rename(source, destination); err != nil {
		return fmt.Errorf("activate tool %q: %w", destination, err)
	}
	return nil
}

type toolUpdateState struct {
	LastCheck time.Time `json:"lastCheck"`
}

func (u *ToolUpdater) readUpdateState() (toolUpdateState, error) {
	payload, err := os.ReadFile(filepath.Join(u.root, "update-state.json"))
	if err != nil {
		return toolUpdateState{}, err
	}
	var state toolUpdateState
	if err := json.Unmarshal(payload, &state); err != nil {
		return toolUpdateState{}, fmt.Errorf("decode FFmpeg update state: %w", err)
	}
	return state, nil
}

func (u *ToolUpdater) recordCheck(checkedAt time.Time) error {
	return writeJSONAtomic(filepath.Join(u.root, "update-state.json"), toolUpdateState{LastCheck: checkedAt})
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state file: %w", err)
	}
	payload = append(payload, '\n')
	temp, err := os.CreateTemp(filepath.Dir(path), ".state-*.json")
	if err != nil {
		return fmt.Errorf("create temporary state file: %w", err)
	}
	tempName := temp.Name()
	if _, err := temp.Write(payload); err != nil {
		closeErr := temp.Close()
		removeErr := removeTempStateFile(tempName)
		return errors.Join(fmt.Errorf("write state file: %w", err), closeErr, removeErr)
	}
	if err := temp.Close(); err != nil {
		removeErr := removeTempStateFile(tempName)
		return errors.Join(fmt.Errorf("close state file: %w", err), removeErr)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		removeErr := removeTempStateFile(tempName)
		return errors.Join(fmt.Errorf("replace state file: %w", err), removeErr)
	}
	if err := os.Rename(tempName, path); err != nil {
		removeErr := removeTempStateFile(tempName)
		return errors.Join(fmt.Errorf("activate state file: %w", err), removeErr)
	}
	return nil
}

func removeTempStateFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove temporary state file: %w", err)
	}
	return nil
}
