package audio

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestValidateProcessingOptionsDefaults(t *testing.T) {
	opts := model.ProcessingOptions{}
	if err := validateProcessingOptions(&opts); err != nil {
		t.Fatalf("validateProcessingOptions() error = %v", err)
	}
	if opts.TargetLUFS != -14 {
		t.Fatalf("TargetLUFS = %v, want -14", opts.TargetLUFS)
	}
	if opts.TargetTruePeakDB != -1 {
		t.Fatalf("TargetTruePeakDB = %v, want -1", opts.TargetTruePeakDB)
	}
	if opts.TargetLRA != 11 {
		t.Fatalf("TargetLRA = %v, want 11", opts.TargetLRA)
	}
}

func TestExtractLastJSONObject(t *testing.T) {
	input := "ffmpeg log\n{\"input_i\":\"-12.3\"}\nmore log\n{\"input_i\":\"-10.2\",\"target_offset\":\"0.1\"}\n"
	got, err := extractLastJSONObject(input)
	if err != nil {
		t.Fatalf("extractLastJSONObject() error = %v", err)
	}
	if !strings.Contains(got, `"input_i":"-10.2"`) {
		t.Fatalf("extractLastJSONObject() = %q, want last JSON object", got)
	}
}

func TestPrefiltersMultibandQuotesInternalCommas(t *testing.T) {
	processor := NewProcessor(&Toolchain{})
	filters, err := processor.prefilters(0, false, true, 0)
	if err != nil {
		t.Fatalf("prefilters() error = %v", err)
	}
	if len(filters) != 1 {
		t.Fatalf("len(filters) = %d, want 1", len(filters))
	}
	if !strings.HasPrefix(filters[0], "mcompand='") || !strings.HasSuffix(filters[0], "'") {
		t.Fatalf("mcompand filter is not quoted: %q", filters[0])
	}
}

func TestReplaceFileKeepsBackupWhenRequested(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "track.wav")
	temp := filepath.Join(dir, "processed.wav")
	if err := os.WriteFile(destination, []byte("original"), 0o644); err != nil {
		t.Fatalf("write destination: %v", err)
	}
	if err := os.WriteFile(temp, []byte("processed"), 0o644); err != nil {
		t.Fatalf("write temp: %v", err)
	}

	if err := replaceFile(temp, destination, true); err != nil {
		t.Fatalf("replaceFile() error = %v", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(got) != "processed" {
		t.Fatalf("destination = %q, want processed", got)
	}
	backup, err := os.ReadFile(destination + ".ccml-backup")
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backup) != "original" {
		t.Fatalf("backup = %q, want original", backup)
	}
}

func TestBuildLoudnormRenderFilterUsesTwoPassForValidMeasurement(t *testing.T) {
	opts := model.ProcessingOptions{TargetLUFS: -14, TargetTruePeakDB: -1, TargetLRA: 11}
	measurement := model.Loudness{
		InputI: -8.2, InputTP: 0.3, InputLRA: 4.1, InputThreshold: -18.4, TargetOffset: -0.1,
	}
	got := buildLoudnormRenderFilter(opts, measurement)
	if !strings.Contains(got, "measured_I=-8.200000") || !strings.Contains(got, "linear=true") {
		t.Fatalf("expected valid two-pass filter, got %q", got)
	}
}

func TestBuildLoudnormRenderFilterFallsBackWhenMeasuredIIsPositive(t *testing.T) {
	opts := model.ProcessingOptions{TargetLUFS: -14, TargetTruePeakDB: -1, TargetLRA: 11}
	measurement := model.Loudness{
		InputI: 29.95, InputTP: 30.2, InputLRA: 4.1, InputThreshold: 19.5, TargetOffset: -0.1,
	}
	got := buildLoudnormRenderFilter(opts, measurement)
	if strings.Contains(got, "measured_I=") {
		t.Fatalf("invalid measured_I must not be forwarded to FFmpeg: %q", got)
	}
	if !strings.Contains(got, "linear=false") {
		t.Fatalf("expected dynamic loudnorm fallback, got %q", got)
	}
}

func TestValidLoudnormTwoPassMeasurementRejectsNonFiniteValues(t *testing.T) {
	measurement := model.Loudness{
		InputI: -8, InputTP: 0, InputLRA: 4, InputThreshold: -18, TargetOffset: 0,
	}
	measurement.InputI = math.Inf(-1)
	if validLoudnormTwoPassMeasurement(measurement) {
		t.Fatal("expected -Inf measurement to be rejected")
	}
}

func TestParseLoudnormFloatRejectsInfinity(t *testing.T) {
	value, ok := parseLoudnormFloat("inf", -100)
	if ok {
		t.Fatal("expected infinity to be rejected")
	}
	if value != -100 {
		t.Fatalf("fallback = %v, want -100", value)
	}
}

