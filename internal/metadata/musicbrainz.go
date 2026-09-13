package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

const (
	musicBrainzBaseURL     = "https://musicbrainz.org/ws/2/"
	musicBrainzMaxAttempts = 4
	musicBrainzBaseBackoff = 1500 * time.Millisecond
	musicBrainzMaxBackoff  = 10 * time.Second
	musicBrainzMinInterval = time.Second
)

// MusicBrainzProvider searches the public MusicBrainz recording endpoint.
// MusicBrainz may answer with HTTP 503 both for temporary overload and for
// throttling, so requests are rate-limited and transient failures are retried.
type MusicBrainzProvider struct {
	client      *http.Client
	userAgent   string
	baseURL     string
	mu          sync.Mutex
	lastCall    time.Time
	minInterval time.Duration
	maxAttempts int
	baseBackoff time.Duration
}

// NewMusicBrainzProvider creates a rate-limited MusicBrainz client.
func NewMusicBrainzProvider(userAgent string) *MusicBrainzProvider {
	return &MusicBrainzProvider{
		client:      &http.Client{Timeout: 15 * time.Second},
		userAgent:   userAgent,
		baseURL:     musicBrainzBaseURL,
		minInterval: musicBrainzMinInterval,
		maxAttempts: musicBrainzMaxAttempts,
		baseBackoff: musicBrainzBaseBackoff,
	}
}

// Name returns the provider name.
func (p *MusicBrainzProvider) Name() string { return "MusicBrainz" }

