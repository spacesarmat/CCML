package audio

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/spacesarmat/CCML/internal/model"
)

const djMixLookaheadDiscount = 0.72

type djMixQualityCandidate struct {
	base       djMixCandidate
	genre      map[string]struct{}
	genreLabel string
	energy     float64
}

type djMixQualityTransition struct {
	base          djMixTransition
	genreRelation string
	energyDelta   float64
	score         float64
	warnings      []string
}

type djMixRankedChoice struct {
	index      int
	candidate  djMixQualityCandidate
	transition djMixQualityTransition
}

// planDJMixQuality adds bounded lookahead, genre/energy evidence and fixed
// positions while preserving the Stage 19.7 BPM/Camelot transition semantics.
func planDJMixQuality(tracks []model.Track, options model.DJMixPlanOptions) model.DJMixPlan {
	options = normalizeDJMixQualityOptions(options)
	plan := model.DJMixPlan{
		SourceCount: len(tracks),
		Lookahead:   options.Lookahead,
	}

	candidates := make([]djMixQualityCandidate, 0, len(tracks))
	baseCandidates := make([]djMixCandidate, 0, len(tracks))
	for _, track := range tracks {
		if !validPlannerBPM(track.BPM) {
			plan.ExcludedMissingBPM++
			continue
		}
		camelot, openKey, ok := DJKeyFormats(track.Key, track.KeyScale)
		if !ok {
			camelot = ""
			openKey = ""
			plan.TracksMissingKey++
		}
		base := djMixCandidate{track: track, camelot: camelot, openKey: openKey}
		tokens, label := djMixGenreTokens(track.Genre)
		candidates = append(candidates, djMixQualityCandidate{
			base:       base,
			genre:      tokens,
			genreLabel: label,
			energy:     djMixEnergyProxy(track),
		})
		baseCandidates = append(baseCandidates, base)
	}
	plan.UsableCount = len(candidates)
	if len(candidates) == 0 {
		plan.Steps = []model.DJMixPlanStep{}
		return plan
	}

	pinsByPosition, pinnedPositionByTrack, ignoredPins := normalizeDJMixPins(candidates, options)
	plan.IgnoredPins = ignoredPins
	plan.PinnedCount = len(pinsByPosition)

	startIndex := -1
	if pinnedID, ok := pinsByPosition[1]; ok {
		startIndex = qualityCandidateIndex(candidates, pinnedID)
	}
	if startIndex < 0 {
		startOptions := options
		if position, reserved := pinnedPositionByTrack[startOptions.StartTrackID]; reserved && position > 1 {
			startOptions.StartTrackID = 0
		}
		startIndex = chooseDJMixStart(baseCandidates, startOptions)
	}
	if startIndex < 0 || startIndex >= len(candidates) {
		startIndex = 0
	}

	start := candidates[startIndex]
	_, startPinned := pinsByPosition[1]
	plan.StartTrackID = start.base.track.ID
	plan.Steps = append(plan.Steps, model.DJMixPlanStep{
		Position:      1,
		Track:         start.base.track,
		Camelot:       start.base.camelot,
		OpenKey:       start.base.openKey,
		AdjustedBPM:   start.base.track.BPM,
		TempoFactor:   1,
		TempoDeltaPct: 0,
		KeyRelation:   "start",
		GenreRelation: "start",
		Energy:        start.energy,
		EnergyDelta:   0,
		Pinned:        startPinned,
		Score:         1,
		Warnings:      []string{},
	})
	plan.TotalDurationMS += start.base.track.DurationMS

	remaining := withoutQualityCandidate(candidates, startIndex)
	current := start
	scoreSum := 0.0
	transitionCount := 0

	for len(remaining) > 0 && len(plan.Steps) < options.Limit {
		position := len(plan.Steps) + 1
		index, transition, ok := chooseDJMixQualityNext(
			current,
			remaining,
			position,
			options,
			pinsByPosition,
			pinnedPositionByTrack,
		)
		if !ok || index < 0 || index >= len(remaining) {
			break
		}

		next := remaining[index]
		_, pinned := pinsByPosition[position]
		warnings := append([]string(nil), transition.warnings...)
		if pinned && transition.score < 0.55 {
			warnings = appendWarningOnce(warnings, "pinned_transition")
		}
		plan.Steps = append(plan.Steps, model.DJMixPlanStep{
			Position:      position,
			Track:         next.base.track,
			Camelot:       next.base.camelot,
			OpenKey:       next.base.openKey,
			AdjustedBPM:   transition.base.adjustedBPM,
			TempoFactor:   transition.base.tempoFactor,
			TempoDeltaPct: transition.base.deltaPct,
			KeyRelation:   transition.base.keyRelation,
			GenreRelation: transition.genreRelation,
			Energy:        next.energy,
			EnergyDelta:   transition.energyDelta,
			Pinned:        pinned,
			Score:         transition.score,
			Warnings:      warnings,
		})
		plan.TotalDurationMS += next.base.track.DurationMS
		scoreSum += transition.score
		transitionCount++
		current = next
		remaining = withoutQualityCandidate(remaining, index)
	}

	if transitionCount > 0 {
		plan.AverageScore = scoreSum / float64(transitionCount)
	}
	return finalizeDJMixTimeline(plan, false)
}

