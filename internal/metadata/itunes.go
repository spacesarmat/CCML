package metadata

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

const iTunesSearchBaseURL = "https://itunes.apple.com/search"

// ITunesProvider searches Apple's public iTunes Search API. It does not
// require a developer token and is used as the default Apple catalog source.
type ITunesProvider struct {
	client      *http.Client
	country     string
	baseURL     string
	mu          sync.Mutex
	lastCall    time.Time
	minInterval time.Duration
}

// NewITunesProvider creates an iTunes Search API provider.
func NewITunesProvider(country string) *ITunesProvider {
	country = strings.ToUpper(strings.TrimSpace(country))
	if country == "" {
		country = "US"
	}
	return &ITunesProvider{
		client:      &http.Client{Timeout: 15 * time.Second},
		country:     country,
		baseURL:     iTunesSearchBaseURL,
		minInterval: 3 * time.Second,
	}
}

func (p *ITunesProvider) Name() string { return "Apple iTunes" }

func (p *ITunesProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	term := strings.TrimSpace(strings.TrimSpace(query.Artist) + " " + strings.TrimSpace(query.Title))
	if term == "" {
		return nil, fmt.Errorf("artist or title is required")
	}
	if err := p.waitForRateLimit(ctx); err != nil {
		return nil, err
	}

	values := url.Values{}
	values.Set("term", term)
	values.Set("country", p.country)
	values.Set("media", "music")
	values.Set("entity", "song")
	values.Set("limit", "10")
	req, err := http.NewRequest(http.MethodGet, p.baseURL+"?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Apple iTunes request: %w", err)
	}
	resp := struct {
		Results []struct {
			TrackID          int64  `json:"trackId"`
			TrackName        string `json:"trackName"`
			ArtistName       string `json:"artistName"`
			CollectionName   string `json:"collectionName"`
			ReleaseDate      string `json:"releaseDate"`
			TrackTimeMillis  int64  `json:"trackTimeMillis"`
			PrimaryGenreName string `json:"primaryGenreName"`
			TrackNumber      int    `json:"trackNumber"`
			TrackCount       int    `json:"trackCount"`
			DiscNumber       int    `json:"discNumber"`
			DiscCount        int    `json:"discCount"`
			ArtworkURL100    string `json:"artworkUrl100"`
			TrackViewURL     string `json:"trackViewUrl"`
		} `json:"results"`
	}{}
	if err := getJSON(ctx, p.client, req, p.Name(), &resp); err != nil {
		return nil, err
	}

	items := make([]model.MetadataCandidate, 0, len(resp.Results))
	for _, track := range resp.Results {
		artwork := upscaleITunesArtwork(track.ArtworkURL100)
		items = append(items, model.MetadataCandidate{
			Source: p.Name(), ExternalID: strconv.FormatInt(track.TrackID, 10), SourceURL: track.TrackViewURL,
			Title: track.TrackName, Artist: track.ArtistName, Album: track.CollectionName,
			ReleaseDate: normalizeRFC3339Date(track.ReleaseDate), Year: yearFromDate(track.ReleaseDate), Genre: track.PrimaryGenreName,
			TrackNumber: track.TrackNumber, TrackTotal: track.TrackCount, DiscNumber: track.DiscNumber, DiscTotal: track.DiscCount,
			ArtworkURL: artwork, ArtworkWidth: 1200, ArtworkHeight: 1200, ArtworkEmbeddable: artwork != "",
			DurationMS: track.TrackTimeMillis,
		})
	}
	return items, nil
}

func (p *ITunesProvider) waitForRateLimit(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	wait := p.minInterval - time.Since(p.lastCall)
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

func upscaleITunesArtwork(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, token := range []string{"100x100bb", "100x100-75"} {
		if strings.Contains(value, token) {
			return strings.Replace(value, token, "1200x1200bb", 1)
		}
	}
	return value
}

func normalizeRFC3339Date(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 10 {
		return value[:10]
	}
	return value
}
