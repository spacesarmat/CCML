// Package tagging implements safe, reversible audio metadata editing.
package tagging

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tommyo123/mtag"

	"github.com/spacesarmat/CCML/internal/model"
	"github.com/spacesarmat/CCML/internal/store"
)

const (
	maxBatchTracks = 1000
	maxCoverBytes  = 20 << 20
)

// Service reads and writes audio tags and keeps reversible history.
type Service struct {
	store      *store.Store
	historyDir string
	httpClient *http.Client
}

// NewService creates a tag editor and its private history directory.
func NewService(db *store.Store, appDir string) (*Service, error) {
	historyDir := filepath.Join(appDir, "tag-history")
	if err := os.MkdirAll(historyDir, 0o700); err != nil {
		return nil, fmt.Errorf("create tag-history directory: %w", err)
	}
	return &Service{
		store:      db,
		historyDir: historyDir,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Read returns the current editable tag state directly from the audio file.
func (s *Service) Read(ctx context.Context, trackID int64) (model.TagSnapshot, error) {
	track, err := s.store.TrackByID(ctx, trackID)
	if err != nil {
		return model.TagSnapshot{}, err
	}
	state, _, err := readState(track.Path, false)
	if err != nil {
		return model.TagSnapshot{}, err
	}
	return state, nil
}

// Preview calculates before/after values without modifying files.
func (s *Service) Preview(ctx context.Context, trackIDs []int64, patch model.TagPatch) ([]model.TagPreview, error) {
	ids, err := validateRequest(trackIDs, patch)
	if err != nil {
		return nil, err
	}

	previews := make([]model.TagPreview, 0, len(ids))
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		track, err := s.store.TrackByID(ctx, id)
		if err != nil {
			return nil, err
		}
		before, _, err := readState(track.Path, false)
		if err != nil {
			return nil, err
		}
		after := applyPatch(before, patch)
		previews = append(previews, model.TagPreview{
			TrackID: id,
			Path:    track.Path,
			Before:  before,
			After:   after,
		})
	}
	return previews, nil
}

// Apply writes a partial tag patch to one or more files and records one undoable change set.
func (s *Service) Apply(ctx context.Context, trackIDs []int64, patch model.TagPatch) (model.TagApplyResult, error) {
	ids, err := validateRequest(trackIDs, patch)
	if err != nil {
		return model.TagApplyResult{}, err
	}
	return s.apply(ctx, ids, patch, coverMutation{}, "tags.edit")
}

// SetCoverArt embeds the same JPEG/PNG cover into all selected tracks.
func (s *Service) SetCoverArt(ctx context.Context, trackIDs []int64, imagePath string) (model.TagApplyResult, error) {
	ids, err := validateIDs(trackIDs)
	if err != nil {
		return model.TagApplyResult{}, err
	}
	data, mime, err := readCoverFile(imagePath)
	if err != nil {
		return model.TagApplyResult{}, err
	}
	return s.apply(ctx, ids, model.TagPatch{}, coverMutation{mode: coverSet, mime: mime, data: data}, "tags.cover")
}

// RemoveCoverArt removes embedded pictures from all selected tracks.
func (s *Service) RemoveCoverArt(ctx context.Context, trackIDs []int64) (model.TagApplyResult, error) {
	ids, err := validateIDs(trackIDs)
	if err != nil {
		return model.TagApplyResult{}, err
	}
	return s.apply(ctx, ids, model.TagPatch{}, coverMutation{mode: coverRemove}, "tags.removeCover")
}

// ApplyMetadataCandidate writes a selected provider candidate and optionally its artwork.
func (s *Service) ApplyMetadataCandidate(ctx context.Context, trackID int64, candidate model.MetadataCandidate, includeArtwork bool) (model.TagApplyResult, error) {
	return s.ApplyMetadataCandidateWithPolicy(ctx, trackID, candidate, includeArtwork, false)
}

// ApplyMetadataCandidateWithPolicy applies provider metadata and can restrict writes to currently empty fields.
func (s *Service) ApplyMetadataCandidateWithPolicy(ctx context.Context, trackID int64, candidate model.MetadataCandidate, includeArtwork, onlyMissing bool) (model.TagApplyResult, error) {
	patch := patchFromCandidate(candidate)
	if onlyMissing {
		track, err := s.store.TrackByID(ctx, trackID)
		if err != nil {
			return model.TagApplyResult{}, err
		}
		before, _, err := readState(track.Path, false)
		if err != nil {
			return model.TagApplyResult{}, err
		}
		patch = filterMissingPatch(before, patch)
	}
	var cover coverMutation
	if includeArtwork && candidate.ArtworkEmbeddable && strings.TrimSpace(candidate.ArtworkURL) != "" {
		data, mime, err := s.downloadCover(ctx, candidate.ArtworkURL)
		if err != nil {
			return model.TagApplyResult{}, err
		}
		cover = coverMutation{mode: coverSet, mime: mime, data: data}
	}
	if len(patch.Fields) == 0 && cover.mode == coverKeep {
		return model.TagApplyResult{}, errors.New("metadata candidate has no applicable fields")
	}
	return s.apply(ctx, []int64{trackID}, patch, cover, "tags.metadata")
}

// Undo restores every track from a previously applied change set.
func (s *Service) Undo(ctx context.Context, changeSetID int64) (model.TagApplyResult, error) {
	items, err := s.store.TagChangeItems(ctx, changeSetID)
	if err != nil {
		return model.TagApplyResult{}, err
	}
	result := model.TagApplyResult{ChangeSetID: changeSetID}
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		track, err := s.store.TrackByID(ctx, item.TrackID)
		if err != nil {
			result.Failed++
			result.Errors = appendLimited(result.Errors, err.Error())
			continue
		}

		cover := coverMutation{}
		if item.CoverChanged {
			if item.Before.CoverSize == 0 {
				cover.mode = coverRemove
			} else {
				data, err := os.ReadFile(item.BeforeCoverPath)
				if err != nil {
					result.Failed++
					result.Errors = appendLimited(result.Errors, fmt.Sprintf("restore cover for %q: %v", track.Path, err))
					continue
				}
				cover = coverMutation{mode: coverSet, mime: item.Before.CoverMIME, data: data}
			}
		}

		if err := writeState(ctx, track.Path, item.Before, cover); err != nil {
			result.Failed++
			result.Errors = appendLimited(result.Errors, err.Error())
			continue
		}
		info, err := os.Stat(track.Path)
		if err != nil {
			result.Failed++
			result.Errors = appendLimited(result.Errors, fmt.Sprintf("stat restored track %q: %v", track.Path, err))
			continue
		}
		if err := s.store.UpdateTrackTags(ctx, track.ID, item.Before, info.Size(), info.ModTime().Unix()); err != nil {
			result.Failed++
			result.Errors = appendLimited(result.Errors, err.Error())
			continue
		}
		if item.CoverChanged {
			if hasCover, coverErr := hasAnyEmbeddedArtwork(track.Path); coverErr == nil {
				if err := s.store.UpdateTrackCoverPresence(ctx, track.ID, hasCover); err != nil {
					_ = s.store.InvalidateTrackCoverPresence(context.Background(), track.ID)
				}
			} else {
				_ = s.store.InvalidateTrackCoverPresence(context.Background(), track.ID)
			}
		}
		result.Changed++
	}
	if result.Failed == 0 {
		if err := s.store.MarkTagChangeUndone(ctx, changeSetID); err != nil {
			return result, err
		}
	}
	return result, nil
}

