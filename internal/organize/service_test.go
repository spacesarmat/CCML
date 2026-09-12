package organize

import (
	"strings"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestRenderRelativeMp3tagStyle(t *testing.T) {
	t.Parallel()

	track := model.Track{
		Artist:      "Massive Attack",
		Album:       "Mezzanine",
		Title:       "Teardrop",
		TrackNumber: 3,
	}
	got, err := renderRelative(track, "%artist%/%album%/%track% - %title%")
	if err != nil {
		t.Fatalf("renderRelative returned error: %v", err)
	}
	want := "Massive Attack/Mezzanine/03 - Teardrop"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSanitizeRelativeBlocksTraversal(t *testing.T) {
	t.Parallel()

	got, err := sanitizeRelative("../../CON/track:name")
	if err != nil {
		t.Fatalf("sanitizeRelative returned error: %v", err)
	}
	if strings.Contains(got, "..") {
		t.Fatalf("sanitized path still contains traversal: %q", got)
	}
	if strings.Contains(got, ":") {
		t.Fatalf("sanitized path still contains invalid colon: %q", got)
	}
}
