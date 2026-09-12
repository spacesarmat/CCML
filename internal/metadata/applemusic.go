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

// AppleMusicProvider searches the Apple Music catalog with a developer token.
type AppleMusicProvider struct {
	client     *http.Client
	token      string
	storefront string
}

// NewAppleMusicProvider creates an Apple Music catalog provider.
func NewAppleMusicProvider(token, storefront string) *AppleMusicProvider {
	storefront = strings.ToLower(strings.TrimSpace(storefront))
	if storefront == "" {
		storefront = "us"
	}
	return &AppleMusicProvider{
		client:     &http.Client{Timeout: 15 * time.Second},
		token:      strings.TrimSpace(token),
		storefront: storefront,
	}
}

// Name returns the provider name.
func (p *AppleMusicProvider) Name() string { return "Apple Music" }

// Search searches Apple Music catalog songs.
func (p *AppleMusicProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	if p.token == "" {
		return nil, fmt.Errorf("Apple Music developer token is not configured")
	}
	term := strings.TrimSpace(strings.TrimSpace(query.Artist) + " " + strings.TrimSpace(query.Title))
	if term == "" {
		return nil, fmt.Errorf("artist or title is required")
	}

	values := url.Values{}
	values.Set("term", term)
	values.Set("types", "songs")
	values.Set("limit", "10")
	endpoint := "https://api.music.apple.com/v1/catalog/" + url.PathEscape(p.storefront) + "/search?" + values.Encode()
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create Apple Music request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Accept", "application/json")

	var payload struct {
		Results struct {
			Songs struct {
				Data []struct {
					ID         string `json:"id"`
					Attributes struct {
						Name             string   `json:"name"`
						ArtistName       string   `json:"artistName"`
						AlbumName        string   `json:"albumName"`
						ReleaseDate      string   `json:"releaseDate"`
						DurationInMillis int64    `json:"durationInMillis"`
						GenreNames       []string `json:"genreNames"`
						TrackNumber      int      `json:"trackNumber"`
						DiscNumber       int      `json:"discNumber"`
						ISRC             string   `json:"isrc"`
						URL              string   `json:"url"`
						Artwork          struct {
							URL string `json:"url"`
						} `json:"artwork"`
					} `json:"attributes"`
				} `json:"data"`
			} `json:"songs"`
		} `json:"results"`
	}
	if err := getJSON(ctx, p.client, req, p.Name(), &payload); err != nil {
		return nil, err
	}

	items := make([]model.MetadataCandidate, 0, len(payload.Results.Songs.Data))
	for _, song := range payload.Results.Songs.Data {
		attr := song.Attributes
		artwork := strings.ReplaceAll(attr.Artwork.URL, "{w}x{h}", "600x600")
		genre := ""
		if len(attr.GenreNames) > 0 {
			genre = attr.GenreNames[0]
		}
		items = append(items, model.MetadataCandidate{
			Source: p.Name(), ExternalID: song.ID, SourceURL: attr.URL,
			Title: attr.Name, Artist: attr.ArtistName, Album: attr.AlbumName, ReleaseDate: attr.ReleaseDate,
			Year: yearFromDate(attr.ReleaseDate), Genre: genre, ISRC: attr.ISRC, TrackNumber: attr.TrackNumber, DiscNumber: attr.DiscNumber,
			ArtworkURL: artwork, ArtworkWidth: 600, ArtworkHeight: 600, ArtworkEmbeddable: false, DurationMS: attr.DurationInMillis,
		})
	}
	return items, nil
}
