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

	"github.com/spacesarmat/CCML/internal/model"
)

const traxsourceBridgeSearchURL = "https://api.parse.bot/scraper/14566d9b-cfe4-4533-94a8-986fdbaebf74/search"

// Search first tries the public Traxsource catalog directly. If that path is
// blocked or yields no usable results, an optional user-configured structured
// JSON fallback can be used. CCML does not emulate Cloudflare challenges.
func (p *TraxsourceProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	directItems, directErr := p.searchDirect(ctx, query)
	if directErr == nil && len(directItems) > 0 {
		return directItems, nil
	}

	if strings.TrimSpace(p.bridgeAPIKey) == "" {
		if isTraxsourceChallenge(directErr) {
			return nil, errors.New("Traxsource blocked the direct catalog request with Cloudflare/human verification; configure the optional Traxsource fallback API key in Settings")
		}
		return directItems, directErr
	}

	bridgeItems, bridgeErr := p.searchBridge(ctx, query)
	if bridgeErr == nil && len(bridgeItems) > 0 {
		return bridgeItems, nil
	}
	if directErr != nil && bridgeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("direct Traxsource lookup: %w", directErr),
			fmt.Errorf("Traxsource fallback lookup: %w", bridgeErr),
		)
	}
	if bridgeErr != nil {
		return directItems, fmt.Errorf("Traxsource fallback lookup: %w", bridgeErr)
	}
	return bridgeItems, directErr
}

func (p *TraxsourceProvider) searchBridge(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	term := strings.TrimSpace(strings.TrimSpace(query.Artist) + " " + strings.TrimSpace(query.Title))
	if term == "" {
		return nil, errors.New("artist or title is required")
	}

	endpoint := strings.TrimSpace(p.bridgeURL)
	if endpoint == "" {
		endpoint = traxsourceBridgeSearchURL
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse Traxsource fallback URL: %w", err)
	}
	values := parsed.Query()
	values.Set("term", term)
	values.Set("type", "tracks")
	values.Set("page", "1")
	parsed.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Traxsource fallback request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", strings.TrimSpace(p.bridgeAPIKey))
	if p.userAgent != "" {
		req.Header.Set("User-Agent", p.userAgent)
	}

	body, err := fetchProviderBytes(ctx, p.client, req, "Traxsource fallback", 2)
	if err != nil {
		return nil, err
	}
	return parseTraxsourceBridgeCandidates(body)
}

func parseTraxsourceBridgeCandidates(body []byte) ([]model.MetadataCandidate, error) {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode Traxsource fallback response: %w", err)
	}

	var rows []map[string]any
	collectTraxsourceBridgeRows(payload, &rows)

	items := make([]model.MetadataCandidate, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		title := bridgeString(row, "title", "track_title", "name")
		sourceURL := bridgeString(row, "url", "track_url", "source_url")
		externalID := bridgeString(row, "track_id", "trackId", "id")
		if externalID == "" && sourceURL != "" {
			externalID = lastNumericPathSegment(sourceURL)
		}
		if title == "" || (externalID == "" && sourceURL == "") {
			continue
		}

		version := bridgeString(row, "version", "mix", "mix_name")
		if version != "" && !strings.Contains(strings.ToLower(title), strings.ToLower(version)) {
			title += " (" + version + ")"
		}

		artistNames := bridgeNames(row["artists"])
		if len(artistNames) == 0 {
			if artist := bridgeString(row, "artist", "artist_name"); artist != "" {
				artistNames = []string{artist}
			}
		}

		releaseDate := bridgeString(row, "release_date", "releaseDate", "date")
		artwork := bridgeString(row, "artwork_url", "artworkUrl", "cover_url", "coverUrl", "image")
		label := bridgeNamedValue(row["label"])
		if label == "" {
			label = bridgeString(row, "label_name")
		}
		genre := bridgeNamedValue(row["genre"])
		if genre == "" {
			genre = bridgeString(row, "genre_name")
		}

		album := bridgeString(row, "release_title", "releaseTitle", "album")
		if album == "" {
			album = bridgeNestedTitle(row["release"])
		}

		key := externalID + "\x00" + sourceURL + "\x00" + strings.ToLower(title)
		if seen[key] {
			continue
		}
		seen[key] = true

		items = append(items, model.MetadataCandidate{
			Source:            "Traxsource",
			ExternalID:        externalID,
			SourceURL:         sourceURL,
			Title:             title,
			Artist:            strings.Join(artistNames, ", "),
			Album:             album,
			ReleaseDate:       releaseDate,
			Year:              yearFromDate(releaseDate),
			Genre:             genre,
			Label:             label,
			CatalogNumber:     bridgeString(row, "catalog_number", "catalogNumber", "catno"),
			ISRC:              bridgeString(row, "isrc", "ISRC"),
			TrackNumber:       bridgeInt(row, "track_number", "trackNumber"),
			TrackTotal:        bridgeInt(row, "track_total", "trackTotal"),
			ArtworkURL:        artwork,
			ArtworkEmbeddable: false,
			DurationMS:        bridgeDurationMS(row),
		})
		if len(items) >= 20 {
			break
		}
	}
	return items, nil
}

