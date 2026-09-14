package audio

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

type djMixCandidate struct {
	track   model.Track
	camelot string
	openKey string
}

type djMixTransition struct {
	adjustedBPM float64
	tempoFactor float64
	deltaPct    float64
	keyRelation string
	score       float64
	warnings    []string
}

// PlanDJMix creates a deterministic BPM/Camelot route. Stage 19.8 adds bounded
// lookahead, genre/energy-flow evidence and fixed positions while retaining the
// conservative Stage 19.7 transition primitives.
func PlanDJMix(tracks []model.Track, options model.DJMixPlanOptions) model.DJMixPlan {
	return planDJMixQuality(tracks, options)
}

func normalizeDJMixOptions(options model.DJMixPlanOptions) model.DJMixPlanOptions {
	if options.Limit < 2 {
		options.Limit = 20
	}
	if options.Limit > 100 {
		options.Limit = 100
	}
	if options.MaxTempoShiftPct <= 0 {
		options.MaxTempoShiftPct = 8
	}
	if options.MaxTempoShiftPct < 1 {
		options.MaxTempoShiftPct = 1
	}
	if options.MaxTempoShiftPct > 25 {
		options.MaxTempoShiftPct = 25
	}
	switch strings.ToLower(strings.TrimSpace(options.Direction)) {
	case "up", "down":
		options.Direction = strings.ToLower(strings.TrimSpace(options.Direction))
	default:
		options.Direction = "any"
	}
	return options
}

func chooseDJMixStart(candidates []djMixCandidate, options model.DJMixPlanOptions) int {
	if options.StartTrackID > 0 {
		for i, candidate := range candidates {
			if candidate.track.ID == options.StartTrackID {
				return i
			}
		}
	}

	if options.Direction == "up" || options.Direction == "down" {
		best := 0
		for i := 1; i < len(candidates); i++ {
			if options.Direction == "up" {
				if candidates[i].track.BPM < candidates[best].track.BPM ||
					(candidates[i].track.BPM == candidates[best].track.BPM && djMixLabel(candidates[i]) < djMixLabel(candidates[best])) {
					best = i
				}
			} else if candidates[i].track.BPM > candidates[best].track.BPM ||
				(candidates[i].track.BPM == candidates[best].track.BPM && djMixLabel(candidates[i]) < djMixLabel(candidates[best])) {
				best = i
			}
		}
		return best
	}

	bpms := make([]float64, len(candidates))
	for i, candidate := range candidates {
		bpms[i] = candidate.track.BPM
	}
	sort.Float64s(bpms)
	median := bpms[len(bpms)/2]

	best := 0
	bestDistance := math.Abs(candidates[0].track.BPM - median)
	if candidates[0].camelot == "" {
		bestDistance += 2
	}
	for i := 1; i < len(candidates); i++ {
		distance := math.Abs(candidates[i].track.BPM - median)
		if candidates[i].camelot == "" {
			distance += 2
		}
		if distance < bestDistance-1e-9 ||
			(math.Abs(distance-bestDistance) <= 1e-9 && djMixLabel(candidates[i]) < djMixLabel(candidates[best])) {
			best = i
			bestDistance = distance
		}
	}
	return best
}

func scoreDJMixTransition(current, next djMixCandidate, options model.DJMixPlanOptions) djMixTransition {
	adjustedBPM, factor, deltaPct := closestPlannerTempo(current.track.BPM, next.track.BPM)
	absDelta := math.Abs(deltaPct)
	bpmScore := 1 - math.Min(1, absDelta/options.MaxTempoShiftPct)

	relation, keyScore := plannerKeyRelation(current.camelot, next.camelot)
	if !options.PreferHarmonic {
		keyScore = 0.5
	}

	artistScore := 1.0
	sameArtist := samePlannerArtist(current.track.Artist, next.track.Artist)
	if options.AvoidSameArtist && sameArtist {
		artistScore = 0
	}

	score := 0.65*bpmScore + 0.30*keyScore + 0.05*artistScore
	if !options.PreferHarmonic {
		score = 0.85*bpmScore + 0.10*keyScore + 0.05*artistScore
	}
	warnings := make([]string, 0, 4)
	if factor != 1 {
		warnings = append(warnings, "half_double")
	}
	if absDelta > options.MaxTempoShiftPct {
		warnings = append(warnings, "tempo_jump")
		score *= 0.55
	}
	if options.PreferHarmonic {
		switch relation {
		case "conflict":
			warnings = append(warnings, "key_conflict")
		case "unknown":
			warnings = append(warnings, "key_unknown")
		}
	}
	if options.AvoidSameArtist && sameArtist {
		warnings = append(warnings, "same_artist")
		score *= 0.82
	}
	if options.Direction == "up" && deltaPct < -0.25 {
		warnings = append(warnings, "direction_reverse")
		score *= 0.68
	}
	if options.Direction == "down" && deltaPct > 0.25 {
		warnings = append(warnings, "direction_reverse")
		score *= 0.68
	}

	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return djMixTransition{
		adjustedBPM: adjustedBPM,
		tempoFactor: factor,
		deltaPct:    deltaPct,
		keyRelation: relation,
		score:       score,
		warnings:    warnings,
	}
}

