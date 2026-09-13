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

const maxDiscogsDetails = 3

// DiscogsProvider searches the Discogs database API using a personal token.
type DiscogsProvider struct {
	client    *http.Client
	token     string
	userAgent string
	mu        sync.Mutex
	lastCall  time.Time
}

func NewDiscogsProvider(token, userAgent string) *DiscogsProvider {
	return &DiscogsProvider{
		client:    &http.Client{Timeout: 15 * time.Second},
		token:     strings.TrimSpace(token),
		userAgent: strings.TrimSpace(userAgent),
	}
}

func (p *DiscogsProvider) Name() string { return "Discogs" }

func (p *DiscogsProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	if p.token == "" {
		return nil, fmt.Errorf("Discogs token is not configured")
	}
	if strings.TrimSpace(query.Title) == "" {
		return nil, fmt.Errorf("title is required")
	}
	values := url.Values{}
	values.Set("type", "release")
	values.Set("track", query.Title)
	values.Set("per_page", "10")
	if strings.TrimSpace(query.Artist) != "" {
		values.Set("artist", query.Artist)
	}
	if strings.TrimSpace(query.Album) != "" {
		values.Set("release_title", query.Album)
	}

	var payload struct {
		Results []struct {
			ID          int64  `json:"id"`
			ResourceURL string `json:"resource_url"`
		} `json:"results"`
	}
	if err := p.getJSON(ctx, "https://api.discogs.com/database/search?"+values.Encode(), &payload); err != nil {
		return nil, err
	}

	limit := len(payload.Results)
	if limit > maxDiscogsDetails {
		limit = maxDiscogsDetails
	}
	items := make([]model.MetadataCandidate, 0, limit)
	var firstDetailErr error
	for _, result := range payload.Results[:limit] {
		if ctx.Err() != nil {
			return items, ctx.Err()
		}
		if strings.TrimSpace(result.ResourceURL) == "" {
			continue
		}
		candidate, ok, err := p.releaseCandidate(ctx, result.ResourceURL, query)
		if err != nil {
			// A single malformed/unavailable release should not discard the other
			// search results. Keep the first error for diagnostics if all details fail.
			if firstDetailErr == nil {
				firstDetailErr = err
			}
			continue
		}
		if ok {
			items = append(items, candidate)
		}
	}
	if len(items) == 0 && firstDetailErr != nil {
		return nil, firstDetailErr
	}
	return items, nil
}

func (p *DiscogsProvider) releaseCandidate(ctx context.Context, endpoint string, query model.MetadataQuery) (model.MetadataCandidate, bool, error) {
	var release struct {
		ID          int64    `json:"id"`
		Title       string   `json:"title"`
		Year        int      `json:"year"`
		Released    string   `json:"released"`
		ArtistsSort string   `json:"artists_sort"`
		URI         string   `json:"uri"`
		Genres      []string `json:"genres"`
		Styles      []string `json:"styles"`
		Labels      []struct {
			Name  string `json:"name"`
			Catno string `json:"catno"`
		} `json:"labels"`
		Images []struct {
			Type   string `json:"type"`
			URI    string `json:"uri"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"images"`
		Tracklist []struct {
			Position string `json:"position"`
			Title    string `json:"title"`
			Duration string `json:"duration"`
			Artists  []struct {
				Name string `json:"name"`
			} `json:"artists"`
		} `json:"tracklist"`
	}
	if err := p.getJSON(ctx, endpoint, &release); err != nil {
		return model.MetadataCandidate{}, false, err
	}

	bestIndex := -1
	bestScore := 0.0
	for i, track := range release.Tracklist {
		score := textSimilarity(query.Title, track.Title)
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}
	if bestIndex < 0 || bestScore < 0.45 {
		return model.MetadataCandidate{}, false, nil
	}
	track := release.Tracklist[bestIndex]
	artist := strings.TrimSpace(release.ArtistsSort)
	if len(track.Artists) > 0 {
		names := make([]string, 0, len(track.Artists))
		for _, a := range track.Artists {
			if strings.TrimSpace(a.Name) != "" {
				names = append(names, strings.TrimSpace(a.Name))
			}
		}
		if len(names) > 0 {
			artist = strings.Join(names, ", ")
		}
	}
	genre := ""
	if len(release.Genres) > 0 {
		genre = release.Genres[0]
	} else if len(release.Styles) > 0 {
		genre = release.Styles[0]
	}
	label, catno := "", ""
	if len(release.Labels) > 0 {
		label = release.Labels[0].Name
		catno = release.Labels[0].Catno
	}
	artwork, width, height := "", 0, 0
	for _, image := range release.Images {
		if artwork == "" || strings.EqualFold(image.Type, "primary") {
			artwork, width, height = image.URI, image.Width, image.Height
			if strings.EqualFold(image.Type, "primary") {
				break
			}
		}
	}
	return model.MetadataCandidate{
		Source: p.Name(), ExternalID: strconv.FormatInt(release.ID, 10), SourceURL: release.URI,
		Title: track.Title, Artist: artist, Album: release.Title,
		ReleaseDate: release.Released, Year: release.Year, Genre: genre, Label: label, CatalogNumber: catno,
		TrackNumber: parseDiscogsPosition(track.Position), DurationMS: parseDiscogsDuration(track.Duration),
		ArtworkURL: artwork, ArtworkWidth: width, ArtworkHeight: height, ArtworkEmbeddable: artwork != "",
	}, true, nil
}

func (p *DiscogsProvider) getJSON(ctx context.Context, endpoint string, target any) error {
	p.mu.Lock()
	wait := time.Second - time.Since(p.lastCall)
	if !p.lastCall.IsZero() && wait > 0 {
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			p.mu.Unlock()
			return ctx.Err()
		}
		timer.Stop()
	}
	p.lastCall = time.Now()
	p.mu.Unlock()

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create Discogs request: %w", err)
	}
	req.Header.Set("Authorization", "Discogs token="+p.token)
	if p.userAgent != "" {
		req.Header.Set("User-Agent", p.userAgent)
	}
	req.Header.Set("Accept", "application/json")
	return getJSON(ctx, p.client, req, p.Name(), target)
}

func parseDiscogsDuration(value string) int64 {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0
	}
	minutes, err1 := strconv.Atoi(parts[0])
	seconds, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || minutes < 0 || seconds < 0 {
		return 0
	}
	return int64(minutes*60+seconds) * 1000
}

func parseDiscogsPosition(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if n, err := strconv.Atoi(value); err == nil {
		return n
	}
	return 0
}