// History returns recent metadata change sets.
func (s *Service) History(ctx context.Context, limit int) ([]model.TagHistory, error) {
	return s.store.ListTagHistory(ctx, limit)
}

type coverMode int

const (
	coverKeep coverMode = iota
	coverSet
	coverRemove
)

type coverMutation struct {
	mode coverMode
	mime string
	data []byte
}

func (s *Service) apply(ctx context.Context, ids []int64, patch model.TagPatch, cover coverMutation, label string) (model.TagApplyResult, error) {
	changeSetID, err := s.store.BeginTagChange(ctx, label)
	if err != nil {
		return model.TagApplyResult{}, err
	}
	result := model.TagApplyResult{ChangeSetID: changeSetID}

	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			if finishErr := s.store.FinishTagChange(context.Background(), changeSetID, result.Changed, "applied"); finishErr != nil {
				return result, errors.Join(err, finishErr)
			}
			return result, err
		}
		changed, applyErr := s.applyOne(ctx, changeSetID, id, patch, cover)
		if applyErr != nil {
			result.Failed++
			result.Errors = appendLimited(result.Errors, applyErr.Error())
			continue
		}
		if changed {
			result.Changed++
		}
	}

	if err := s.store.FinishTagChange(ctx, changeSetID, result.Changed, "applied"); err != nil {
		return result, err
	}
	if result.Changed == 0 {
		result.ChangeSetID = 0
	}
	return result, nil
}

