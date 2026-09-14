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
		},
	}}
	got := renderDJMixM3U8(plan)
	for _, want := range []string{"#EXTM3U\n", "#EXTINF:185,Artist - One\n", "C:\\Music\\One.mp3\n"} {
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
			AdjustedBPM: 128.5,
			Camelot:     "8A",
			OpenKey:     "1m",
			Energy:      0.7,
			Score:       0.9,
		},
	}}
	raw, err := renderDJMixCSV(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range [][]byte{
		[]byte("Position,Artist,Title"),
		[]byte(`"A, ""DJ"""`),
		[]byte("8A,1m,House"),
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
