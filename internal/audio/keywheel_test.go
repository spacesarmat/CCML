package audio

import "testing"

func TestDJKeyFormatsMusicalKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key, scale, camelot, open string
	}{
		{"C", "major", "8B", "1d"},
		{"A", "minor", "8A", "1m"},
		{"G#", "minor", "1A", "6m"},
		{"F#", "major", "2B", "7d"},
		{"Db", "major", "3B", "8d"},
	}
	for _, tt := range tests {
		camelot, openKey, ok := DJKeyFormats(tt.key, tt.scale)
		if !ok || camelot != tt.camelot || openKey != tt.open {
			t.Fatalf("DJKeyFormats(%q, %q) = %q, %q, %v; want %q, %q, true",
				tt.key, tt.scale, camelot, openKey, ok, tt.camelot, tt.open)
		}
	}
}

func TestDJKeyFormatsNormalizesWheelNotation(t *testing.T) {
	t.Parallel()

	if camelot, openKey, ok := DJKeyFormats("8A", "camelot"); !ok || camelot != "8A" || openKey != "1m" {
		t.Fatalf("Camelot normalization = %q, %q, %v", camelot, openKey, ok)
	}
	if camelot, openKey, ok := DJKeyFormats("1d", "openkey"); !ok || camelot != "8B" || openKey != "1d" {
		t.Fatalf("Open Key normalization = %q, %q, %v", camelot, openKey, ok)
	}
}

func TestDJKeyFormatsRejectsUnknownKey(t *testing.T) {
	t.Parallel()

	if camelot, openKey, ok := DJKeyFormats("H", "minor"); ok || camelot != "" || openKey != "" {
		t.Fatalf("unexpected normalization = %q, %q, %v", camelot, openKey, ok)
	}
}

func TestHarmonicCamelotKeys(t *testing.T) {
	t.Parallel()

	got := HarmonicCamelotKeys("8A")
	want := []string{"8A", "7A", "9A", "8B"}
	if len(got) != len(want) {
		t.Fatalf("compatibility = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("compatibility = %+v, want %+v", got, want)
		}
	}

	wrapped := HarmonicCamelotKeys("1B")
	wrappedWant := []string{"1B", "12B", "2B", "1A"}
	for i := range wrappedWant {
		if wrapped[i] != wrappedWant[i] {
			t.Fatalf("wrapped compatibility = %+v, want %+v", wrapped, wrappedWant)
		}
	}

	if got := HarmonicCamelotKeys("not-a-key"); len(got) != 0 {
		t.Fatalf("invalid compatibility = %+v", got)
	}
}