func normalizeDJMixQualityOptions(options model.DJMixPlanOptions) model.DJMixPlanOptions {
	options = normalizeDJMixOptions(options)
	if options.Lookahead <= 0 {
		options.Lookahead = 3
	}
	if options.Lookahead < 1 {
		options.Lookahead = 1
	}
	if options.Lookahead > 4 {
		options.Lookahead = 4
	}
	return options
}

func normalizeDJMixPins(
	candidates []djMixQualityCandidate,
	options model.DJMixPlanOptions,
) (map[int]int64, map[int64]int, int) {
	validTracks := make(map[int64]struct{}, len(candidates))
	for _, candidate := range candidates {
		validTracks[candidate.base.track.ID] = struct{}{}
	}
	byPosition := make(map[int]int64)
	byTrack := make(map[int64]int)
	ignored := 0
	maxPosition := options.Limit
	if len(candidates) < maxPosition {
		maxPosition = len(candidates)
	}

	for _, pin := range options.PinnedTracks {
		if pin.TrackID <= 0 || pin.Position < 1 || pin.Position > maxPosition {
			ignored++
			continue
		}
		if _, ok := validTracks[pin.TrackID]; !ok {
			ignored++
			continue
		}
		if _, exists := byPosition[pin.Position]; exists {
			ignored++
			continue
		}
		if _, exists := byTrack[pin.TrackID]; exists {
			ignored++
			continue
		}
		byPosition[pin.Position] = pin.TrackID
		byTrack[pin.TrackID] = pin.Position
	}
	return byPosition, byTrack, ignored
}

func chooseDJMixQualityNext(
	current djMixQualityCandidate,
	remaining []djMixQualityCandidate,
	position int,
	options model.DJMixPlanOptions,
	pinsByPosition map[int]int64,
	pinnedPositionByTrack map[int64]int,
) (int, djMixQualityTransition, bool) {
	choices := rankDJMixQualityChoices(
		current,
		remaining,
		position,
		options,
		pinsByPosition,
		pinnedPositionByTrack,
	)
	if len(choices) == 0 {
		return -1, djMixQualityTransition{}, false
	}

	depth := options.Lookahead - 1
	if depth <= 0 || len(choices) == 1 {
		best := choices[0]
		return best.index, best.transition, true
	}

	branchWidth := djMixLookaheadBranchWidth(options.Lookahead)
	if len(choices) > branchWidth {
		choices = choices[:branchWidth]
	}

	bestChoice := choices[0]
	bestUtility := math.Inf(-1)
	for _, choice := range choices {
		nextRemaining := withoutQualityCandidate(remaining, choice.index)
		utility := choice.transition.score + djMixLookaheadDiscount*djMixQualityFutureUtility(
			choice.candidate,
			nextRemaining,
			position+1,
			depth,
			options,
			pinsByPosition,
			pinnedPositionByTrack,
		)
		if utility > bestUtility+1e-9 ||
			(math.Abs(utility-bestUtility) <= 1e-9 && betterQualityChoice(choice, bestChoice)) {
			bestChoice = choice
			bestUtility = utility
		}
	}
	return bestChoice.index, bestChoice.transition, true
}

