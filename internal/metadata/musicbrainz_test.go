package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

func TestMusicBrainzRetriesTemporary503(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if call < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
			"recordings": [{
				"id": "recording-id",
				"title": "Test Track",
				"length": 123000,
				"first-release-date": "2026-01-02",
				"isrcs": ["USAAA2600001"],
				"artist-credit": [{"name": "Test Artist", "joinphrase": ""}],
				"releases": []
			}]
		}`))
		if err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	provider := NewMusicBrainzProvider("CCML/test")
	provider.baseURL = server.URL + "/"
	provider.minInterval = 0
	provider.baseBackoff = time.Millisecond
	provider.maxAttempts = 3

	items, err := provider.Search(context.Background(), model.MetadataQuery{Title: "Test Track", Artist: "Test Artist"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("calls = %d, want 3", got)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Title != "Test Track" || items[0].ISRC != "USAAA2600001" {
		t.Fatalf("unexpected candidate: %+v", items[0])
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, time.September, 12, 10, 0, 0, 0, time.UTC)
	if got := parseRetryAfter("3", now); got != 3*time.Second {
		t.Fatalf("seconds Retry-After = %s, want 3s", got)
	}
	future := now.Add(4 * time.Second).Format(http.TimeFormat)
	if got := parseRetryAfter(future, now); got != 4*time.Second {
		t.Fatalf("date Retry-After = %s, want 4s", got)
	}
}