func collectTraxsourceBridgeRows(value any, out *[]map[string]any) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectTraxsourceBridgeRows(item, out)
		}
	case map[string]any:
		if looksLikeTraxsourceBridgeTrack(typed) {
			*out = append(*out, typed)
			return
		}
		for _, child := range typed {
			collectTraxsourceBridgeRows(child, out)
		}
	}
}

func looksLikeTraxsourceBridgeTrack(row map[string]any) bool {
	if bridgeString(row, "track_id", "trackId") != "" {
		return bridgeString(row, "title", "track_title", "name") != ""
	}
	sourceURL := bridgeString(row, "url", "track_url", "source_url")
	return strings.Contains(sourceURL, "/track/") && bridgeString(row, "title", "track_title", "name") != ""
}

func bridgeString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := row[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if text := strings.TrimSpace(typed); text != "" {
				return text
			}
		case float64:
			if typed == float64(int64(typed)) {
				return strconv.FormatInt(int64(typed), 10)
			}
		}
	}
	return ""
}

func bridgeInt(row map[string]any, keys ...string) int {
	for _, key := range keys {
		value, ok := row[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return int(typed)
		case string:
			n, err := strconv.Atoi(strings.TrimSpace(typed))
			if err == nil {
				return n
			}
		}
	}
	return 0
}

func bridgeNames(value any) []string {
	var result []string
	seen := map[string]bool{}
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			name := bridgeNamedValue(item)
			if name != "" && !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}
	case string:
		for _, part := range strings.Split(typed, ",") {
			name := strings.TrimSpace(part)
			if name != "" && !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}
	default:
		if name := bridgeNamedValue(typed); name != "" {
			result = append(result, name)
		}
	}
	return result
}

func bridgeNamedValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		return bridgeString(typed, "name", "title", "label", "genre")
	}
	return ""
}

func bridgeNestedTitle(value any) string {
	if row, ok := value.(map[string]any); ok {
		return bridgeString(row, "title", "name")
	}
	return bridgeNamedValue(value)
}

func bridgeDurationMS(row map[string]any) int64 {
	for _, key := range []string{"duration_ms", "durationMs"} {
		if value, ok := row[key]; ok {
			switch typed := value.(type) {
			case float64:
				if typed > 0 {
					return int64(typed)
				}
			case string:
				if n, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil && n > 0 {
					return n
				}
			}
		}
	}
	return parseClockDurationMS(bridgeString(row, "duration", "length"))
}

func isTraxsourceChallenge(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *providerHTTPError
	if errors.As(err, &httpErr) {
		if httpErr.Status == http.StatusForbidden {
			return true
		}
		body := strings.ToLower(httpErr.Body)
		if strings.Contains(body, "cloudflare") ||
			strings.Contains(body, "cf-chl-") ||
			strings.Contains(body, "just a moment") ||
			strings.Contains(body, "verify you are human") ||
			strings.Contains(body, "human verification") {
			return true
		}
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "cloudflare") ||
		strings.Contains(message, "human verification") ||
		strings.Contains(message, "http 403")
}
