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

func TestClassifyDuplicateSimilarityRequiresCloseDurationForSame(t *testing.T) {
	t.Parallel()

	if got := classifyDuplicateSimilarity(0.995, 900); got != "same" {
		t.Fatalf("got %q, want same", got)
	}
	if got := classifyDuplicateSimilarity(0.995, 12_000); got != "similar" {
		t.Fatalf("got %q, want similar for large duration delta", got)
	}
	if got := classifyDuplicateSimilarity(0.70, 0); got != "different" {
		t.Fatalf("got %q, want different", got)
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
