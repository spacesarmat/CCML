package metadata

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

const mixcloudBaseURL = "https://api.mixcloud.com"

type MixcloudProvider struct {
	client    *http.Client
	baseURL   string
	userAgent string
}

type mixcloudSearchResponse struct {
	Data []mixcloudCloudcast `json:"data"`
}

type mixcloudCloudcast struct {
	Key  string        `json:"key"`
	Name string        `json:"name"`
	URL  string        `json:"url"`
	User mixcloudUser  `json:"user"`
	Tags []mixcloudTag `json:"tags"`
}

type mixcloudUser struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Key      string `json:"key"`
}

type mixcloudTag struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

func NewMixcloudProvider(userAgent string) *MixcloudProvider {
	return newMixcloudProviderWithBaseURL(mixcloudBaseURL, userAgent)
}

func newMixcloudProviderWithBaseURL(baseURL, userAgent string) *MixcloudProvider {
	return &MixcloudProvider{
		client:    &http.Client{Timeout: 15 * time.Second},
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		userAgent: strings.TrimSpace(userAgent),
	}
}

func (p *MixcloudProvider) Name() string { return "Mixcloud" }
func (p *MixcloudProvider) Kind() string { return ProviderKindDJPool }

func (p *MixcloudProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	term := mixcloudSearchTerm(query)
	if term == "" {
		return nil, fmt.Errorf("artist or title is required")
	}

	target := mixcloudSearchURL(p.baseURL, term)
	mixcloudDebugf("search start artist=%q title=%q term=%q url=%s", query.Artist, query.Title, term, target)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("create Mixcloud request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.8,ru;q=0.7")
	if p.userAgent != "" {
		req.Header.Set("User-Agent", p.userAgent)
	} else {
		req.Header.Set("User-Agent", "CCML metadata client")
	}

	mixcloudDebugf("HTTP GET %s", target)
	body, err := fetchProviderBytes(ctx, p.client, req, p.Name(), 1)
	if err != nil {
		mixcloudDebugf("HTTP FAIL url=%s error=%v", target, err)
		return nil, err
	}
	mixcloudDebugf("HTTP OK url=%s bytes=%d", target, len(body))

	items, err := mixcloudCandidatesFromJSON(body, target, query)
	if err != nil {
		mixcloudDebugf("JSON parse failed: %v", err)
		return nil, err
	}
	mixcloudDebugCandidates("matched", query, items)

	sort.SliceStable(items, func(i, j int) bool {
		return mixcloudQueryFit(query, items[i]) > mixcloudQueryFit(query, items[j])
	})
	if len(items) > 12 {
		items = items[:12]
	}
	mixcloudDebugf("search finished matches=%d", len(items))
	return items, nil
}

func mixcloudSearchTerm(query model.MetadataQuery) string {
	artist := strings.TrimSpace(query.Artist)
	title := strings.TrimSpace(query.Title)
	switch {
	case artist != "" && title != "":
		return artist + " - " + title
	case artist != "":
		return artist
	default:
		return title
	}
}

func mixcloudSearchURL(baseURL, term string) string {
	values := url.Values{}
	values.Set("limit", "20")
	values.Set("q", strings.TrimSpace(term))
	values.Set("type", "cloudcast")
	return strings.TrimRight(baseURL, "/") + "/search/?" + values.Encode()
}

func mixcloudCandidatesFromJSON(body []byte, sourceURL string, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	var response mixcloudSearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode Mixcloud search response: %w", err)
	}

	mixcloudDebugf("JSON data=%d", len(response.Data))
	out := make([]model.MetadataCandidate, 0, len(response.Data))
	seen := map[string]bool{}

	for index, cloudcast := range response.Data {
		mixcloudDebugf(
			"raw[%d] name=%q user=%q username=%q tags=%q key=%q",
			index,
			cloudcast.Name,
			cloudcast.User.Name,
			cloudcast.User.Username,
			mixcloudTagNames(cloudcast.Tags),
			cloudcast.Key,
		)

		item, ok := mixcloudCandidateFromCloudcast(cloudcast, sourceURL, query)
		if !ok {
			continue
		}
		key := normalizeText(item.Artist) + "\x00" + normalizeText(item.Title) + "\x00" + item.ExternalID
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out, nil
}