func (s *Service) applyOne(ctx context.Context, changeSetID, trackID int64, patch model.TagPatch, cover coverMutation) (bool, error) {
	track, err := s.store.TrackByID(ctx, trackID)
	if err != nil {
		return false, err
	}
	before, beforeCover, err := readState(track.Path, cover.mode != coverKeep)
	if err != nil {
		return false, err
	}
	after := applyPatch(before, patch)
	coverChanged := false
	if cover.mode == coverSet {
		coverChanged = before.CoverMIME != cover.mime || !bytes.Equal(beforeCover.data, cover.data)
		after.CoverMIME = cover.mime
		after.CoverSize = len(cover.data)
	} else if cover.mode == coverRemove {
		coverChanged = before.CoverSize > 0
		after.CoverMIME = ""
		after.CoverSize = 0
	}
	if sameSnapshot(before, after) && !coverChanged {
		return false, nil
	}

	beforeCoverPath := ""
	if coverChanged && len(beforeCover.data) > 0 {
		beforeCoverPath, err = s.backupCover(changeSetID, trackID, beforeCover.data)
		if err != nil {
			return false, err
		}
	}

	if err := writeState(ctx, track.Path, after, cover); err != nil {
		return false, err
	}
	info, err := os.Stat(track.Path)
	if err != nil {
		rollbackErr := writeState(context.Background(), track.Path, before, restoreCover(before, beforeCover))
		return false, errors.Join(fmt.Errorf("stat updated track %q: %w", track.Path, err), rollbackErr)
	}
	if err := s.store.UpdateTrackTags(ctx, track.ID, after, info.Size(), info.ModTime().Unix()); err != nil {
		rollbackErr := writeState(context.Background(), track.Path, before, restoreCover(before, beforeCover))
		return false, errors.Join(err, rollbackErr)
	}
	if err := s.store.AddTagChangeItem(ctx, changeSetID, track.ID, before, after, beforeCoverPath, coverChanged); err != nil {
		rollbackErr := writeState(context.Background(), track.Path, before, restoreCover(before, beforeCover))
		if rollbackErr == nil {
			if info, statErr := os.Stat(track.Path); statErr == nil {
				rollbackErr = s.store.UpdateTrackTags(context.Background(), track.ID, before, info.Size(), info.ModTime().Unix())
			} else {
				rollbackErr = statErr
			}
		}
		return false, errors.Join(err, rollbackErr)
	}
	if coverChanged {
		if hasCover, coverErr := hasAnyEmbeddedArtwork(track.Path); coverErr == nil {
			if err := s.store.UpdateTrackCoverPresence(ctx, track.ID, hasCover); err != nil {
				_ = s.store.InvalidateTrackCoverPresence(context.Background(), track.ID)
			}
		} else {
			_ = s.store.InvalidateTrackCoverPresence(context.Background(), track.ID)
		}
	}
	return true, nil
}

type coverData struct {
	mime string
	data []byte
}

func readState(path string, includeCover bool) (state model.TagSnapshot, cover coverData, resultErr error) {
	f, err := mtag.Open(path)
	if err != nil {
		return model.TagSnapshot{}, coverData{}, fmt.Errorf("open tags %q: %w", path, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close tags %q: %w", path, err))
		}
	}()

	state = snapshotFromFile(f)
	if includeCover {
		cover = frontCover(f.Images())
	}
	return state, cover, nil
}

