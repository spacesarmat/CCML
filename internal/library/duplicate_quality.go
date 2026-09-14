package library

import (
	"path/filepath"
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

func scoreDuplicateTracks(tracks []model.Track) ([]model.DuplicateTrackQuality, int64) {
	scores := make([]model.DuplicateTrackQuality, 0, len(tracks))
	for _, track := range tracks {
		scores = append(scores, scoreDuplicateTrack(track))
	}

	if len(scores) == 0 {
		return scores, 0
	}

	bestIndex := 0
	for i := 1; i < len(scores); i++ {
		if betterDuplicateQuality(scores[i], scores[bestIndex]) {
			bestIndex = i
		}
	}

	best := scores[bestIndex]
	for i := range scores {
		if i == bestIndex {
			continue
		}
		if equalDuplicateQuality(scores[i], best) {
			// A deterministic arbitrary winner would be misleading. When the
			// measurable quality is tied, leave the recommendation unset.
			return scores, 0
		}
	}

	return scores, best.TrackID
}

func scoreDuplicateTrack(track model.Track) model.DuplicateTrackQuality {
	formatClass, codecScore, codecReason := duplicateCodecScore(track)
	bitrateScore, bitrateReason := duplicateBitrateScore(track, formatClass)
	sampleRateScore, sampleRateReason := duplicateSampleRateScore(track.SampleRate)
	channelScore, channelReason := duplicateChannelScore(track.Channels)

	audioScore := codecScore + bitrateScore + sampleRateScore + channelScore
	if audioScore > 80 {
		audioScore = 80
	}

	metadataScore, metadataReasons := duplicateMetadataScore(track)
	score := audioScore + metadataScore

	reasons := make([]string, 0, 8)
	for _, reason := range []string{codecReason, bitrateReason, sampleRateReason, channelReason} {
		if reason != "" {
			reasons = append(reasons, reason)
		}
	}
	reasons = append(reasons, metadataReasons...)

	if strings.TrimSpace(track.ScanError) != "" {
		score -= 20
		reasons = append(reasons, "scan_error")
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return model.DuplicateTrackQuality{
		TrackID:       track.ID,
		Score:         score,
		AudioScore:    audioScore,
		MetadataScore: metadataScore,
		FormatClass:   formatClass,
		Reasons:       reasons,
	}
}

func betterDuplicateQuality(left, right model.DuplicateTrackQuality) bool {
	if left.Score != right.Score {
		return left.Score > right.Score
	}
	if left.AudioScore != right.AudioScore {
		return left.AudioScore > right.AudioScore
	}
	if left.MetadataScore != right.MetadataScore {
		return left.MetadataScore > right.MetadataScore
	}
	return left.TrackID < right.TrackID
}

func equalDuplicateQuality(left, right model.DuplicateTrackQuality) bool {
	return left.Score == right.Score &&
		left.AudioScore == right.AudioScore &&
		left.MetadataScore == right.MetadataScore
}

func duplicateCodecScore(track model.Track) (string, int, string) {
	codec := strings.ToLower(strings.TrimSpace(track.Codec))
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(track.Path), "."))
	if ext == "" {
		ext = strings.ToLower(strings.TrimPrefix(track.Extension, "."))
	}

	value := codec + " " + ext

	switch {
	case strings.Contains(value, "flac"),
		strings.Contains(value, "alac"),
		strings.Contains(value, "ape"),
		strings.Contains(value, "wavpack"),
		strings.Contains(value, "wv"),
		strings.Contains(value, "pcm"),
		ext == "wav",
		ext == "wave",
		ext == "aif",
		ext == "aiff":
		return "lossless", 45, "lossless"

	case strings.Contains(value, "opus"):
		return "lossy", 32, "efficient_lossy"

	case strings.Contains(value, "aac"),
		strings.Contains(value, "mp4a"),
		ext == "m4a":
		return "lossy", 31, "efficient_lossy"

	case strings.Contains(value, "vorbis"),
		ext == "ogg",
		ext == "oga":
		return "lossy", 30, "lossy"

	case strings.Contains(value, "mp3"),
		strings.Contains(value, "mpeg layer 3"),
		ext == "mp3":
		return "lossy", 28, "lossy"

	default:
		return "unknown", 18, "unknown_codec"
	}
}

func duplicateBitrateScore(track model.Track, formatClass string) (int, string) {
	if formatClass == "lossless" {
		// FLAC/ALAC/WAV bitrate is not a useful fidelity comparison: lossless
		// compression and PCM representation can have very different rates.
		return 20, "lossless_bitrate"
	}

	kbps := track.BitRate
	if kbps >= 10_000 {
		kbps /= 1_000
	}

	switch {
	case kbps >= 320:
		return 20, "bitrate_320"
	case kbps >= 256:
		return 18, "bitrate_256"
	case kbps >= 192:
		return 15, "bitrate_192"
	case kbps >= 160:
		return 13, "bitrate_160"
	case kbps >= 128:
		return 10, "bitrate_128"
	case kbps >= 96:
		return 7, "bitrate_96"
	case kbps > 0:
		return 4, "bitrate_low"
	default:
		return 0, "bitrate_unknown"
	}
}

func duplicateSampleRateScore(sampleRate int) (int, string) {
	switch {
	case sampleRate >= 96_000:
		return 10, "sample_rate_high"
	case sampleRate >= 48_000:
		return 9, "sample_rate_standard"
	case sampleRate >= 44_100:
		return 8, "sample_rate_standard"
	case sampleRate >= 32_000:
		return 6, "sample_rate_low"
	case sampleRate > 0:
		return 4, "sample_rate_low"
	default:
		return 0, "sample_rate_unknown"
	}
}

func duplicateChannelScore(channels int) (int, string) {
	switch {
	case channels >= 2:
		return 5, "stereo"
	case channels == 1:
		return 3, "mono"
	default:
		return 0, "channels_unknown"
	}
}

func duplicateMetadataScore(track model.Track) (int, []string) {
	score := 0
	reasons := make([]string, 0, 3)

	if strings.TrimSpace(track.Title) != "" {
		score += 2
	}
	if strings.TrimSpace(track.Artist) != "" {
		score += 2
	}
	if strings.TrimSpace(track.Album) != "" {
		score += 2
	}
	if strings.TrimSpace(track.AlbumArtist) != "" {
		score++
	}
	if strings.TrimSpace(track.Genre) != "" {
		score++
	}
	if track.Year > 0 || strings.TrimSpace(track.ReleaseDate) != "" {
		score++
	}
	if normalizeISRC(track.ISRC) != "" {
		score += 3
	}
	if strings.TrimSpace(track.Label) != "" {
		score++
	}
	if strings.TrimSpace(track.CatalogNumber) != "" {
		score++
	}
	if track.TrackNumber > 0 {
		score++
	}
	if track.HasCover {
		score += 4
		reasons = append(reasons, "embedded_cover")
	}
	if strings.TrimSpace(track.Composer) != "" || strings.TrimSpace(track.Comment) != "" {
		score++
	}

	if score > 20 {
		score = 20
	}
	if score >= 15 {
		reasons = append(reasons, "metadata_complete")
	} else if score >= 9 {
		reasons = append(reasons, "metadata_partial")
	} else {
		reasons = append(reasons, "metadata_sparse")
	}

	return score, reasons
}
