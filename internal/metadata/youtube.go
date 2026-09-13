package metadata

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

// YouTubeProvider searches YouTube Data API v3. It is an approximation for
// YouTube Music discovery because Google does not expose a separate public
// YouTube Music catalog-search API.
type YouTubeProvider struct {
	client *http.Client
	apiKey string
}

// NewYouTubeProvider creates a YouTube Data API provider.
func NewYouTubeProvider(apiKey string) *YouTubeProvider {
	return &YouTubeProvider{client: &http.Client{Timeout: 15 * time.Second}, apiKey: strings.TrimSpace(apiKey)}
}

// Name returns the provider name.
func (p *YouTubeProvider) Name() string { return "YouTube" }

// Search searches YouTube videos by artist and title.
func (p *YouTubeProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("YouTube API key is not configured")
	}
	term := strings.TrimSpace(strings.TrimSpace(query.Artist) + " " + strings.TrimSpace(query.Title))
	if term == "" {
		return nil, fmt.Errorf("artist or title is required")
	}
	values := url.Values{}
	values.Set("part", "snippet")
	values.Set("type", "video")
	values.Set("maxResults", "10")
	values.Set("q", term)
	values.Set("key", p.apiKey)
	req, err := http.NewRequest(http.MethodGet, "https://www.googleapis.com/youtube/v3/search?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create YouTube request: %w", err)
	}

	var payload struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
			Snippet struct {
				Title        string `json:"title"`
				ChannelTitle string `json:"channelTitle"`
				Thumbnails   struct {
					High struct {
						URL string `json:"url"`
					} `json:"high"`
					Default struct {
						URL string `json:"url"`
					} `json:"default"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := getJSON(ctx, p.client, req, p.Name(), &payload); err != nil {
		return nil, err
	}

	items := make([]model.MetadataCandidate, 0, len(payload.Items))
	for _, video := range payload.Items {
		artwork := video.Snippet.Thumbnails.High.URL
		if artwork == "" {
			artwork = video.Snippet.Thumbnails.Default.URL
		}
		title := html.UnescapeString(video.Snippet.Title)
		artist := html.UnescapeString(video.Snippet.ChannelTitle)
		items = append(items, model.MetadataCandidate{
			Source:     p.Name(),
			ExternalID: video.ID.VideoID,
			SourceURL:  "https://www.youtube.com/watch?v=" + video.ID.VideoID,
			Title:      title,
			Artist:     artist,
			ArtworkURL: artwork,
			Confidence: metadataSimilarity(query, artist, title, 0),
		})
	}
	return items, nil
}
