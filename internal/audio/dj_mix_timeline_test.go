package audio

import (
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestGeneratedDJMixGetsTimelineOffsets(t *testing.T) {
	t.Parallel()

	plan := PlanDJMix([]model.Track{
		{ID: 1, Artist: "A", Title: "One", BPM: 128, Key: "A", KeyScale: "minor", DurationMS: 180000},
		{ID: 2, Artist: "B", Title: "Two", BPM: 129, Key: "A", KeyScale: "minor", DurationMS: 210000},
	}, model.DJMixPlanOptions{
		StartTrackID: 1, Limit: 2, MaxTempoShiftPct: 8, PreferHarmonic: true, Lookahead: 3,
	})
	if len(plan.Steps) != 2 {
		t.Fatalf("steps = %d", len(plan.Steps))
	}
	if plan.ManualOrder {
		t.Fatal("generated plan must not be marked manual")
	}
	if plan.Steps[0].TimelineStartMS != 0 || plan.Steps[0].TimelineEndMS != 180000 {
		t.Fatalf("first timeline = %+v", plan.Steps[0])
	}
	if plan.Steps[1].TimelineStartMS != 180000 || plan.Steps[1].TimelineEndMS != 390000 || plan.TotalDurationMS != 390000 {
		t.Fatalf("second timeline = %+v total=%d", plan.Steps[1], plan.TotalDurationMS)
	}
}

func TestRecalculateDJMixTimelinePreservesManualFieldsAndRescores(t *testing.T) {
	t.Parallel()

	plan := model.DJMixPlan{
		Steps: []model.DJMixPlanStep{
			{
				Track:  model.Track{ID: 2, Artist: "B", Title: "Two", BPM: 129, Key: "A", KeyScale: "minor", DurationMS: 200000},
				Locked: true, CueNote: " cue B ",
			},
			{
				Track:          model.Track{ID: 1, Artist: "A", Title: "One", BPM: 128, Key: "A", KeyScale: "minor", DurationMS: 180000},
				TransitionNote: " blend 16 bars ",
			},
		},
	}
	got := RecalculateDJMixTimeline(plan, model.DJMixPlanOptions{
		MaxTempoShiftPct: 8, PreferHarmonic: true, Lookahead: 3,
	})
	if !got.ManualOrder || got.StartTrackID != 2 || got.LockedCount != 1 {
		t.Fatalf("manual summary = %+v", got)
	}
	if got.Steps[0].Position != 1 || got.Steps[1].Position != 2 {
		t.Fatalf("positions = %+v", got.Steps)
	}
	if got.Steps[0].TimelineEndMS != 200000 || got.Steps[1].TimelineStartMS != 200000 || got.TotalDurationMS != 380000 {
		t.Fatalf("timeline = %+v", got.Steps)
	}
	if got.Steps[0].CueNote != "cue B" || got.Steps[1].TransitionNote != "blend 16 bars" {
		t.Fatalf("notes = %#v / %#v", got.Steps[0].CueNote, got.Steps[1].TransitionNote)
	}
	if got.Steps[0].KeyRelation != "start" || got.Steps[1].Score <= 0 {
		t.Fatalf("transition = %+v", got.Steps[1])
	}
}

func TestRecalculateDJMixTimelineCountsPinnedAndLocked(t *testing.T) {
	t.Parallel()

	plan := model.DJMixPlan{Steps: []model.DJMixPlanStep{
		{Track: model.Track{ID: 1, BPM: 124, DurationMS: 1000}, Pinned: true, Locked: true},
		{Track: model.Track{ID: 2, BPM: 125, DurationMS: 1000}, Pinned: true},
	}}
	got := RecalculateDJMixTimeline(plan, model.DJMixPlanOptions{Lookahead: 2, MaxTempoShiftPct: 8})
	if got.PinnedCount != 2 || got.LockedCount != 1 {
		t.Fatalf("counts = pinned %d locked %d", got.PinnedCount, got.LockedCount)
	}
	if got.Lookahead != 2 {
		t.Fatalf("lookahead = %d", got.Lookahead)
	}
}

func TestTimelineNoteIsBounded(t *testing.T) {
	t.Parallel()

	long := make([]rune, 600)
	for i := range long {
		long[i] = 'x'
	}
	got := RecalculateDJMixTimeline(model.DJMixPlan{Steps: []model.DJMixPlanStep{
		{Track: model.Track{ID: 1, BPM: 128}, CueNote: string(long)},
	}}, model.DJMixPlanOptions{Lookahead: 3})
	if len([]rune(got.Steps[0].CueNote)) != 500 {
		t.Fatalf("note length = %d", len([]rune(got.Steps[0].CueNote)))
	}
}