func TestParseLoudnormFloatRejectsNaN(t *testing.T) {
	value, ok := parseLoudnormFloat("nan", -100)
	if ok {
		t.Fatal("expected NaN to be rejected")
	}
	if value != -100 {
		t.Fatalf("fallback = %v, want -100", value)
	}
}

func TestProcessingFFmpegArgsPreserveArtworkMetadataAndChapters(t *testing.T) {
	args := processingFFmpegArgs("input.mp3", "output.mp3", "loudnorm=I=-14:TP=-1:LRA=11", []string{"-c:a", "libmp3lame", "-q:a", "2"})
	joined := strings.Join(args, " ")
	for _, want := range []string{"-map 0:a:0", "-map 0:v?", "-map_metadata 0", "-map_chapters 0", "-c:v copy"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("processing args %q do not contain %q", joined, want)
		}
	}
}

func TestValidateRenderedFileRejectsEmptyOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.mp3")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	if err := validateRenderedFile(path); err == nil {
		t.Fatal("validateRenderedFile() expected an error for an empty output")
	}
}

func TestProcessIntegrationNormalizesMP3AndPreservesCover(t *testing.T) {
	ffmpeg, ffprobe := testFFmpegPair(t)
	if ffmpeg == "" || ffprobe == "" {
		t.Skip("FFmpeg/ffprobe pair not available for integration test")
	}

	dir := t.TempDir()
	audioOnly := filepath.Join(dir, "audio.mp3")
	cover := filepath.Join(dir, "cover.jpg")
	input := filepath.Join(dir, "input.mp3")
	output := filepath.Join(dir, "processed.mp3")

	generateAudio := exec.Command(ffmpeg,
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "sine=frequency=997:sample_rate=44100:duration=10",
		"-c:a", "libmp3lame", "-b:a", "192k", audioOnly,
	)
	if payload, err := generateAudio.CombinedOutput(); err != nil {
		t.Fatalf("generate integration audio: %v: %s", err, payload)
	}
	generateCover := exec.Command(ffmpeg,
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "color=c=blue:s=128x128",
		"-frames:v", "1", cover,
	)
	if payload, err := generateCover.CombinedOutput(); err != nil {
		t.Fatalf("generate integration cover: %v: %s", err, payload)
	}
	mux := exec.Command(ffmpeg,
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", audioOnly, "-i", cover,
		"-map", "0:a:0", "-map", "1:v:0",
		"-c:a", "copy", "-c:v", "mjpeg",
		"-disposition:v:0", "attached_pic",
		"-metadata", "title=CCML loudnorm integration test", input,
	)
	if payload, err := mux.CombinedOutput(); err != nil {
		t.Fatalf("mux integration MP3 artwork: %v: %s", err, payload)
	}

	tools := newToolchain(ffmpeg, ffprobe, "test", "", toolSourcePath)
	processor := NewProcessor(tools)
	result, err := processor.Process(context.Background(), input, model.ProcessingOptions{
		OutputPath:       output,
		TargetLUFS:       -14,
		TargetTruePeakDB: -1,
		TargetLRA:        11,
		Limit:            true,
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if result.OutputPath != output {
		t.Fatalf("OutputPath = %q, want %q", result.OutputPath, output)
	}
	if math.IsNaN(result.Measurement.InputI) || math.IsInf(result.Measurement.InputI, 0) {
		t.Fatalf("output loudness is non-finite: %+v", result.Measurement)
	}
	if delta := math.Abs(result.Measurement.InputI - (-14)); delta > 1.5 {
		t.Fatalf("output loudness = %.2f LUFS, want close to -14 (delta %.2f)", result.Measurement.InputI, delta)
	}

	probe := exec.Command(ffprobe,
		"-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=codec_type", "-of", "default=nw=1:nk=1", output,
	)
	payload, err := probe.Output()
	if err != nil {
		t.Fatalf("ffprobe cover stream: %v", err)
	}
	if strings.TrimSpace(string(payload)) != "video" {
		t.Fatalf("processed MP3 lost attached artwork; ffprobe output = %q", payload)
	}
}

func testFFmpegPair(t *testing.T) (string, string) {
	t.Helper()
	rootTools := filepath.Join("..", "..", "tools")
	ffmpegName := "ffmpeg"
	ffprobeName := "ffprobe"
	if runtime.GOOS == "windows" {
		ffmpegName += ".exe"
		ffprobeName += ".exe"
	}
	bundledFFmpeg := filepath.Join(rootTools, ffmpegName)
	bundledFFprobe := filepath.Join(rootTools, ffprobeName)
	if _, err := os.Stat(bundledFFmpeg); err == nil {
		if _, err := os.Stat(bundledFFprobe); err == nil {
			return bundledFFmpeg, bundledFFprobe
		}
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", ""
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		return "", ""
	}
	return ffmpeg, ffprobe
}
