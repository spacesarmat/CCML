package audio

import (
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

// RecalculateDJMixTimeline preserves a manual order and free-text notes while
// recomputing every derived transition value using the Stage 19.8 quality layer.
func RecalculateDJMixTimeline(plan model.DJMixPlan, options model.DJMixPlanOptions) model.DJMixPlan {
	options = normalizeDJMixQualityOptions(options)
	if len(plan.Steps) == 0 {
		plan.AverageScore = 0
		plan.Lookahead = options.Lookahead
		return finalizeDJMixTimeline(plan, true)
	}

	scoreSum := 0.0
	transitionCount := 0
	pinnedCount := 0
	var previous djMixQualityCandidate

	for i := range plan.Steps {
		step := &plan.Steps[i]
		candidate := timelineQualityCandidate(step.Track)
		step.Camelot = candidate.base.camelot
		step.OpenKey = candidate.base.openKey
		step.Energy = candidate.energy
		if step.Pinned {
			pinnedCount++
		}

		if i == 0 {
			step.AdjustedBPM = step.Track.BPM
			step.TempoFactor = 1
			step.TempoDeltaPct = 0
			step.KeyRelation = "start"
			step.GenreRelation = "start"
			step.EnergyDelta = 0
			step.Score = 1
			step.Warnings = []string{}
		} else {
			transition := scoreDJMixQualityTransition(previous, candidate, options)
			warnings := append([]string(nil), transition.warnings...)
			if step.Pinned && transition.score < 0.55 {
				warnings = appendWarningOnce(warnings, "pinned_transition")
			}
			step.AdjustedBPM = transition.base.adjustedBPM
			step.TempoFactor = transition.base.tempoFactor
			step.TempoDeltaPct = transition.base.deltaPct
			step.KeyRelation = transition.base.keyRelation
			step.GenreRelation = transition.genreRelation
			step.EnergyDelta = transition.energyDelta
			step.Score = transition.score
			step.Warnings = warnings
			scoreSum += transition.score
			transitionCount++
		}

		step.TransitionNote = trimTimelineNote(step.TransitionNote)
		step.CueNote = trimTimelineNote(step.CueNote)
		previous = candidate
	}

	plan.PinnedCount = pinnedCount
	plan.Lookahead = options.Lookahead
	if transitionCount > 0 {
		plan.AverageScore = scoreSum / float64(transitionCount)
	} else {
		plan.AverageScore = 0
	}
	return finalizeDJMixTimeline(plan, true)
}

func finalizeDJMixTimeline(plan model.DJMixPlan, manual bool) model.DJMixPlan {
	var cursor int64
	lockedCount := 0
	for i := range plan.Steps {
		step := &plan.Steps[i]
		step.Position = i + 1
		step.TimelineStartMS = cursor
		duration := step.Track.DurationMS
		if duration < 0 {
			duration = 0
		}
		cursor += duration
		step.TimelineEndMS = cursor
		if step.Locked {
			lockedCount++
		}
	}
	plan.TotalDurationMS = cursor
	plan.LockedCount = lockedCount
	plan.ManualOrder = manual
	if len(plan.Steps) > 0 {
		plan.StartTrackID = plan.Steps[0].Track.ID
	}
	return plan
}

func timelineQualityCandidate(track model.Track) djMixQualityCandidate {
	camelot, openKey, ok := DJKeyFormats(track.Key, track.KeyScale)
	if !ok {
		camelot = ""
		openKey = ""
	}
	base := djMixCandidate{track: track, camelot: camelot, openKey: openKey}
	tokens, label := djMixGenreTokens(track.Genre)
	return djMixQualityCandidate{
		base:       base,
		genre:      tokens,
		genreLabel: label,
		energy:     djMixEnergyProxy(track),
	}
}

func trimTimelineNote(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > 500 {
		value = string(runes[:500])
	}
	return value
}
