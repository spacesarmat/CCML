package tagging

import (
	"reflect"
	"testing"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestTransformPatchTrimReplaceAndCopy(t *testing.T) {
	t.Parallel()

	before := model.TagSnapshot{
		Title:       "  Track title  ",
		Artist:      "Artist Name",
		AlbumArtist: "",
		Genre:       "TECH house / Dance",
	}

	trim, err := transformPatch(before, model.TagTransformRequest{
		Operation: "trim",
		Fields:    []string{"title"},
	})
	if err != nil {
		t.Fatalf("trim transformPatch() error = %v", err)
	}
	afterTrim := applyPatch(before, trim)
	if afterTrim.Title != "Track title" {
		t.Fatalf("trim title = %q, want %q", afterTrim.Title, "Track title")
	}

	replace, err := transformPatch(before, model.TagTransformRequest{
		Operation:     "replace",
		Fields:        []string{"genre"},
		Search:        "house",
		Replace:       "House",
		CaseSensitive: false,
	})
	if err != nil {
		t.Fatalf("replace transformPatch() error = %v", err)
	}
	afterReplace := applyPatch(before, replace)
	if afterReplace.Genre != "TECH House / Dance" {
		t.Fatalf("replace genre = %q, want %q", afterReplace.Genre, "TECH House / Dance")
	}

	copyPatch, err := transformPatch(before, model.TagTransformRequest{
		Operation:   "copy",
		SourceField: "artist",
		TargetField: "albumArtist",
	})
	if err != nil {
		t.Fatalf("copy transformPatch() error = %v", err)
	}
	afterCopy := applyPatch(before, copyPatch)
	if afterCopy.AlbumArtist != before.Artist {
		t.Fatalf("copy album artist = %q, want %q", afterCopy.AlbumArtist, before.Artist)
	}

	if !reflect.DeepEqual(copyPatch.Fields, []string{"albumArtist"}) {
		t.Fatalf("copy fields = %v, want [albumArtist]", copyPatch.Fields)
	}
}

func TestValidateTransformRequestRejectsEmptyReplaceSearch(t *testing.T) {
	t.Parallel()

	_, _, err := validateTransformRequest([]int64{1}, model.TagTransformRequest{
		Operation: "replace",
		Fields:    []string{"title"},
		Search:    "",
	})
	if err == nil {
		t.Fatal("validateTransformRequest() error = nil, want error")
	}
}
