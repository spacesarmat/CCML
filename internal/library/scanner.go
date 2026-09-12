// Package library implements scanning and duplicate detection.
package library

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
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
	path string
	info fs.FileInfo
}

type scanOutcome struct {
	track model.Track
	err   error
}

// Scan indexes supported audio files below root.
func (s *Scanner) Scan(ctx context.Context, root string) (model.ScanResult, error) {
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

	jobs := make(chan scanItem)
	outcomes := make(chan scanOutcome)
	workerCount := min(4, max(1, runtime.NumCPU()))

	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range jobs {
				track, err := s.probe.Read(ctx, item.path)
				if err == nil {
					track.Size = item.info.Size()
					track.ModifiedUnix = item.info.ModTime().Unix()
				}
				select {
				case outcomes <- scanOutcome{track: track, err: err}:
				case <-ctx.Done():
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
			if ctx.Err() != nil {
				return ctx.Err()
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
			select {
			case jobs <- scanItem{path: path, info: fileInfo}:
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
	for outcome := range outcomes {
		result.Found++
		if outcome.err != nil {
			result.Failed++
			if len(result.Errors) < 100 {
				result.Errors = append(result.Errors, outcome.err.Error())
			}
			continue
		}
		if _, err := s.store.UpsertTrack(ctx, outcome.track); err != nil {
			result.Failed++
			if len(result.Errors) < 100 {
				result.Errors = append(result.Errors, err.Error())
			}
			continue
		}
		result.Indexed++
	}

	if err := <-walkDone; err != nil {
		return result, fmt.Errorf("walk music directory: %w", err)
	}
	result.Duration = time.Since(started)
	return result, nil
}
