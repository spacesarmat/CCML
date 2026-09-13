package metadata

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

var leadingTrackNumber = regexp.MustCompile(`^\s*\d{1,3}\s*[-._)]\s*`)

// QueryFromTrack builds a provider query from indexed metadata and falls back
// to a conservative Artist - Title filename parser for untagged files.
func QueryFromTrack(track model.Track, isrc string) model.MetadataQuery {
	title := strings.TrimSpace(track.Title)
	artist := strings.TrimSpace(track.Artist)
	album := strings.TrimSpace(track.Album)

	if title == "" || artist == "" {
		fileBase := strings.TrimSpace(strings.TrimSuffix(track.FileName, filepath.Ext(track.FileName)))
		fileBase = leadingTrackNumber.ReplaceAllString(fileBase, "")
		if parsedArtist, parsedTitle, ok := splitArtistTitle(fileBase); ok {
			if artist == "" {
				artist = parsedArtist
			}
			if title == "" {
				title = parsedTitle
			}
		} else if title == "" {
			title = fileBase
		}
	}

	return model.MetadataQuery{
		Title: title, Artist: artist, Album: album,
		DurationMS: track.DurationMS, ISRC: strings.TrimSpace(isrc),
	}
}

func splitArtistTitle(value string) (artist, title string, ok bool) {
	for _, separator := range []string{" - ", " – ", " — "} {
		parts := strings.SplitN(value, separator, 2)
		if len(parts) != 2 {
			continue
		}
		artist = strings.TrimSpace(parts[0])
		title = strings.TrimSpace(parts[1])
		if artist != "" && title != "" {
			return artist, title, true
		}
	}
	return "", "", false
}