func snapshotFromFile(f *mtag.File) model.TagSnapshot {
	state := model.TagSnapshot{
		Title:         f.Title(),
		Artist:        f.Artist(),
		Album:         f.Album(),
		AlbumArtist:   f.AlbumArtist(),
		Genre:         f.Genre(),
		Composer:      f.Composer(),
		Comment:       f.Comment(),
		Label:         f.Publisher(),
		CatalogNumber: f.CustomValue("CATALOGNUMBER"),
		ISRC:          f.CustomValue("ISRC"),
		ReleaseDate:   f.CustomValue("DATE"),
		Year:          f.Year(),
		TrackNumber:   f.Track(),
		TrackTotal:    f.TrackTotal(),
		DiscNumber:    f.Disc(),
		DiscTotal:     f.DiscTotal(),
	}
	for _, image := range f.ImageSummaries() {
		if image.Type == mtag.PictureCoverFront {
			state.CoverMIME = image.MIME
			state.CoverSize = image.Size
			break
		}
	}
	return state
}

func frontCover(images []mtag.Picture) coverData {
	for _, image := range images {
		if image.Type == mtag.PictureCoverFront {
			return coverData{mime: image.MIME, data: append([]byte(nil), image.Data...)}
		}
	}
	return coverData{}
}

func hasAnyEmbeddedArtwork(path string) (result bool, resultErr error) {
	f, err := mtag.Open(path)
	if err != nil {
		return false, fmt.Errorf("open tags %q for cover index: %w", path, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close tags %q after cover index: %w", path, err))
		}
	}()

	for _, image := range f.ImageSummaries() {
		if image.Size > 0 {
			return true, nil
		}
	}
	return false, nil
}

func writeState(ctx context.Context, path string, state model.TagSnapshot, cover coverMutation) (resultErr error) {
	f, err := mtag.Open(path)
	if err != nil {
		return fmt.Errorf("open tags %q: %w", path, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close tags %q: %w", path, err))
		}
	}()

	f.SetTitle(state.Title)
	f.SetArtist(state.Artist)
	f.SetAlbum(state.Album)
	f.SetAlbumArtist(state.AlbumArtist)
	f.SetGenre(state.Genre)
	f.SetComposer(state.Composer)
	f.SetComment(state.Comment)
	f.SetPublisher(state.Label)
	f.SetCustomValues("CATALOGNUMBER", singleOrNoneString(state.CatalogNumber)...)
	f.SetCustomValues("ISRC", singleOrNoneString(state.ISRC)...)
	// Set the year before the full release date. On Vorbis-based formats
	// (FLAC/OGG), SetYear writes DATE; writing DATE afterwards preserves
	// the more precise YYYY-MM-DD value when one is available.
	f.SetYear(state.Year)
	f.SetCustomValues("DATE", singleOrNoneString(state.ReleaseDate)...)
	f.SetTrack(state.TrackNumber, state.TrackTotal)
	f.SetDisc(state.DiscNumber, state.DiscTotal)

	switch cover.mode {
	case coverSet:
		f.SetCoverArt(cover.mime, cover.data)
	case coverRemove:
		removeFrontCover(f)
	}

	if err := f.SaveContext(ctx); err != nil {
		return fmt.Errorf("save tags %q: %w", path, err)
	}
	return nil
}

func removeFrontCover(f *mtag.File) {
	images := f.Images()
	f.RemoveImages()
	for _, image := range images {
		if image.Type == mtag.PictureCoverFront {
			continue
		}
		f.AddImage(image)
	}
}

func restoreCover(before model.TagSnapshot, cover coverData) coverMutation {
	if before.CoverSize == 0 {
		return coverMutation{mode: coverRemove}
	}
	if len(cover.data) == 0 {
		return coverMutation{}
	}
	return coverMutation{mode: coverSet, mime: before.CoverMIME, data: cover.data}
}

func (s *Service) backupCover(changeSetID, trackID int64, data []byte) (string, error) {
	dir := filepath.Join(s.historyDir, fmt.Sprintf("%d", changeSetID))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create cover history directory: %w", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%d.cover", trackID))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("backup cover art: %w", err)
	}
	return path, nil
}

