package main

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/spacesarmat/CCML/internal/model"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SaveDJMixPlan persists the exact current route plus the scope/options needed
// to reopen and optionally rebuild it later.
func (a *App) SaveDJMixPlan(
	id int64,
	name string,
	scopeTrackIDs []int64,
	options model.DJMixPlanOptions,
	plan model.DJMixPlan,
) (model.DJMixSavedPlan, error) {
	if a.store == nil {
		return model.DJMixSavedPlan{}, errors.New("library store is not available")
	}
	if len(plan.Steps) == 0 {
		return model.DJMixSavedPlan{}, errors.New("DJ mix plan is empty")
	}
	return a.store.SaveDJMixPlan(a.context(), model.DJMixSavedPlan{
		ID:            id,
		Name:          name,
		ScopeTrackIDs: scopeTrackIDs,
		Options:       options,
		Plan:          plan,
	})
}

// ListSavedDJMixPlans returns persistent planner snapshots, newest first.
func (a *App) ListSavedDJMixPlans(limit int) ([]model.DJMixSavedPlan, error) {
	if a.store == nil {
		return nil, errors.New("library store is not available")
	}
	return a.store.ListDJMixPlans(a.context(), limit)
}

// LoadSavedDJMixPlan returns one exact persisted snapshot.
func (a *App) LoadSavedDJMixPlan(id int64) (model.DJMixSavedPlan, error) {
	if a.store == nil {
		return model.DJMixSavedPlan{}, errors.New("library store is not available")
	}
	return a.store.DJMixPlanByID(a.context(), id)
}

// DeleteSavedDJMixPlan deletes the planner record only.
func (a *App) DeleteSavedDJMixPlan(id int64) error {
	if a.store == nil {
		return errors.New("library store is not available")
	}
	return a.store.DeleteDJMixPlan(a.context(), id)
}

// ExportDJMixPlan opens the native Save dialog and writes the current route as
// M3U8 or CSV. It never changes source files or the SQLite snapshot.
func (a *App) ExportDJMixPlan(name, format string, plan model.DJMixPlan) (string, error) {
	if a.ctx == nil {
		return "", errors.New("application is not ready")
	}
	if len(plan.Steps) == 0 {
		return "", errors.New("DJ mix plan is empty")
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "m3u8" && format != "csv" {
		return "", fmt.Errorf("unsupported DJ mix export format: %s", format)
	}

	base := safeDJMixExportName(name)
	extension := "." + format
	filterName := "M3U8 playlist (*.m3u8)"
	pattern := "*.m3u8"
	if format == "csv" {
		filterName = "CSV table (*.csv)"
		pattern = "*.csv"
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export DJ Mix Plan",
		DefaultFilename: base + extension,
		Filters: []runtime.FileFilter{{
			DisplayName: filterName,
			Pattern:     pattern,
		}},
	})
	if err != nil {
		return "", fmt.Errorf("select DJ mix export file: %w", err)
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if !strings.EqualFold(filepath.Ext(path), extension) {
		path += extension
	}

	var data []byte
	switch format {
	case "m3u8":
		data = []byte(renderDJMixM3U8(plan))
	case "csv":
		raw, err := renderDJMixCSV(plan)
		if err != nil {
			return "", err
		}
		data = append([]byte{0xEF, 0xBB, 0xBF}, raw...)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write DJ mix export %q: %w", filepath.Base(path), err)
	}
	return path, nil
}

func renderDJMixM3U8(plan model.DJMixPlan) string {
	var out strings.Builder
	out.WriteString("#EXTM3U\n")
	for _, step := range plan.Steps {
		track := step.Track
		duration := int64(-1)
		if track.DurationMS > 0 {
			duration = track.DurationMS / 1000
		}
		label := strings.TrimSpace(track.Title)
		if label == "" {
			label = strings.TrimSpace(track.FileName)
		}
		artist := strings.TrimSpace(track.Artist)
		if artist != "" && label != "" {
			label = artist + " - " + label
		} else if artist != "" {
			label = artist
		}
		label = sanitizeDJMixExportText(label)
		out.WriteString("#EXTINF:")
		out.WriteString(strconv.FormatInt(duration, 10))
		out.WriteString(",")
		out.WriteString(label)
		out.WriteString("\n")
		out.WriteString("#CCML-TIMELINE-MS:")
		out.WriteString(strconv.FormatInt(step.TimelineStartMS, 10))
		out.WriteString(",")
		out.WriteString(strconv.FormatInt(step.TimelineEndMS, 10))
		out.WriteString("\n")
		if step.Locked {
			out.WriteString("#CCML-LOCKED:1\n")
		}
		if note := sanitizeDJMixExportText(step.TransitionNote); note != "" {
			out.WriteString("#CCML-TRANSITION:")
			out.WriteString(note)
			out.WriteString("\n")
		}
		if note := sanitizeDJMixExportText(step.CueNote); note != "" {
			out.WriteString("#CCML-CUE:")
			out.WriteString(note)
			out.WriteString("\n")
		}
		out.WriteString(sanitizeDJMixExportText(track.Path))
		out.WriteString("\n")
	}
	return out.String()
}

func renderDJMixCSV(plan model.DJMixPlan) ([]byte, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	writer.UseCRLF = true
	header := []string{
		"Position", "Timeline Start MS", "Timeline End MS", "Artist", "Title",
		"BPM", "Adjusted BPM", "Tempo Delta %", "Camelot", "Open Key", "Genre",
		"Energy", "Energy Delta", "Key Relation", "Genre Relation", "Score",
		"Pinned", "Locked", "Transition Note", "Cue Note", "Path",
	}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("write DJ mix CSV header: %w", err)
	}
	for _, step := range plan.Steps {
		row := []string{
			strconv.Itoa(step.Position),
			strconv.FormatInt(step.TimelineStartMS, 10),
			strconv.FormatInt(step.TimelineEndMS, 10),
			step.Track.Artist,
			step.Track.Title,
			formatDJMixExportFloat(step.Track.BPM, 2),
			formatDJMixExportFloat(step.AdjustedBPM, 2),
			formatDJMixExportFloat(step.TempoDeltaPct, 2),
			step.Camelot,
			step.OpenKey,
			step.Track.Genre,
			formatDJMixExportFloat(step.Energy, 4),
			formatDJMixExportFloat(step.EnergyDelta, 4),
			step.KeyRelation,
			step.GenreRelation,
			formatDJMixExportFloat(step.Score, 4),
			strconv.FormatBool(step.Pinned),
			strconv.FormatBool(step.Locked),
			step.TransitionNote,
			step.CueNote,
			step.Track.Path,
		}
		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("write DJ mix CSV row %d: %w", step.Position, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush DJ mix CSV: %w", err)
	}
	return buffer.Bytes(), nil
}

func formatDJMixExportFloat(value float64, precision int) string {
	return strconv.FormatFloat(value, 'f', precision, 64)
}

func sanitizeDJMixExportText(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func safeDJMixExportName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "DJ Mix Plan"
	}
	value = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		if unicode.IsControl(r) {
			return '_'
		}
		return r
	}, value)
	value = strings.Trim(value, " .")
	if value == "" {
		return "DJ Mix Plan"
	}
	runes := []rune(value)
	if len(runes) > 100 {
		value = string(runes[:100])
	}
	return value
}
