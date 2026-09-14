package audio

import (
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestDJMixQualityLookaheadAvoidsPinnedFutureTrap(t *testing.T) {
	t.Parallel()

	tracks := []model.Track{
		{ID: 1, Artist: "Start", Title: "Start", BPM: 128, Key: "A", KeyScale: "minor"},
		{ID: 2, Artist: "Immediate", Title: "Perfect now", BPM: 128, Key: "A", KeyScale: "minor"},
		{ID: 3, Artist: "Bridge", Title: "Better route", BPM: 130, Key: "E", KeyScale: "minor"},
		{ID: 4, Artist: "Pinned", Title: "Future", BPM: 132, Key: "B", KeyScale: "minor"},
	}

	greedy := PlanDJMix(tracks, model.DJMixPlanOptions{
		StartTrackID: 1, Limit: 3, MaxTempoShiftPct: 8, Direction: "any",
		PreferHarmonic: true, Lookahead: 1,
		PinnedTracks: []model.DJMixPin{{TrackID: 4, Position: 3}},
	})
	if len(greedy.Steps) != 3 || greedy.Steps[1].Track.ID != 2 {
		t.Fatalf("lookahead=1 second = %+v", greedy.Steps)
	}

	lookahead := PlanDJMix(tracks, model.DJMixPlanOptions{
		StartTrackID: 1, Limit: 3, MaxTempoShiftPct: 8, Direction: "any",
		PreferHarmonic: true, Lookahead: 2,
		PinnedTracks: []model.DJMixPin{{TrackID: 4, Position: 3}},
	})
	if len(lookahead.Steps) != 3 {
		t.Fatalf("lookahead steps = %d", len(lookahead.Steps))
	}
	if lookahead.Steps[1].Track.ID != 3 {
		t.Fatalf("lookahead=2 second = %d, want bridge track 3", lookahead.Steps[1].Track.ID)
	}
	if !lookahead.Steps[2].Pinned || lookahead.Steps[2].Track.ID != 4 {
		t.Fatalf("pinned third step = %+v", lookahead.Steps[2])
	}
}

func TestDJMixQualityPinReservesTrackForPosition(t *testing.T) {
	t.Parallel()

	tracks := []model.Track{
		{ID: 1, Artist: "A", Title: "One", BPM: 124, Key: "A", KeyScale: "minor"},
		{ID: 2, Artist: "B", Title: "Two", BPM: 125, Key: "A", KeyScale: "minor"},
		{ID: 3, Artist: "C", Title: "Three", BPM: 126, Key: "A", KeyScale: "minor"},
	}
	plan := PlanDJMix(tracks, model.DJMixPlanOptions{
		StartTrackID: 1, Limit: 3, MaxTempoShiftPct: 8, PreferHarmonic: true, Lookahead: 3,
		PinnedTracks: []model.DJMixPin{{TrackID: 3, Position: 3}},
	})
	if plan.PinnedCount != 1 || plan.IgnoredPins != 0 {
		t.Fatalf("pin summary = %+v", plan)
	}
	if plan.Steps[2].Track.ID != 3 || !plan.Steps[2].Pinned {
		t.Fatalf("third step = %+v", plan.Steps[2])
	}
}

func TestDJMixQualityCountsInvalidPins(t *testing.T) {
	t.Parallel()

	tracks := []model.Track{
		{ID: 1, BPM: 124},
		{ID: 2, BPM: 125},
	}
	plan := PlanDJMix(tracks, model.DJMixPlanOptions{
		Limit: 2, Lookahead: 3,
		PinnedTracks: []model.DJMixPin{
			{TrackID: 99, Position: 1},
			{TrackID: 1, Position: 9},
		},
	})
	if plan.IgnoredPins != 2 {
		t.Fatalf("ignored pins = %d", plan.IgnoredPins)
	}
}

func TestDJMixQualityGenreContinuityCanChangeChoice(t *testing.T) {
	t.Parallel()

	tracks := []model.Track{
		{ID: 1, Artist: "Start", Title: "Start", BPM: 128, Key: "A", KeyScale: "minor", Genre: "Deep House"},
		{ID: 2, Artist: "Other", Title: "Exact BPM", BPM: 128, Key: "A", KeyScale: "minor", Genre: "Drum & Bass"},
		{ID: 3, Artist: "House", Title: "Related", BPM: 128.4, Key: "A", KeyScale: "minor", Genre: "Organic Deep House"},
	}
	plan := PlanDJMix(tracks, model.DJMixPlanOptions{
		StartTrackID: 1, Limit: 2, MaxTempoShiftPct: 8, PreferHarmonic: true,
		PreferGenreContinuity: true, Lookahead: 1,
	})
	if plan.Steps[1].Track.ID != 3 {
		t.Fatalf("second = %d, want related-genre track 3", plan.Steps[1].Track.ID)
	}
	if plan.Steps[1].GenreRelation != "related" {
		t.Fatalf("genre relation = %q", plan.Steps[1].GenreRelation)
	}
}

func TestDJMixEnergyProxyUsesMeasuredLoudness(t *testing.T) {
	t.Parallel()

	quiet := djMixEnergyProxy(model.Track{BPM: 128, LoudnessI: -20})
	loud := djMixEnergyProxy(model.Track{BPM: 128, LoudnessI: -8})
	if loud <= quiet {
		t.Fatalf("loud energy=%v quiet energy=%v", loud, quiet)
	}
}

func TestDJMixQualityLookaheadBounds(t *testing.T) {
	t.Parallel()

	got := normalizeDJMixQualityOptions(model.DJMixPlanOptions{Lookahead: 99})
	if got.Lookahead != 4 {
		t.Fatalf("lookahead = %d", got.Lookahead)
	}
	got = normalizeDJMixQualityOptions(model.DJMixPlanOptions{})
	if got.Lookahead != 3 {
		t.Fatalf("default lookahead = %d", got.Lookahead)
	}
}
