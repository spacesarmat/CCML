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

// TheAudioDBProvider searches TheAudioDB v1 track endpoint.
type TheAudioDBProvider struct {
	client      *http.Client
	apiKey      string
	baseURL     string
	mu          sync.Mutex
	lastCall    time.Time
	minInterval time.Duration
}

// NewTheAudioDBProvider creates a provider. The public free key is used when
// THEAUDIODB_API_KEY is not configured.
func NewTheAudioDBProvider(apiKey string) *TheAudioDBProvider {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		apiKey = "123"
	}
	return &TheAudioDBProvider{
		client:      &http.Client{Timeout: 15 * time.Second},
		apiKey:      apiKey,
		baseURL:     "https://www.theaudiodb.com/api/v1/json",
		minInterval: 2 * time.Second,
	}
}

func (p *TheAudioDBProvider) Name() string { return "TheAudioDB" }

func (p *TheAudioDBProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	if strings.TrimSpace(query.Artist) == "" || strings.TrimSpace(query.Title) == "" {
		return nil, fmt.Errorf("artist and title are required")
	}
	if err := p.waitForRateLimit(ctx); err != nil {
		return nil, err
	}
	values := url.Values{}
	values.Set("s", query.Artist)
	values.Set("t", query.Title)
	endpoint := strings.TrimRight(p.baseURL, "/") + "/" + url.PathEscape(p.apiKey) + "/searchtrack.php?" + values.Encode()
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create TheAudioDB request: %w", err)
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
			TrackNumber string `json:"intTrackNumber"`
			Year        string `json:"intYearReleased"`
		} `json:"track"`
	}
	if err := getJSON(ctx, p.client, req, p.Name(), &payload); err != nil {
		return nil, err
	}

	items := make([]model.MetadataCandidate, 0, len(payload.Track))
	for _, track := range payload.Track {
		duration, _ := strconv.ParseInt(strings.TrimSpace(track.Duration), 10, 64)
		trackNumber, _ := strconv.Atoi(strings.TrimSpace(track.TrackNumber))
		year, _ := strconv.Atoi(strings.TrimSpace(track.Year))
		externalID := track.ID
		if track.MusicBrainz != "" {
			externalID = track.MusicBrainz
		}
		items = append(items, model.MetadataCandidate{
			Source: p.Name(), ExternalID: externalID,
			Title: track.Title, Artist: track.Artist, Album: track.Album, Genre: track.Genre,
			Year: year, TrackNumber: trackNumber,
			ArtworkURL: track.Artwork, ArtworkEmbeddable: strings.TrimSpace(track.Artwork) != "", DurationMS: duration,
		})
	}
	return items, nil
}

func (p *TheAudioDBProvider) waitForRateLimit(ctx context.Context) error {
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
