package audio

import (
	"math"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestPlanDJMixHonorsStartAndPrefersSmoothHarmonicTransition(t *testing.T) {
	t.Parallel()

	tracks := []model.Track{
		{ID: 1, Artist: "Start", Title: "One", BPM: 128, Key: "A", KeyScale: "minor", DurationMS: 180000},
		{ID: 2, Artist: "Next", Title: "Two", BPM: 128.5, Key: "A", KeyScale: "minor", DurationMS: 180000},
		{ID: 3, Artist: "Far", Title: "Three", BPM: 135, Key: "F#", KeyScale: "major", DurationMS: 180000},
	}
	plan := PlanDJMix(tracks, model.DJMixPlanOptions{
		StartTrackID: 1, Limit: 3, MaxTempoShiftPct: 8, Direction: "any",
		PreferHarmonic: true, AvoidSameArtist: true,
	})
	if len(plan.Steps) != 3 {
		t.Fatalf("steps = %d", len(plan.Steps))
	}
	if plan.Steps[0].Track.ID != 1 {
		t.Fatalf("start = %d", plan.Steps[0].Track.ID)
	}
	if plan.Steps[1].Track.ID != 2 {
		t.Fatalf("second = %d, want 2", plan.Steps[1].Track.ID)
	}
	if plan.Steps[1].KeyRelation != "same" {
		t.Fatalf("relation = %q", plan.Steps[1].KeyRelation)
	}
}

func TestPlanDJMixNormalizesHalfDoubleTempo(t *testing.T) {
	t.Parallel()

	tracks := []model.Track{
		{ID: 1, Artist: "A", Title: "128", BPM: 128, Key: "A", KeyScale: "minor"},
		{ID: 2, Artist: "B", Title: "64", BPM: 64, Key: "A", KeyScale: "minor"},
	}
	plan := PlanDJMix(tracks, model.DJMixPlanOptions{
		StartTrackID: 1, Limit: 2, MaxTempoShiftPct: 8, PreferHarmonic: true,
	})
	step := plan.Steps[1]
	if step.TempoFactor != 2 {
		t.Fatalf("tempo factor = %v, want 2", step.TempoFactor)
	}
	if math.Abs(step.AdjustedBPM-128) > 0.001 || math.Abs(step.TempoDeltaPct) > 0.001 {
		t.Fatalf("adjusted=%v delta=%v", step.AdjustedBPM, step.TempoDeltaPct)
	}
	found := false
	for _, warning := range step.Warnings {
		if warning == "half_double" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %#v", step.Warnings)
	}
}

func TestPlanDJMixCountsMissingAnalysis(t *testing.T) {
	t.Parallel()

	tracks := []model.Track{
		{ID: 1, BPM: 0},
		{ID: 2, BPM: 125},
		{ID: 3, BPM: 126, Key: "A", KeyScale: "minor"},
	}
	plan := PlanDJMix(tracks, model.DJMixPlanOptions{Limit: 10, MaxTempoShiftPct: 8, PreferHarmonic: true})
	if plan.SourceCount != 3 || plan.UsableCount != 2 || plan.ExcludedMissingBPM != 1 || plan.TracksMissingKey != 1 {
		t.Fatalf("plan summary = %+v", plan)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("steps = %d", len(plan.Steps))
	}
}

func TestPlanDJMixDirectionUpChoosesLowestAutoStart(t *testing.T) {
	t.Parallel()

	tracks := []model.Track{
		{ID: 1, Artist: "A", Title: "High", BPM: 130, Key: "A", KeyScale: "minor"},
		{ID: 2, Artist: "B", Title: "Low", BPM: 120, Key: "A", KeyScale: "minor"},
		{ID: 3, Artist: "C", Title: "Mid", BPM: 125, Key: "A", KeyScale: "minor"},
	}
	plan := PlanDJMix(tracks, model.DJMixPlanOptions{
		Limit: 3, MaxTempoShiftPct: 8, Direction: "up", PreferHarmonic: true,
	})
	if plan.StartTrackID != 2 {
		t.Fatalf("start = %d, want lowest-BPM track 2", plan.StartTrackID)
	}
}

func TestPlannerKeyRelationWrapsCamelotWheel(t *testing.T) {
	t.Parallel()

	relation, score := plannerKeyRelation("1A", "12A")
	if relation != "adjacent" || score < 0.8 {
		t.Fatalf("relation=%q score=%v", relation, score)
	}
	relation, _ = plannerKeyRelation("8A", "8B")
	if relation != "relative" {
		t.Fatalf("relative relation = %q", relation)
	}
}
