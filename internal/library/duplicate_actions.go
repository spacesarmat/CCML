package library

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

const duplicateActionLimit = 500

type duplicateActionStore interface {
	TrackByID(ctx context.Context, id int64) (model.Track, error)
	AllTracks(ctx context.Context) ([]model.Track, error)
	ListLibraryRoots(ctx context.Context) ([]model.LibraryRoot, error)
	DeleteTracksByID(ctx context.Context, ids []int64) error
}

type duplicateMove struct {
	track  model.Track
	source string
	target string
}

// QuarantineDuplicateTracks moves selected duplicate files to a user-selected
// folder outside all managed library roots, then removes their index rows.
//
// The filesystem move happens before the database delete. If the database
// transaction fails, every completed move is rolled back.
func QuarantineDuplicateTracks(
	ctx context.Context,
	store duplicateActionStore,
	trackIDs []int64,
	destinationRoot string,
) (model.DuplicateActionResult, error) {
	ids, err := normalizeDuplicateActionIDs(trackIDs)
	if err != nil {
		return model.DuplicateActionResult{}, err
	}

	tracks, err := loadDuplicateActionTracks(ctx, store, ids)
	if err != nil {
		return model.DuplicateActionResult{}, err
	}
	if err := validateDuplicateActionSelection(ctx, store, ids); err != nil {
		return model.DuplicateActionResult{}, err
	}

	destinationRoot = strings.TrimSpace(destinationRoot)
	if destinationRoot == "" {
		return model.DuplicateActionResult{}, errors.New("quarantine folder is required")
	}
	destinationRoot, err = filepath.Abs(filepath.Clean(destinationRoot))
	if err != nil {
		return model.DuplicateActionResult{}, fmt.Errorf("resolve quarantine folder: %w", err)
	}

	quarantineRoot := filepath.Join(destinationRoot, "CCML Duplicates")
	if err := validateQuarantineOutsideLibrary(ctx, store, quarantineRoot); err != nil {
		return model.DuplicateActionResult{}, err
	}
	if err := os.MkdirAll(quarantineRoot, 0o755); err != nil {
		return model.DuplicateActionResult{}, fmt.Errorf("create quarantine folder %q: %w", quarantineRoot, err)
	}

	moves, err := planQuarantineMoves(tracks, quarantineRoot)
	if err != nil {
		return model.DuplicateActionResult{}, err
	}

	completed := make([]duplicateMove, 0, len(moves))
	for _, move := range moves {
		if err := moveFileExclusive(move.source, move.target); err != nil {
			rollbackErr := rollbackDuplicateMoves(completed)
			if rollbackErr != nil {
				return model.DuplicateActionResult{}, errors.Join(
					fmt.Errorf("move duplicate to quarantine: %w", err),
					rollbackErr,
				)
			}
			return model.DuplicateActionResult{}, fmt.Errorf("move duplicate to quarantine: %w", err)
		}
		completed = append(completed, move)
	}

	if err := store.DeleteTracksByID(ctx, ids); err != nil {
		rollbackErr := rollbackDuplicateMoves(completed)
		if rollbackErr != nil {
			return model.DuplicateActionResult{}, errors.Join(
				fmt.Errorf("remove quarantined tracks from library index: %w", err),
				rollbackErr,
			)
		}
		return model.DuplicateActionResult{}, fmt.Errorf("remove quarantined tracks from library index: %w", err)
	}

	paths := make([]string, 0, len(completed))
	for _, move := range completed {
		paths = append(paths, move.target)
	}

	return model.DuplicateActionResult{
		Action:      "quarantine",
		Requested:   len(ids),
		Completed:   len(ids),
		Destination: quarantineRoot,
		Paths:       paths,
	}, nil
}

