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

// DeezerProvider searches Deezer's track catalog endpoint.
type DeezerProvider struct {
	client *http.Client
}

// NewDeezerProvider creates a Deezer provider.
func NewDeezerProvider() *DeezerProvider {
	return &DeezerProvider{client: &http.Client{Timeout: 15 * time.Second}}
}

// Name returns the provider name.
func (p *DeezerProvider) Name() string { return "Deezer" }

// Search searches Deezer tracks. Availability is subject to the developer
// account/API terms applicable to the deploying application.
func (p *DeezerProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	if strings.TrimSpace(query.Title) == "" {
		return nil, fmt.Errorf("title is required")
	}
	terms := []string{fmt.Sprintf(`track:"%s"`, strings.ReplaceAll(query.Title, `"`, ``))}
	if strings.TrimSpace(query.Artist) != "" {
		terms = append(terms, fmt.Sprintf(`artist:"%s"`, strings.ReplaceAll(query.Artist, `"`, ``)))
	}
	values := url.Values{}
	values.Set("q", strings.Join(terms, " "))
	values.Set("limit", "10")
	req, err := http.NewRequest(http.MethodGet, "https://api.deezer.com/search?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Deezer request: %w", err)
	}

	var payload struct {
		Data []struct {
			ID       int64  `json:"id"`
			Title    string `json:"title"`
			Duration int64  `json:"duration"`
			Artist   struct {
				Name string `json:"name"`
			} `json:"artist"`
			Album struct {
				Title   string `json:"title"`
				CoverXL string `json:"cover_xl"`
			} `json:"album"`
		} `json:"data"`
	}
	if err := getJSON(ctx, p.client, req, p.Name(), &payload); err != nil {
		return nil, err
	}
	items := make([]model.MetadataCandidate, 0, len(payload.Data))
	for _, track := range payload.Data {
		durationMS := track.Duration * 1000
		items = append(items, model.MetadataCandidate{
			Source:     p.Name(),
			ExternalID: strconv.FormatInt(track.ID, 10),
			Title:      track.Title,
			Artist:     track.Artist.Name,
			Album:      track.Album.Title,
			ArtworkURL: track.Album.CoverXL,
			DurationMS: durationMS,
			Confidence: metadataSimilarity(query, track.Artist.Name, track.Title, durationMS),
		})
	}
	return items, nil
}
