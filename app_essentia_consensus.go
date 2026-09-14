package main

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/spacesarmat/CCML/internal/audio"
	"github.com/spacesarmat/CCML/internal/model"
)

const localEssentiaOptionSource = "Essentia local"

func (a *App) attachEssentiaDJComparison(ctx context.Context, trackID int64, result model.MetadataLookupResult) model.MetadataLookupResult {
	if a.store == nil {
		return result
	}
	analysis, ok, err := a.store.EssentiaAnalysis(ctx, trackID)
	if err != nil {
		result.Warnings = append(result.Warnings, err.Error())
		return result
	}
	if !ok {
		return result
	}
	analysis.Camelot, analysis.OpenKey, _ = audio.DJKeyFormats(analysis.Key, analysis.Scale)
	result.AudioComparison = buildEssentiaDJComparison(analysis, result.FieldOptions, result.ProviderReports)
	result.FieldOptions = addEssentiaFieldEvidence(result.FieldOptions, analysis)
	return result
}

func buildEssentiaDJComparison(
	analysis model.EssentiaAnalysis,
	options []model.MetadataFieldOption,
	reports []model.MetadataProviderReport,
) model.MetadataAudioComparison {
	comparison := model.MetadataAudioComparison{
		EssentiaAvailable: true,
		Essentia:          analysis,
		BPMRelation:       "none",
		KeyRelation:       "none",
		BPMRecommendation: "essentia",
		KeyRecommendation: "essentia",
	}

	poolSources := map[string]struct{}{}
	for _, report := range reports {
		if strings.EqualFold(strings.TrimSpace(report.Kind), "dj_pool") {
			poolSources[strings.ToLower(strings.TrimSpace(report.Name))] = struct{}{}
		}
	}

	if option, sources, ok := bestPoolFieldOption(options, "bpm", poolSources); ok {
		comparison.PoolBPM = option.Decimal
		comparison.PoolBPMSources = sources
		comparison.PoolBPMSupport = len(sources)
		comparison.PoolBPMQuality = option.Quality
		comparison.BPMRelation = bpmRelation(analysis.BPM, option.Decimal)
		switch comparison.BPMRelation {
		case "agree":
			comparison.BPMRecommendation = "agreement"
		case "half_double":
			comparison.BPMRecommendation = "dj_pool"
		case "conflict":
			if comparison.PoolBPMSupport >= 2 {
				comparison.BPMRecommendation = "dj_pool"
			}
		}
	}

	keyOption, keySources, hasPoolKey := bestPoolFieldOption(options, "key", poolSources)
	if hasPoolKey {
		comparison.PoolKey = keyOption.Value
		comparison.PoolKeySources = keySources
		comparison.PoolKeySupport = len(keySources)
		comparison.PoolKeyQuality = keyOption.Quality

		scale := ""
		if scaleOption, _, ok := bestPoolFieldOption(options, "keyScale", poolSources); ok {
			scale = scaleOption.Value
		}
		comparison.PoolKeyScale = scale
		comparison.PoolCamelot, comparison.PoolOpenKey, _ = audio.DJKeyFormats(keyOption.Value, scale)
		comparison.KeyRelation = camelotRelation(analysis.Camelot, comparison.PoolCamelot)
		switch comparison.KeyRelation {
		case "agree":
			comparison.KeyRecommendation = "agreement"
		case "relative", "unresolved":
			comparison.KeyRecommendation = "review"
		case "conflict":
			if comparison.PoolKeySupport >= 2 || analysis.Strength < 0.65 {
				comparison.KeyRecommendation = "dj_pool"
			}
		}
	}
	return comparison
}

func bestPoolFieldOption(
	options []model.MetadataFieldOption,
	field string,
	poolSources map[string]struct{},
) (model.MetadataFieldOption, []string, bool) {
	var best model.MetadataFieldOption
	var bestSources []string
	found := false
	for _, option := range options {
		if option.Field != field {
			continue
		}
		sources := optionPoolSources(option, poolSources)
		if len(sources) == 0 {
			continue
		}
		if !found || option.Quality > best.Quality ||
			(option.Quality == best.Quality && len(sources) > len(bestSources)) ||
			(option.Quality == best.Quality && len(sources) == len(bestSources) && option.Confidence > best.Confidence) {
			best = option
			bestSources = sources
			found = true
		}
	}
	return best, bestSources, found
}

