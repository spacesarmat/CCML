package audio

import "testing"

func TestHasAttachedPicture(t *testing.T) {
	t.Parallel()

	withCover := []byte(`{"streams":[{"codec_type":"audio"},{"codec_type":"video","disposition":{"attached_pic":1}}]}`)
	if !hasAttachedPicture(withCover) {
		t.Fatal("hasAttachedPicture(withCover) = false, want true")
	}

	withoutCover := []byte(`{"streams":[{"codec_type":"audio"},{"codec_type":"video","disposition":{"attached_pic":0}}]}`)
	if hasAttachedPicture(withoutCover) {
		t.Fatal("hasAttachedPicture(withoutCover) = true, want false")
	}

	if hasAttachedPicture([]byte(`not-json`)) {
		t.Fatal("hasAttachedPicture(invalid JSON) = true, want false")
	}
}
