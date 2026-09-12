package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

// AnalysisOptions allows analysis to include the same pre-filters used during processing.
type AnalysisOptions struct {
	PreGainDB         float64
	RepairClipping    bool
	MultibandCompress bool
	PitchSemitones    float64
}

// Processor performs loudness analysis and non-destructive FFmpeg rendering.
type Processor struct {
	tools *Toolchain
}

// NewProcessor creates an audio processor.
func NewProcessor(tools *Toolchain) *Processor {
	return &Processor{tools: tools}
}

type loudnormJSON struct {
	InputI       string `json:"input_i"`
	InputTP      string `json:"input_tp"`
	InputLRA     string `json:"input_lra"`
	InputThresh  string `json:"input_thresh"`
	TargetOffset string `json:"target_offset"`
}

// Analyze measures integrated loudness, true peak and loudness range using EBU R128.
func (p *Processor) Analyze(ctx context.Context, input string, opts AnalysisOptions) (model.Loudness, error) {
	if err := p.requireTools(); err != nil {
		return model.Loudness{}, err
	}
	filters, err := p.prefilters(opts.PreGainDB, opts.RepairClipping, opts.MultibandCompress, opts.PitchSemitones)
	if err != nil {
		return model.Loudness{}, err
	}
	filters = append(filters, "loudnorm=I=-14:TP=-1:LRA=11:print_format=json")
	return p.runLoudnessPass(ctx, input, strings.Join(filters, ","))
}

// Process creates a processed copy. Originals are preserved by default.
func (p *Processor) Process(ctx context.Context, input string, opts model.ProcessingOptions) (model.ProcessingResult, error) {
	if err := p.requireTools(); err != nil {
		return model.ProcessingResult{}, err
	}
	if err := validateProcessingOptions(&opts); err != nil {
		return model.ProcessingResult{}, err
	}

	prefilters, err := p.prefilters(opts.PreGainDB, opts.RepairClipping, opts.MultibandCompress, opts.PitchSemitones)
	if err != nil {
		return model.ProcessingResult{}, err
	}

	firstPass := appendCopy(prefilters,
		fmt.Sprintf("loudnorm=I=%s:TP=%s:LRA=%s:print_format=json",
			ff(opts.TargetLUFS), ff(opts.TargetTruePeakDB), ff(opts.TargetLRA)))
	measurement, err := p.runLoudnessPass(ctx, input, strings.Join(firstPass, ","))
	if err != nil {
		return model.ProcessingResult{}, err
	}

	secondPass := appendCopy(prefilters, fmt.Sprintf(
		"loudnorm=I=%s:TP=%s:LRA=%s:measured_I=%s:measured_TP=%s:measured_LRA=%s:measured_thresh=%s:offset=%s:linear=true:print_format=summary",
		ff(opts.TargetLUFS), ff(opts.TargetTruePeakDB), ff(opts.TargetLRA),
		ff(measurement.InputI), ff(measurement.InputTP), ff(measurement.InputLRA),
		ff(measurement.InputThreshold), ff(measurement.TargetOffset),
	))
	if opts.Limit {
		linearLimit := math.Pow(10, opts.TargetTruePeakDB/20)
		secondPass = append(secondPass, fmt.Sprintf("alimiter=limit=%s:attack=5:release=50:level=false:latency=true", ff(linearLimit)))
	}
	filterGraph := strings.Join(secondPass, ",")

	output, tempOutput, replaceOriginal, err := prepareOutputPath(input, opts)
	if err != nil {
		return model.ProcessingResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(tempOutput), 0o755); err != nil {
		return model.ProcessingResult{}, fmt.Errorf("create output directory: %w", err)
	}

	codecArgs, err := codecArguments(filepath.Ext(output))
	if err != nil {
		return model.ProcessingResult{}, err
	}
	args := []string{"-hide_banner", "-y", "-i", input, "-map", "0:a:0", "-map", "0:v?", "-map_metadata", "0", "-c:v", "copy", "-af", filterGraph}
	args = append(args, codecArgs...)
	args = append(args, tempOutput)

	cmd := exec.CommandContext(ctx, p.tools.FFmpeg, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		removeErr := removeIfExists(tempOutput)
		baseErr := fmt.Errorf("ffmpeg processing failed: %w: %s", err, tail(stderr.String(), 4000))
		if removeErr != nil {
			return model.ProcessingResult{}, errors.Join(baseErr, removeErr)
		}
		return model.ProcessingResult{}, baseErr
	}

	if replaceOriginal {
		if err := replaceFile(tempOutput, input, opts.KeepOriginal); err != nil {
			return model.ProcessingResult{}, err
		}
		output = input
	}

	return model.ProcessingResult{
		InputPath:   input,
		OutputPath:  output,
		Measurement: measurement,
		FilterGraph: filterGraph,
	}, nil
}