// DeleteDuplicateTracks permanently removes selected duplicate files.
//
// Safety sequence:
//  1. validate duplicate membership and ensure every affected group keeps one;
//  2. atomically rename each source to a temporary sibling;
//  3. delete all SQLite rows in one transaction;
//  4. remove the temporary files.
//
// If staging or the database transaction fails, files are restored to their
// original paths. If final physical removal fails, the database operation stays
// committed and the leftover temporary path is returned for manual cleanup.
func DeleteDuplicateTracks(
	ctx context.Context,
	store duplicateActionStore,
	trackIDs []int64,
	confirmation string,
) (model.DuplicateActionResult, error) {
	if confirmation != "DELETE" {
		return model.DuplicateActionResult{}, errors.New("permanent duplicate deletion requires DELETE confirmation")
	}

	ids, err := normalizeDuplicateActionIDs(trackIDs)
	if err != nil {
		return model.DuplicateActionResult{}, err
	}
	tracks, err := loadDuplicateActionTracks(ctx, store, ids)
	if err != nil {
		return model.DuplicateActionResult{}, err
	}
	if err := validateDuplicateActionSelection(ctx, store, ids); err != nil {
		return model.DuplicateActionResult{}, err
	}

	staged := make([]duplicateMove, 0, len(tracks))
	reserved := make(map[string]struct{}, len(tracks))
	for _, track := range tracks {
		source, err := filepath.Abs(filepath.Clean(track.Path))
		if err != nil {
			rollbackErr := rollbackDeleteStages(staged)
			if rollbackErr != nil {
				return model.DuplicateActionResult{}, errors.Join(
					fmt.Errorf("resolve duplicate path: %w", err),
					rollbackErr,
				)
			}
			return model.DuplicateActionResult{}, fmt.Errorf("resolve duplicate path: %w", err)
		}
		if err := validateSourceFile(source); err != nil {
			rollbackErr := rollbackDeleteStages(staged)
			if rollbackErr != nil {
				return model.DuplicateActionResult{}, errors.Join(err, rollbackErr)
			}
			return model.DuplicateActionResult{}, err
		}

		temp, err := uniqueDeleteStagePath(source, track.ID, reserved)
		if err != nil {
			rollbackErr := rollbackDeleteStages(staged)
			if rollbackErr != nil {
				return model.DuplicateActionResult{}, errors.Join(err, rollbackErr)
			}
			return model.DuplicateActionResult{}, err
		}

		if err := os.Rename(source, temp); err != nil {
			rollbackErr := rollbackDeleteStages(staged)
			if rollbackErr != nil {
				return model.DuplicateActionResult{}, errors.Join(
					fmt.Errorf("stage duplicate for deletion %q: %w", source, err),
					rollbackErr,
				)
			}
			return model.DuplicateActionResult{}, fmt.Errorf("stage duplicate for deletion %q: %w", source, err)
		}
		staged = append(staged, duplicateMove{track: track, source: source, target: temp})
	}

	if err := store.DeleteTracksByID(ctx, ids); err != nil {
		rollbackErr := rollbackDeleteStages(staged)
		if rollbackErr != nil {
			return model.DuplicateActionResult{}, errors.Join(
				fmt.Errorf("delete duplicate index rows: %w", err),
				rollbackErr,
			)
		}
		return model.DuplicateActionResult{}, fmt.Errorf("delete duplicate index rows: %w", err)
	}

	result := model.DuplicateActionResult{
		Action:    "delete",
		Requested: len(ids),
		Completed: len(ids),
		Paths:     make([]string, 0),
		Errors:    make([]string, 0),
	}

	for _, move := range staged {
		if err := os.Remove(move.target); err != nil {
			result.Completed--
			result.Failed++
			result.Paths = append(result.Paths, move.target)
			result.Errors = append(result.Errors, fmt.Sprintf("remove %q: %v", move.target, err))
		}
	}

	return result, nil
}

