package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

func muzvizorAPIURL(baseURL, term string) string {
	return strings.TrimRight(baseURL, "/") + "/api/v1/tracks/?query=" + url.PathEscape(strings.TrimSpace(term))
}

func (p *MuzvizorProvider) fetchAPI(ctx context.Context, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("create MUZVIZOR API request: %w", err)
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Referer", p.baseURL+"/tracks")
	if p.userAgent != "" {
		req.Header.Set("User-Agent", p.userAgent)
	} else {
		req.Header.Set("User-Agent", "CCML metadata client")
	}

	// Deliberately do not copy browser Cookie/Authorization state. The metadata
	// provider only uses the public GET endpoint observed in the site's own
	// anonymous track-search flow.
	muzvizorDebugf("API GET %s", target)
	body, err := fetchProviderBytes(ctx, p.client, req, p.Name(), 1)
	if err != nil {
		muzvizorDebugf("API FAIL url=%s error=%v", target, err)
		return nil, err
	}
	muzvizorDebugf("API OK url=%s bytes=%d", target, len(body))
	muzvizorDebugAPIShape(body)
	return body, nil
}

func muzvizorCandidatesFromAPIJSON(body []byte, sourceURL string, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("decode MUZVIZOR API JSON: %w", err)
	}

	objects := make([]map[string]any, 0, 16)
	muzvizorAPICollectObjects(root, &objects)

	items := make([]model.MetadataCandidate, 0, len(objects))
	for _, object := range objects {
		item, ok := muzvizorAPICandidateFromObject(object, sourceURL)
		if !ok {
			continue
		}
		if muzvizorQueryFit(query, item) < 0 {
			continue
		}
		items = append(items, item)
	}
	return muzvizorLimitCandidates(query, items), nil
}

func muzvizorAPICandidateFromObject(object map[string]any, sourceURL string) (model.MetadataCandidate, bool) {
	// A JSON envelope such as {"count": 1, "results": [...]} is not a track.
	// Require the core track fields to belong to this object itself. Nested
	// artist/key/genre values are still supported because their direct field
	// value can be an object or array.
	title := muzvizorAPIStringField(object, "title", "track_title", "trackTitle")
	if title == "" {
		title = muzvizorAPIStringField(object, "name")
	}

	artistValue, ok := muzvizorAPIField(object,
		"artist", "artists", "artist_name", "artistName", "artist_title", "artistTitle",
		"performer", "performers", "authors",
	)
	if !ok {
		return model.MetadataCandidate{}, false
	}
	artist := muzvizorAPITextList(artistValue)
	if strings.TrimSpace(title) == "" || strings.TrimSpace(artist) == "" {
		return model.MetadataCandidate{}, false
	}

	bpmValue, ok := muzvizorAPIField(object, "bpm", "tempo", "bpm_value", "bpmValue")
	if !ok {
		return model.MetadataCandidate{}, false
	}
	bpm, ok := muzvizorAPIBPM(bpmValue)
	if !ok {
		return model.MetadataCandidate{}, false
	}

	keyValue, ok := muzvizorAPIField(object,
		"key", "camelot", "camelot_key", "camelotKey", "key_name", "keyName",
		"tonality", "tonality_name", "tonalityName",
	)
	if !ok {
		return model.MetadataCandidate{}, false
	}
	key, ok := muzvizorAPICamelot(keyValue)
	if !ok {
		return model.MetadataCandidate{}, false
	}

	genre := ""
	if value, ok := muzvizorAPIField(object, "genre", "genres", "genre_name", "genreName"); ok {
		genre = muzvizorAPITextList(value)
	}

	stage := ""
	if value, ok := muzvizorAPIField(object,
		"stage", "stages", "stage_name", "stageName", "party_stage", "partyStage",
	); ok {
		stage = muzvizorAPIStage(value)
	}

	externalID := ""
	if value, ok := muzvizorAPIField(object, "id", "uuid", "pk", "slug"); ok {
		externalID = muzvizorAPIID(value)
	}
	if externalID == "" {
		externalID = muzvizorExternalID(artist, title, bpm, key)
	} else {
		externalID = "api:" + externalID
	}

	return model.MetadataCandidate{
		Source:     "MUZVIZOR",
		SourceKind: ProviderKindDJPool,
		ExternalID: externalID,
		SourceURL:  sourceURL,
		Title:      strings.TrimSpace(title),
		Artist:     strings.TrimSpace(artist),
		Genre:      strings.TrimSpace(genre),
		Stage:      strings.TrimSpace(stage),
		BPM:        bpm,
		Key:        key,
		KeyScale:   "camelot",
	}, true
}

func muzvizorAPICollectObjects(value any, out *[]map[string]any) {
	switch typed := value.(type) {
	case map[string]any:
		*out = append(*out, typed)
		for _, child := range typed {
			muzvizorAPICollectObjects(child, out)
		}
	case []any:
		for _, child := range typed {
			muzvizorAPICollectObjects(child, out)
		}
	}
}

func muzvizorAPIField(object map[string]any, keys ...string) (any, bool) {
	targets := make(map[string]bool, len(keys))
	for _, key := range keys {
		targets[muzvizorAPINormalizeKey(key)] = true
	}
	for key, value := range object {
		if targets[muzvizorAPINormalizeKey(key)] {
			return value, true
		}
	}
	return nil, false
}

func muzvizorAPIFieldDeep(object map[string]any, keys ...string) (any, bool) {
	return muzvizorAPIFieldDeepLimit(object, 0, 3, keys...)
}

