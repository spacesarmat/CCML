package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

const yandexMusicBaseURL = "https://api.music.yandex.net"

// YandexMusicProvider uses the private JSON API consumed by Yandex Music clients.
// Yandex does not publish this as a supported third-party Music API, so the
// provider is intentionally opt-in and marked experimental in Settings.
type YandexMusicProvider struct {
	client    *http.Client
	baseURL   string
	token     string
	language  string
	userAgent string
}

func NewYandexMusicProvider(token, language, userAgent string) *YandexMusicProvider {
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "" {
		language = "ru"
	}
	return &YandexMusicProvider{
		client: &http.Client{Timeout: 15 * time.Second}, baseURL: yandexMusicBaseURL,
		token: strings.TrimSpace(token), language: language, userAgent: strings.TrimSpace(userAgent),
	}
}

func (p *YandexMusicProvider) Name() string { return "Yandex Music" }

func (p *YandexMusicProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	term := strings.TrimSpace(strings.TrimSpace(query.Artist) + " " + strings.TrimSpace(query.Title))
	if term == "" {
		return nil, fmt.Errorf("artist or title is required")
	}
	values := url.Values{}
	values.Set("text", term)
	values.Set("type", "track")
	values.Set("page", "0")
	values.Set("nocorrect", "false")
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(p.baseURL, "/")+"/search?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Yandex Music request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", p.language)
	req.Header.Set("X-Yandex-Music-Client", "CCML-desktop")
	if p.userAgent != "" {
		req.Header.Set("User-Agent", p.userAgent)
	}
	if p.token != "" {
		req.Header.Set("Authorization", "OAuth "+p.token)
	}

	var payload struct {
		Result struct {
			Tracks struct {
				Results []yandexMusicTrack `json:"results"`
			} `json:"tracks"`
		} `json:"result"`
		Error struct {
			Name    string `json:"name"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := getJSON(ctx, p.client, req, p.Name(), &payload); err != nil {
		return nil, err
	}
	if payload.Error.Name != "" || payload.Error.Message != "" {
		return nil, fmt.Errorf("Yandex Music API error: %s %s", payload.Error.Name, payload.Error.Message)
	}

	items := make([]model.MetadataCandidate, 0, min(10, len(payload.Result.Tracks.Results)))
	for _, track := range payload.Result.Tracks.Results {
		if len(items) >= 10 {
			break
		}
		items = append(items, yandexMusicCandidate(track))
	}
	return items, nil
}

type yandexJSONID json.RawMessage

func (id *yandexJSONID) UnmarshalJSON(data []byte) error {
	*id = append((*id)[:0], data...)
	return nil
}

func (id yandexJSONID) String() string {
	if len(id) == 0 || string(id) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(id, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(id, &n) == nil {
		return n.String()
	}
	return strings.Trim(string(id), `"`)
}

type yandexMusicArtist struct {
	ID   yandexJSONID `json:"id"`
	Name string       `json:"name"`
}

type yandexMusicLabel struct {
	Name string `json:"name"`
}

type yandexMusicAlbum struct {
	ID          yandexJSONID        `json:"id"`
	Title       string              `json:"title"`
	TrackCount  int                 `json:"trackCount"`
	Artists     []yandexMusicArtist `json:"artists"`
	Labels      []yandexMusicLabel  `json:"labels"`
	Genre       string              `json:"genre"`
	Year        int                 `json:"year"`
	ReleaseDate string              `json:"releaseDate"`
	TrackPos    struct {
		Volume int `json:"volume"`
		Index  int `json:"index"`
	} `json:"trackPosition"`
}

type yandexMusicTrack struct {
	ID         yandexJSONID        `json:"id"`
	Title      string              `json:"title"`
	Version    string              `json:"version"`
	Artists    []yandexMusicArtist `json:"artists"`
	Albums     []yandexMusicAlbum  `json:"albums"`
	DurationMS int64               `json:"durationMs"`
	CoverURI   string              `json:"coverUri"`
	OGImage    string              `json:"ogImage"`
}

func yandexMusicCandidate(track yandexMusicTrack) model.MetadataCandidate {
	title := strings.TrimSpace(track.Title)
	version := strings.TrimSpace(track.Version)
	if version != "" && !strings.Contains(strings.ToLower(title), strings.ToLower(version)) {
		title += " (" + version + ")"
	}
	artists := make([]string, 0, len(track.Artists))
	for _, artist := range track.Artists {
		if name := strings.TrimSpace(artist.Name); name != "" {
			artists = append(artists, name)
		}
	}

	candidate := model.MetadataCandidate{
		Source: "Yandex Music", ExternalID: track.ID.String(), Title: title,
		Artist: strings.Join(artists, ", "), DurationMS: track.DurationMS,
		ArtworkURL:   yandexArtworkURL(firstNonEmpty(track.CoverURI, track.OGImage)),
		ArtworkWidth: 1000, ArtworkHeight: 1000, ArtworkEmbeddable: false,
	}
	if len(track.Albums) > 0 {
		album := track.Albums[0]
		candidate.Album = strings.TrimSpace(album.Title)
		candidate.ReleaseDate = strings.TrimSpace(album.ReleaseDate)
		candidate.Year = album.Year
		if candidate.Year == 0 {
			candidate.Year = yearFromDate(candidate.ReleaseDate)
		}
		candidate.Genre = strings.TrimSpace(album.Genre)
		candidate.TrackNumber = album.TrackPos.Index
		candidate.DiscNumber = album.TrackPos.Volume
		candidate.TrackTotal = album.TrackCount
		if len(album.Labels) > 0 {
			candidate.Label = strings.TrimSpace(album.Labels[0].Name)
		}
		albumArtists := make([]string, 0, len(album.Artists))
		for _, artist := range album.Artists {
			if name := strings.TrimSpace(artist.Name); name != "" {
				albumArtists = append(albumArtists, name)
			}
		}
		candidate.AlbumArtist = strings.Join(albumArtists, ", ")
		albumID := album.ID.String()
		if albumID != "" && candidate.ExternalID != "" {
			candidate.SourceURL = "https://music.yandex.ru/album/" + url.PathEscape(albumID) + "/track/" + url.PathEscape(candidate.ExternalID)
		}
	}
	if candidate.SourceURL == "" && candidate.ExternalID != "" {
		candidate.SourceURL = "https://music.yandex.ru/track/" + url.PathEscape(candidate.ExternalID)
	}
	return candidate
}

func yandexArtworkURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "%%", "1000x1000")
	if strings.HasPrefix(value, "//") {
		return "https:" + value
	}
	if !strings.Contains(value, "://") {
		return "https://" + value
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
