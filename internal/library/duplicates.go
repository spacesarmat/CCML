package library

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/spacesarmat/CCML/internal/model"
)

var duplicateBracketRE = regexp.MustCompile(`[\(\[][^\)\]]*[\)\]]`)

var duplicateVersionWords = map[string]struct{}{
	"remix": {}, "mix": {}, "edit": {}, "intro": {}, "outro": {},
	"clean": {}, "dirty": {}, "radio": {}, "extended": {}, "club": {},
	"vip": {}, "bootleg": {}, "mashup": {}, "rework": {}, "remaster": {},
	"remastered": {}, "version": {}, "instrumental": {}, "acapella": {},
	"acappella": {}, "dub": {},
}

// FindDuplicates builds non-overlapping duplicate components from three kinds
// of evidence, strongest first:
//
//  1. identical valid ISRC;
//  2. normalized Artist + Title with near-equal duration;
//  3. normalized Artist + version-stripped Title with near-equal duration.
//
// The relaxed third rule deliberately produces "possible" groups instead of
// claiming that Remix/Edit/Intro/Clean variants are identical.
func FindDuplicates(tracks []model.Track, toleranceMS int64) []model.DuplicateGroup {
	if toleranceMS <= 0 {
		toleranceMS = 2_000
	}
	if len(tracks) < 2 {
		return nil
	}

	uf := newDuplicateUnion(len(tracks))

	unionByISRC(tracks, uf)
	unionByDurationKey(tracks, toleranceMS, uf, func(track model.Track) string {
		artist := normalizeText(track.Artist)
		title := normalizeText(track.Title)
		if artist == "" || title == "" {
			return ""
		}
		return artist + "\x00" + title
	})
	unionByDurationKey(tracks, toleranceMS, uf, func(track model.Track) string {
		artist := normalizeText(track.Artist)
		title := normalizeDuplicateBaseTitle(track.Title)
		if artist == "" || title == "" {
			return ""
		}
		return artist + "\x00" + title
	})

	components := make(map[int][]model.Track)
	for i, track := range tracks {
		root := uf.find(i)
		components[root] = append(components[root], track)
	}

	groups := make([]model.DuplicateGroup, 0, len(components))
	for _, cluster := range components {
		if len(cluster) < 2 {
			continue
		}
		group := buildDuplicateGroup(cluster, toleranceMS)
		groups = append(groups, group)
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Confidence != groups[j].Confidence {
			return groups[i].Confidence > groups[j].Confidence
		}
		if len(groups[i].Tracks) != len(groups[j].Tracks) {
			return len(groups[i].Tracks) > len(groups[j].Tracks)
		}
		left := strings.ToLower(groups[i].Artist + "\x00" + groups[i].Title)
		right := strings.ToLower(groups[j].Artist + "\x00" + groups[j].Title)
		return left < right
	})
	return groups
}

func unionByISRC(tracks []model.Track, uf *duplicateUnion) {
	buckets := make(map[string][]int)
	for i, track := range tracks {
		isrc := normalizeISRC(track.ISRC)
		if isrc == "" {
			continue
		}
		buckets[isrc] = append(buckets[isrc], i)
	}
	for _, indices := range buckets {
		if len(indices) < 2 {
			continue
		}
		for _, index := range indices[1:] {
			uf.union(indices[0], index)
		}
	}
}

func unionByDurationKey(
	tracks []model.Track,
	toleranceMS int64,
	uf *duplicateUnion,
	keyFor func(model.Track) string,
) {
	buckets := make(map[string][]int)
	for i, track := range tracks {
		if track.DurationMS <= 0 {
			continue
		}
		key := keyFor(track)
		if key == "" {
			continue
		}
		buckets[key] = append(buckets[key], i)
	}

	for _, indices := range buckets {
		if len(indices) < 2 {
			continue
		}
		sort.Slice(indices, func(i, j int) bool {
			return tracks[indices[i]].DurationMS < tracks[indices[j]].DurationMS
		})

		for start := 0; start < len(indices); {
			end := start + 1
			baseDuration := tracks[indices[start]].DurationMS
			for end < len(indices) && tracks[indices[end]].DurationMS-baseDuration <= toleranceMS {
				end++
			}
			if end-start >= 2 {
				for position := start + 1; position < end; position++ {
					uf.union(indices[start], indices[position])
				}
			}
			start = end
		}
	}
}

func buildDuplicateGroup(cluster []model.Track, toleranceMS int64) model.DuplicateGroup {
	tracks := append([]model.Track(nil), cluster...)
	sort.Slice(tracks, func(i, j int) bool {
		left := strings.ToLower(tracks[i].FileName + "\x00" + tracks[i].Path)
		right := strings.ToLower(tracks[j].FileName + "\x00" + tracks[j].Path)
		return left < right
	})

	artist, title := duplicateDisplayLabel(tracks)
	spread := durationSpread(tracks)
	sharedISRC := commonISRC(tracks)
	sameArtist := sameNormalized(tracks, func(track model.Track) string { return track.Artist }, normalizeText)
	sameFullTitle := sameNormalized(tracks, func(track model.Track) string { return track.Title }, normalizeText)
	sameBaseTitle := sameNormalized(tracks, func(track model.Track) string { return track.Title }, normalizeDuplicateBaseTitle)

	matchClass := "possible"
	confidence := 0.72
	reasons := make([]string, 0, 4)

	if sharedISRC != "" {
		matchClass = "isrc"
		confidence = 1
		reasons = append(reasons, "same_isrc")
	} else if sameArtist && sameFullTitle && spread <= toleranceMS {
		matchClass = "metadata"
		confidence = 0.95
		reasons = append(reasons, "same_artist_title")
	} else {
		if sameArtist {
			reasons = append(reasons, "same_artist")
		}
		if sameBaseTitle {
			reasons = append(reasons, "version_normalized_title")
		}
		if repeatedISRCExists(tracks) {
			reasons = append(reasons, "linked_isrc")
			confidence = 0.82
		}
	}

	if spread <= toleranceMS {
		reasons = append(reasons, "duration_close")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "linked_candidates")
	}

	return model.DuplicateGroup{
		Key:              duplicateGroupKey(tracks),
		Artist:           artist,
		Title:            title,
		DurationMS:       medianDuration(tracks),
		DurationSpreadMS: spread,
		MatchClass:       matchClass,
		Confidence:       confidence,
		Reasons:          reasons,
		SharedISRC:       sharedISRC,
		Tracks:           tracks,
	}
}

