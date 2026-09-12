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
				ID          string `json:"id"`
				Name        string `json:"name"`
				DurationMS  int64  `json:"duration_ms"`
				TrackNumber int    `json:"track_number"`
				DiscNumber  int    `json:"disc_number"`
				ExternalIDs struct {
					ISRC string `json:"isrc"`
				} `json:"external_ids"`
				ExternalURLs struct {
					Spotify string `json:"spotify"`
				} `json:"external_urls"`
				Artists []struct {
					Name string `json:"name"`
				} `json:"artists"`
				Album struct {
					Name        string `json:"name"`
					ReleaseDate string `json:"release_date"`
					TotalTracks int    `json:"total_tracks"`
					Artists     []struct {
						Name string `json:"name"`
					} `json:"artists"`
					Images []struct {
						URL    string `json:"url"`
						Width  int    `json:"width"`
						Height int    `json:"height"`
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
		artwork, artworkWidth, artworkHeight := "", 0, 0
		if len(track.Album.Images) > 0 {
			artwork = track.Album.Images[0].URL
			artworkWidth = track.Album.Images[0].Width
			artworkHeight = track.Album.Images[0].Height
		}
		albumArtists := make([]string, 0, len(track.Album.Artists))
		for _, a := range track.Album.Artists {
			if strings.TrimSpace(a.Name) != "" {
				albumArtists = append(albumArtists, a.Name)
			}
		}
		items = append(items, model.MetadataCandidate{
			Source: p.Name(), ExternalID: track.ID, SourceURL: track.ExternalURLs.Spotify,
			Title: track.Name, Artist: artist, Album: track.Album.Name, AlbumArtist: strings.Join(albumArtists, ", "),
			ReleaseDate: track.Album.ReleaseDate, Year: yearFromDate(track.Album.ReleaseDate),
			ISRC: track.ExternalIDs.ISRC, TrackNumber: track.TrackNumber, TrackTotal: track.Album.TotalTracks, DiscNumber: track.DiscNumber,
			ArtworkURL: artwork, ArtworkWidth: artworkWidth, ArtworkHeight: artworkHeight, ArtworkEmbeddable: false,
			DurationMS: track.DurationMS,
		})
	}
	return items, nil
}