// WriteReplayGain analyzes loudness and writes gain/peak tags using stream copy.
// Audio samples are not re-encoded; FFmpeg remuxes metadata into a temporary file.
func (p *Processor) WriteReplayGain(ctx context.Context, input string, targetLUFS float64) (model.Loudness, error) {
	if err := p.requireTools(); err != nil {
		return model.Loudness{}, err
	}
	if targetLUFS == 0 {
		targetLUFS = -14
	}
	measurement, err := p.Analyze(ctx, input, AnalysisOptions{})
	if err != nil {
		return model.Loudness{}, err
	}
	gainDB := targetLUFS - measurement.InputI
	peakLinear := math.Pow(10, measurement.InputTP/20)

	temp, err := tempSibling(input, ".replaygain")
	if err != nil {
		return model.Loudness{}, err
	}
	args := []string{
		"-hide_banner", "-y", "-i", input,
		"-map", "0", "-map_metadata", "0", "-c", "copy",
		"-metadata", fmt.Sprintf("REPLAYGAIN_TRACK_GAIN=%+.2f dB", gainDB),
		"-metadata", fmt.Sprintf("REPLAYGAIN_TRACK_PEAK=%.8f", peakLinear),
		temp,
	}
	cmd := exec.CommandContext(ctx, p.tools.FFmpeg, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		removeErr := removeIfExists(temp)
		baseErr := fmt.Errorf("write ReplayGain metadata: %w: %s", err, tail(stderr.String(), 3000))
		if removeErr != nil {
			return model.Loudness{}, errors.Join(baseErr, removeErr)
		}
		return model.Loudness{}, baseErr
	}
	if err := replaceFile(temp, input, false); err != nil {
		return model.Loudness{}, err
	}
	return measurement, nil
}

func (p *Processor) runLoudnessPass(ctx context.Context, input, filterGraph string) (model.Loudness, error) {
	cmd := exec.CommandContext(ctx, p.tools.FFmpeg,
		"-hide_banner", "-nostats", "-i", input,
		"-map", "0:a:0", "-af", filterGraph,
		"-f", "null", "-",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return model.Loudness{}, fmt.Errorf("ffmpeg loudness analysis: %w: %s", err, tail(stderr.String(), 4000))
	}
	payload, err := extractLastJSONObject(stderr.String())
	if err != nil {
		return model.Loudness{}, fmt.Errorf("parse loudnorm output: %w", err)
	}
	var raw loudnormJSON
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return model.Loudness{}, fmt.Errorf("decode loudnorm JSON: %w", err)
	}
	return model.Loudness{
		InputI:         parseFloat(raw.InputI),
		InputTP:        parseFloat(raw.InputTP),
		InputLRA:       parseFloat(raw.InputLRA),
		InputThreshold: parseFloat(raw.InputThresh),
		TargetOffset:   parseFloat(raw.TargetOffset),
	}, nil
}

func (p *Processor) prefilters(preGainDB float64, repair, multiband bool, pitchSemitones float64) ([]string, error) {
	filters := make([]string, 0, 4)
	if preGainDB != 0 {
		if preGainDB < -24 || preGainDB > 24 {
			return nil, fmt.Errorf("pre-gain %.2f dB is outside supported range -24..24", preGainDB)
		}
		filters = append(filters, fmt.Sprintf("volume=%sdB", ff(preGainDB)))
	}
	if repair {
		filters = append(filters, "adeclip=window=55:overlap=75:arorder=8:threshold=10:method=save")
	}
	if multiband {
		// FFmpeg's documented multi-band compander profile. The whole option
		// string is quoted because it contains commas that belong to mcompand,
		// not to the outer filter chain. Keep this preset opt-in: mastering
		// settings should ultimately be exposed as user presets.
		filters = append(filters, "mcompand='0.005,0.1 6 -47/-40,-34/-34,-17/-33 100 | 0.003,0.05 6 -47/-40,-34/-34,-17/-33 400 | 0.000625,0.0125 6 -47/-40,-34/-34,-15/-33 1600 | 0.0001,0.025 6 -47/-40,-34/-34,-31/-31,-0/-30 6400 | 0,0.025 6 -38/-31,-28/-28,-0/-25 22000'")
	}
	if pitchSemitones != 0 {
		if pitchSemitones < -12 || pitchSemitones > 12 {
			return nil, fmt.Errorf("pitch shift %.2f semitones is outside supported range -12..12", pitchSemitones)
		}
		if !p.hasFilter("rubberband") {
			return nil, errors.New("pitch shifting requires an FFmpeg build with the rubberband filter (librubberband)")
		}
		pitchScale := math.Pow(2, pitchSemitones/12)
		filters = append(filters, fmt.Sprintf("rubberband=pitch=%s:tempo=1", ff(pitchScale)))
	}
	return filters, nil
}

func (p *Processor) hasFilter(name string) bool {
	if p.tools == nil || p.tools.FFmpeg == "" {
		return false
	}
	cmd := exec.Command(p.tools.FFmpeg, "-hide_banner", "-filters")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == name {
			return true
		}
	}
	return false
}

func (p *Processor) requireTools() error {
	if p.tools == nil || !p.tools.Ready() {
		return errors.New("FFmpeg toolchain is not available")
	}
	return nil
}