func normalizeDuplicateActionIDs(trackIDs []int64) ([]int64, error) {
	if len(trackIDs) == 0 {
		return nil, errors.New("select at least one duplicate track")
	}
	if len(trackIDs) > duplicateActionLimit {
		return nil, fmt.Errorf("too many duplicate tracks selected: %d (maximum %d)", len(trackIDs), duplicateActionLimit)
	}

	seen := make(map[int64]struct{}, len(trackIDs))
	ids := make([]int64, 0, len(trackIDs))
	for _, id := range trackIDs {
		if id <= 0 {
			return nil, fmt.Errorf("invalid track id: %d", id)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, errors.New("select at least one duplicate track")
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func loadDuplicateActionTracks(
	ctx context.Context,
	store duplicateActionStore,
	ids []int64,
) ([]model.Track, error) {
	tracks := make([]model.Track, 0, len(ids))
	for _, id := range ids {
		track, err := store.TrackByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := validateSourceFile(track.Path); err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}
	return tracks, nil
}

func validateDuplicateActionSelection(
	ctx context.Context,
	store duplicateActionStore,
	ids []int64,
) error {
	allTracks, err := store.AllTracks(ctx)
	if err != nil {
		return fmt.Errorf("load duplicate groups before action: %w", err)
	}
	groups := FindDuplicates(allTracks, 2_000)
	selected := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		selected[id] = struct{}{}
	}

	matched := make(map[int64]struct{}, len(ids))
	for _, group := range groups {
		selectedInGroup := 0
		for _, track := range group.Tracks {
			if _, ok := selected[track.ID]; ok {
				selectedInGroup++
				matched[track.ID] = struct{}{}
			}
		}
		if selectedInGroup == len(group.Tracks) && selectedInGroup > 0 {
			return fmt.Errorf(
				"duplicate action would remove every file from group %q — keep at least one track",
				group.Artist+" — "+group.Title,
			)
		}
	}

	if len(matched) != len(ids) {
		return errors.New("duplicate action contains a track that is no longer in a duplicate group; recalculate duplicates and retry")
	}
	return nil
}

func validateQuarantineOutsideLibrary(
	ctx context.Context,
	store duplicateActionStore,
	quarantineRoot string,
) error {
	roots, err := store.ListLibraryRoots(ctx)
	if err != nil {
		return fmt.Errorf("load library roots: %w", err)
	}
	for _, root := range roots {
		inside, err := pathInside(root.Path, quarantineRoot)
		if err != nil {
			return err
		}
		if inside {
			return fmt.Errorf(
				"quarantine folder %q is inside managed library root %q; choose a folder outside the library",
				quarantineRoot,
				root.Path,
			)
		}
	}
	return nil
}

func planQuarantineMoves(tracks []model.Track, root string) ([]duplicateMove, error) {
	reserved := make(map[string]struct{}, len(tracks))
	moves := make([]duplicateMove, 0, len(tracks))

	for _, track := range tracks {
		source, err := filepath.Abs(filepath.Clean(track.Path))
		if err != nil {
			return nil, fmt.Errorf("resolve source path %q: %w", track.Path, err)
		}
		if err := validateSourceFile(source); err != nil {
			return nil, err
		}
		target, err := uniqueQuarantineTarget(root, track.FileName, track.ID, reserved)
		if err != nil {
			return nil, err
		}
		moves = append(moves, duplicateMove{track: track, source: source, target: target})
	}
	return moves, nil
}

func uniqueQuarantineTarget(root, fileName string, trackID int64, reserved map[string]struct{}) (string, error) {
	fileName = strings.TrimSpace(filepath.Base(fileName))
	if fileName == "" || fileName == "." || fileName == string(filepath.Separator) {
		return "", fmt.Errorf("track %d has invalid file name", trackID)
	}

	ext := filepath.Ext(fileName)
	stem := strings.TrimSuffix(fileName, ext)

	for attempt := 0; attempt < 10_000; attempt++ {
		name := fileName
		if attempt == 1 {
			name = fmt.Sprintf("%s (CCML %d)%s", stem, trackID, ext)
		} else if attempt > 1 {
			name = fmt.Sprintf("%s (CCML %d-%d)%s", stem, trackID, attempt, ext)
		}

		target := filepath.Join(root, name)
		key := strings.ToLower(filepath.Clean(target))
		if _, exists := reserved[key]; exists {
			continue
		}
		if _, err := os.Stat(target); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("check quarantine target %q: %w", target, err)
		}
		reserved[key] = struct{}{}
		return target, nil
	}

	return "", fmt.Errorf("could not allocate a unique quarantine filename for track %d", trackID)
}

func uniqueDeleteStagePath(source string, trackID int64, reserved map[string]struct{}) (string, error) {
	dir := filepath.Dir(source)
	ext := filepath.Ext(source)
	for attempt := 0; attempt < 10_000; attempt++ {
		suffix := fmt.Sprintf(".ccml-delete-%d", trackID)
		if attempt > 0 {
			suffix += fmt.Sprintf("-%d", attempt)
		}
		target := filepath.Join(dir, suffix+ext+".tmp")
		key := strings.ToLower(filepath.Clean(target))
		if _, exists := reserved[key]; exists {
			continue
		}
		if _, err := os.Stat(target); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("check delete staging path %q: %w", target, err)
		}
		reserved[key] = struct{}{}
		return target, nil
	}
	return "", fmt.Errorf("could not allocate delete staging path for track %d", trackID)
}

func validateSourceFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("access duplicate source %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("duplicate source is not a regular file: %s", path)
	}
	return nil
}

func rollbackDuplicateMoves(moves []duplicateMove) error {
	var result error
	for i := len(moves) - 1; i >= 0; i-- {
		move := moves[i]
		if err := moveFileExclusive(move.target, move.source); err != nil {
			result = errors.Join(result, fmt.Errorf("rollback %q to %q: %w", move.target, move.source, err))
		}
	}
	return result
}

func rollbackDeleteStages(moves []duplicateMove) error {
	var result error
	for i := len(moves) - 1; i >= 0; i-- {
		move := moves[i]
		if err := os.Rename(move.target, move.source); err != nil {
			result = errors.Join(result, fmt.Errorf("restore staged file %q to %q: %w", move.target, move.source, err))
		}
	}
	return result
}

func pathInside(root, target string) (bool, error) {
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return false, fmt.Errorf("resolve library root %q: %w", root, err)
	}
	targetAbs, err := filepath.Abs(filepath.Clean(target))
	if err != nil {
		return false, fmt.Errorf("resolve target %q: %w", target, err)
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return false, fmt.Errorf("compare %q and %q: %w", rootAbs, targetAbs, err)
	}
	if rel == "." {
		return true, nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return false, nil
	}
	return true, nil
}

func moveFileExclusive(source, target string) error {
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("target already exists: %s", target)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check target %q: %w", target, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	if err := os.Rename(source, target); err == nil {
		return nil
	}

	if err := copyFileExclusive(source, target); err != nil {
		return fmt.Errorf("move %q to %q: %w", source, target, err)
	}
	if err := os.Remove(source); err != nil {
		removeTargetErr := os.Remove(target)
		if removeTargetErr != nil && !errors.Is(removeTargetErr, os.ErrNotExist) {
			return errors.Join(
				fmt.Errorf("remove source after copy: %w", err),
				fmt.Errorf("remove copied target after failed move: %w", removeTargetErr),
			)
		}
		return fmt.Errorf("remove source after copy: %w", err)
	}
	return nil
}

func copyFileExclusive(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	info, err := in.Stat()
	if err != nil {
		_ = in.Close()
		return fmt.Errorf("stat source: %w", err)
	}

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		_ = in.Close()
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
