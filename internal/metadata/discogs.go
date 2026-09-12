package metadata

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/your-github/ccml/internal/model"
)

// DiscogsProvider searches the Discogs database API using a personal token.
type DiscogsProvider struct {
	client    *http.Client
	token     string
	userAgent string
}

// NewDiscogsProvider creates a Discogs provider.
func NewDiscogsProvider(token, userAgent string) *DiscogsProvider {
	return &DiscogsProvider{
		client:    &http.Client{Timeout: 15 * time.Second},
		token:     strings.TrimSpace(token),
		userAgent: strings.TrimSpace(userAgent),
	}
}

// Name returns the provider name.
func (p *DiscogsProvider) Name() string { return "Discogs" }

// Search searches Discogs releases containing the requested artist/track.
func (p *DiscogsProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	if p.token == "" {
		return nil, fmt.Errorf("Discogs token is not configured")
	}
	if strings.TrimSpace(query.Title) == "" {
		return nil, fmt.Errorf("title is required")
	}
	values := url.Values{}
	values.Set("type", "release")
	values.Set("track", query.Title)
	values.Set("per_page", "10")
	if strings.TrimSpace(query.Artist) != "" {
		values.Set("artist", query.Artist)
	}
	req, err := http.NewRequest(http.MethodGet, "https://api.discogs.com/database/search?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Discogs request: %w", err)
	}
	req.Header.Set("Authorization", "Discogs token="+p.token)
	if p.userAgent != "" {
		req.Header.Set("User-Agent", p.userAgent)
	}
	req.Header.Set("Accept", "application/json")

	var payload struct {
		Results []struct {
			ID         int64    `json:"id"`
			Title      string   `json:"title"`
			Year       int      `json:"year"`
			Genre      []string `json:"genre"`
			Style      []string `json:"style"`
			CoverImage string   `json:"cover_image"`
		} `json:"results"`
	}
	if err := getJSON(ctx, p.client, req, p.Name(), &payload); err != nil {
		return nil, err
	}

	items := make([]model.MetadataCandidate, 0, len(payload.Results))
	for _, release := range payload.Results {
		artist, album := splitDiscogsTitle(release.Title)
		genre := ""
		if len(release.Genre) > 0 {
			genre = release.Genre[0]
		} else if len(release.Style) > 0 {
			genre = release.Style[0]
		}
		items = append(items, model.MetadataCandidate{
			Source:     p.Name(),
			ExternalID: strconv.FormatInt(release.ID, 10),
			Title:      query.Title,
			Artist:     artist,
			Album:      album,
			Year:       release.Year,
			Genre:      genre,
			ArtworkURL: release.CoverImage,
			Confidence: metadataSimilarity(query, artist, query.Title, 0),
		})
	}
	return items, nil
}

func splitDiscogsTitle(value string) (artist, album string) {
	parts := strings.SplitN(value, " - ", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return "", strings.TrimSpace(value)
}
