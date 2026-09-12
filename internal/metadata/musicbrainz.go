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

// MusicBrainzProvider searches the public MusicBrainz recording endpoint.
type MusicBrainzProvider struct {
	client    *http.Client
	userAgent string
	mu        sync.Mutex
	lastCall  time.Time
}

// NewMusicBrainzProvider creates a rate-limited MusicBrainz client.
func NewMusicBrainzProvider(userAgent string) *MusicBrainzProvider {
	return &MusicBrainzProvider{client: &http.Client{Timeout: 15 * time.Second}, userAgent: userAgent}
}

// Name returns the provider name.
func (p *MusicBrainzProvider) Name() string { return "MusicBrainz" }

// Search searches MusicBrainz recordings and exposes release/ISRC context when available.
func (p *MusicBrainzProvider) Search(ctx context.Context, query model.MetadataQuery) (items []model.MetadataCandidate, resultErr error) {
	if strings.TrimSpace(query.Title) == "" && strings.TrimSpace(query.ISRC) == "" {
		return nil, fmt.Errorf("title or ISRC is required")
	}
	if err := p.waitForRateLimit(ctx); err != nil {
		return nil, err
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
			release := recording.Releases[0]
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
