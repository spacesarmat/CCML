package tagging

import (
	"context"
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

// ApplyBPMKeyAnalysis writes an Essentia BPM/key result through the normal
// reversible tag-change path. It returns a zero Changed count when the requested
// values are already present or OnlyMissing leaves nothing to write.
func (s *Service) ApplyBPMKeyAnalysis(ctx context.Context, trackID int64, result model.BPMKey, onlyMissing bool) (model.TagApplyResult, error) {
	patch := bpmKeyAnalysisPatch(result)
	if len(patch.Fields) == 0 {
		return model.TagApplyResult{}, nil
	}

	track, err := s.store.TrackByID(ctx, trackID)
	if err != nil {
		return model.TagApplyResult{}, err
	}
	before, _, err := readState(track.Path, false)
	if err != nil {
		return model.TagApplyResult{}, err
	}
	if onlyMissing {
		patch = filterMissingPatch(before, patch)
	}
	if len(patch.Fields) == 0 || sameSnapshot(before, applyPatch(before, patch)) {
		return model.TagApplyResult{}, nil
	}
	return s.apply(ctx, []int64{trackID}, patch, coverMutation{}, "tags.essentia")
}

func bpmKeyAnalysisPatch(result model.BPMKey) model.TagPatch {
	patch := model.TagPatch{}
	if result.BPM >= 20 && result.BPM <= 400 {
		patch.BPM = result.BPM
		patch.Fields = append(patch.Fields, "bpm")
	}
	if key := strings.TrimSpace(result.Key); key != "" {
		patch.Key = key
		patch.Fields = append(patch.Fields, "key")
	}
	if scale := strings.TrimSpace(result.Scale); scale != "" {
		patch.KeyScale = scale
		patch.Fields = append(patch.Fields, "keyScale")
	}
	return patch
}
