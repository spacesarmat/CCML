package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tommyo123/mtag"

	"github.com/spacesarmat/CCML/internal/audio"
	"github.com/spacesarmat/CCML/internal/model"
	"github.com/spacesarmat/CCML/internal/store"
)

const (
	mediaPreviewVersion    = "audio-preview-v1"
	spectrogramVersion     = "spectrogram-v1"
	mediaPreviewBitrate    = "160k"
	spectrogramFilterGraph = "showspectrumpic=s=1000x320:legend=disabled:scale=log"
)

// mediaService prepares browser-compatible audio previews, artwork and
// spectrograms in CCML's private cache and exposes the cache through a
// random-token loopback HTTP server. http.ServeContent provides byte ranges,
// so the HTML audio element can seek without loading the entire track first.
type mediaService struct {
	store    *store.Store
	tools    *audio.Toolchain
	cacheDir string
	token    string
	baseURL  string
	server   *http.Server
	listener net.Listener
	mu       sync.Mutex
}

func newMediaService(db *store.Store, tools *audio.Toolchain, appDir string) (*mediaService, error) {
	cacheDir := filepath.Join(appDir, "media-cache")
	for _, dir := range []string{
		cacheDir,
		filepath.Join(cacheDir, "audio"),
		filepath.Join(cacheDir, "artwork"),
		filepath.Join(cacheDir, "spectrogram"),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create media cache: %w", err)
		}
	}

	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("create media-server token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start local media server: %w", err)
	}

	service := &mediaService{
		store:    db,
		tools:    tools,
		cacheDir: cacheDir,
		token:    token,
		listener: listener,
	}
	service.baseURL = "http://" + listener.Addr().String() + "/ccml-media/" + token
	service.server = &http.Server{
		Handler:           service,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		err := service.server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			// The Wails logger is not available here during construction. A later
			// media request will surface the failure to the frontend.
			_ = err
		}
	}()
	return service, nil
}

func (m *mediaService) Close() error {
	if m == nil || m.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return m.server.Shutdown(ctx)
}