func patchFromCandidate(candidate model.MetadataCandidate) model.TagPatch {
	patch := model.TagPatch{}
	addString := func(field, value string, target *string) {
		if value = strings.TrimSpace(value); value != "" {
			patch.Fields = append(patch.Fields, field)
			*target = value
		}
	}
	addInt := func(field string, value int, target *int) {
		if value > 0 {
			patch.Fields = append(patch.Fields, field)
			*target = value
		}
	}
	addString("title", candidate.Title, &patch.Title)
	addString("artist", candidate.Artist, &patch.Artist)
	addString("album", candidate.Album, &patch.Album)
	addString("albumArtist", candidate.AlbumArtist, &patch.AlbumArtist)
	addString("genre", candidate.Genre, &patch.Genre)
	addString("label", candidate.Label, &patch.Label)
	addString("catalogNumber", candidate.CatalogNumber, &patch.CatalogNumber)
	addString("isrc", candidate.ISRC, &patch.ISRC)
	addString("releaseDate", candidate.ReleaseDate, &patch.ReleaseDate)
	addInt("year", candidate.Year, &patch.Year)
	addInt("trackNumber", candidate.TrackNumber, &patch.TrackNumber)
	addInt("trackTotal", candidate.TrackTotal, &patch.TrackTotal)
	addInt("discNumber", candidate.DiscNumber, &patch.DiscNumber)
	addInt("discTotal", candidate.DiscTotal, &patch.DiscTotal)
	return patch
}

func filterMissingPatch(before model.TagSnapshot, patch model.TagPatch) model.TagPatch {
	keep := make([]string, 0, len(patch.Fields))
	for _, field := range patch.Fields {
		empty := false
		switch field {
		case "title":
			empty = strings.TrimSpace(before.Title) == ""
		case "artist":
			empty = strings.TrimSpace(before.Artist) == ""
		case "album":
			empty = strings.TrimSpace(before.Album) == ""
		case "albumArtist":
			empty = strings.TrimSpace(before.AlbumArtist) == ""
		case "genre":
			empty = strings.TrimSpace(before.Genre) == ""
		case "label":
			empty = strings.TrimSpace(before.Label) == ""
		case "catalogNumber":
			empty = strings.TrimSpace(before.CatalogNumber) == ""
		case "isrc":
			empty = strings.TrimSpace(before.ISRC) == ""
		case "releaseDate":
			empty = strings.TrimSpace(before.ReleaseDate) == ""
		case "year":
			empty = before.Year == 0
		case "trackNumber":
			empty = before.TrackNumber == 0
		case "trackTotal":
			empty = before.TrackTotal == 0
		case "discNumber":
			empty = before.DiscNumber == 0
		case "discTotal":
			empty = before.DiscTotal == 0
		default:
			empty = true
		}
		if empty {
			keep = append(keep, field)
		}
	}
	patch.Fields = keep
	return patch
}

func applyPatch(before model.TagSnapshot, patch model.TagPatch) model.TagSnapshot {
	after := before
	for _, field := range patch.Fields {
		switch field {
		case "title":
			after.Title = patch.Title
		case "artist":
			after.Artist = patch.Artist
		case "album":
			after.Album = patch.Album
		case "albumArtist":
			after.AlbumArtist = patch.AlbumArtist
		case "genre":
			after.Genre = patch.Genre
		case "composer":
			after.Composer = patch.Composer
		case "comment":
			after.Comment = patch.Comment
		case "label":
			after.Label = patch.Label
		case "catalogNumber":
			after.CatalogNumber = patch.CatalogNumber
		case "isrc":
			after.ISRC = patch.ISRC
		case "releaseDate":
			after.ReleaseDate = patch.ReleaseDate
		case "year":
			after.Year = patch.Year
		case "trackNumber":
			after.TrackNumber = patch.TrackNumber
		case "trackTotal":
			after.TrackTotal = patch.TrackTotal
		case "discNumber":
			after.DiscNumber = patch.DiscNumber
		case "discTotal":
			after.DiscTotal = patch.DiscTotal
		}
	}
	return after
}

func sameSnapshot(a, b model.TagSnapshot) bool {
	return a == b
}

