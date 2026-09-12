package tagging

import (
	"reflect"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestApplyPatchChangesOnlySelectedFields(t *testing.T) {
	before := model.TagSnapshot{
		Title: "Old title", Artist: "Artist", Album: "Album", Genre: "House",
		Label: "Old Label", CatalogNumber: "CAT-001", ISRC: "USAAA2000001", ReleaseDate: "2020-04-03",
		Year: 2020, TrackNumber: 3, TrackTotal: 12, DiscNumber: 1, DiscTotal: 2,
	}
	patch := model.TagPatch{
		Fields:      []string{"title", "genre", "trackNumber", "label", "releaseDate"},
		Title:       "New title",
		Genre:       "",
		TrackNumber: 7,
		Label:       "New Label",
		ReleaseDate: "2021-05-06",
	}
	after := applyPatch(before, patch)
	want := before
	want.Title = "New title"
	want.Genre = ""
	want.TrackNumber = 7
	want.Label = "New Label"
	want.ReleaseDate = "2021-05-06"
	if !reflect.DeepEqual(after, want) {
		t.Fatalf("applyPatch() = %#v, want %#v", after, want)
	}
}

func TestValidateRequestRejectsInvalidYear(t *testing.T) {
	_, err := validateRequest([]int64{1}, model.TagPatch{Fields: []string{"year"}, Year: 99})
	if err == nil {
		t.Fatal("validateRequest() error = nil, want invalid year error")
	}
}

func TestValidateIDsDeduplicatesAndSorts(t *testing.T) {
	got, err := validateIDs([]int64{9, 2, 9, 4})
	if err != nil {
		t.Fatalf("validateIDs() error = %v", err)
	}
	want := []int64{2, 4, 9}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("validateIDs() = %v, want %v", got, want)
	}
}
