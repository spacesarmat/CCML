package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

const (
	duplicateVerifySampleRate             = 4000
	duplicateVerifyFrameMS                = 100
	duplicateVerifyFrameSize              = duplicateVerifySampleRate * duplicateVerifyFrameMS / 1000
	duplicateVerifyMaxSeconds             = 900
	duplicateVerifyBaseShift              = 60  // Always search at least +/-6 seconds.
	duplicateVerifyMaxShift               = 600 // Up to +/-60 seconds at 100 ms/frame.
	duplicateVerifyCoarseShiftStep        = 5   // 500 ms coarse scan before frame-level refinement.
	duplicateVerifySameMaxDurationDeltaMS = 1_500
	duplicateVerifySimilarMaxDurationMS   = 60_000
)

var duplicateVerifySpectrumFrequencies = [...]float64{80, 140, 220, 350, 550, 850, 1300, 1750}

// DuplicateComparator compares decoded waveforms using low-rate mono PCM.
//
// This is intentionally a conservative heuristic, not a cryptographic or
// Chromaprint identity check. It is designed to add another signal before
// destructive duplicate actions.
type DuplicateComparator struct {
	tools *Toolchain
}

// NewDuplicateComparator creates a decoded-waveform comparator.
func NewDuplicateComparator(tools *Toolchain) *DuplicateComparator {
	return &DuplicateComparator{tools: tools}
}

type duplicateFeatures struct {
	energy    []float64
	roughness []float64
	zcr       []float64
	spectrum  []float64
}

// Compare compares every track against one reference track.
func (c *DuplicateComparator) Compare(
	ctx context.Context,
	tracks []model.Track,
	referenceTrackID int64,
) (model.DuplicateAudioVerification, error) {
	if c == nil || c.tools == nil || strings.TrimSpace(c.tools.FFmpegPath()) == "" {
		return model.DuplicateAudioVerification{}, errors.New("FFmpeg is not available")
	}
	if len(tracks) < 2 {
		return model.DuplicateAudioVerification{}, errors.New("audio verification requires at least two tracks")
	}

	var reference model.Track
	foundReference := false
	for _, track := range tracks {
		if track.ID == referenceTrackID {
			reference = track
			foundReference = true
			break
		}
	}
	if !foundReference {
		return model.DuplicateAudioVerification{}, fmt.Errorf("reference track %d is not in the duplicate group", referenceTrackID)
	}

	referenceFeatures, err := c.decodeFeatures(ctx, reference.Path)
	if err != nil {
		return model.DuplicateAudioVerification{}, fmt.Errorf("decode reference track %d: %w", reference.ID, err)
	}

	result := model.DuplicateAudioVerification{
		ReferenceTrackID: referenceTrackID,
		Comparisons:      make([]model.DuplicateAudioComparison, 0, len(tracks)),
	}

	for _, track := range tracks {
		if track.ID == referenceTrackID {
			result.Comparisons = append(result.Comparisons, model.DuplicateAudioComparison{
				TrackID:         track.ID,
				Similarity:      1,
				OffsetMS:        0,
				DurationDeltaMS: 0,
				Status:          "reference",
			})
			continue
		}

		features, err := c.decodeFeatures(ctx, track.Path)
		if err != nil {
			result.ErrorCount++
			result.Comparisons = append(result.Comparisons, model.DuplicateAudioComparison{
				TrackID:         track.ID,
				DurationDeltaMS: absInt64(track.DurationMS - reference.DurationMS),
				Status:          "error",
				Error:           err.Error(),
			})
			continue
		}

		similarity, offsetFrames := bestDuplicateFeatureSimilarity(referenceFeatures, features)
		durationDelta := absInt64(track.DurationMS - reference.DurationMS)
		status := classifyDuplicateSimilarity(similarity, durationDelta)

		switch status {
		case "same":
			result.SameCount++
		case "similar":
			result.SimilarCount++
		case "different":
			result.DifferentCount++
		}

		result.Comparisons = append(result.Comparisons, model.DuplicateAudioComparison{
			TrackID:         track.ID,
			Similarity:      similarity,
			OffsetMS:        int64(offsetFrames * duplicateVerifyFrameMS),
			DurationDeltaMS: durationDelta,
			Status:          status,
		})
	}

	return result, nil
}