func djMixQualityFutureUtility(
	current djMixQualityCandidate,
	remaining []djMixQualityCandidate,
	position int,
	depth int,
	options model.DJMixPlanOptions,
	pinsByPosition map[int]int64,
	pinnedPositionByTrack map[int64]int,
) float64 {
	if depth <= 0 || len(remaining) == 0 || position > options.Limit {
		return 0
	}
	choices := rankDJMixQualityChoices(
		current,
		remaining,
		position,
		options,
		pinsByPosition,
		pinnedPositionByTrack,
	)
	if len(choices) == 0 {
		return 0
	}
	branchWidth := djMixLookaheadBranchWidth(options.Lookahead)
	if len(choices) > branchWidth {
		choices = choices[:branchWidth]
	}

	best := math.Inf(-1)
	for _, choice := range choices {
		nextRemaining := withoutQualityCandidate(remaining, choice.index)
		utility := choice.transition.score
		if depth > 1 {
			utility += djMixLookaheadDiscount * djMixQualityFutureUtility(
				choice.candidate,
				nextRemaining,
				position+1,
				depth-1,
				options,
				pinsByPosition,
				pinnedPositionByTrack,
			)
		}
		if utility > best {
			best = utility
		}
	}
	if math.IsInf(best, -1) {
		return 0
	}
	return best
}

func rankDJMixQualityChoices(
	current djMixQualityCandidate,
	remaining []djMixQualityCandidate,
	position int,
	options model.DJMixPlanOptions,
	pinsByPosition map[int]int64,
	pinnedPositionByTrack map[int64]int,
) []djMixRankedChoice {
	requiredTrackID := pinsByPosition[position]
	choices := make([]djMixRankedChoice, 0, len(remaining))

	for index, candidate := range remaining {
		trackID := candidate.base.track.ID
		if requiredTrackID > 0 && trackID != requiredTrackID {
			continue
		}
		if pinnedPosition, reserved := pinnedPositionByTrack[trackID]; reserved && pinnedPosition != position {
			continue
		}
		transition := scoreDJMixQualityTransition(current, candidate, options)
		choices = append(choices, djMixRankedChoice{
			index:      index,
			candidate:  candidate,
			transition: transition,
		})
	}
	sort.SliceStable(choices, func(i, j int) bool {
		return betterQualityChoice(choices[i], choices[j])
	})
	return choices
}

func betterQualityChoice(left, right djMixRankedChoice) bool {
	if left.transition.score > right.transition.score+1e-9 {
		return true
	}
	if left.transition.score < right.transition.score-1e-9 {
		return false
	}
	leftDelta := math.Abs(left.transition.base.deltaPct)
	rightDelta := math.Abs(right.transition.base.deltaPct)
	if leftDelta < rightDelta-1e-9 {
		return true
	}
	if leftDelta > rightDelta+1e-9 {
		return false
	}
	return djMixLabel(left.candidate.base) < djMixLabel(right.candidate.base)
}

