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
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

// TheAudioDBProvider searches TheAudioDB v1 track endpoint.
type TheAudioDBProvider struct {
	client *http.Client
	apiKey string
}

// NewTheAudioDBProvider creates a provider. The public free key is used when
// THEAUDIODB_API_KEY is not configured.
func NewTheAudioDBProvider(apiKey string) *TheAudioDBProvider {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		apiKey = "123"
	}
	return &TheAudioDBProvider{
		client: &http.Client{Timeout: 15 * time.Second},
		apiKey: apiKey,
	}
}

// Name returns the provider name.
func (p *TheAudioDBProvider) Name() string { return "TheAudioDB" }

// Search searches tracks by artist/title.
func (p *TheAudioDBProvider) Search(ctx context.Context, query model.MetadataQuery) (items []model.MetadataCandidate, resultErr error) {
	if strings.TrimSpace(query.Artist) == "" || strings.TrimSpace(query.Title) == "" {
		return nil, fmt.Errorf("artist and title are required")
	}
	values := url.Values{}
	values.Set("s", query.Artist)
	values.Set("t", query.Title)
	endpoint := "https://www.theaudiodb.com/api/v1/json/" + url.PathEscape(p.apiKey) + "/searchtrack.php?" + values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create TheAudioDB request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request TheAudioDB: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close TheAudioDB response body: %w", err))
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TheAudioDB returned HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Track []struct {
			ID          string `json:"idTrack"`
			Title       string `json:"strTrack"`
			Album       string `json:"strAlbum"`
			Artist      string `json:"strArtist"`
			Duration    string `json:"intDuration"`
			Genre       string `json:"strGenre"`
			Artwork     string `json:"strTrackThumb"`
			MusicBrainz string `json:"strMusicBrainzID"`
		} `json:"track"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode TheAudioDB response: %w", err)
	}

	items = make([]model.MetadataCandidate, 0, len(payload.Track))
	for _, track := range payload.Track {
		duration, err := strconv.ParseInt(strings.TrimSpace(track.Duration), 10, 64)
		if err != nil {
			duration = 0
		}
		externalID := track.ID
		if track.MusicBrainz != "" {
			externalID = track.MusicBrainz
		}
		items = append(items, model.MetadataCandidate{
			Source:     p.Name(),
			ExternalID: externalID,
			Title:      track.Title,
			Artist:     track.Artist,
			Album:      track.Album,
			Genre:      track.Genre,
			ArtworkURL: track.Artwork,
			DurationMS: duration,
			Confidence: metadataSimilarity(query, track.Artist, track.Title, duration),
		})
	}
	return items, nil
}

func metadataSimilarity(query model.MetadataQuery, artist, title string, duration int64) float64 {
	score := 0.0
	if strings.EqualFold(strings.TrimSpace(query.Artist), strings.TrimSpace(artist)) {
		score += 0.45
	}
	if strings.EqualFold(strings.TrimSpace(query.Title), strings.TrimSpace(title)) {
		score += 0.45
	}
	if query.DurationMS > 0 && duration > 0 {
		delta := query.DurationMS - duration
		if delta < 0 {
			delta = -delta
		}
		if delta <= 2_000 {
			score += 0.10
		}
	}
	return score
}