func (c *DuplicateComparator) decodeFeatures(ctx context.Context, path string) (duplicateFeatures, error) {
	cmd := exec.CommandContext(
		ctx,
		c.tools.FFmpegPath(),
		"-v", "error",
		"-nostdin",
		"-i", path,
		"-map", "0:a:0",
		"-vn",
		"-ac", "1",
		"-ar", fmt.Sprintf("%d", duplicateVerifySampleRate),
		"-t", fmt.Sprintf("%d", duplicateVerifyMaxSeconds),
		"-f", "s16le",
		"pipe:1",
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return duplicateFeatures{}, ctx.Err()
		}
		return duplicateFeatures{}, fmt.Errorf(
			"FFmpeg decode failed: %w: %s",
			err,
			strings.TrimSpace(stderr.String()),
		)
	}

	payload := stdout.Bytes()
	if len(payload) < duplicateVerifyFrameSize*2*10 {
		return duplicateFeatures{}, errors.New("decoded audio is too short for reliable comparison")
	}
	if len(payload)%2 != 0 {
		payload = payload[:len(payload)-1]
	}

	samples := make([]int16, len(payload)/2)
	for i := range samples {
		samples[i] = int16(binary.LittleEndian.Uint16(payload[i*2 : i*2+2]))
	}
	return extractDuplicateFeatures(samples), nil
}

func extractDuplicateFeatures(samples []int16) duplicateFeatures {
	frameCount := len(samples) / duplicateVerifyFrameSize
	features := duplicateFeatures{
		energy:    make([]float64, 0, frameCount),
		roughness: make([]float64, 0, frameCount),
		zcr:       make([]float64, 0, frameCount),
		spectrum:  make([]float64, len(duplicateVerifySpectrumFrequencies)),
	}
	spectrumFrames := 0

	for frame := 0; frame < frameCount; frame++ {
		start := frame * duplicateVerifyFrameSize
		end := start + duplicateVerifyFrameSize
		block := samples[start:end]

		var sumSquares float64
		var diffSum float64
		zeroCrossings := 0
		previous := float64(block[0])

		for i, sample := range block {
			value := float64(sample) / 32768.0
			sumSquares += value * value
			if i > 0 {
				diffSum += math.Abs(value - previous)
				if (value >= 0) != (previous >= 0) {
					zeroCrossings++
				}
			}
			previous = value
		}

		rms := math.Sqrt(sumSquares / float64(len(block)))
		// Log energy makes the feature resilient to simple gain differences.
		features.energy = append(features.energy, math.Log1p(rms*1000))
		features.roughness = append(features.roughness, diffSum/float64(maxInt(len(block)-1, 1)))
		features.zcr = append(features.zcr, float64(zeroCrossings)/float64(maxInt(len(block)-1, 1)))

		frameSpectrum := make([]float64, len(duplicateVerifySpectrumFrequencies))
		frameSpectrumTotal := 0.0
		for bin, frequency := range duplicateVerifySpectrumFrequencies {
			coefficient := 2 * math.Cos(2*math.Pi*frequency/float64(duplicateVerifySampleRate))
			var previous float64
			var previousPrevious float64
			for _, sample := range block {
				value := float64(sample) / 32768.0
				current := value + coefficient*previous - previousPrevious
				previousPrevious = previous
				previous = current
			}
			power := previous*previous + previousPrevious*previousPrevious - coefficient*previous*previousPrevious
			if power < 0 {
				power = 0
			}
			frameSpectrum[bin] = power
			frameSpectrumTotal += power
		}
		if frameSpectrumTotal > 1e-12 {
			for bin, power := range frameSpectrum {
				features.spectrum[bin] += power / frameSpectrumTotal
			}
			spectrumFrames++
		}
	}

	features.energy = zNormalize(features.energy)
	features.roughness = zNormalize(features.roughness)
	features.zcr = zNormalize(features.zcr)
	if spectrumFrames > 0 {
		for bin := range features.spectrum {
			features.spectrum[bin] /= float64(spectrumFrames)
		}
	}
	return features
}

func bestDuplicateFeatureSimilarity(left, right duplicateFeatures) (float64, int) {
	maxFrames := maxInt(len(left.energy), len(right.energy))
	minFrames := minInt(len(left.energy), len(right.energy))
	if minFrames < 10 {
		return 0, 0
	}

	minOverlap := maxInt(10, int(float64(minFrames)*0.70))
	lengthDelta := absInt(len(left.energy) - len(right.energy))
	shiftLimit := duplicateVerifyBaseShift + lengthDelta
	if shiftLimit > duplicateVerifyMaxShift {
		shiftLimit = duplicateVerifyMaxShift
	}
	if shiftLimit < duplicateVerifyBaseShift {
		shiftLimit = duplicateVerifyBaseShift
	}

	spectrumScore := duplicateSpectrumSimilarity(left.spectrum, right.spectrum)
	bestScore := -1.0
	bestShift := 0
	evaluate := func(shift int) {
		score, ok := duplicateAlignmentScore(left, right, shift, minOverlap, maxFrames, spectrumScore)
		if ok && score > bestScore {
			bestScore = score
			bestShift = shift
		}
	}

	// Preserve the old frame-by-frame +/-6 second behavior for near-equal
	// lengths. Wider Radio Edit / Extended Mix differences use a 500 ms coarse
	// scan and then refine the winning neighborhood at the native 100 ms frame.
	step := 1
	if shiftLimit > duplicateVerifyBaseShift {
		step = duplicateVerifyCoarseShiftStep
	}
	for shift := -shiftLimit; shift <= shiftLimit; shift += step {
		evaluate(shift)
	}
	if step > 1 {
		evaluate(-shiftLimit)
		evaluate(shiftLimit)
		from := maxInt(-shiftLimit, bestShift-step+1)
		to := minInt(shiftLimit, bestShift+step-1)
		for shift := from; shift <= to; shift++ {
			evaluate(shift)
		}
	}

	if bestScore < 0 {
		return 0, 0
	}
	if bestScore > 1 {
		bestScore = 1
	}
	return bestScore, bestShift
}

