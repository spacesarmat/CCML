package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

const (
	waveformVersion        = "waveform-peaks-v1"
	waveformSampleRate     = 800
	waveformDefaultBuckets = 900
	waveformMinBuckets     = 256
	waveformMaxBuckets     = 2048
)

// PrepareTrackWaveform returns a cached full-track peak envelope for the DJ
// Mix Planner. The first request decodes low-rate mono PCM through FFmpeg;
// subsequent requests are served from CCML's private media cache.
func (a *App) PrepareTrackWaveform(trackID int64, buckets int) (model.TrackWaveform, error) {
	if a.media == nil {
		return model.TrackWaveform{}, errors.New("media service is not available")
	}
	return a.media.PrepareWaveform(a.context(), trackID, buckets)
}

func (m *mediaService) PrepareWaveform(ctx context.Context, trackID int64, buckets int) (model.TrackWaveform, error) {
	track, err := m.store.TrackByID(ctx, trackID)
	if err != nil {
		return model.TrackWaveform{}, err
	}
	if m.tools == nil || !m.tools.Ready() {
		return model.TrackWaveform{}, errors.New("FFmpeg toolchain is not available for waveform generation")
	}

	buckets = clampWaveformBuckets(buckets)

	m.mu.Lock()
	defer m.mu.Unlock()

	result, err := m.ensureWaveform(ctx, track.ID, track.Path, track.DurationMS, buckets)
	if err != nil {
		return model.TrackWaveform{}, err
	}
	return result, nil
}

func (m *mediaService) ensureWaveform(
	ctx context.Context,
	trackID int64,
	input string,
	durationMS int64,
	buckets int,
) (model.TrackWaveform, error) {
	profile := fmt.Sprintf("%s-%d-%d", waveformVersion, waveformSampleRate, buckets)
	key, err := fileCacheKey(input, profile)
	if err != nil {
		return model.TrackWaveform{}, err
	}

	dir := filepath.Join(m.cacheDir, "waveform")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return model.TrackWaveform{}, fmt.Errorf("create waveform cache: %w", err)
	}
	cachePath := filepath.Join(dir, key+".json")

	if cached, ok := readWaveformCache(cachePath, trackID, buckets); ok {
		return cached, nil
	}

	result, err := decodeWaveform(ctx, m.tools.FFmpegPath(), trackID, input, durationMS, buckets)
	if err != nil {
		return model.TrackWaveform{}, err
	}

	data, err := json.Marshal(result)
	if err != nil {
		return model.TrackWaveform{}, fmt.Errorf("encode waveform cache: %w", err)
	}
	if err := writeCacheFile(cachePath, data); err != nil {
		return model.TrackWaveform{}, fmt.Errorf("write waveform cache: %w", err)
	}
	return result, nil
}

func readWaveformCache(path string, trackID int64, buckets int) (model.TrackWaveform, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.TrackWaveform{}, false
	}
	var result model.TrackWaveform
	if err := json.Unmarshal(data, &result); err != nil {
		return model.TrackWaveform{}, false
	}
	if result.TrackID != trackID || result.BucketCount != buckets || len(result.Peaks) != buckets {
		return model.TrackWaveform{}, false
	}
	for _, peak := range result.Peaks {
		if math.IsNaN(peak) || math.IsInf(peak, 0) || peak < 0 || peak > 1 {
			return model.TrackWaveform{}, false
		}
	}
	return result, true
}

func decodeWaveform(
	ctx context.Context,
	ffmpegPath string,
	trackID int64,
	input string,
	durationMS int64,
	buckets int,
) (model.TrackWaveform, error) {
	cmd := exec.CommandContext(
		ctx,
		ffmpegPath,
		"-hide_banner", "-loglevel", "error",
		"-i", input,
		"-map", "0:a:0", "-vn", "-sn", "-dn",
		"-ac", "1",
		"-ar", fmt.Sprintf("%d", waveformSampleRate),
		"-f", "f32le",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return model.TrackWaveform{}, fmt.Errorf("prepare waveform stdout: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return model.TrackWaveform{}, fmt.Errorf("start waveform decode: %w", err)
	}

	peaks := make([]float64, buckets)
	expectedSamples := int64(math.Round(float64(durationMS) * float64(waveformSampleRate) / 1000))
	if expectedSamples <= 0 {
		expectedSamples = int64(buckets)
	}

	reader := bufio.NewReaderSize(stdout, 64*1024)
	var sampleIndex int64
	var readErr error

	for {
		var sample float32
		err := binary.Read(reader, binary.LittleEndian, &sample)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			readErr = err
			break
		}

		amplitude := math.Abs(float64(sample))
		if !math.IsNaN(amplitude) && !math.IsInf(amplitude, 0) {
			bucket := waveformBucketForSample(sampleIndex, expectedSamples, buckets)
			if amplitude > peaks[bucket] {
				peaks[bucket] = amplitude
			}
		}
		sampleIndex++
	}

	if readErr != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		return model.TrackWaveform{}, fmt.Errorf("read waveform PCM: %w", readErr)
	}

	if err := cmd.Wait(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return model.TrackWaveform{}, fmt.Errorf("generate waveform: %w: %s", err, message)
	}
	if sampleIndex == 0 {
		return model.TrackWaveform{}, errors.New("generate waveform: decoded audio contained no samples")
	}

	normalizeWaveformPeaks(peaks)

	actualDurationMS := durationMS
	if actualDurationMS <= 0 {
		actualDurationMS = sampleIndex * 1000 / waveformSampleRate
	}

	return model.TrackWaveform{
		TrackID:     trackID,
		DurationMS:  actualDurationMS,
		SampleRate:  waveformSampleRate,
		BucketCount: buckets,
		Peaks:       peaks,
	}, nil
}

func clampWaveformBuckets(value int) int {
	if value <= 0 {
		return waveformDefaultBuckets
	}
	if value < waveformMinBuckets {
		return waveformMinBuckets
	}
	if value > waveformMaxBuckets {
		return waveformMaxBuckets
	}
	return value
}

func waveformBucketForSample(sampleIndex, expectedSamples int64, buckets int) int {
	if buckets <= 1 || expectedSamples <= 1 || sampleIndex <= 0 {
		return 0
	}
	bucket := int(sampleIndex * int64(buckets) / expectedSamples)
	if bucket < 0 {
		return 0
	}
	if bucket >= buckets {
		return buckets - 1
	}
	return bucket
}

func normalizeWaveformPeaks(peaks []float64) {
	maxPeak := 0.0
	for _, peak := range peaks {
		if math.IsNaN(peak) || math.IsInf(peak, 0) || peak < 0 {
			continue
		}
		if peak > maxPeak {
			maxPeak = peak
		}
	}
	if maxPeak <= 0 {
		for i := range peaks {
			peaks[i] = 0
		}
		return
	}

	for i, peak := range peaks {
		if math.IsNaN(peak) || math.IsInf(peak, 0) || peak <= 0 {
			peaks[i] = 0
			continue
		}
		value := peak / maxPeak
		if value > 1 {
			value = 1
		}
		peaks[i] = value
	}
}
