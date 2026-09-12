package audio

import (
	"os"
	"path/filepath"
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
