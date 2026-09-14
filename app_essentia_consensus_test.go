package main

import (
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestEssentiaDJComparisonAgreesWithPoolConsensus(t *testing.T) {
	t.Parallel()

	analysis := model.EssentiaAnalysis{
		TrackID: 1, BPM: 128.2, Key: "A", Scale: "minor", Strength: 0.88,
		Camelot: "8A", OpenKey: "1m",
	}
	options := []model.MetadataFieldOption{
		{Field: "bpm", Decimal: 128.0, Confidence: 0.93, Quality: 1.10, Support: 2, Sources: []string{"Jestei Pool", "MUZVIZOR"}},
		{Field: "key", Value: "8A", Confidence: 0.92, Quality: 1.09, Support: 2, Sources: []string{"Jestei Pool", "MUZVIZOR"}},
		{Field: "keyScale", Value: "camelot", Confidence: 0.92, Quality: 1.09, Support: 2, Sources: []string{"Jestei Pool", "MUZVIZOR"}},
	}
	reports := []model.MetadataProviderReport{
		{Name: "Jestei Pool", Kind: "dj_pool", Status: "ok"},
		{Name: "MUZVIZOR", Kind: "dj_pool", Status: "ok"},
	}

	got := buildEssentiaDJComparison(analysis, options, reports)
	if got.BPMRelation != "agree" || got.KeyRelation != "agree" {
		t.Fatalf("relations = BPM %q, Key %q", got.BPMRelation, got.KeyRelation)
	}
	if got.BPMRecommendation != "agreement" || got.KeyRecommendation != "agreement" {
		t.Fatalf("recommendations = BPM %q, Key %q", got.BPMRecommendation, got.KeyRecommendation)
	}
	if got.PoolBPMSupport != 2 || got.PoolKeySupport != 2 {
		t.Fatalf("support = BPM %d, Key %d", got.PoolBPMSupport, got.PoolKeySupport)
	}
}

func TestEssentiaDJComparisonDetectsHalfDoubleTempo(t *testing.T) {
	t.Parallel()

	analysis := model.EssentiaAnalysis{BPM: 64, Key: "A", Scale: "minor", Strength: 0.8, Camelot: "8A"}
	options := []model.MetadataFieldOption{
		{Field: "bpm", Decimal: 128, Quality: 1, Sources: []string{"Jestei Pool"}},
	}
	reports := []model.MetadataProviderReport{{Name: "Jestei Pool", Kind: "dj_pool", Status: "ok"}}
	got := buildEssentiaDJComparison(analysis, options, reports)
	if got.BPMRelation != "half_double" || got.BPMRecommendation != "dj_pool" {
		t.Fatalf("half/double comparison = %+v", got)
	}
}

func TestEssentiaDJComparisonPrefersMultiPoolConflict(t *testing.T) {
	t.Parallel()

	analysis := model.EssentiaAnalysis{BPM: 126, Key: "C", Scale: "major", Strength: 0.9, Camelot: "8B"}
	options := []model.MetadataFieldOption{
		{Field: "bpm", Decimal: 128, Quality: 1.1, Sources: []string{"Jestei Pool", "MUZVIZOR"}},
		{Field: "key", Value: "9A", Quality: 1.1, Sources: []string{"Jestei Pool", "MUZVIZOR"}},
		{Field: "keyScale", Value: "camelot", Quality: 1.1, Sources: []string{"Jestei Pool", "MUZVIZOR"}},
	}
	reports := []model.MetadataProviderReport{
		{Name: "Jestei Pool", Kind: "dj_pool", Status: "ok"},
		{Name: "MUZVIZOR", Kind: "dj_pool", Status: "ok"},
	}
	got := buildEssentiaDJComparison(analysis, options, reports)
	if got.BPMRecommendation != "dj_pool" || got.KeyRecommendation != "dj_pool" {
		t.Fatalf("multi-pool conflict = %+v", got)
	}
}

func TestEssentiaFieldEvidenceMergesAgreementAndKeepsConflictSeparate(t *testing.T) {
	t.Parallel()

	analysis := model.EssentiaAnalysis{BPM: 128.2, Key: "A", Scale: "minor", Strength: 0.8, Camelot: "8A"}
	options := []model.MetadataFieldOption{
		{Field: "bpm", Decimal: 128, Source: "Jestei Pool", Sources: []string{"Jestei Pool"}, Support: 1},
		{Field: "key", Value: "9A", Source: "Jestei Pool", Sources: []string{"Jestei Pool"}, Support: 1},
	}
	got := addEssentiaFieldEvidence(options, analysis)

	if got[0].Support != 2 {
		t.Fatalf("agreeing BPM support = %d, want 2", got[0].Support)
	}
	foundLocalKey := false
	for _, option := range got {
		if option.Field == "key" && option.Source == localEssentiaOptionSource && option.Value == "8A" {
			foundLocalKey = true
		}
	}
	if !foundLocalKey {
		t.Fatalf("conflicting local Camelot option missing: %+v", got)
	}
}
