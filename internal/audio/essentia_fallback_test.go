package audio

import "testing"

func TestEssentiaNeedsFFmpegFallbackForDecoderFailures(t *testing.T) {
	t.Parallel()

	for _, output := range []string{
		"AudioLoader: Error reading frame: Error number -5 occurred",
		"ERROR: File looks like a completely silent file... Aborting...",
		"audioLoader: decoder failed",
	} {
		if !essentiaNeedsFFmpegFallback(output) {
			t.Fatalf("expected fallback for %q", output)
		}
	}
}

func TestEssentiaDoesNotRetryUnrelatedFailure(t *testing.T) {
	t.Parallel()

	if essentiaNeedsFFmpegFallback("ERROR: invalid profile configuration") {
		t.Fatal("unrelated Essentia failure must not trigger FFmpeg retry")
	}
}

func TestEssentiaAnalyzerUsesCurrentToolchainFFmpeg(t *testing.T) {
	t.Parallel()

	tools := &Toolchain{ffmpeg: "first-ffmpeg"}
	analyzer := NewEssentiaAnalyzer()
	analyzer.SetToolchain(tools)
	if got := analyzer.ffmpegPath(); got != "first-ffmpeg" {
		t.Fatalf("ffmpeg path = %q", got)
	}
	tools.Replace("second-ffmpeg", "ffprobe", "test", "", toolSourceManaged)
	if got := analyzer.ffmpegPath(); got != "second-ffmpeg" {
		t.Fatalf("updated ffmpeg path = %q", got)
	}
}
