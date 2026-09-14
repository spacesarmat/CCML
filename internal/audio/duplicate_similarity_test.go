package audio

import (
	"math"
	"testing"
)

func TestDuplicateFeatureSimilarityRecognizesGainIndependentMatch(t *testing.T) {
	t.Parallel()

	left := syntheticPCM(120, 1.0, 0)
	right := syntheticPCM(120, 0.52, 0)

	leftFeatures := extractDuplicateFeatures(left)
	rightFeatures := extractDuplicateFeatures(right)
	similarity, shift := bestDuplicateFeatureSimilarity(leftFeatures, rightFeatures)

	if similarity < 0.985 {
		t.Fatalf("similarity = %.4f, want >= 0.985", similarity)
	}
	if shift != 0 {
		t.Fatalf("shift = %d, want 0", shift)
	}
}

func TestDuplicateFeatureSimilarityFindsSmallOffset(t *testing.T) {
	t.Parallel()

	left := syntheticPCM(120, 1.0, 0)
	right := syntheticPCM(120, 1.0, 8)

	leftFeatures := extractDuplicateFeatures(left)
	rightFeatures := extractDuplicateFeatures(right)
	similarity, shift := bestDuplicateFeatureSimilarity(leftFeatures, rightFeatures)

	if similarity < 0.96 {
		t.Fatalf("similarity = %.4f, want >= 0.96", similarity)
	}
	if shift == 0 {
		t.Fatalf("expected non-zero alignment shift")
	}
}

func TestDuplicateFeatureSimilarityRejectsDifferentShape(t *testing.T) {
	t.Parallel()

	left := syntheticPCM(120, 1.0, 0)
	right := syntheticDifferentPCM(120)

	leftFeatures := extractDuplicateFeatures(left)
	rightFeatures := extractDuplicateFeatures(right)
	similarity, _ := bestDuplicateFeatureSimilarity(leftFeatures, rightFeatures)

	if similarity >= 0.93 {
		t.Fatalf("similarity = %.4f, want < 0.93", similarity)
	}
}

func TestDuplicateFeatureSimilarityRejectsSameEnvelopeDifferentPitch(t *testing.T) {
	t.Parallel()

	left := syntheticPCM(120, 1.0, 0)
	right := syntheticSameEnvelopeDifferentPitchPCM(120)

	leftFeatures := extractDuplicateFeatures(left)
	rightFeatures := extractDuplicateFeatures(right)
	similarity, _ := bestDuplicateFeatureSimilarity(leftFeatures, rightFeatures)

	if similarity >= 0.93 {
		t.Fatalf("similarity = %.4f, want < 0.93 for same envelope with different spectrum", similarity)
	}
	if spectrum := duplicateSpectrumSimilarity(leftFeatures.spectrum, rightFeatures.spectrum); spectrum >= 0.50 {
		t.Fatalf("spectrum similarity = %.4f, want clearly different profiles", spectrum)
	}
}

func TestClassifyDuplicateSimilarityRequiresCloseDurationForSame(t *testing.T) {
	t.Parallel()

	if got := classifyDuplicateSimilarity(0.995, 900); got != "same" {
		t.Fatalf("got %q, want same", got)
	}
	if got := classifyDuplicateSimilarity(0.995, 12_000); got != "similar" {
		t.Fatalf("got %q, want similar for moderate duration delta", got)
	}
	if got := classifyDuplicateSimilarity(0.70, 0); got != "different" {
		t.Fatalf("got %q, want different", got)
	}
}

func TestClassifyDuplicateSimilarityRejectsLargeDurationDelta(t *testing.T) {
	t.Parallel()

	if got := classifyDuplicateSimilarity(0.999, duplicateVerifySimilarMaxDurationMS); got != "similar" {
		t.Fatalf("boundary got %q, want similar", got)
	}
	if got := classifyDuplicateSimilarity(0.999, duplicateVerifySimilarMaxDurationMS+1); got != "different" {
		t.Fatalf("over-boundary got %q, want different", got)
	}
	if got := classifyDuplicateSimilarity(1.0, 180_000); got != "different" {
		t.Fatalf("three-minute delta got %q, want different", got)
	}
}

func syntheticPCM(seconds int, gain float64, offsetFrames int) []int16 {
	sampleRate := duplicateVerifySampleRate
	total := seconds * sampleRate
	offsetSamples := offsetFrames * duplicateVerifyFrameSize
	out := make([]int16, total+offsetSamples)

	for i := 0; i < total; i++ {
		t := float64(i) / float64(sampleRate)
		envelope := 0.25 +
			0.20*math.Sin(2*math.Pi*0.17*t) +
			0.10*math.Sin(2*math.Pi*0.041*t)
		carrier := math.Sin(2 * math.Pi * (170 + 25*math.Sin(2*math.Pi*0.023*t)) * t)
		value := gain * envelope * carrier
		if value > 0.95 {
			value = 0.95
		}
		if value < -0.95 {
			value = -0.95
		}
		out[i+offsetSamples] = int16(value * 32767)
	}
	return out
}

func syntheticSameEnvelopeDifferentPitchPCM(seconds int) []int16 {
	sampleRate := duplicateVerifySampleRate
	total := seconds * sampleRate
	out := make([]int16, total)

	for i := 0; i < total; i++ {
		t := float64(i) / float64(sampleRate)
		envelope := 0.25 +
			0.20*math.Sin(2*math.Pi*0.17*t) +
			0.10*math.Sin(2*math.Pi*0.041*t)
		carrier := math.Sin(2 * math.Pi * (690 + 55*math.Sin(2*math.Pi*0.023*t)) * t)
		value := envelope * carrier
		if value > 0.95 {
			value = 0.95
		}
		if value < -0.95 {
			value = -0.95
		}
		out[i] = int16(value * 32767)
	}
	return out
}

func syntheticDifferentPCM(seconds int) []int16 {
	sampleRate := duplicateVerifySampleRate
	total := seconds * sampleRate
	out := make([]int16, total)

	for i := 0; i < total; i++ {
		t := float64(i) / float64(sampleRate)
		envelope := 0.18 + 0.13*math.Sin(2*math.Pi*0.31*t)
		carrier := math.Sin(2 * math.Pi * (530 + 90*math.Sin(2*math.Pi*0.071*t)) * t)
		value := envelope * carrier
		out[i] = int16(value * 32767)
	}
	return out
}
