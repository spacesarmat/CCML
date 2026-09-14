// Package library implements scanning and duplicate detection.
package library

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spacesarmat/CCML/internal/audio"
	"github.com/spacesarmat/CCML/internal/model"
	"github.com/spacesarmat/CCML/internal/store"
)

var supportedExtensions = map[string]struct{}{
	".mp3": {}, ".flac": {}, ".m4a": {}, ".aac": {},
	".wav": {}, ".aif": {}, ".aiff": {}, ".ogg": {}, ".oga": {},
}

// ProgressFunc receives scanner progress snapshots. It must return quickly.
type ProgressFunc func(model.ScanProgress)

// Scanner walks folders, probes audio files and stores them in SQLite.
type Scanner struct {
	store *store.Store
	probe *audio.Probe
}

// NewScanner creates a library scanner.
func NewScanner(store *store.Store, probe *audio.Probe) *Scanner {
	return &Scanner{store: store, probe: probe}
}

type scanItem struct {
	path    string
	info    fs.FileInfo
	existed bool
}

type scanOutcome struct {
	item    scanItem
	track   model.Track
	skipped bool
	err     error
}

// Scan incrementally indexes supported audio files below root. Unchanged files
// are not probed again. Missing database entries are removed only after a full,
// non-cancelled directory walk.
func (s *Scanner) Scan(ctx context.Context, root string, onProgress ProgressFunc) (model.ScanResult, error) {
	started := time.Now()
	root, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return model.ScanResult{}, fmt.Errorf("resolve scan root: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return model.ScanResult{}, fmt.Errorf("stat scan root %q: %w", root, err)
	}
	if !info.IsDir() {
		return model.ScanResult{}, fmt.Errorf("scan root is not a directory: %s", root)
	}

	if err := s.store.UpsertLibraryRoot(ctx, root); err != nil {
		return model.ScanResult{}, err
	}
	existing, err := s.store.TrackStatesUnderRoot(ctx, root)
	if err != nil {
		return model.ScanResult{}, err
	}

	scanID := strconv.FormatInt(time.Now().UnixNano(), 36)
	jobs := make(chan scanItem)
	outcomes := make(chan scanOutcome)
	workerCount := min(4, max(1, runtime.NumCPU()))

	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range jobs {
				state, exists := existing[item.path]
				if exists && state.Size == item.info.Size() && state.ModifiedUnix == item.info.ModTime().Unix() && state.CoverIndexed {
					if !sendOutcome(ctx, outcomes, scanOutcome{item: item, skipped: true}) {
						return
					}
					continue
				}

				track, err := s.probe.Read(ctx, item.path)
				if err == nil {
					track.Size = item.info.Size()
					track.ModifiedUnix = item.info.ModTime().Unix()
				}
				if !sendOutcome(ctx, outcomes, scanOutcome{item: item, track: track, err: err}) {
					return
				}
			}
		}()
	}

	walkDone := make(chan error, 1)
	go func() {
		defer close(jobs)
		walkDone <- filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if _, ok := supportedExtensions[strings.ToLower(filepath.Ext(path))]; !ok {
				return nil
			}
			fileInfo, err := entry.Info()
			if err != nil {
				return fmt.Errorf("read file info %q: %w", path, err)
			}
			_, existed := existing[path]
			select {
			case jobs <- scanItem{path: path, info: fileInfo, existed: existed}:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()

	go func() {
		workers.Wait()
		close(outcomes)
	}()

	result := model.ScanResult{Root: root}
	progress := model.ScanProgress{Root: root}
	emit := func(current string, finished bool) {
		if onProgress == nil {
			return
		}
		progress.CurrentFile = current
		progress.Found = result.Found
		progress.Scanned = result.Found
		progress.Added = result.Added
		progress.Updated = result.Updated
		progress.Skipped = result.Skipped
		progress.Removed = result.Removed
		progress.Failed = result.Failed
		progress.Cancelled = result.Cancelled
		progress.Finished = finished
		onProgress(progress)
	}

	for outcome := range outcomes {
		result.Found++
		if errors.Is(outcome.err, context.Canceled) || errors.Is(outcome.err, context.DeadlineExceeded) {
			result.Cancelled = true
			emit(outcome.item.path, false)
			continue
		}
		if outcome.err != nil {
			result.Failed++
			appendScanError(&result, outcome.err)
			emit(outcome.item.path, false)
			continue
		}

		if !outcome.skipped {
			trackID, err := s.store.UpsertTrack(ctx, outcome.track)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					result.Cancelled = true
				} else {
					result.Failed++
					appendScanError(&result, err)
				}
				emit(outcome.item.path, false)
				continue
			}
			if err := s.store.UpdateTrackCoverPresence(ctx, trackID, outcome.track.HasCover); err != nil {
				// Do not leave the otherwise valid file unseen: keep the track,
				// report the index problem, and let a later scan retry it.
				_ = s.store.InvalidateTrackCoverPresence(context.Background(), trackID)
				result.Failed++
				appendScanError(&result, err)
			}
		}
		if err := s.store.MarkTrackSeen(ctx, outcome.item.path, root, scanID); err != nil {
			if errors.Is(err, context.Canceled) {
				result.Cancelled = true
			} else {
				result.Failed++
				appendScanError(&result, err)
			}
			emit(outcome.item.path, false)
			continue
		}

		switch {
		case outcome.skipped:
			result.Skipped++
		case outcome.item.existed:
			result.Updated++
		default:
			result.Added++
		}
		result.Indexed = result.Added + result.Updated
		emit(outcome.item.path, false)
	}

	walkErr := <-walkDone
	if errors.Is(walkErr, context.Canceled) || errors.Is(walkErr, context.DeadlineExceeded) || ctx.Err() != nil {
		result.Cancelled = true
		result.Duration = time.Since(started)
		emit("", true)
		return result, nil
	}
	if walkErr != nil {
		result.Duration = time.Since(started)
		emit("", true)
		return result, fmt.Errorf("walk music directory: %w", walkErr)
	}

	removed, err := s.store.DeleteUnseenTracks(ctx, root, scanID)
	if err != nil {
		return result, err
	}
	result.Removed = int(removed)
	if err := s.store.MarkLibraryRootScanned(ctx, root); err != nil {
		return result, err
	}

	result.Duration = time.Since(started)
	emit("", true)
	return result, nil
}

func sendOutcome(ctx context.Context, outcomes chan<- scanOutcome, outcome scanOutcome) bool {
	select {
	case outcomes <- outcome:
		return true
	case <-ctx.Done():
		return false
	}
}

func appendScanError(result *model.ScanResult, err error) {
	if len(result.Errors) < 100 {
		result.Errors = append(result.Errors, err.Error())
	}
}