func optionPoolSources(option model.MetadataFieldOption, poolSources map[string]struct{}) []string {
	values := option.Sources
	if len(values) == 0 && strings.TrimSpace(option.Source) != "" {
		values = []string{option.Source}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, source := range values {
		name := strings.TrimSpace(source)
		key := strings.ToLower(name)
		if _, ok := poolSources[key]; !ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func bpmRelation(local, pool float64) string {
	if local <= 0 || pool <= 0 {
		return "none"
	}
	if math.Abs(local-pool) <= 0.5 {
		return "agree"
	}
	if math.Abs(local*2-pool) <= 1.0 || math.Abs(pool*2-local) <= 1.0 {
		return "half_double"
	}
	return "conflict"
}

func camelotRelation(local, pool string) string {
	local = strings.ToUpper(strings.TrimSpace(local))
	pool = strings.ToUpper(strings.TrimSpace(pool))
	if local == "" || pool == "" {
		return "unresolved"
	}
	if local == pool {
		return "agree"
	}
	localNumber, localSuffix, localOK := parseCamelot(local)
	poolNumber, poolSuffix, poolOK := parseCamelot(pool)
	if !localOK || !poolOK {
		return "unresolved"
	}
	if localNumber == poolNumber && localSuffix != poolSuffix {
		return "relative"
	}
	return "conflict"
}

func parseCamelot(value string) (int, byte, bool) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) < 2 || len(value) > 3 {
		return 0, 0, false
	}
	suffix := value[len(value)-1]
	if suffix != 'A' && suffix != 'B' {
		return 0, 0, false
	}
	number := 0
	for _, ch := range value[:len(value)-1] {
		if ch < '0' || ch > '9' {
			return 0, 0, false
		}
		number = number*10 + int(ch-'0')
	}
	return number, suffix, number >= 1 && number <= 12
}

func addEssentiaFieldEvidence(options []model.MetadataFieldOption, analysis model.EssentiaAnalysis) []model.MetadataFieldOption {
	out := append([]model.MetadataFieldOption(nil), options...)
	addSource := func(option *model.MetadataFieldOption) {
		sources := append([]string(nil), option.Sources...)
		if len(sources) == 0 && strings.TrimSpace(option.Source) != "" {
			sources = append(sources, option.Source)
		}
		for _, source := range sources {
			if strings.EqualFold(strings.TrimSpace(source), "Essentia") {
				option.Sources = sources
				option.Support = len(sources)
				return
			}
		}
		sources = append(sources, "Essentia")
		sort.Strings(sources)
		option.Sources = sources
		option.Support = len(sources)
	}

	if analysis.BPM > 0 {
		merged := false
		for i := range out {
			if out[i].Field == "bpm" && out[i].Decimal > 0 && math.Abs(out[i].Decimal-analysis.BPM) <= 0.5 {
				addSource(&out[i])
				merged = true
				break
			}
		}
		if !merged {
			out = append(out, model.MetadataFieldOption{
				Field: "bpm", Decimal: analysis.BPM, Source: localEssentiaOptionSource,
				ExternalID: "local-analysis", Support: 1, Sources: []string{"Essentia"},
			})
		}
	}

	localKey := analysis.Camelot
	localScale := "camelot"
	if localKey == "" {
		localKey = strings.TrimSpace(analysis.Key)
		localScale = strings.TrimSpace(analysis.Scale)
	}
	if localKey != "" {
		merged := false
		for i := range out {
			if out[i].Field != "key" {
				continue
			}
			camelot, _, ok := audio.DJKeyFormats(out[i].Value, "")
			if analysis.Camelot != "" && ok && camelot == analysis.Camelot {
				addSource(&out[i])
				merged = true
				break
			}
			if analysis.Camelot == "" && strings.EqualFold(strings.TrimSpace(out[i].Value), localKey) {
				addSource(&out[i])
				merged = true
				break
			}
		}
		if !merged {
			out = append(out, model.MetadataFieldOption{
				Field: "key", Value: localKey, Source: localEssentiaOptionSource,
				ExternalID: "local-analysis", Confidence: analysis.Strength,
				Support: 1, Sources: []string{"Essentia"},
			})
		}
	}
	if localScale != "" {
		merged := false
		for i := range out {
			if out[i].Field == "keyScale" && strings.EqualFold(strings.TrimSpace(out[i].Value), localScale) {
				addSource(&out[i])
				merged = true
				break
			}
		}
		if !merged {
			out = append(out, model.MetadataFieldOption{
				Field: "keyScale", Value: localScale, Source: localEssentiaOptionSource,
				ExternalID: "local-analysis", Confidence: analysis.Strength,
				Support: 1, Sources: []string{"Essentia"},
			})
		}
	}
	return out
}