func closestPlannerTempo(currentBPM, candidateBPM float64) (adjusted, factor, deltaPct float64) {
	bestAdjusted := candidateBPM
	bestFactor := 1.0
	bestDelta := plannerTempoDeltaPct(currentBPM, candidateBPM)
	for _, f := range []float64{0.5, 2} {
		value := candidateBPM * f
		delta := plannerTempoDeltaPct(currentBPM, value)
		if math.Abs(delta) < math.Abs(bestDelta)-1e-9 {
			bestAdjusted = value
			bestFactor = f
			bestDelta = delta
		}
	}
	return bestAdjusted, bestFactor, bestDelta
}

func plannerTempoDeltaPct(currentBPM, nextBPM float64) float64 {
	if currentBPM <= 0 {
		return 0
	}
	return (nextBPM - currentBPM) / currentBPM * 100
}

func plannerKeyRelation(current, next string) (string, float64) {
	if strings.TrimSpace(current) == "" || strings.TrimSpace(next) == "" {
		return "unknown", 0.35
	}
	currentNumber, currentSide, okCurrent := parsePlannerCamelot(current)
	nextNumber, nextSide, okNext := parsePlannerCamelot(next)
	if !okCurrent || !okNext {
		return "unknown", 0.35
	}
	if currentNumber == nextNumber && currentSide == nextSide {
		return "same", 1
	}
	if currentNumber == nextNumber && currentSide != nextSide {
		return "relative", 0.92
	}
	if currentSide == nextSide {
		previous := currentNumber - 1
		if previous < 1 {
			previous = 12
		}
		following := currentNumber + 1
		if following > 12 {
			following = 1
		}
		if nextNumber == previous || nextNumber == following {
			return "adjacent", 0.90
		}
	}
	return "conflict", 0.20
}

func parsePlannerCamelot(value string) (int, string, bool) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) < 2 {
		return 0, "", false
	}
	side := value[len(value)-1:]
	if side != "A" && side != "B" {
		return 0, "", false
	}
	number, err := strconv.Atoi(value[:len(value)-1])
	if err != nil || number < 1 || number > 12 {
		return 0, "", false
	}
	return number, side, true
}

func validPlannerBPM(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 20 && value <= 400
}

func samePlannerArtist(left, right string) bool {
	left = strings.ToLower(strings.TrimSpace(left))
	right = strings.ToLower(strings.TrimSpace(right))
	return left != "" && left == right
}

func betterDJMixChoice(
	candidate djMixCandidate,
	transition djMixTransition,
	bestCandidate djMixCandidate,
	best djMixTransition,
) bool {
	if transition.score > best.score+1e-9 {
		return true
	}
	if transition.score < best.score-1e-9 {
		return false
	}
	if math.Abs(transition.deltaPct) < math.Abs(best.deltaPct)-1e-9 {
		return true
	}
	if math.Abs(transition.deltaPct) > math.Abs(best.deltaPct)+1e-9 {
		return false
	}
	return djMixLabel(candidate) < djMixLabel(bestCandidate)
}

func djMixLabel(candidate djMixCandidate) string {
	return strings.ToLower(strings.TrimSpace(candidate.track.Artist)) + "\x00" +
		strings.ToLower(strings.TrimSpace(candidate.track.Title)) + "\x00" +
		strconv.FormatInt(candidate.track.ID, 10)
}