func scoreDJMixQualityTransition(
	current, next djMixQualityCandidate,
	options model.DJMixPlanOptions,
) djMixQualityTransition {
	base := scoreDJMixTransition(current.base, next.base, options)
	genreRelation, genreScore := djMixGenreRelation(current, next)
	energyDelta := next.energy - current.energy
	energyScore := 1 - math.Min(1, math.Abs(energyDelta)/0.35)

	genreWeight := 0.0
	if options.PreferGenreContinuity {
		genreWeight = 0.15
	}
	energyWeight := 0.0
	if options.PreferEnergyFlow {
		energyWeight = 0.10
	}
	baseWeight := 1 - genreWeight - energyWeight
	score := baseWeight*base.score + genreWeight*genreScore + energyWeight*energyScore
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	warnings := append([]string(nil), base.warnings...)
	if options.PreferGenreContinuity && genreRelation == "different" {
		warnings = appendWarningOnce(warnings, "genre_jump")
	}
	if options.PreferEnergyFlow && math.Abs(energyDelta) > 0.35 {
		warnings = appendWarningOnce(warnings, "energy_jump")
	}
	return djMixQualityTransition{
		base:          base,
		genreRelation: genreRelation,
		energyDelta:   energyDelta,
		score:         score,
		warnings:      warnings,
	}
}

func djMixGenreRelation(current, next djMixQualityCandidate) (string, float64) {
	if len(current.genre) == 0 || len(next.genre) == 0 {
		return "unknown", 0.5
	}
	if current.genreLabel != "" && current.genreLabel == next.genreLabel {
		return "same", 1
	}
	intersection := 0
	for token := range current.genre {
		if _, ok := next.genre[token]; ok {
			intersection++
		}
	}
	union := len(current.genre) + len(next.genre) - intersection
	if union <= 0 {
		return "unknown", 0.5
	}
	similarity := float64(intersection) / float64(union)
	if similarity >= 0.5 {
		return "related", 0.85
	}
	if similarity > 0 {
		return "related", 0.68
	}
	return "different", 0.25
}

func djMixGenreTokens(value string) (map[string]struct{}, string) {
	label := strings.ToLower(strings.TrimSpace(value))
	if label == "" {
		return nil, ""
	}
	parts := strings.FieldsFunc(label, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	tokens := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len([]rune(part)) < 2 {
			continue
		}
		tokens[part] = struct{}{}
	}
	if len(tokens) == 0 {
		return nil, label
	}
	return tokens, label
}

// djMixEnergyProxy is deliberately transparent: canonicalized BPM contributes
// 65%, and measured Integrated LUFS contributes 35% when a plausible loudness
// measurement exists. It is not an Essentia mood/energy classifier.
func djMixEnergyProxy(track model.Track) float64 {
	bpm := track.BPM
	for bpm > 0 && bpm < 80 {
		bpm *= 2
	}
	for bpm > 160 {
		bpm /= 2
	}
	bpmEnergy := clampDJMix01((bpm - 80) / 80)
	if track.LoudnessI < -1 && track.LoudnessI > -60 {
		loudnessEnergy := clampDJMix01((track.LoudnessI + 24) / 18)
		return clampDJMix01(0.65*bpmEnergy + 0.35*loudnessEnergy)
	}
	return bpmEnergy
}

func clampDJMix01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func qualityCandidateIndex(candidates []djMixQualityCandidate, trackID int64) int {
	for i, candidate := range candidates {
		if candidate.base.track.ID == trackID {
			return i
		}
	}
	return -1
}

func withoutQualityCandidate(candidates []djMixQualityCandidate, index int) []djMixQualityCandidate {
	if index < 0 || index >= len(candidates) {
		return append([]djMixQualityCandidate(nil), candidates...)
	}
	result := make([]djMixQualityCandidate, 0, len(candidates)-1)
	result = append(result, candidates[:index]...)
	result = append(result, candidates[index+1:]...)
	return result
}

func djMixLookaheadBranchWidth(lookahead int) int {
	switch lookahead {
	case 4:
		return 8
	case 3:
		return 7
	case 2:
		return 6
	default:
		return 1
	}
}

func appendWarningOnce(warnings []string, value string) []string {
	for _, existing := range warnings {
		if existing == value {
			return warnings
		}
	}
	return append(warnings, value)
}
