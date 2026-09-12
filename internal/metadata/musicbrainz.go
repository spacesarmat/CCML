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

	"github.com/your-github/ccml/internal/model"
)

// MusicBrainzProvider searches the public MusicBrainz recording endpoint.
type MusicBrainzProvider struct {
	client    *http.Client
	userAgent string
	mu        sync.Mutex
	lastCall  time.Time
}

// NewMusicBrainzProvider creates a rate-limited MusicBrainz client.
func NewMusicBrainzProvider(userAgent string) *MusicBrainzProvider {
	return &MusicBrainzProvider{
		client:    &http.Client{Timeout: 15 * time.Second},
		userAgent: userAgent,
	}
}

// Name returns the provider name.
func (p *MusicBrainzProvider) Name() string { return "MusicBrainz" }

// Search searches MusicBrainz recordings and normalizes the first matches.
func (p *MusicBrainzProvider) Search(ctx context.Context, query model.MetadataQuery) (items []model.MetadataCandidate, resultErr error) {
	if strings.TrimSpace(query.Title) == "" {
		return nil, fmt.Errorf("title is required")
	}
	if err := p.waitForRateLimit(ctx); err != nil {
		return nil, err
	}

	lucene := `recording:"` + luceneEscape(query.Title) + `"`
	if strings.TrimSpace(query.Artist) != "" {
		lucene += ` AND artist:"` + luceneEscape(query.Artist) + `"`
	}
	values := url.Values{}
	values.Set("query", lucene)
	values.Set("fmt", "json")
	values.Set("limit", "10")
	endpoint := "https://musicbrainz.org/ws/2/recording/?" + values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create MusicBrainz request: %w", err)
	}
	req.Header.Set("User-Agent", p.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request MusicBrainz: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close MusicBrainz response body: %w", err))
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MusicBrainz returned HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Recordings []struct {
			ID               string `json:"id"`
			Score            int    `json:"score"`
			Title            string `json:"title"`
			Length           int64  `json:"length"`
			FirstReleaseDate string `json:"first-release-date"`
			ArtistCredit     []struct {
				Name string `json:"name"`
				Join string `json:"joinphrase"`
			} `json:"artist-credit"`
			Releases []struct {
				Title string `json:"title"`
			} `json:"releases"`
		} `json:"recordings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode MusicBrainz response: %w", err)
	}

	items = make([]model.MetadataCandidate, 0, len(payload.Recordings))
	for _, recording := range payload.Recordings {
		var artist strings.Builder
		for _, credit := range recording.ArtistCredit {
			artist.WriteString(credit.Name)
			artist.WriteString(credit.Join)
		}
		album := ""
		if len(recording.Releases) > 0 {
			album = recording.Releases[0].Title
		}
		items = append(items, model.MetadataCandidate{
			Source:     p.Name(),
			ExternalID: recording.ID,
			Title:      recording.Title,
			Artist:     artist.String(),
			Album:      album,
			Year:       yearFromDate(recording.FirstReleaseDate),
			DurationMS: recording.Length,
			Confidence: float64(recording.Score) / 100,
		})
	}
	return items, nil
}

func (p *MusicBrainzProvider) waitForRateLimit(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	wait := time.Second - time.Since(p.lastCall)
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
