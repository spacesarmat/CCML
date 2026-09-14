package library

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/spacesarmat/CCML/internal/model"
)

const duplicateVerifyTrackLimit = 12

type duplicateVerifyStore interface {
	AllTracks(ctx context.Context) ([]model.Track, error)
}

type duplicateAudioComparator interface {
	Compare(ctx context.Context, tracks []model.Track, referenceTrackID int64) (model.DuplicateAudioVerification, error)
}

// VerifyDuplicateAudio verifies the exact current members of one duplicate
// group against decoded audio features.
//
// Requiring the whole current group prevents stale frontend data from silently
// checking a subset after the library has changed.
func VerifyDuplicateAudio(
	ctx context.Context,
	store duplicateVerifyStore,
	comparator duplicateAudioComparator,
	trackIDs []int64,
) (model.DuplicateAudioVerification, error) {
	ids, err := normalizeDuplicateVerifyIDs(trackIDs)
	if err != nil {
		return model.DuplicateAudioVerification{}, err
	}
	if comparator == nil {
		return model.DuplicateAudioVerification{}, errors.New("audio comparator is not available")
	}

	allTracks, err := store.AllTracks(ctx)
	if err != nil {
		return model.DuplicateAudioVerification{}, fmt.Errorf("load library before audio verification: %w", err)
	}

	groups := FindDuplicates(allTracks, 2_000)
	group, ok := exactDuplicateGroupForIDs(groups, ids)
	if !ok {
		return model.DuplicateAudioVerification{}, errors.New(
			"duplicate group changed or no longer exists; recalculate duplicates and retry",
		)
	}

	referenceID := group.RecommendedTrackID
	if referenceID <= 0 {
		referenceID = group.Tracks[0].ID
		for _, track := range group.Tracks[1:] {
			if track.ID < referenceID {
				referenceID = track.ID
			}
		}
	}

	result, err := comparator.Compare(ctx, group.Tracks, referenceID)
	if err != nil {
		return model.DuplicateAudioVerification{}, err
	}
	result.GroupKey = group.Key
	return result, nil
}

func normalizeDuplicateVerifyIDs(trackIDs []int64) ([]int64, error) {
	if len(trackIDs) < 2 {
		return nil, errors.New("audio verification requires at least two tracks")
	}
	if len(trackIDs) > duplicateVerifyTrackLimit {
		return nil, fmt.Errorf(
			"audio verification supports at most %d tracks per group",
			duplicateVerifyTrackLimit,
		)
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
	if len(ids) < 2 {
		return nil, errors.New("audio verification requires at least two unique tracks")
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func exactDuplicateGroupForIDs(groups []model.DuplicateGroup, ids []int64) (model.DuplicateGroup, bool) {
	for _, group := range groups {
		if len(group.Tracks) != len(ids) {
			continue
		}
		groupIDs := make([]int64, 0, len(group.Tracks))
		for _, track := range group.Tracks {
			groupIDs = append(groupIDs, track.ID)
		}
		sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })

		matches := true
		for i := range ids {
			if ids[i] != groupIDs[i] {
				matches = false
				break
			}
		}
		if matches {
			return group, true
		}
	}
	return model.DuplicateGroup{}, false
}
