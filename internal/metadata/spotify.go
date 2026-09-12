package metadata

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

// SpotifyProvider searches the Spotify Web API with a caller-supplied OAuth token.
// Desktop builds should obtain that token with Authorization Code + PKCE rather
// than embedding a client secret in the application.
type SpotifyProvider struct {
	client      *http.Client
	accessToken string
	market      string
}

// NewSpotifyProvider creates a Spotify metadata provider.
func NewSpotifyProvider(accessToken, market string) *SpotifyProvider {
	market = strings.TrimSpace(market)
	if market == "" {
		market = "US"
	}
	return &SpotifyProvider{
		client:      &http.Client{Timeout: 15 * time.Second},
		accessToken: strings.TrimSpace(accessToken),
		market:      market,
	}
}

// Name returns the provider name.
func (p *SpotifyProvider) Name() string { return "Spotify" }

// Search searches Spotify tracks.
func (p *SpotifyProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	if p.accessToken == "" {
		return nil, fmt.Errorf("Spotify access token is not configured")
	}
	if strings.TrimSpace(query.Title) == "" {
		return nil, fmt.Errorf("title is required")
	}

	parts := []string{fmt.Sprintf("track:%s", query.Title)}
	if strings.TrimSpace(query.Artist) != "" {
		parts = append(parts, fmt.Sprintf("artist:%s", query.Artist))
	}
	values := url.Values{}
	values.Set("q", strings.Join(parts, " "))
	values.Set("type", "track")
	values.Set("limit", "10")
	values.Set("market", p.market)

	req, err := http.NewRequest(http.MethodGet, "https://api.spotify.com/v1/search?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Spotify request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Accept", "application/json")

	var payload struct {
		Tracks struct {
			Items []struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				DurationMS int64  `json:"duration_ms"`
				Artists    []struct {
					Name string `json:"name"`
				} `json:"artists"`
				Album struct {
					Name        string `json:"name"`
					ReleaseDate string `json:"release_date"`
					Images      []struct {
						URL string `json:"url"`
					} `json:"images"`
				} `json:"album"`
			} `json:"items"`
		} `json:"tracks"`
	}
	if err := getJSON(ctx, p.client, req, p.Name(), &payload); err != nil {
		return nil, err
	}

	items := make([]model.MetadataCandidate, 0, len(payload.Tracks.Items))
	for _, track := range payload.Tracks.Items {
		artistNames := make([]string, 0, len(track.Artists))
		for _, artist := range track.Artists {
			if name := strings.TrimSpace(artist.Name); name != "" {
				artistNames = append(artistNames, name)
			}
		}
		artist := strings.Join(artistNames, ", ")
		artwork := ""
		if len(track.Album.Images) > 0 {
			artwork = track.Album.Images[0].URL
		}
		items = append(items, model.MetadataCandidate{
			Source:     p.Name(),
			ExternalID: track.ID,
			Title:      track.Name,
			Artist:     artist,
			Album:      track.Album.Name,
			Year:       yearFromDate(track.Album.ReleaseDate),
			ArtworkURL: artwork,
			DurationMS: track.DurationMS,
			Confidence: metadataSimilarity(query, artist, track.Name, track.DurationMS),
		})
	}
	return items, nil
}