func muzvizorAPIFieldDeepLimit(object map[string]any, depth, maxDepth int, keys ...string) (any, bool) {
	if value, ok := muzvizorAPIField(object, keys...); ok {
		return value, true
	}
	if depth >= maxDepth {
		return nil, false
	}
	for _, child := range object {
		switch typed := child.(type) {
		case map[string]any:
			if value, ok := muzvizorAPIFieldDeepLimit(typed, depth+1, maxDepth, keys...); ok {
				return value, true
			}
		case []any:
			for _, item := range typed {
				if nested, ok := item.(map[string]any); ok {
					if value, ok := muzvizorAPIFieldDeepLimit(nested, depth+1, maxDepth, keys...); ok {
						return value, true
					}
				}
			}
		}
	}
	return nil, false
}

func muzvizorAPIStringField(object map[string]any, keys ...string) string {
	value, ok := muzvizorAPIField(object, keys...)
	if !ok {
		return ""
	}
	return muzvizorAPIText(value)
}

func muzvizorAPIStringFieldDeep(object map[string]any, keys ...string) string {
	value, ok := muzvizorAPIFieldDeep(object, keys...)
	if !ok {
		return ""
	}
	return muzvizorAPIText(value)
}

func muzvizorAPINormalizeKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("_", "", "-", "", " ", "")
	return replacer.Replace(value)
}

func muzvizorAPIText(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case map[string]any:
		for _, key := range []string{"name", "title", "label", "value", "text", "code", "slug"} {
			if child, ok := muzvizorAPIField(typed, key); ok {
				if text := muzvizorAPIText(child); text != "" {
					return text
				}
			}
		}
	case []any:
		parts := muzvizorAPITextParts(typed)
		return strings.Join(parts, ", ")
	}
	return ""
}

func muzvizorAPITextList(value any) string {
	switch typed := value.(type) {
	case []any:
		return strings.Join(muzvizorAPITextParts(typed), ", ")
	default:
		return muzvizorAPIText(value)
	}
}

func muzvizorAPITextParts(values []any) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		text := strings.Trim(strings.TrimSpace(muzvizorAPIText(value)), ",")
		if text == "" {
			continue
		}
		key := strings.ToLower(text)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, text)
	}
	return out
}

func muzvizorAPIBPM(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		if typed >= 20 && typed <= 300 {
			return typed, true
		}
	case float32:
		number := float64(typed)
		if number >= 20 && number <= 300 {
			return number, true
		}
	case int:
		number := float64(typed)
		if number >= 20 && number <= 300 {
			return number, true
		}
	case int64:
		number := float64(typed)
		if number >= 20 && number <= 300 {
			return number, true
		}
	case string:
		return parseMuzvizorBPM(typed)
	case map[string]any:
		for _, key := range []string{"value", "bpm", "tempo", "name"} {
			if child, ok := muzvizorAPIField(typed, key); ok {
				if bpm, ok := muzvizorAPIBPM(child); ok {
					return bpm, true
				}
			}
		}
	}
	return 0, false
}

func muzvizorAPICamelot(value any) (string, bool) {
	if text := muzvizorAPIText(value); text != "" {
		if key, ok := parseMuzvizorCamelot(text); ok {
			return key, true
		}
	}
	switch typed := value.(type) {
	case map[string]any:
		for _, child := range typed {
			if key, ok := muzvizorAPICamelot(child); ok {
				return key, true
			}
		}
	case []any:
		for _, child := range typed {
			if key, ok := muzvizorAPICamelot(child); ok {
				return key, true
			}
		}
	}
	return "", false
}

func muzvizorAPIStage(value any) string {
	text := strings.ToLower(strings.TrimSpace(muzvizorAPIText(value)))
	text = strings.NewReplacer("_", " ", "-", " ").Replace(text)
	text = strings.Join(strings.Fields(text), " ")
	switch {
	case text == "":
		return ""
	case strings.Contains(text, "prime"):
		return "Prime Time"
	case strings.Contains(text, "warm"):
		return "Warm Up"
	case strings.Contains(text, "mainstage"), strings.Contains(text, "main stage"):
		return "Mainstage"
	case strings.Contains(text, "event"):
		return "Event"
	default:
		return strings.TrimSpace(muzvizorAPIText(value))
	}
}

func muzvizorAPIID(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return muzvizorAPIText(value)
	}
}

func muzvizorAPIObjectKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func muzvizorDebugAPIShape(body []byte) {
	if !muzvizorDebugEnabled() {
		return
	}
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		muzvizorDebugf("API JSON shape decode failed: %v", err)
		return
	}
	switch typed := root.(type) {
	case map[string]any:
		keys := muzvizorAPIObjectKeys(typed)
		muzvizorDebugf("API JSON top-level object keys=%q", keys)
		if results, ok := muzvizorAPIField(typed, "results", "items", "data"); ok {
			switch list := results.(type) {
			case []any:
				if len(list) > 0 {
					if first, ok := list[0].(map[string]any); ok {
						muzvizorDebugf("API JSON first result keys=%q", muzvizorAPIObjectKeys(first))
					}
				}
			case map[string]any:
				muzvizorDebugf("API JSON result object keys=%q", muzvizorAPIObjectKeys(list))
			}
		}
	case []any:
		muzvizorDebugf("API JSON top-level array items=%d", len(typed))
	default:
		muzvizorDebugf("API JSON top-level type=%T", root)
	}
}