func validateProcessingOptions(opts *model.ProcessingOptions) error {
	if opts.TargetLUFS == 0 {
		opts.TargetLUFS = -14
	}
	if opts.TargetTruePeakDB == 0 {
		opts.TargetTruePeakDB = -1
	}
	if opts.TargetLRA == 0 {
		opts.TargetLRA = 11
	}
	if opts.TargetLUFS < -70 || opts.TargetLUFS > -5 {
		return fmt.Errorf("target loudness %.2f LUFS is outside -70..-5", opts.TargetLUFS)
	}
	if opts.TargetTruePeakDB < -9 || opts.TargetTruePeakDB > 0 {
		return fmt.Errorf("target true peak %.2f dBTP is outside -9..0", opts.TargetTruePeakDB)
	}
	if opts.TargetLRA < 1 || opts.TargetLRA > 50 {
		return fmt.Errorf("target LRA %.2f is outside 1..50", opts.TargetLRA)
	}
	return nil
}

func prepareOutputPath(input string, opts model.ProcessingOptions) (output, tempOutput string, replaceOriginal bool, err error) {
	output = strings.TrimSpace(opts.OutputPath)
	if output == "" {
		output = filepath.Join(filepath.Dir(input), "CCML Processed", filepath.Base(input))
	}
	absIn, err := filepath.Abs(input)
	if err != nil {
		return "", "", false, fmt.Errorf("resolve input path: %w", err)
	}
	absOut, err := filepath.Abs(output)
	if err != nil {
		return "", "", false, fmt.Errorf("resolve output path: %w", err)
	}
	if filepath.Clean(absIn) == filepath.Clean(absOut) {
		tempOutput, err = tempSibling(input, ".processed")
		if err != nil {
			return "", "", false, err
		}
		return output, tempOutput, true, nil
	}
	return output, output, false, nil
}

func tempSibling(path, marker string) (string, error) {
	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	ext := filepath.Ext(path)
	file, err := os.CreateTemp(dir, base+marker+"-*"+ext)
	if err != nil {
		return "", fmt.Errorf("create temporary audio file: %w", err)
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close temporary audio file: %w", err)
	}
	if err := os.Remove(name); err != nil {
		return "", fmt.Errorf("prepare temporary audio path: %w", err)
	}
	return name, nil
}

func replaceFile(temp, destination string, keepBackup bool) error {
	backup := destination + ".ccml-backup"
	if !keepBackup {
		var err error
		backup, err = tempSibling(destination, ".rollback")
		if err != nil {
			return fmt.Errorf("prepare rollback path: %w", err)
		}
	}

	if _, err := os.Stat(backup); err == nil {
		return fmt.Errorf("backup already exists: %s", backup)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check backup path: %w", err)
	}
	if err := os.Rename(destination, backup); err != nil {
		return fmt.Errorf("stage original for replacement: %w", err)
	}

	if err := os.Rename(temp, destination); err != nil {
		restoreErr := os.Rename(backup, destination)
		if restoreErr != nil {
			return errors.Join(fmt.Errorf("replace original: %w", err), fmt.Errorf("restore original: %w", restoreErr))
		}
		return fmt.Errorf("replace original: %w", err)
	}

	if keepBackup {
		return nil
	}
	if err := os.Remove(backup); err != nil {
		return fmt.Errorf("remove rollback backup after replacement: %w", err)
	}
	return nil
}

func codecArguments(ext string) ([]string, error) {
	switch strings.ToLower(ext) {
	case ".mp3":
		return []string{"-c:a", "libmp3lame", "-q:a", "2"}, nil
	case ".flac":
		return []string{"-c:a", "flac", "-compression_level", "8"}, nil
	case ".m4a", ".mp4", ".aac":
		return []string{"-c:a", "aac", "-b:a", "256k"}, nil
	case ".wav":
		return []string{"-c:a", "pcm_s24le"}, nil
	case ".aif", ".aiff":
		return []string{"-c:a", "pcm_s24be"}, nil
	case ".ogg", ".oga":
		return []string{"-c:a", "libvorbis", "-q:a", "6"}, nil
	default:
		return nil, fmt.Errorf("unsupported output extension %q", ext)
	}
}

func extractLastJSONObject(text string) (string, error) {
	end := strings.LastIndex(text, "}")
	if end < 0 {
		return "", errors.New("no JSON object found in ffmpeg output")
	}
	start := strings.LastIndex(text[:end+1], "{")
	if start < 0 || start >= end {
		return "", errors.New("incomplete JSON object in ffmpeg output")
	}
	return text[start : end+1], nil
}

func parseFloat(raw string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0
	}
	return value
}

func ff(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func appendCopy(source []string, items ...string) []string {
	out := make([]string, len(source), len(source)+len(items))
	copy(out, source)
	return append(out, items...)
}

func tail(text string, max int) string {
	text = strings.TrimSpace(text)
	if len(text) <= max {
		return text
	}
	return text[len(text)-max:]
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove temporary file %q: %w", path, err)
	}
	return nil
}
