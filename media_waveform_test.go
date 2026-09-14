package main

import (
	"math"
	"testing"
)

func TestClampWaveformBuckets(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{0, waveformDefaultBuckets},
		{1, waveformMinBuckets},
		{waveformMinBuckets, waveformMinBuckets},
		{900, 900},
		{waveformMaxBuckets, waveformMaxBuckets},
		{waveformMaxBuckets + 500, waveformMaxBuckets},
	}

	for _, test := range tests {
		if got := clampWaveformBuckets(test.input); got != test.want {
			t.Fatalf("clampWaveformBuckets(%d) = %d, want %d", test.input, got, test.want)
		}
	}
}

func TestWaveformBucketForSample(t *testing.T) {
	if got := waveformBucketForSample(0, 1000, 10); got != 0 {
		t.Fatalf("first sample bucket = %d, want 0", got)
	}
	if got := waveformBucketForSample(500, 1000, 10); got != 5 {
		t.Fatalf("middle sample bucket = %d, want 5", got)
	}
	if got := waveformBucketForSample(999, 1000, 10); got != 9 {
		t.Fatalf("last expected sample bucket = %d, want 9", got)
	}
	if got := waveformBucketForSample(2000, 1000, 10); got != 9 {
		t.Fatalf("overflow sample bucket = %d, want 9", got)
	}
}

func TestNormalizeWaveformPeaks(t *testing.T) {
	peaks := []float64{0, 2, 1, math.NaN(), math.Inf(1)}
	normalizeWaveformPeaks(peaks)

	want := []float64{0, 1, 0.5, 0, 0}
	for i := range want {
		if math.Abs(peaks[i]-want[i]) > 0.000001 {
			t.Fatalf("peak %d = %.6f, want %.6f", i, peaks[i], want[i])
		}
	}
}
