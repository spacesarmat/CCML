package library

import (
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestDuplicateQualityPrefersLosslessOverMP3320(t *testing.T) {
	t.Parallel()

	tracks := []model.Track{
		{
			ID:         1,
			Path:       "/music/a.flac",
			Extension:  ".flac",
			Codec:      "flac",
			BitRate:    900_000,
			SampleRate: 44_100,
			Channels:   2,
			Title:      "Track",
			Artist:     "Artist",
		},
		{
			ID:         2,
			Path:       "/music/a.mp3",
			Extension:  ".mp3",
			Codec:      "mp3",
			BitRate:    320_000,
			SampleRate: 44_100,
			Channels:   2,
			Title:      "Track",
			Artist:     "Artist",
			Album:      "Album",
			Genre:      "House",
			HasCover:   true,
		},
	}

	scores, recommended := scoreDuplicateTracks(tracks)
	if recommended != 1 {
		t.Fatalf("recommended track = %d, want 1 (lossless)", recommended)
	}
	if scores[0].AudioScore <= scores[1].AudioScore {
		t.Fatalf("lossless audio score %d <= mp3 audio score %d", scores[0].AudioScore, scores[1].AudioScore)
	}
}

func TestDuplicateQualityUsesMetadataForEquivalentAudio(t *testing.T) {
	t.Parallel()

	tracks := []model.Track{
		{
			ID:         10,
			Path:       "/music/a.mp3",
			Codec:      "mp3",
			BitRate:    320_000,
			SampleRate: 44_100,
			Channels:   2,
			Title:      "Track",
			Artist:     "Artist",
		},
		{
			ID:            11,
			Path:          "/music/b.mp3",
			Codec:         "mp3",
			BitRate:       320_000,
			SampleRate:    44_100,
			Channels:      2,
			Title:         "Track",
			Artist:        "Artist",
			Album:         "Album",
			AlbumArtist:   "Artist",
			Genre:         "House",
			Year:          2026,
			ISRC:          "USABC2612345",
			Label:         "Label",
			CatalogNumber: "CAT001",
			TrackNumber:   1,
			HasCover:      true,
		},
	}

	scores, recommended := scoreDuplicateTracks(tracks)
	if recommended != 11 {
		t.Fatalf("recommended track = %d, want 11", recommended)
	}
	if scores[1].MetadataScore <= scores[0].MetadataScore {
		t.Fatalf("complete metadata score %d <= sparse %d", scores[1].MetadataScore, scores[0].MetadataScore)
	}
}

func TestDuplicateQualityDoesNotPickArbitraryWinnerOnTie(t *testing.T) {
	t.Parallel()

	tracks := []model.Track{
		{
			ID:         20,
			Path:       "/music/a.mp3",
			Codec:      "mp3",
			BitRate:    320_000,
			SampleRate: 44_100,
			Channels:   2,
			Title:      "Track",
			Artist:     "Artist",
		},
		{
			ID:         21,
			Path:       "/music/b.mp3",
			Codec:      "mp3",
			BitRate:    320_000,
			SampleRate: 44_100,
			Channels:   2,
			Title:      "Track",
			Artist:     "Artist",
		},
	}

	_, recommended := scoreDuplicateTracks(tracks)
	if recommended != 0 {
		t.Fatalf("recommended track = %d, want 0 for exact quality tie", recommended)
	}
}

func TestDuplicateQualityPenalizesScanError(t *testing.T) {
	t.Parallel()

	clean := scoreDuplicateTrack(model.Track{
		ID:         30,
		Path:       "/music/a.mp3",
		Codec:      "mp3",
		BitRate:    320_000,
		SampleRate: 44_100,
		Channels:   2,
		Title:      "Track",
		Artist:     "Artist",
	})
	broken := scoreDuplicateTrack(model.Track{
		ID:         31,
		Path:       "/music/b.mp3",
		Codec:      "mp3",
		BitRate:    320_000,
		SampleRate: 44_100,
		Channels:   2,
		Title:      "Track",
		Artist:     "Artist",
		ScanError:  "probe failed",
	})

	if broken.Score >= clean.Score {
		t.Fatalf("broken score %d >= clean score %d", broken.Score, clean.Score)
	}
}
