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

type deezerTrack struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	Duration      int64  `json:"duration"`
	TrackPosition int    `json:"track_position"`
	DiskNumber    int    `json:"disk_number"`
	Link          string `json:"link"`
	ISRC          string `json:"isrc"`
	ReleaseDate   string `json:"release_date"`
	Artist        struct {
		Name string `json:"name"`
	} `json:"artist"`
	Album struct {
		Title    string `json:"title"`
		CoverXL  string `json:"cover_xl"`
		NbTracks int    `json:"nb_tracks"`
	} `json:"album"`
}

// DeezerProvider searches Deezer's public catalog endpoints.
type DeezerProvider struct {
	client  *http.Client
	baseURL string
}

func NewDeezerProvider() *DeezerProvider {
	return &DeezerProvider{client: &http.Client{Timeout: 15 * time.Second}, baseURL: "https://api.deezer.com"}
}

func (p *DeezerProvider) Name() string { return "Deezer" }

func (p *DeezerProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	if isrc := normalizeIdentifier(query.ISRC); isrc != "" {
		if item, found, err := p.searchISRC(ctx, isrc); err != nil {
			return nil, err
		} else if found {
			return []model.MetadataCandidate{item}, nil
		}
	}
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
	values.Set("strict", "on")
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(p.baseURL, "/")+"/search?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Deezer request: %w", err)
	}
	var payload struct {
		Data  []deezerTrack `json:"data"`
		Error *struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if err := getJSON(ctx, p.client, req, p.Name(), &payload); err != nil {
		return nil, err
	}
	if payload.Error != nil {
		return nil, fmt.Errorf("Deezer API error %d: %s", payload.Error.Code, payload.Error.Message)
	}
	items := make([]model.MetadataCandidate, 0, len(payload.Data))
	for _, track := range payload.Data {
		items = append(items, deezerCandidate(track))
	}
	return items, nil
}

func (p *DeezerProvider) searchISRC(ctx context.Context, isrc string) (model.MetadataCandidate, bool, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(p.baseURL, "/")+"/track/isrc:"+url.PathEscape(isrc), nil)
	if err != nil {
		return model.MetadataCandidate{}, false, fmt.Errorf("create Deezer ISRC request: %w", err)
	}
	var payload struct {
		deezerTrack
		Error *struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if err := getJSON(ctx, p.client, req, p.Name(), &payload); err != nil {
		return model.MetadataCandidate{}, false, err
	}
	if payload.Error != nil || payload.ID == 0 {
		return model.MetadataCandidate{}, false, nil
	}
	return deezerCandidate(payload.deezerTrack), true, nil
}

func deezerCandidate(track deezerTrack) model.MetadataCandidate {
	return model.MetadataCandidate{
		Source: "Deezer", ExternalID: strconv.FormatInt(track.ID, 10), SourceURL: track.Link,
		Title: track.Title, Artist: track.Artist.Name, Album: track.Album.Title,
		ReleaseDate: track.ReleaseDate, Year: yearFromDate(track.ReleaseDate), ISRC: track.ISRC,
		TrackNumber: track.TrackPosition, TrackTotal: track.Album.NbTracks, DiscNumber: track.DiskNumber,
		ArtworkURL: track.Album.CoverXL, ArtworkWidth: 1000, ArtworkHeight: 1000,
		ArtworkEmbeddable: strings.TrimSpace(track.Album.CoverXL) != "",
		DurationMS:        track.Duration * 1000,
	}
}
