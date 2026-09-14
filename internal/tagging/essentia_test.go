package tagging

import (
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestBPMKeyAnalysisPatchCarriesEssentiaFields(t *testing.T) {
	t.Parallel()

	patch := bpmKeyAnalysisPatch(model.BPMKey{
		BPM:   126.5,
		Key:   "F#",
		Scale: "minor",
	})
	if patch.BPM != 126.5 || patch.Key != "F#" || patch.KeyScale != "minor" {
		t.Fatalf("patch = %+v", patch)
	}
	want := map[string]bool{"bpm": false, "key": false, "keyScale": false}
	for _, field := range patch.Fields {
		if _, ok := want[field]; ok {
			want[field] = true
		}
	}
	for field, present := range want {
		if !present {
			t.Fatalf("field %q missing from %+v", field, patch.Fields)
		}
	}
}

func TestBPMKeyAnalysisPatchIgnoresInvalidBPM(t *testing.T) {
	t.Parallel()

	patch := bpmKeyAnalysisPatch(model.BPMKey{BPM: 900, Key: "C", Scale: "major"})
	if patch.BPM != 0 {
		t.Fatalf("invalid BPM leaked into patch: %+v", patch)
	}
	for _, field := range patch.Fields {
		if field == "bpm" {
			t.Fatalf("invalid BPM field selected: %+v", patch.Fields)
		}
	}
}