func validateRequest(trackIDs []int64, patch model.TagPatch) ([]int64, error) {
	ids, err := validateIDs(trackIDs)
	if err != nil {
		return nil, err
	}
	if len(patch.Fields) == 0 {
		return nil, errors.New("at least one tag field must be selected")
	}
	allowed := map[string]struct{}{
		"title": {}, "artist": {}, "album": {}, "albumArtist": {}, "genre": {}, "composer": {}, "comment": {},
		"label": {}, "catalogNumber": {}, "isrc": {}, "releaseDate": {},
		"year": {}, "trackNumber": {}, "trackTotal": {}, "discNumber": {}, "discTotal": {},
	}
	seen := make(map[string]struct{}, len(patch.Fields))
	for _, field := range patch.Fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("unsupported tag field %q", field)
		}
		if _, duplicate := seen[field]; duplicate {
			return nil, fmt.Errorf("duplicate tag field %q", field)
		}
		seen[field] = struct{}{}
	}
	if _, ok := seen["year"]; ok && patch.Year != 0 && (patch.Year < 1000 || patch.Year > 9999) {
		return nil, fmt.Errorf("year must be 0 or between 1000 and 9999")
	}
	for name, value := range map[string]int{
		"track number": patch.TrackNumber,
		"track total":  patch.TrackTotal,
		"disc number":  patch.DiscNumber,
		"disc total":   patch.DiscTotal,
	} {
		if value < 0 || value > 9999 {
			return nil, fmt.Errorf("%s must be between 0 and 9999", name)
		}
	}
	return ids, nil
}

func validateIDs(trackIDs []int64) ([]int64, error) {
	if len(trackIDs) == 0 {
		return nil, errors.New("no tracks selected")
	}
	if len(trackIDs) > maxBatchTracks {
		return nil, fmt.Errorf("too many tracks selected: %d (max %d)", len(trackIDs), maxBatchTracks)
	}
	seen := make(map[int64]struct{}, len(trackIDs))
	ids := make([]int64, 0, len(trackIDs))
	for _, id := range trackIDs {
		if id <= 0 {
			return nil, fmt.Errorf("invalid track id %d", id)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func readCoverFile(path string) ([]byte, string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, "", errors.New("cover image path is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("open cover image: %w", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxCoverBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		if closeErr != nil {
			return nil, "", errors.Join(fmt.Errorf("read cover image: %w", readErr), fmt.Errorf("close cover image: %w", closeErr))
		}
		return nil, "", fmt.Errorf("read cover image: %w", readErr)
	}
	if closeErr != nil {
		return nil, "", fmt.Errorf("close cover image: %w", closeErr)
	}
	if len(data) > maxCoverBytes {
		return nil, "", fmt.Errorf("cover image exceeds %d MiB", maxCoverBytes>>20)
	}
	mime, err := supportedImageMIME(data)
	if err != nil {
		return nil, "", err
	}
	return data, mime, nil
}

func (s *Service) downloadCover(ctx context.Context, rawURL string) ([]byte, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, "", fmt.Errorf("parse artwork URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, "", fmt.Errorf("unsupported artwork URL scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return nil, "", errors.New("artwork URL has no host")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("create artwork request: %w", err)
	}
	req.Header.Set("User-Agent", "CCML/0.4 (https://github.com/spacesarmat/CCML)")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download artwork: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		closeErr := resp.Body.Close()
		httpErr := fmt.Errorf("download artwork: HTTP %s", resp.Status)
		if closeErr != nil {
			return nil, "", errors.Join(httpErr, fmt.Errorf("close artwork response: %w", closeErr))
		}
		return nil, "", httpErr
	}
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxCoverBytes+1))
	closeErr := resp.Body.Close()
	if readErr != nil {
		if closeErr != nil {
			return nil, "", errors.Join(fmt.Errorf("read artwork response: %w", readErr), fmt.Errorf("close artwork response: %w", closeErr))
		}
		return nil, "", fmt.Errorf("read artwork response: %w", readErr)
	}
	if closeErr != nil {
		return nil, "", fmt.Errorf("close artwork response: %w", closeErr)
	}
	if len(data) > maxCoverBytes {
		return nil, "", fmt.Errorf("artwork exceeds %d MiB", maxCoverBytes>>20)
	}
	mime, err := supportedImageMIME(data)
	if err != nil {
		return nil, "", err
	}
	return data, mime, nil
}

func supportedImageMIME(data []byte) (string, error) {
	if len(data) == 0 {
		return "", errors.New("cover image is empty")
	}
	mime := http.DetectContentType(data)
	switch mime {
	case "image/jpeg", "image/png":
		return mime, nil
	default:
		return "", fmt.Errorf("unsupported cover image type %q; use JPEG or PNG", mime)
	}
}

func singleOrNoneString(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return []string{value}
}

func appendLimited(values []string, value string) []string {
	if len(values) >= 100 {
		return values
	}
	return append(values, value)
}