func duplicateAlignmentScore(
	left, right duplicateFeatures,
	shift, minOverlap, maxFrames int,
	spectrumScore float64,
) (float64, bool) {
	leftStart := 0
	rightStart := 0
	if shift > 0 {
		rightStart = shift
	} else if shift < 0 {
		leftStart = -shift
	}

	overlap := minInt(len(left.energy)-leftStart, len(right.energy)-rightStart)
	if overlap < minOverlap {
		return 0, false
	}

	energyCorr := positiveCorrelation(
		left.energy[leftStart:leftStart+overlap],
		right.energy[rightStart:rightStart+overlap],
	)
	roughCorr := positiveCorrelation(
		left.roughness[leftStart:leftStart+overlap],
		right.roughness[rightStart:rightStart+overlap],
	)
	zcrCorr := positiveCorrelation(
		left.zcr[leftStart:leftStart+overlap],
		right.zcr[rightStart:rightStart+overlap],
	)

	// Envelope/rhythm alone can make unrelated tracks at the same tempo look
	// deceptively similar. The coarse gain-independent spectral profile adds
	// timbral/pitch evidence while keeping waveform alignment as the primary
	// signal.
	contentScore := 0.46*energyCorr + 0.20*roughCorr + 0.10*zcrCorr + 0.24*spectrumScore
	durationRatio := float64(minInt(len(left.energy), len(right.energy))) / float64(maxFrames)
	// Duration matters, but only modestly: an edit can still be reported
	// "similar" without being misclassified as the same full recording.
	return contentScore * (0.88 + 0.12*durationRatio), true
}

func classifyDuplicateSimilarity(similarity float64, durationDeltaMS int64) string {
	switch {
	case similarity >= 0.985 && durationDeltaMS <= duplicateVerifySameMaxDurationDeltaMS:
		return "same"
	case similarity >= 0.93 && durationDeltaMS <= duplicateVerifySimilarMaxDurationMS:
		return "similar"
	default:
		return "different"
	}
}

func duplicateSpectrumSimilarity(left, right []float64) float64 {
	if len(left) != len(right) || len(left) == 0 {
		return 0
	}

	var dot float64
	var leftSquares float64
	var rightSquares float64
	for i := range left {
		dot += left[i] * right[i]
		leftSquares += left[i] * left[i]
		rightSquares += right[i] * right[i]
	}
	if leftSquares <= 1e-12 || rightSquares <= 1e-12 {
		return 0
	}

	value := dot / math.Sqrt(leftSquares*rightSquares)
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func positiveCorrelation(left, right []float64) float64 {
	if len(left) != len(right) || len(left) < 2 {
		return 0
	}

	var leftMean float64
	var rightMean float64
	for i := range left {
		leftMean += left[i]
		rightMean += right[i]
	}
	leftMean /= float64(len(left))
	rightMean /= float64(len(right))

	var dot float64
	var leftSquares float64
	var rightSquares float64
	for i := range left {
		leftValue := left[i] - leftMean
		rightValue := right[i] - rightMean
		dot += leftValue * rightValue
		leftSquares += leftValue * leftValue
		rightSquares += rightValue * rightValue
	}
	if leftSquares <= 1e-12 || rightSquares <= 1e-12 {
		return 0
	}

	value := dot / math.Sqrt(leftSquares*rightSquares)
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func zNormalize(values []float64) []float64 {
	if len(values) == 0 {
		return values
	}
	var mean float64
	for _, value := range values {
		mean += value
	}
	mean /= float64(len(values))

	var variance float64
	for _, value := range values {
		delta := value - mean
		variance += delta * delta
	}
	variance /= float64(len(values))
	stdDev := math.Sqrt(variance)

	out := make([]float64, len(values))
	if stdDev <= 1e-12 {
		return out
	}
	for i, value := range values {
		out[i] = (value - mean) / stdDev
	}
	return out
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
