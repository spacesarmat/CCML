package library

import (
	"context"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

type fakeDuplicateVerifyStore struct {
	tracks []model.Track
}

func (f fakeDuplicateVerifyStore) AllTracks(context.Context) ([]model.Track, error) {
	return append([]model.Track(nil), f.tracks...), nil
}

type fakeDuplicateComparator struct {
	gotTracks    []model.Track
	gotReference int64
	result       model.DuplicateAudioVerification
}

func (f *fakeDuplicateComparator) Compare(
	_ context.Context,
	tracks []model.Track,
	referenceTrackID int64,
) (model.DuplicateAudioVerification, error) {
	f.gotTracks = append([]model.Track(nil), tracks...)
	f.gotReference = referenceTrackID
	return f.result, nil
}

func TestVerifyDuplicateAudioUsesRecommendedQualityReference(t *testing.T) {
	t.Parallel()

	store := fakeDuplicateVerifyStore{tracks: []model.Track{
		{
			ID: 1, Artist: "Artist", Title: "Track", DurationMS: 180_000,
			Path: "/a.mp3", Codec: "mp3", BitRate: 128_000, SampleRate: 44_100, Channels: 2,
		},
		{
			ID: 2, Artist: "Artist", Title: "Track", DurationMS: 180_500,
			Path: "/b.flac", Codec: "flac", BitRate: 900_000, SampleRate: 44_100, Channels: 2,
		},
	}}
	comparator := &fakeDuplicateComparator{
		result: model.DuplicateAudioVerification{ReferenceTrackID: 2},
	}

	result, err := VerifyDuplicateAudio(context.Background(), store, comparator, []int64{2, 1})
	if err != nil {
		t.Fatal(err)
	}
	if comparator.gotReference != 2 {
		t.Fatalf("reference = %d, want quality leader 2", comparator.gotReference)
	}
	if result.GroupKey == "" {
		t.Fatal("expected group key")
	}
}

func TestVerifyDuplicateAudioRejectsStaleSubset(t *testing.T) {
	t.Parallel()

	store := fakeDuplicateVerifyStore{tracks: []model.Track{
		{ID: 1, Artist: "Artist", Title: "Track", DurationMS: 180_000},
		{ID: 2, Artist: "Artist", Title: "Track", DurationMS: 180_500},
		{ID: 3, Artist: "Artist", Title: "Track", DurationMS: 181_000},
	}}
	comparator := &fakeDuplicateComparator{}

	_, err := VerifyDuplicateAudio(context.Background(), store, comparator, []int64{1, 2})
	if err == nil {
		t.Fatal("expected stale/subset group error")
	}
}

func TestVerifyDuplicateAudioRejectsTooLargeGroup(t *testing.T) {
	t.Parallel()

	ids := make([]int64, duplicateVerifyTrackLimit+1)
	for i := range ids {
		ids[i] = int64(i + 1)
	}

	_, err := VerifyDuplicateAudio(
		context.Background(),
		fakeDuplicateVerifyStore{},
		&fakeDuplicateComparator{},
		ids,
	)
	if err == nil {
		t.Fatal("expected group-size limit error")
	}
}
