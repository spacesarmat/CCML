package audio

import (
	"strconv"
	"strings"
)

// DJKeyFormats normalizes musical, Camelot, or Open Key notation into both DJ
// wheel formats. Camelot uses A for minor/B for major; Open Key uses m/d.
func DJKeyFormats(key, scale string) (camelot string, openKey string, ok bool) {
	key = normalizeKeyToken(key)
	scale = strings.ToLower(strings.TrimSpace(scale))
	if key == "" {
		return "", "", false
	}

	if number, suffix, parsed := parseWheelCode(key, "AB"); parsed {
		camelot = strconv.Itoa(number) + string(suffix)
		return camelot, camelotToOpenKey(number, suffix), true
	}
	if number, suffix, parsed := parseWheelCode(strings.ToLower(key), "md"); parsed {
		openKey = strconv.Itoa(number) + string(suffix)
		camelotNumber := ((number + 6) % 12) + 1
		camelotSuffix := byte('A')
		if suffix == 'd' {
			camelotSuffix = 'B'
		}
		return strconv.Itoa(camelotNumber) + string(camelotSuffix), openKey, true
	}

	note, mode := splitMusicalKey(key, scale)
	pitch, found := musicalPitchClass(note)
	if !found {
		return "", "", false
	}
	major := normalizeKeyMode(mode) == "major"
	minor := normalizeKeyMode(mode) == "minor"
	if !major && !minor {
		return "", "", false
	}

	majorCamelot := [12]int{8, 3, 10, 5, 12, 7, 2, 9, 4, 11, 6, 1}
	minorCamelot := [12]int{5, 12, 7, 2, 9, 4, 11, 6, 1, 8, 3, 10}
	number := minorCamelot[pitch]
	suffix := byte('A')
	if major {
		number = majorCamelot[pitch]
		suffix = 'B'
	}
	camelot = strconv.Itoa(number) + string(suffix)
	return camelot, camelotToOpenKey(number, suffix), true
}

func normalizeKeyToken(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "♯", "#")
	value = strings.ReplaceAll(value, "♭", "b")
	return value
}

func parseWheelCode(value, suffixes string) (int, byte, bool) {
	value = strings.TrimSpace(value)
	if len(value) < 2 || len(value) > 3 {
		return 0, 0, false
	}
	suffix := value[len(value)-1]
	if !strings.ContainsRune(suffixes, rune(suffix)) {
		return 0, 0, false
	}
	number, err := strconv.Atoi(value[:len(value)-1])
	if err != nil || number < 1 || number > 12 {
		return 0, 0, false
	}
	return number, suffix, true
}

func camelotToOpenKey(number int, suffix byte) string {
	openNumber := ((number + 4) % 12) + 1
	openSuffix := byte('m')
	if suffix == 'B' {
		openSuffix = 'd'
	}
	return strconv.Itoa(openNumber) + string(openSuffix)
}

func splitMusicalKey(key, scale string) (string, string) {
	if normalizeKeyMode(scale) != "" {
		return key, scale
	}
	lower := strings.ToLower(strings.TrimSpace(key))
	for _, suffix := range []struct {
		text string
		mode string
	}{
		{" major", "major"}, {" maj", "major"}, {"major", "major"}, {"maj", "major"},
		{" minor", "minor"}, {" min", "minor"}, {"minor", "minor"}, {"min", "minor"},
	} {
		if strings.HasSuffix(lower, suffix.text) {
			note := strings.TrimSpace(key[:len(key)-len(suffix.text)])
			if note != "" {
				return note, suffix.mode
			}
		}
	}
	if len(key) >= 2 && key[len(key)-1] == 'm' {
		return strings.TrimSpace(key[:len(key)-1]), "minor"
	}
	return key, scale
}

func normalizeKeyMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "major", "maj", "dur", "d":
		return "major"
	case "minor", "min", "moll", "m":
		return "minor"
	default:
		return ""
	}
}

func musicalPitchClass(note string) (int, bool) {
	note = normalizeKeyToken(note)
	if note == "" {
		return 0, false
	}
	note = strings.ToUpper(note[:1]) + note[1:]
	pitches := map[string]int{
		"C": 0, "B#": 0,
		"C#": 1, "Db": 1,
		"D":  2,
		"D#": 3, "Eb": 3,
		"E": 4, "Fb": 4,
		"F": 5, "E#": 5,
		"F#": 6, "Gb": 6,
		"G":  7,
		"G#": 8, "Ab": 8,
		"A":  9,
		"A#": 10, "Bb": 10,
		"B": 11, "Cb": 11,
	}
	pitch, ok := pitches[note]
	return pitch, ok
}
