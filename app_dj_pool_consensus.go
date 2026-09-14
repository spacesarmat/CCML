package main

import (
	"context"
	"strings"

	"github.com/spacesarmat/CCML/internal/audio"
	"github.com/spacesarmat/CCML/internal/model"
)

func extractDJPoolConsensus(
	trackID int64,
	options []model.MetadataFieldOption,
	reports []model.MetadataProviderReport,
) (model.DJPoolConsensus, bool) {
	poolSources := map[string]struct{}{}
	for _, report := range reports {
		if strings.EqualFold(strings.TrimSpace(report.Kind), "dj_pool") {
			poolSources[strings.ToLower(strings.TrimSpace(report.Name))] = struct{}{}
		}
	}
	consensus := model.DJPoolConsensus{TrackID: trackID}
	if option, sources, ok := bestPoolFieldOption(options, "bpm", poolSources); ok {
		consensus.BPM = option.Decimal
		consensus.BPMSupport = len(sources)
		consensus.BPMQuality = option.Quality
	}
	if option, sources, ok := bestPoolFieldOption(options, "key", poolSources); ok {
		scale := ""
		if scaleOption, _, scaleOK := bestPoolFieldOption(options, "keyScale", poolSources); scaleOK {
			scale = scaleOption.Value
		}
		consensus.Camelot, _, _ = audio.DJKeyFormats(option.Value, scale)
		consensus.KeySupport = len(sources)
		consensus.KeyQuality = option.Quality
	}
	return consensus, consensus.BPM > 0 || consensus.Camelot != ""
}

func (a *App) essentiaAdaptiveHint(ctx context.Context, trackID int64) (audio.AdaptiveHint, error) {
	if a.store == nil {
		return audio.AdaptiveHint{}, nil
	}
	consensus, ok, err := a.store.DJPoolConsensus(ctx, trackID)
	if err != nil || !ok {
		return audio.AdaptiveHint{}, err
	}
	return audio.AdaptiveHint{
		PoolBPM:        consensus.BPM,
		PoolBPMSupport: consensus.BPMSupport,
		PoolCamelot:    consensus.Camelot,
		PoolKeySupport: consensus.KeySupport,
	}, nil
}
