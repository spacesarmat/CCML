package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestRenderDJMixM3U8(t *testing.T) {
	t.Parallel()

	plan := model.DJMixPlan{Steps: []model.DJMixPlanStep{
		{
			Position: 1,
			Track: model.Track{
				Path: "C:\\Music\\One.mp3", Artist: "Artist", Title: "One", DurationMS: 185900,
			},
			TimelineStartMS: 0,
			TimelineEndMS:   185900,
			Locked:          true,
			TransitionNote:  "blend 16 bars",
			CueNote:         "vocal starts 0:32",
		},
	}}
	got := renderDJMixM3U8(plan)
	for _, want := range []string{
		"#EXTM3U\n",
		"#EXTINF:185,Artist - One\n",
		"#CCML-TIMELINE-MS:0,185900\n",
		"#CCML-LOCKED:1\n",
		"#CCML-TRANSITION:blend 16 bars\n",
		"#CCML-CUE:vocal starts 0:32\n",
		"C:\\Music\\One.mp3\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("M3U8 missing %q:\n%s", want, got)
		}
	}
}

func TestRenderDJMixCSV(t *testing.T) {
	t.Parallel()

	plan := model.DJMixPlan{Steps: []model.DJMixPlanStep{
		{
			Position: 1,
			Track: model.Track{
				Path: "C:\\Music\\One.mp3", Artist: `A, "DJ"`, Title: "One", BPM: 128.5, Genre: "House",
			},
			AdjustedBPM:     128.5,
			Camelot:         "8A",
			OpenKey:         "1m",
			Energy:          0.7,
			Score:           0.9,
			TimelineStartMS: 30000,
			TimelineEndMS:   215900,
			Locked:          true,
			TransitionNote:  "long blend",
			CueNote:         "cue A",
		},
	}}
	raw, err := renderDJMixCSV(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range [][]byte{
		[]byte("Position,Timeline Start MS,Timeline End MS,Artist,Title"),
		[]byte(`"A, ""DJ"""`),
		[]byte("8A,1m,House"),
		[]byte("true,long blend,cue A"),
		[]byte(`C:\Music\One.mp3`),
	} {
		if !bytes.Contains(raw, want) {
			t.Fatalf("CSV missing %q:\n%s", want, raw)
		}
	}
}

func TestSafeDJMixExportName(t *testing.T) {
	t.Parallel()

	got := safeDJMixExportName(` Friday: Set / Main? `)
	if strings.ContainsAny(got, `<>:"/\|?*`) {
		t.Fatalf("unsafe filename = %q", got)
	}
	if got == "" {
		t.Fatal("safe filename is empty")
	}
}
