package tagging

import (
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestPatchFromCandidateCarriesDJMetadata(t *testing.T) {
	t.Parallel()

	patch := patchFromCandidate(model.MetadataCandidate{
		Title:    "Track",
		Artist:   "Artist",
		BPM:      126.5,
		Key:      "6A",
		KeyScale: "camelot",
	})

	if patch.BPM != 126.5 || patch.Key != "6A" || patch.KeyScale != "camelot" {
		t.Fatalf("DJ fields missing from patch: %+v", patch)
	}
	want := map[string]bool{"bpm": false, "key": false, "keyScale": false}
	for _, field := range patch.Fields {
		if _, ok := want[field]; ok {
			want[field] = true
		}
	}
	for field, present := range want {
		if !present {
			t.Fatalf("field %q was not selected in patch: %+v", field, patch.Fields)
		}
	}
}

func TestApplyPatchAndMissingPolicySupportDJMetadata(t *testing.T) {
	t.Parallel()

	patch := model.TagPatch{
		Fields:   []string{"bpm", "key", "keyScale"},
		BPM:      124,
		Key:      "8A",
		KeyScale: "camelot",
	}
	before := model.TagSnapshot{}
	filtered := filterMissingPatch(before, patch)
	if len(filtered.Fields) != 3 {
		t.Fatalf("missing-field filter removed DJ fields: %+v", filtered.Fields)
	}

	after := applyPatch(before, filtered)
	if after.BPM != 124 || after.Key != "8A" || after.KeyScale != "camelot" {
		t.Fatalf("DJ fields were not applied: %+v", after)
	}

	alreadyFilled := model.TagSnapshot{BPM: 120, Key: "5A", KeyScale: "camelot"}
	filtered = filterMissingPatch(alreadyFilled, patch)
	if len(filtered.Fields) != 0 {
		t.Fatalf("only-missing should preserve existing DJ metadata: %+v", filtered.Fields)
	}
}

func TestValidateRequestRejectsImpossibleBPM(t *testing.T) {
	t.Parallel()

	_, err := validateRequest([]int64{1}, model.TagPatch{
		Fields: []string{"bpm"},
		BPM:    900,
	})
	if err == nil {
		t.Fatal("expected invalid BPM error")
	}
}