func duplicateDisplayLabel(tracks []model.Track) (string, string) {
	for _, track := range tracks {
		if strings.TrimSpace(track.Artist) != "" && strings.TrimSpace(track.Title) != "" {
			return track.Artist, track.Title
		}
	}
	if len(tracks) == 0 {
		return "", ""
	}
	return tracks[0].Artist, tracks[0].Title
}

func duplicateGroupKey(tracks []model.Track) string {
	ids := make([]int64, 0, len(tracks))
	for _, track := range tracks {
		ids = append(ids, track.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return fmt.Sprintf("dup-%d-%d", ids[0], len(ids))
}

func durationSpread(tracks []model.Track) int64 {
	var min int64
	var max int64
	for _, track := range tracks {
		if track.DurationMS <= 0 {
			continue
		}
		if min == 0 || track.DurationMS < min {
			min = track.DurationMS
		}
		if track.DurationMS > max {
			max = track.DurationMS
		}
	}
	if min == 0 || max == 0 {
		return 0
	}
	return max - min
}

func commonISRC(tracks []model.Track) string {
	if len(tracks) == 0 {
		return ""
	}
	first := normalizeISRC(tracks[0].ISRC)
	if first == "" {
		return ""
	}
	for _, track := range tracks[1:] {
		if normalizeISRC(track.ISRC) != first {
			return ""
		}
	}
	return first
}

func repeatedISRCExists(tracks []model.Track) bool {
	seen := make(map[string]struct{})
	for _, track := range tracks {
		isrc := normalizeISRC(track.ISRC)
		if isrc == "" {
			continue
		}
		if _, ok := seen[isrc]; ok {
			return true
		}
		seen[isrc] = struct{}{}
	}
	return false
}

func sameNormalized(
	tracks []model.Track,
	read func(model.Track) string,
	normalize func(string) string,
) bool {
	if len(tracks) == 0 {
		return false
	}
	first := normalize(read(tracks[0]))
	if first == "" {
		return false
	}
	for _, track := range tracks[1:] {
		if normalize(read(track)) != first {
			return false
		}
	}
	return true
}

func normalizeISRC(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	normalized := builder.String()
	if len(normalized) != 12 {
		return ""
	}
	return normalized
}

func normalizeDuplicateBaseTitle(value string) string {
	value = duplicateBracketRE.ReplaceAllStringFunc(value, func(block string) string {
		if containsDuplicateVersionWord(block) {
			return " "
		}
		return block
	})

	for _, separator := range []string{" - ", " – ", " — "} {
		if index := strings.LastIndex(value, separator); index >= 0 {
			tail := value[index+len(separator):]
			if containsDuplicateVersionWord(tail) {
				value = value[:index]
				break
			}
		}
	}
	return normalizeText(value)
}

func containsDuplicateVersionWord(value string) bool {
	for _, word := range strings.Fields(normalizeText(value)) {
		if _, ok := duplicateVersionWords[word]; ok {
			return true
		}
	}
	return false
}

func normalizeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	builder.Grow(len(value))
	previousSpace := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			previousSpace = false
			continue
		}
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			if !previousSpace && builder.Len() > 0 {
				builder.WriteByte(' ')
			}
			previousSpace = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func medianDuration(tracks []model.Track) int64 {
	values := make([]int64, 0, len(tracks))
	for _, track := range tracks {
		if track.DurationMS > 0 {
			values = append(values, track.DurationMS)
		}
	}
	if len(values) == 0 {
		return 0
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values[len(values)/2]
}

type duplicateUnion struct {
	parent []int
	rank   []uint8
}

func newDuplicateUnion(size int) *duplicateUnion {
	parent := make([]int, size)
	for i := range parent {
		parent[i] = i
	}
	return &duplicateUnion{
		parent: parent,
		rank:   make([]uint8, size),
	}
}

func (u *duplicateUnion) find(value int) int {
	if u.parent[value] != value {
		u.parent[value] = u.find(u.parent[value])
	}
	return u.parent[value]
}

func (u *duplicateUnion) union(left, right int) {
	leftRoot := u.find(left)
	rightRoot := u.find(right)
	if leftRoot == rightRoot {
		return
	}
	if u.rank[leftRoot] < u.rank[rightRoot] {
		leftRoot, rightRoot = rightRoot, leftRoot
	}
	u.parent[rightRoot] = leftRoot
	if u.rank[leftRoot] == u.rank[rightRoot] {
		u.rank[leftRoot]++
	}
}
