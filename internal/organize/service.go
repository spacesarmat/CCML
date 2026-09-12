// Package organize implements safe mp3tag-style file organization.
package organize

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/spacesarmat/CCML/internal/model"
	"github.com/spacesarmat/CCML/internal/store"
)

// Service renders templates and moves/renames indexed files.
type Service struct {
	store *store.Store
}

// NewService creates an organizer.
func NewService(store *store.Store) *Service {
	return &Service{store: store}
}

// Preview computes a target path without changing the filesystem.
func (s *Service) Preview(track model.Track, req model.OrganizeRequest) (string, error) {
	template := strings.TrimSpace(req.Template)
	if template == "" {
		template = "%artist%/%album%/%track% - %title%"
	}

	relative, err := renderRelative(track, template)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(req.RegexPattern) != "" {
		re, err := regexp.Compile(req.RegexPattern)
		if err != nil {
			return "", fmt.Errorf("compile rename regex: %w", err)
		}
		relative = re.ReplaceAllString(relative, req.RegexReplace)
		relative, err = sanitizeRelative(relative)
		if err != nil {
			return "", err
		}
	}

	ext := strings.ToLower(filepath.Ext(track.Path))
	if ext == "" {
		return "", fmt.Errorf("track has no file extension: %s", track.Path)
	}
	if !strings.EqualFold(filepath.Ext(relative), ext) {
		relative += ext
	}

	root := filepath.Dir(track.Path)
	if req.Move && strings.TrimSpace(req.RootDir) != "" {
		root = strings.TrimSpace(req.RootDir)
	}
	root, err = filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", fmt.Errorf("resolve organize root: %w", err)
	}
	target := filepath.Join(root, filepath.FromSlash(relative))
	if err := ensureInside(root, target); err != nil {
		return "", err
	}
	return target, nil
}

// Apply moves/renames a track and updates its indexed path. The source is never
// overwritten and a failed database update attempts to roll the move back.
func (s *Service) Apply(ctx context.Context, track model.Track, req model.OrganizeRequest) (string, error) {
	target, err := s.Preview(track, req)
	if err != nil {
		return "", err
	}
	sourceAbs, err := filepath.Abs(track.Path)
	if err != nil {
		return "", fmt.Errorf("resolve source path: %w", err)
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve target path: %w", err)
	}
	if filepath.Clean(sourceAbs) == filepath.Clean(targetAbs) {
		return targetAbs, nil
	}
	if _, err := os.Stat(targetAbs); err == nil {
		return "", fmt.Errorf("target file already exists: %s", targetAbs)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("check target file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
		return "", fmt.Errorf("create target directory: %w", err)
	}
	if err := moveFile(sourceAbs, targetAbs); err != nil {
		return "", err
	}

	if err := s.store.UpdateTrackPath(ctx, track.ID, targetAbs, filepath.Base(targetAbs), strings.ToLower(filepath.Ext(targetAbs))); err != nil {
		rollbackErr := moveFile(targetAbs, sourceAbs)
		if rollbackErr != nil {
			return "", errors.Join(err, fmt.Errorf("rollback file move: %w", rollbackErr))
		}
		return "", err
	}
	return targetAbs, nil
}

func renderRelative(track model.Track, template string) (string, error) {
	replacements := map[string]string{
		"%artist%":      track.Artist,
		"%albumartist%": track.AlbumArtist,
		"%album%":       track.Album,
		"%title%":       track.Title,
		"%genre%":       track.Genre,
		"%year%":        zeroEmpty(track.Year),
		"%track%":       padNumber(track.TrackNumber, 2),
		"%discnumber%":  padNumber(track.DiscNumber, 1),
		"{artist}":      track.Artist,
		"{albumArtist}": track.AlbumArtist,
		"{album}":       track.Album,
		"{title}":       track.Title,
		"{genre}":       track.Genre,
		"{year}":        zeroEmpty(track.Year),
		"{track}":       zeroEmpty(track.TrackNumber),
		"{track:02}":    padNumber(track.TrackNumber, 2),
		"{disc}":        zeroEmpty(track.DiscNumber),
	}
	result := template
	for token, value := range replacements {
		result = strings.ReplaceAll(result, token, value)
	}
	return sanitizeRelative(result)
}

func sanitizeRelative(value string) (string, error) {
	value = strings.ReplaceAll(value, `\`, "/")
	parts := strings.Split(value, "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = sanitizeComponent(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		clean = append(clean, part)
	}
	if len(clean) == 0 {
		return "", errors.New("rename template produced an empty path")
	}
	return strings.Join(clean, "/"), nil
}

func sanitizeComponent(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			b.WriteByte('_')
			continue
		}
		if unicode.IsControl(r) {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	result := strings.TrimSpace(b.String())
	result = strings.TrimRight(result, ". ")
	if result == "" {
		return "_"
	}
	if isWindowsReserved(result) {
		result = "_" + result
	}
	return result
}

func isWindowsReserved(value string) bool {
	base := strings.ToUpper(strings.TrimSuffix(value, filepath.Ext(value)))
	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	for i := 1; i <= 9; i++ {
		if base == "COM"+strconv.Itoa(i) || base == "LPT"+strconv.Itoa(i) {
			return true
		}
	}
	return false
}

func ensureInside(root, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return fmt.Errorf("validate target path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("target escapes organize root: %s", target)
	}
	return nil
}

func moveFile(source, target string) error {
	if err := os.Rename(source, target); err == nil {
		return nil
	}
	// os.Rename may fail across volumes on Windows/macOS. Fall back to an
	// exclusive copy followed by source removal, never overwriting target.
	if err := copyFile(source, target); err != nil {
		return fmt.Errorf("move %q to %q: %w", source, target, err)
	}
	if err := os.Remove(source); err != nil {
		return fmt.Errorf("remove source after cross-volume copy: %w", err)
	}
	return nil
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	info, err := in.Stat()
	if err != nil {
		closeErr := in.Close()
		if closeErr != nil {
			return errors.Join(fmt.Errorf("stat source: %w", err), fmt.Errorf("close source: %w", closeErr))
		}
		return fmt.Errorf("stat source: %w", err)
	}

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		closeErr := in.Close()
		if closeErr != nil {
			return errors.Join(fmt.Errorf("create target: %w", err), fmt.Errorf("close source: %w", closeErr))
		}
		return fmt.Errorf("create target: %w", err)
	}

	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	outCloseErr := out.Close()
	inCloseErr := in.Close()
	if err := errors.Join(copyErr, syncErr, outCloseErr, inCloseErr); err != nil {
		removeErr := os.Remove(target)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return errors.Join(fmt.Errorf("copy file: %w", err), fmt.Errorf("remove partial target: %w", removeErr))
		}
		return fmt.Errorf("copy file: %w", err)
	}
	return nil
}

func zeroEmpty(value int) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func padNumber(value, width int) string {
	if value == 0 {
		return ""
	}
	return fmt.Sprintf("%0*d", width, value)
}
