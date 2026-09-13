package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAttempts = 3
	maxRetryDelay       = 8 * time.Second
)

type providerHTTPError struct {
	Provider  string
	Status    int
	Body      string
	Retryable bool
}

func (e *providerHTTPError) Error() string {
	message := fmt.Sprintf("%s returned HTTP %d %s", e.Provider, e.Status, http.StatusText(e.Status))
	if body := providerErrorSummary(e.Body); body != "" {
		message += ": " + body
	}
	return message
}

// providerErrorSummary keeps provider diagnostics useful without leaking an
// entire HTML error/challenge page into the desktop UI. Some catalog sites
// (notably Traxsource) use Cloudflare and may return a full challenge document
// for HTTP 403 responses.
func providerErrorSummary(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	lower := strings.ToLower(body)
	looksHTML := strings.Contains(lower, "<!doctype html") ||
		strings.Contains(lower, "<html") ||
		strings.Contains(lower, "<body")
	if looksHTML {
		if strings.Contains(lower, "cloudflare") ||
			strings.Contains(lower, "cf-chl-") ||
			strings.Contains(lower, "just a moment") {
			return "request blocked by Cloudflare anti-bot protection"
		}
		return "HTML error page omitted"
	}

	// JSON/plain-text API errors are useful, but keep them compact and on one line.
	body = strings.Join(strings.Fields(body), " ")
	const maxProviderErrorBody = 600
	if len(body) > maxProviderErrorBody {
		body = body[:maxProviderErrorBody] + "…"
	}
	return body
}

func getJSON(ctx context.Context, client *http.Client, req *http.Request, provider string, target any) error {
	return getJSONWithAttempts(ctx, client, req, provider, target, defaultHTTPAttempts)
}

func getJSONWithAttempts(ctx context.Context, client *http.Client, req *http.Request, provider string, target any, attempts int) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if client == nil {
		return errors.New("http client is required")
	}
	if req == nil {
		return errors.New("http request is required")
	}
	if attempts <= 0 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		resp, err := client.Do(req.Clone(ctx))
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = fmt.Errorf("request %s: %w", provider, err)
			if attempt == attempts || !isRetryableNetworkError(err) {
				return lastErr
			}
			if err := waitMetadataRetry(ctx, time.Duration(attempt)*500*time.Millisecond); err != nil {
				return err
			}
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
			closeErr := resp.Body.Close()
			retryable := isRetryableHTTPStatus(resp.StatusCode)
			httpErr := &providerHTTPError{
				Provider:  provider,
				Status:    resp.StatusCode,
				Body:      strings.TrimSpace(string(body)),
				Retryable: retryable,
			}
			lastErr = httpErr
			if readErr != nil {
				lastErr = errors.Join(lastErr, fmt.Errorf("read error response: %w", readErr))
			}
			if closeErr != nil {
				lastErr = errors.Join(lastErr, fmt.Errorf("close %s error response body: %w", provider, closeErr))
			}
			if !retryable || attempt == attempts {
				return lastErr
			}
			delay := retryAfterDelay(resp.Header.Get("Retry-After"), time.Now())
			if delay <= 0 {
				delay = time.Duration(1<<(attempt-1)) * time.Second
			}
			if delay > maxRetryDelay {
				delay = maxRetryDelay
			}
			if err := waitMetadataRetry(ctx, delay); err != nil {
				return err
			}
			continue
		}

		decodeErr := json.NewDecoder(resp.Body).Decode(target)
		closeErr := resp.Body.Close()
		if decodeErr != nil {
			return errors.Join(fmt.Errorf("decode %s response: %w", provider, decodeErr), closeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close %s response body: %w", provider, closeErr)
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("request %s failed", provider)
}

func isRetryableHTTPStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout,
		http.StatusTooEarly,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func isRetryableNetworkError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr)
}

func isRetryableMetadataError(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *providerHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Retryable
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "temporarily unavailable") ||
		strings.Contains(message, "after 3 attempts") ||
		strings.Contains(message, "after 4 attempts")
}

func retryAfterDelay(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if delay := when.Sub(now); delay > 0 {
			return delay
		}
	}
	return 0
}

func waitMetadataRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