// Search searches MusicBrainz recordings and exposes release/ISRC context when available.
func (p *MusicBrainzProvider) Search(ctx context.Context, query model.MetadataQuery) (items []model.MetadataCandidate, resultErr error) {
	if strings.TrimSpace(query.Title) == "" && strings.TrimSpace(query.ISRC) == "" {
		return nil, fmt.Errorf("title or ISRC is required")
	}

	lucene := ""
	if strings.TrimSpace(query.ISRC) != "" {
		lucene = `isrc:"` + luceneEscape(query.ISRC) + `"`
	} else {
		lucene = `recording:"` + luceneEscape(query.Title) + `"`
		if strings.TrimSpace(query.Artist) != "" {
			lucene += ` AND artist:"` + luceneEscape(query.Artist) + `"`
		}
		if strings.TrimSpace(query.Album) != "" {
			lucene += ` AND release:"` + luceneEscape(query.Album) + `"`
		}
	}
	values := url.Values{}
	values.Set("query", lucene)
	values.Set("fmt", "json")
	values.Set("limit", "10")
	endpoint := strings.TrimRight(p.baseURL, "/") + "/recording/?" + values.Encode()

	resp, err := p.doRequest(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close MusicBrainz response body: %w", err))
		}
	}()

	var payload struct {
		Recordings []struct {
			ID               string   `json:"id"`
			Title            string   `json:"title"`
			Length           int64    `json:"length"`
			FirstReleaseDate string   `json:"first-release-date"`
			ISRCs            []string `json:"isrcs"`
			ArtistCredit     []struct {
				Name string `json:"name"`
				Join string `json:"joinphrase"`
			} `json:"artist-credit"`
			Releases []struct {
				ID           string `json:"id"`
				Title        string `json:"title"`
				Date         string `json:"date"`
				Status       string `json:"status"`
				ArtistCredit []struct {
					Name string `json:"name"`
					Join string `json:"joinphrase"`
				} `json:"artist-credit"`
				Media []struct {
					Position    int `json:"position"`
					TrackCount  int `json:"track-count"`
					TrackOffset int `json:"track-offset"`
				} `json:"media"`
			} `json:"releases"`
		} `json:"recordings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode MusicBrainz response: %w", err)
	}

	items = make([]model.MetadataCandidate, 0, len(payload.Recordings))
	for _, recording := range payload.Recordings {
		var artistBuilder strings.Builder
		for _, credit := range recording.ArtistCredit {
			artistBuilder.WriteString(credit.Name)
			artistBuilder.WriteString(credit.Join)
		}
		artist := artistBuilder.String()
		candidate := model.MetadataCandidate{
			Source: p.Name(), ExternalID: recording.ID,
			SourceURL: "https://musicbrainz.org/recording/" + recording.ID,
			Title:     recording.Title, Artist: artist, ReleaseDate: recording.FirstReleaseDate,
			Year: yearFromDate(recording.FirstReleaseDate), DurationMS: recording.Length,
		}
		if len(recording.ISRCs) > 0 {
			candidate.ISRC = recording.ISRCs[0]
		}
		if len(recording.Releases) > 0 {
			releaseIndex := 0
			bestReleaseScore := -1.0
			for i, release := range recording.Releases {
				score := 0.0
				if strings.TrimSpace(query.Album) != "" {
					score = textSimilarity(query.Album, release.Title) * 100
				} else if release.Date != "" && recording.FirstReleaseDate != "" && strings.HasPrefix(release.Date, recording.FirstReleaseDate) {
					score += 2
				}
				if strings.EqualFold(release.Status, "Official") {
					score += 5
				}
				if score > bestReleaseScore {
					bestReleaseScore = score
					releaseIndex = i
				}
			}
			release := recording.Releases[releaseIndex]
			candidate.Album = release.Title
			var albumArtistBuilder strings.Builder
			for _, credit := range release.ArtistCredit {
				albumArtistBuilder.WriteString(credit.Name)
				albumArtistBuilder.WriteString(credit.Join)
			}
			candidate.AlbumArtist = albumArtistBuilder.String()
			if candidate.ReleaseDate == "" {
				candidate.ReleaseDate = release.Date
				candidate.Year = yearFromDate(release.Date)
			}
			if release.ID != "" {
				candidate.ArtworkURL = "https://coverartarchive.org/release/" + release.ID + "/front-1200"
				candidate.ArtworkWidth = 1200
				candidate.ArtworkHeight = 1200
				candidate.ArtworkEmbeddable = true
			}
			if len(release.Media) > 0 {
				candidate.DiscNumber = release.Media[0].Position
				candidate.TrackTotal = release.Media[0].TrackCount
			}
		}
		items = append(items, candidate)
	}
	return items, nil
}

func (p *MusicBrainzProvider) doRequest(ctx context.Context, endpoint string) (*http.Response, error) {
	attempts := p.maxAttempts
	if attempts <= 0 {
		attempts = musicBrainzMaxAttempts
	}

	var lastStatus int
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := p.waitForRateLimit(ctx); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("create MusicBrainz request: %w", err)
		}
		req.Header.Set("User-Agent", p.userAgent)
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = err
			if attempt == attempts {
				break
			}
			if err := p.waitBeforeRetry(ctx, "", attempt); err != nil {
				return nil, err
			}
			continue
		}

		lastStatus = resp.StatusCode
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		if !isRetryableMusicBrainzStatus(resp.StatusCode) {
			if closeErr := resp.Body.Close(); closeErr != nil {
				return nil, errors.Join(
					fmt.Errorf("MusicBrainz returned HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode)),
					fmt.Errorf("close MusicBrainz error response body: %w", closeErr),
				)
			}
			return nil, fmt.Errorf("MusicBrainz returned HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		}

		retryAfter := resp.Header.Get("Retry-After")
		if closeErr := resp.Body.Close(); closeErr != nil {
			lastErr = closeErr
		}
		if attempt == attempts {
			break
		}
		if err := p.waitBeforeRetry(ctx, retryAfter, attempt); err != nil {
			return nil, err
		}
	}

	if lastStatus != 0 {
		return nil, fmt.Errorf("MusicBrainz temporarily unavailable (HTTP %d %s) after %d attempts", lastStatus, http.StatusText(lastStatus), attempts)
	}
	if lastErr != nil {
		return nil, fmt.Errorf("request MusicBrainz failed after %d attempts: %w", attempts, lastErr)
	}
	return nil, fmt.Errorf("request MusicBrainz failed after %d attempts", attempts)
}

func isRetryableMusicBrainzStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (p *MusicBrainzProvider) waitBeforeRetry(ctx context.Context, retryAfter string, attempt int) error {
	delay := parseRetryAfter(retryAfter, time.Now())
	if delay <= 0 {
		base := p.baseBackoff
		if base <= 0 {
			base = musicBrainzBaseBackoff
		}
		delay = base << max(0, attempt-1)
	}
	if delay > musicBrainzMaxBackoff {
		delay = musicBrainzMaxBackoff
	}
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

func parseRetryAfter(value string, now time.Time) time.Duration {
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

func (p *MusicBrainzProvider) waitForRateLimit(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	interval := p.minInterval
	if interval < 0 {
		interval = 0
	}
	wait := interval - time.Since(p.lastCall)
	if !p.lastCall.IsZero() && wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	p.lastCall = time.Now()
	return nil
}

func luceneEscape(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return replacer.Replace(strings.TrimSpace(value))
}

func yearFromDate(value string) int {
	if len(value) < 4 {
		return 0
	}
	year, err := strconv.Atoi(value[:4])
	if err != nil {
		return 0
	}
	return year
}
