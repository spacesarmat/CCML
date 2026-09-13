package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetJSONRetriesTemporaryHTTPError(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	var target struct {
		OK bool `json:"ok"`
	}
	client := &http.Client{Timeout: 5 * time.Second}
	if err := getJSONWithAttempts(context.Background(), client, req, "test", &target, 3); err != nil {
		t.Fatalf("getJSONWithAttempts() error = %v", err)
	}
	if !target.OK || calls.Load() != 3 {
		t.Fatalf("target=%+v calls=%d", target, calls.Load())
	}
}

func TestProviderHTTPErrorClassifiesUnauthorizedAsPermanent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`bad token`))
	}))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	var target map[string]any
	err := getJSONWithAttempts(context.Background(), server.Client(), req, "test", &target, 3)
	if err == nil {
		t.Fatal("expected error")
	}
	if isRetryableMetadataError(err) {
		t.Fatalf("401 should not be retryable: %v", err)
	}
}

func TestProviderHTTPErrorOmitsCloudflareHTML(t *testing.T) {
	err := (&providerHTTPError{
		Provider: "Traxsource",
		Status:   http.StatusForbidden,
		Body:     `<!DOCTYPE html><html><head><title>Just a moment...</title></head><body><script src="/cdn-cgi/challenge-platform/x"></script><p>Cloudflare</p></body></html>`,
	}).Error()
	if strings.Contains(strings.ToLower(err), "<!doctype") || strings.Contains(strings.ToLower(err), "<script") {
		t.Fatalf("HTML leaked into error: %q", err)
	}
	if !strings.Contains(err, "Cloudflare") {
		t.Fatalf("expected Cloudflare diagnostic, got %q", err)
	}
}

func TestProviderHTTPErrorTruncatesPlainText(t *testing.T) {
	body := strings.Repeat("x", 1000)
	err := (&providerHTTPError{Provider: "test", Status: http.StatusBadRequest, Body: body}).Error()
	if len(err) > 750 {
		t.Fatalf("error was not compacted: len=%d", len(err))
	}
}