func (m *mediaService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	prefix := "/ccml-media/" + m.token + "/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, prefix)
	if rel == "" || strings.Contains(rel, "\\") {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(rel, "track/") {
		m.serveTrack(w, r, strings.TrimPrefix(rel, "track/"))
		return
	}
	rel = filepath.Clean(filepath.FromSlash(rel))
	if rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(m.cacheDir, rel)
	if !isWithin(path, m.cacheDir) {
		http.NotFound(w, r)
		return
	}

	file, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	contentType := contentTypeForPath(path)
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func (m *mediaService) PrepareTrack(ctx context.Context, trackID int64) (model.TrackMedia, error) {
	track, err := m.store.TrackByID(ctx, trackID)
	if err != nil {
		return model.TrackMedia{}, err
	}
	info, err := os.Stat(track.Path)
	if err != nil {
		return model.TrackMedia{}, fmt.Errorf("open track for player: %w", err)
	}
	if info.IsDir() {
		return model.TrackMedia{}, errors.New("track path is a directory")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	audioURL := m.trackURL(track.ID, track.Path)
	isPreview := false
	if !browserFriendlyAudio(track.Extension) {
		if m.tools == nil || !m.tools.Ready() {
			return model.TrackMedia{}, errors.New("FFmpeg toolchain is not available for compatibility preview")
		}
		audioPath, err := m.ensureAudioPreview(ctx, track.Path)
		if err != nil {
			return model.TrackMedia{}, err
		}
		audioURL = m.resourceURL(audioPath)
		isPreview = true
	}
	coverPath, _ := m.ensureArtwork(track.Path)

	result := model.TrackMedia{
		AudioURL:   audioURL,
		DurationMS: track.DurationMS,
		IsPreview:  isPreview,
	}
	if coverPath != "" {
		result.CoverURL = m.resourceURL(coverPath)
	}
	return result, nil
}

// PrepareCover returns only the cached embedded artwork URL for Library
// thumbnails. Unlike PrepareTrack it never prepares/transcodes audio.
func (m *mediaService) PrepareCover(ctx context.Context, trackID int64) (string, error) {
	track, err := m.store.TrackByID(ctx, trackID)
	if err != nil {
		return "", err
	}

	coverPath, err := m.ensureArtwork(track.Path)
	if err != nil {
		return "", err
	}
	if coverPath == "" {
		return "", nil
	}

	url := m.resourceURL(coverPath)
	if url == "" {
		return "", errors.New("failed to create artwork URL")
	}
	return url, nil
}

// PrepareAudioPreview forces a browser-compatible MP3 preview. The frontend
// uses this only after native WebView playback rejects the direct stream.
func (m *mediaService) PrepareAudioPreview(ctx context.Context, trackID int64) (string, error) {
	track, err := m.store.TrackByID(ctx, trackID)
	if err != nil {
		return "", err
	}
	if m.tools == nil || !m.tools.Ready() {
		return "", errors.New("FFmpeg toolchain is not available for compatibility preview")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	audioPath, err := m.ensureAudioPreview(ctx, track.Path)
	if err != nil {
		return "", err
	}
	url := m.resourceURL(audioPath)
	if url == "" {
		return "", errors.New("failed to create compatibility preview URL")
	}
	return url, nil
}

func (m *mediaService) Spectrograms(ctx context.Context, trackID int64, processedPath string) (model.SpectrogramComparison, error) {
	track, err := m.store.TrackByID(ctx, trackID)
	if err != nil {
		return model.SpectrogramComparison{}, err
	}
	if m.tools == nil || !m.tools.Ready() {
		return model.SpectrogramComparison{}, errors.New("FFmpeg toolchain is not available")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	beforePath, err := m.ensureSpectrogram(ctx, track.Path)
	if err != nil {
		return model.SpectrogramComparison{}, err
	}
	result := model.SpectrogramComparison{
		BeforeURL:  m.resourceURL(beforePath),
		BeforePath: track.Path,
	}

	processedPath = strings.TrimSpace(processedPath)
	if processedPath == "" {
		return result, nil
	}
	absolute, err := filepath.Abs(processedPath)
	if err != nil {
		return model.SpectrogramComparison{}, fmt.Errorf("resolve processed file: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return model.SpectrogramComparison{}, fmt.Errorf("processed file is not available: %w", err)
	}
	if info.IsDir() {
		return model.SpectrogramComparison{}, errors.New("processed output is a directory")
	}
	afterPath, err := m.ensureSpectrogram(ctx, absolute)
	if err != nil {
		return model.SpectrogramComparison{}, err
	}
	result.AfterURL = m.resourceURL(afterPath)
	result.AfterPath = absolute
	return result, nil
}

func (m *mediaService) ensureAudioPreview(ctx context.Context, input string) (string, error) {
	key, err := fileCacheKey(input, mediaPreviewVersion)
	if err != nil {
		return "", err
	}
	output := filepath.Join(m.cacheDir, "audio", key+".mp3")
	if regularFileExists(output) {
		return output, nil
	}
	temp := output + ".tmp.mp3"
	if err := os.Remove(temp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("remove stale audio preview: %w", err)
	}

	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", input,
		"-map", "0:a:0", "-vn", "-sn", "-dn",
		"-map_metadata", "-1",
		"-ac", "2",
		"-c:a", "libmp3lame", "-b:a", mediaPreviewBitrate,
		"-write_xing", "1",
		temp,
	}
	cmd := exec.CommandContext(ctx, m.tools.FFmpegPath(), args...)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(temp)
		return "", fmt.Errorf("prepare audio preview: %w: %s", err, strings.TrimSpace(string(outputBytes)))
	}
	if err := replaceCacheFile(temp, output); err != nil {
		return "", err
	}
	return output, nil
}

func (m *mediaService) ensureArtwork(input string) (resultPath string, resultErr error) {
	key, err := fileCacheKey(input, "artwork-v1")
	if err != nil {
		return "", err
	}
	for _, ext := range []string{".jpg", ".png", ".webp", ".gif"} {
		path := filepath.Join(m.cacheDir, "artwork", key+ext)
		if regularFileExists(path) {
			return path, nil
		}
	}

	file, err := mtag.Open(input, mtag.WithReadOnly())
	if err != nil {
		return "", fmt.Errorf("read cover art: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	picture, ok := selectArtwork(file.Images())
	if !ok {
		return "", nil
	}
	ext := extensionForImage(picture.MIME, picture.Data)
	path := filepath.Join(m.cacheDir, "artwork", key+ext)
	if err := writeCacheFile(path, picture.Data); err != nil {
		return "", err
	}
	return path, nil
}

func selectArtwork(images []mtag.Picture) (mtag.Picture, bool) {
	var fallback mtag.Picture
	fallbackSet := false
	for _, picture := range images {
		if len(picture.Data) == 0 {
			continue
		}
		if picture.Type == mtag.PictureCoverFront {
			return picture, true
		}
		if !fallbackSet {
			fallback = picture
			fallbackSet = true
		}
	}
	return fallback, fallbackSet
}

func (m *mediaService) ensureSpectrogram(ctx context.Context, input string) (string, error) {
	key, err := fileCacheKey(input, spectrogramVersion)
	if err != nil {
		return "", err
	}
	output := filepath.Join(m.cacheDir, "spectrogram", key+".png")
	if regularFileExists(output) {
		return output, nil
	}
	temp := output + ".tmp.png"
	if err := os.Remove(temp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("remove stale spectrogram: %w", err)
	}
	cmd := exec.CommandContext(ctx, m.tools.FFmpegPath(),
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", input,
		"-lavfi", spectrogramFilterGraph,
		"-frames:v", "1",
		temp,
	)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(temp)
		return "", fmt.Errorf("generate spectrogram: %w: %s", err, strings.TrimSpace(string(outputBytes)))
	}
	if err := replaceCacheFile(temp, output); err != nil {
		return "", err
	}
	return output, nil
}

func (m *mediaService) serveTrack(w http.ResponseWriter, r *http.Request, rawID string) {
	id, err := strconv.ParseInt(strings.TrimSpace(rawID), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	track, err := m.store.TrackByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(track.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	contentType := contentTypeForPath(track.Path)
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func (m *mediaService) trackURL(trackID int64, path string) string {
	url := fmt.Sprintf("%s/track/%d", m.baseURL, trackID)
	key, err := fileCacheKey(path, "track-stream-v1")
	if err != nil || key == "" {
		return url
	}
	if len(key) > 12 {
		key = key[:12]
	}
	return url + "?v=" + key
}

func browserFriendlyAudio(extension string) bool {
	switch strings.ToLower(strings.TrimSpace(extension)) {
	case ".mp3", ".m4a", ".aac", ".wav", ".flac":
		return true
	default:
		return false
	}
}

func (m *mediaService) resourceURL(path string) string {
	rel, err := filepath.Rel(m.cacheDir, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	return m.baseURL + "/" + filepath.ToSlash(rel)
}

func fileCacheKey(path, profile string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve media path: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("stat media file: %w", err)
	}
	if info.IsDir() {
		return "", errors.New("media path is a directory")
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d\x00%d", profile, filepath.Clean(absolute), info.Size(), info.ModTime().UnixNano())))
	return hex.EncodeToString(hash[:16]), nil
}

func contentTypeForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		return "audio/mpeg"
	case ".m4a", ".mp4":
		return "audio/mp4"
	case ".aac":
		return "audio/aac"
	case ".wav":
		return "audio/wav"
	case ".flac":
		return "audio/flac"
	case ".ogg", ".oga":
		return "audio/ogg"
	case ".aiff", ".aif":
		return "audio/aiff"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	}
	return mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
}

func extensionForImage(value string, data []byte) string {
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return ".jpg"
	}
	if len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' &&
		data[4] == 0x0d && data[5] == 0x0a && data[6] == 0x1a && data[7] == 0x0a {
		return ".png"
	}
	if len(data) >= 6 && data[0] == 'G' && data[1] == 'I' && data[2] == 'F' {
		return ".gif"
	}
	if len(data) >= 12 &&
		data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' &&
		data[8] == 'W' && data[9] == 'E' && data[10] == 'B' && data[11] == 'P' {
		return ".webp"
	}

	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".jpg"
	}
}

func writeCacheFile(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".ccml-cache-*")
	if err != nil {
		return fmt.Errorf("create cache file: %w", err)
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write cache file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close cache file: %w", err)
	}
	if err := replaceCacheFile(tempName, path); err != nil {
		return err
	}
	return nil
}

func replaceCacheFile(temp, target string) error {
	if err := os.Rename(temp, target); err == nil {
		return nil
	}
	// Windows does not replace an existing destination atomically. Another
	// concurrent request may have won the race, which is already good enough.
	if regularFileExists(target) {
		_ = os.Remove(temp)
		return nil
	}
	input, err := os.Open(temp)
	if err != nil {
		return fmt.Errorf("open temporary cache file: %w", err)
	}
	defer input.Close()
	output, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("create cache target: %w", err)
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return fmt.Errorf("copy cache file: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close cache target: %w", closeErr)
	}
	_ = os.Remove(temp)
	return nil
}

func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

func isWithin(path, root string) bool {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absoluteRoot, absolutePath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
