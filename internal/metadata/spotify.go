package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

// SpotifyProvider searches Spotify catalog metadata. A fixed access token can
// be supplied for development, or client credentials can be configured so CCML
// refreshes application tokens automatically.
type SpotifyProvider struct {
	client       *http.Client
	accessToken  string
	clientID     string
	clientSecret string
	market       string

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

type spotifyImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func NewSpotifyProvider(accessToken, clientID, clientSecret, market string) *SpotifyProvider {
	market = strings.ToUpper(strings.TrimSpace(market))
	if market == "" {
		market = "US"
	}
	return &SpotifyProvider{
		client:       &http.Client{Timeout: 15 * time.Second},
		accessToken:  strings.TrimSpace(accessToken),
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
		market:       market,
	}
}

func (p *SpotifyProvider) Name() string { return "Spotify" }

func (p *SpotifyProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	if strings.TrimSpace(query.Title) == "" && strings.TrimSpace(query.ISRC) == "" {
		return nil, fmt.Errorf("title or ISRC is required")
	}
	token, err := p.token(ctx)
	if err != nil {
		return nil, err
	}

	items, err := p.searchWithToken(ctx, query, token)
	if err == nil || p.accessToken != "" {
		return items, err
	}
	var httpErr *providerHTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusUnauthorized {
		return nil, err
	}

	p.mu.Lock()
	p.cachedToken = ""
	p.tokenExpiry = time.Time{}
	p.mu.Unlock()
	token, tokenErr := p.token(ctx)
	if tokenErr != nil {
		return nil, tokenErr
	}
	return p.searchWithToken(ctx, query, token)
}

func (p *SpotifyProvider) searchWithToken(ctx context.Context, query model.MetadataQuery, token string) ([]model.MetadataCandidate, error) {
	parts := make([]string, 0, 2)
	if isrc := normalizeIdentifier(query.ISRC); isrc != "" {
		parts = append(parts, "isrc:"+isrc)
	} else {
		parts = append(parts, fmt.Sprintf("track:%s", query.Title))
		if strings.TrimSpace(query.Artist) != "" {
			parts = append(parts, fmt.Sprintf("artist:%s", query.Artist))
		}
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
	req.Header.Set("Authorization", "Bearer "+token)
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
					Images []spotifyImage `json:"images"`
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
		artwork, artworkWidth, artworkHeight, artworkEmbeddable := selectSpotifyArtwork(track.Album.Images)
		albumArtists := make([]string, 0, len(track.Album.Artists))
		for _, artist := range track.Album.Artists {
			if name := strings.TrimSpace(artist.Name); name != "" {
				albumArtists = append(albumArtists, name)
			}
		}
		items = append(items, model.MetadataCandidate{
			Source: p.Name(), ExternalID: track.ID, SourceURL: track.ExternalURLs.Spotify,
			Title: track.Name, Artist: strings.Join(artistNames, ", "), Album: track.Album.Name, AlbumArtist: strings.Join(albumArtists, ", "),
			ReleaseDate: track.Album.ReleaseDate, Year: yearFromDate(track.Album.ReleaseDate),
			ISRC: track.ExternalIDs.ISRC, TrackNumber: track.TrackNumber, TrackTotal: track.Album.TotalTracks, DiscNumber: track.DiscNumber,
			ArtworkURL: artwork, ArtworkWidth: artworkWidth, ArtworkHeight: artworkHeight, ArtworkEmbeddable: artworkEmbeddable,
			DurationMS: track.DurationMS,
		})
	}
	return items, nil
}

// selectSpotifyArtwork chooses the largest valid public HTTP(S) album image.
//
// Spotify track search already returns album.images URLs. Historically CCML
// copied the URL into MetadataCandidate but hard-coded ArtworkEmbeddable=false,
// so the shared Tagging Service never even attempted the download.
func selectSpotifyArtwork(images []spotifyImage) (string, int, int, bool) {
	bestURL := ""
	bestWidth := 0
	bestHeight := 0
	bestArea := -1

	for _, image := range images {
		rawURL := strings.TrimSpace(image.URL)
		if rawURL == "" {
			continue
		}
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Host == "" {
			continue
		}
		if parsed.Scheme != "https" && parsed.Scheme != "http" {
			continue
		}

		area := image.Width * image.Height
		if bestURL == "" || area > bestArea {
			bestURL = rawURL
			bestWidth = image.Width
			bestHeight = image.Height
			bestArea = area
		}
	}

	if bestURL == "" {
		return "", 0, 0, false
	}
	return bestURL, bestWidth, bestHeight, true
}

func (p *SpotifyProvider) token(ctx context.Context) (string, error) {
	if p.accessToken != "" {
		return p.accessToken, nil
	}
	if p.clientID == "" || p.clientSecret == "" {
		return "", fmt.Errorf("Spotify credentials are not configured")
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cachedToken != "" && time.Until(p.tokenExpiry) > time.Minute {
		return p.cachedToken, nil
	}

	values := url.Values{}
	values.Set("grant_type", "client_credentials")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://accounts.spotify.com/api/token", strings.NewReader(values.Encode()))
	if err != nil {
		return "", fmt.Errorf("create Spotify token request: %w", err)
	}
	req.SetBasicAuth(p.clientID, p.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request Spotify token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &providerHTTPError{Provider: "Spotify token", Status: resp.StatusCode, Retryable: isRetryableHTTPStatus(resp.StatusCode)}
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode Spotify token response: %w", err)
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return "", fmt.Errorf("Spotify token response did not include an access token")
	}
	expiresIn := payload.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	p.cachedToken = payload.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
	return p.cachedToken, nil
}
