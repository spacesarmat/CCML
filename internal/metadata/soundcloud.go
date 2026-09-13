package metadata

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

// SoundCloudProvider searches tracks using a SoundCloud OAuth access token.
type SoundCloudProvider struct {
	client *http.Client
	token  string
}

// NewSoundCloudProvider creates a SoundCloud provider.
func NewSoundCloudProvider(token string) *SoundCloudProvider {
	return &SoundCloudProvider{client: &http.Client{Timeout: 15 * time.Second}, token: strings.TrimSpace(token)}
}

// Name returns the provider name.
func (p *SoundCloudProvider) Name() string { return "SoundCloud" }

// Search searches SoundCloud tracks.
func (p *SoundCloudProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	if p.token == "" {
		return nil, fmt.Errorf("SoundCloud access token is not configured")
	}
	term := strings.TrimSpace(strings.TrimSpace(query.Artist) + " " + strings.TrimSpace(query.Title))
	if term == "" {
		return nil, fmt.Errorf("artist or title is required")
	}
	values := url.Values{}
	values.Set("q", term)
	values.Set("limit", "10")
	values.Set("linked_partitioning", "true")
	req, err := http.NewRequest(http.MethodGet, "https://api.soundcloud.com/tracks?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create SoundCloud request: %w", err)
	}
	req.Header.Set("Authorization", "OAuth "+p.token)
	req.Header.Set("Accept", "application/json")

	type track struct {
		ID             int64  `json:"id"`
		Title          string `json:"title"`
		Duration       int64  `json:"duration"`
		Genre          string `json:"genre"`
		ArtworkURL     string `json:"artwork_url"`
		MetadataArtist string `json:"metadata_artist"`
		PermalinkURL   string `json:"permalink_url"`
		User           struct {
			Username string `json:"username"`
		} `json:"user"`
	}
	var wrapped struct {
		Collection []track `json:"collection"`
	}
	if err := getJSON(ctx, p.client, req, p.Name(), &wrapped); err != nil {
		return nil, err
	}

	items := make([]model.MetadataCandidate, 0, len(wrapped.Collection))
	for _, result := range wrapped.Collection {
		artist := strings.TrimSpace(result.MetadataArtist)
		if artist == "" {
			artist = result.User.Username
		}
		items = append(items, model.MetadataCandidate{
			Source:     p.Name(),
			ExternalID: strconv.FormatInt(result.ID, 10),
			SourceURL:  result.PermalinkURL,
			Title:      result.Title,
			Artist:     artist,
			Genre:      result.Genre,
			ArtworkURL: result.ArtworkURL,
			DurationMS: result.Duration,
			Confidence: metadataSimilarity(query, artist, result.Title, result.Duration),
		})
	}
	return items, nil
}
