package main

import (
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestExtractDJPoolConsensusUsesPoolOnlyEvidence(t *testing.T) {
	t.Parallel()

	options := []model.MetadataFieldOption{
		{Field: "bpm", Decimal: 128, Quality: 1.1, Sources: []string{"Jestei Pool", "MUZVIZOR"}},
		{Field: "key", Value: "8A", Quality: 1.05, Sources: []string{"Jestei Pool", "MUZVIZOR"}},
		{Field: "keyScale", Value: "camelot", Quality: 1.05, Sources: []string{"Jestei Pool", "MUZVIZOR"}},
		{Field: "bpm", Decimal: 127, Quality: 1.5, Sources: []string{"MusicBrainz"}},
	}
	reports := []model.MetadataProviderReport{
		{Name: "Jestei Pool", Kind: "dj_pool", Status: "ok"},
		{Name: "MUZVIZOR", Kind: "dj_pool", Status: "ok"},
		{Name: "MusicBrainz", Kind: "catalog", Status: "ok"},
	}
	got, ok := extractDJPoolConsensus(42, options, reports)
	if !ok {
		t.Fatal("expected consensus")
	}
	if got.TrackID != 42 || got.BPM != 128 || got.BPMSupport != 2 || got.Camelot != "8A" || got.KeySupport != 2 {
		t.Fatalf("consensus = %+v", got)
	}
}