func mixcloudCandidateFromCloudcast(cloudcast mixcloudCloudcast, searchURL string, query model.MetadataQuery) (model.MetadataCandidate, bool) {
	name := strings.TrimSpace(cloudcast.Name)
	if name == "" {
		return model.MetadataCandidate{}, false
	}

	sourceURL := strings.TrimSpace(cloudcast.URL)
	if sourceURL == "" {
		if key := strings.TrimSpace(cloudcast.Key); key != "" {
			sourceURL = "https://www.mixcloud.com/" + strings.TrimLeft(key, "/")
		} else {
			sourceURL = searchURL
		}
	}

	genre := mixcloudGenreFromTags(cloudcast.Tags)
	bestScore := -1.0
	best := model.MetadataCandidate{}

	add := func(artist, title string) {
		artist = strings.TrimSpace(artist)
		title = strings.TrimSpace(title)
		if artist == "" || title == "" {
			return
		}
		item := model.MetadataCandidate{
			Source:     "Mixcloud",
			SourceKind: ProviderKindDJPool,
			ExternalID: mixcloudExternalID(cloudcast.Key, artist, title),
			SourceURL:  sourceURL,
			Title:      title,
			Artist:     artist,
			Genre:      genre,
		}
		score := mixcloudQueryFit(query, item)
		if score < 0 {
			return
		}
		if score > bestScore {
			bestScore = score
			best = item
		}
	}

	// Many public DJ uploads use "Artist - Title" directly as the show name.
	// Try every separator because the title itself may contain " - ".
	if strings.TrimSpace(query.Artist) != "" && strings.TrimSpace(query.Title) != "" {
		searchFrom := 0
		for {
			relative := strings.Index(name[searchFrom:], " - ")
			if relative < 0 {
				break
			}
			index := searchFrom + relative
			add(name[:index], name[index+3:])
			searchFrom = index + 3
		}
	}

	// For creator-owned uploads, the Mixcloud user can be the artist and the
	// show name can be the track title. This path is accepted only when the
	// shared query-fit thresholds are met.
	userName := strings.TrimSpace(cloudcast.User.Name)
	if userName == "" {
		userName = strings.TrimSpace(cloudcast.User.Username)
	}
	add(userName, name)

	if bestScore < 0 {
		return model.MetadataCandidate{}, false
	}
	return best, true
}

func mixcloudGenreFromTags(tags []mixcloudTag) string {
	known := []string{
		"house", "deep house", "tech house", "afro house", "progressive house",
		"melodic house", "electro house", "disco house", "organic house",
		"techno", "melodic techno", "trance", "progressive trance", "edm",
		"dance", "pop", "hip hop", "hip-hop", "trap", "r&b", "rnb", "soul",
		"rock", "electronica", "ambient", "downtempo", "lounge", "chillout",
		"dubstep", "drum and bass", "drum & bass", "dnb", "breakbeat",
		"hardstyle", "disco", "nu disco", "funk", "afrobeats", "baile funk",
		"indie dance", "open format", "moombahton", "latin",
	}

	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		name := strings.TrimSpace(tag.Name)
		if name == "" {
			continue
		}
		normalized := normalizeText(name)
		if normalized == "" {
			continue
		}
		matched := false
		for _, genre := range known {
			genreNorm := normalizeText(genre)
			if normalized == genreNorm {
				matched = true
				break
			}
		}
		if !matched || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, name)
	}
	return strings.Join(out, ", ")
}

func mixcloudTagNames(tags []mixcloudTag) []string {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		if name := strings.TrimSpace(tag.Name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

func mixcloudQueryFit(query model.MetadataQuery, candidate model.MetadataCandidate) float64 {
	titleQuery := strings.TrimSpace(query.Title)
	artistQuery := strings.TrimSpace(query.Artist)

	titleFit := 1.0
	if titleQuery != "" {
		titleFit = textSimilarity(titleQuery, candidate.Title)
		if queryBase := strings.TrimSpace(stripVersionText(titleQuery)); queryBase != "" {
			candidateBase := strings.TrimSpace(stripVersionText(candidate.Title))
			if candidateBase == "" {
				candidateBase = candidate.Title
			}
			if baseFit := textSimilarity(queryBase, candidateBase); baseFit > titleFit {
				titleFit = baseFit
			}
		}
		if titleFit < 0.58 {
			return -1
		}
	}

	artistFit := 1.0
	if artistQuery != "" {
		artistFit = artistSimilarity(artistQuery, candidate.Artist)
		if artistFit < 0.44 {
			return -1
		}
	}
	return titleFit*0.72 + artistFit*0.28
}

func mixcloudExternalID(key, artist, title string) string {
	if value := strings.TrimSpace(key); value != "" {
		return "cloudcast:" + value
	}
	sum := sha1.Sum([]byte(normalizeText(artist) + "\x00" + normalizeText(title)))
	return fmt.Sprintf("%x", sum[:8])
}

func mixcloudDebugEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("CCML_MIXCLOUD_DEBUG")))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func mixcloudDebugf(format string, args ...any) {
	if !mixcloudDebugEnabled() {
		return
	}
	log.Printf("[MIXCLOUD DEBUG] "+format, args...)
}

func mixcloudDebugCandidates(label string, query model.MetadataQuery, items []model.MetadataCandidate) {
	if !mixcloudDebugEnabled() {
		return
	}
	mixcloudDebugf("%s candidates=%d", label, len(items))
	limit := len(items)
	if limit > 12 {
		limit = 12
	}
	for i := 0; i < limit; i++ {
		item := items[i]
		mixcloudDebugf(
			"%s candidate[%d] artist=%q title=%q genre=%q fit=%.3f sourceURL=%s",
			label,
			i,
			item.Artist,
			item.Title,
			item.Genre,
			mixcloudQueryFit(query, item),
			item.SourceURL,
		)
	}
}
