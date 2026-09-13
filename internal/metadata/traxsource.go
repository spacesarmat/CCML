package metadata

import (
	"context"
	"errors"
	"fmt"
	htmlstd "html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spacesarmat/CCML/internal/model"
)

const traxsourceBaseURL = "https://www.traxsource.com"

var (
	traxsourceTitleID = regexp.MustCompile(`/title/(\d+)(?:/|$)`)
	leadingInteger    = regexp.MustCompile(`\d+`)
	htmlAttribute     = regexp.MustCompile(`(?is)([a-zA-Z0-9:_-]+)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	htmlTagStrip      = regexp.MustCompile(`(?is)<[^>]+>`)
	htmlSpace         = regexp.MustCompile(`\s+`)
	numericSegment    = regexp.MustCompile(`\d+`)
)

// TraxsourceProvider reads the public Traxsource web catalog. Traxsource does
// not expose a public developer API/token program for this lookup, so this
// provider is opt-in and marked experimental in Settings.
type TraxsourceProvider struct {
	client    *http.Client
	baseURL   string
	userAgent string
}

func NewTraxsourceProvider(userAgent string) *TraxsourceProvider {
	return &TraxsourceProvider{
		client: &http.Client{Timeout: 15 * time.Second}, baseURL: traxsourceBaseURL,
		userAgent: strings.TrimSpace(userAgent),
	}
}

func (p *TraxsourceProvider) Name() string { return "Traxsource" }

func (p *TraxsourceProvider) Search(ctx context.Context, query model.MetadataQuery) ([]model.MetadataCandidate, error) {
	term := strings.TrimSpace(strings.TrimSpace(query.Artist) + " " + strings.TrimSpace(query.Title))
	if term == "" {
		return nil, fmt.Errorf("artist or title is required")
	}

	values := url.Values{}
	values.Set("term", `"`+term+`"`)
	searchURL := strings.TrimRight(p.baseURL, "/") + "/search/titles?" + values.Encode()
	doc, err := p.fetchHTML(ctx, searchURL)
	if err != nil {
		return nil, err
	}
	releases := traxsourceSearchReleases(doc, p.baseURL)
	if len(releases) == 0 {
		return nil, nil
	}
	if len(releases) > 3 {
		releases = releases[:3]
	}

	type result struct {
		items []model.MetadataCandidate
		err   error
	}
	responses := make(chan result, len(releases))
	var wg sync.WaitGroup
	for _, releaseURL := range releases {
		releaseURL := releaseURL
		wg.Add(1)
		go func() {
			defer wg.Done()
			doc, err := p.fetchHTML(ctx, releaseURL)
			if err != nil {
				responses <- result{err: err}
				return
			}
			responses <- result{items: traxsourceReleaseCandidates(doc, releaseURL)}
		}()
	}
	go func() {
		wg.Wait()
		close(responses)
	}()

	var items []model.MetadataCandidate
	var errs []error
	for response := range responses {
		if response.err != nil {
			errs = append(errs, response.err)
			continue
		}
		items = append(items, response.items...)
	}
	if len(items) == 0 && len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return items, nil
}

func (p *TraxsourceProvider) fetchHTML(ctx context.Context, target string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return "", fmt.Errorf("create Traxsource request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.8")
	if p.userAgent != "" {
		req.Header.Set("User-Agent", p.userAgent)
	} else {
		req.Header.Set("User-Agent", "CCML metadata client")
	}
	body, err := fetchProviderBytes(ctx, p.client, req, p.Name(), 3)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func fetchProviderBytes(ctx context.Context, client *http.Client, req *http.Request, provider string, attempts int) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		resp, err := client.Do(req.Clone(ctx))
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = fmt.Errorf("request %s: %w", provider, err)
			if attempt == attempts || !isRetryableNetworkError(err) {
				return nil, lastErr
			}
			if err := waitMetadataRetry(ctx, time.Duration(attempt)*500*time.Millisecond); err != nil {
				return nil, err
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		closeErr := resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read %s response: %w", provider, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close %s response: %w", provider, closeErr)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return body, nil
		}
		errorBody := strings.TrimSpace(string(body))
		if len(errorBody) > 4096 {
			errorBody = errorBody[:4096]
		}
		httpErr := &providerHTTPError{Provider: provider, Status: resp.StatusCode, Body: errorBody, Retryable: isRetryableHTTPStatus(resp.StatusCode)}
		lastErr = httpErr
		if !httpErr.Retryable || attempt == attempts {
			return nil, httpErr
		}
		delay := retryAfterDelay(resp.Header.Get("Retry-After"), time.Now())
		if delay <= 0 {
			delay = time.Duration(1<<(attempt-1)) * time.Second
		}
		if delay > maxRetryDelay {
			delay = maxRetryDelay
		}
		if err := waitMetadataRetry(ctx, delay); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

type htmlBlock struct {
	open  string
	inner string
	full  string
}

func traxsourceSearchReleases(doc, baseURL string) []string {
	seen := map[string]bool{}
	var out []string
	for _, anchor := range htmlBlocks(doc, "a") {
		if !hasClass(anchor.open, "com-title") {
			continue
		}
		href := attrValue(anchor.open, "href")
		if href == "" || !strings.Contains(href, "/title/") {
			continue
		}
		resolved := resolveHTMLURL(baseURL, href)
		if resolved != "" && !seen[resolved] {
			seen[resolved] = true
			out = append(out, resolved)
		}
	}
	return out
}

func traxsourceReleaseCandidates(doc, releaseURL string) []model.MetadataCandidate {
	album := textOfFirstBlock(doc, "h1", "title")
	albumArtists := textsOfAnchors(firstBlock(doc, "h1", "artists"), "com-artists")
	if len(albumArtists) == 0 {
		albumArtists = nonEmptySlice(textOfFirstBlock(doc, "h1", "artists"))
	}
	label := textOfFirstBlock(doc, "a", "com-label")
	catalogNumber, releaseDate := parseTraxsourceCatalogDate(textOfFirstBlock(doc, "div", "cat-rdate"))
	artwork := ""
	for _, tag := range openingTags(doc, "meta") {
		if strings.EqualFold(attrValue(tag, "property"), "og:image") {
			artwork = attrValue(tag, "content")
			break
		}
	}

	releaseID := ""
	if match := traxsourceTitleID.FindStringSubmatch(releaseURL); len(match) == 2 {
		releaseID = match[1]
	}
	rows := blocksByClass(doc, "div", "trk-row")
	items := make([]model.MetadataCandidate, 0, len(rows))
	for index, row := range rows {
		titleScope := firstBlock(row.inner, "div", "title")
		titleLink := firstBlockValue(titleScope, "a", "")
		title := blockText(titleLink.inner)
		if title == "" {
			continue
		}
		if version := textOfFirstBlock(titleScope.inner, "span", "version"); version != "" && !strings.Contains(strings.ToLower(title), strings.ToLower(version)) {
			title += " (" + version + ")"
		}
		trackArtists := textsOfAnchors(firstBlock(row.inner, "div", "artists"), "com-artists")
		if len(trackArtists) == 0 {
			trackArtists = albumArtists
		}
		genre := strings.Join(textsOfAnchors(firstBlock(row.inner, "div", "genre"), ""), "; ")
		trackNumber := index + 1
		if value := textOfFirstBlock(row.inner, "div", "tnum"); value != "" {
			if match := leadingInteger.FindString(value); match != "" {
				trackNumber, _ = strconv.Atoi(match)
			}
		}
		durationMS := parseClockDurationMS(textOfFirstBlock(row.inner, "span", "duration"))
		externalID := releaseID + ":" + strconv.Itoa(trackNumber)
		if href := attrValue(titleLink.open, "href"); href != "" {
			if id := lastNumericPathSegment(href); id != "" {
				externalID = id
			}
		}
		items = append(items, model.MetadataCandidate{
			Source: "Traxsource", ExternalID: externalID, SourceURL: releaseURL,
			Title: title, Artist: strings.Join(trackArtists, ", "), Album: album, AlbumArtist: strings.Join(albumArtists, ", "),
			ReleaseDate: releaseDate, Year: yearFromDate(releaseDate), Genre: genre, Label: label, CatalogNumber: catalogNumber,
			TrackNumber: trackNumber, TrackTotal: len(rows), ArtworkURL: artwork, ArtworkEmbeddable: false, DurationMS: durationMS,
		})
	}
	return items
}

func parseTraxsourceCatalogDate(value string) (catalog, releaseDate string) {
	parts := strings.SplitN(strings.TrimSpace(value), "|", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return "", strings.TrimSpace(value)
}

func parseClockDurationMS(value string) int64 {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0
	}
	var total int64
	for _, part := range parts {
		n, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			return 0
		}
		total = total*60 + n
	}
	return total * 1000
}

func lastNumericPathSegment(value string) string {
	all := numericSegment.FindAllString(value, -1)
	if len(all) == 0 {
		return ""
	}
	return all[len(all)-1]
}

func htmlBlocks(input, tag string) []htmlBlock {
	pattern := regexp.MustCompile(`(?is)</?` + regexp.QuoteMeta(tag) + `\b[^>]*>`)
	locs := pattern.FindAllStringIndex(input, -1)
	blocks := make([]htmlBlock, 0)
	for i, loc := range locs {
		open := input[loc[0]:loc[1]]
		if strings.HasPrefix(strings.TrimSpace(strings.ToLower(open)), "</") || strings.HasSuffix(strings.TrimSpace(open), "/>") {
			continue
		}
		depth := 1
		for j := i + 1; j < len(locs); j++ {
			token := input[locs[j][0]:locs[j][1]]
			trimmed := strings.TrimSpace(strings.ToLower(token))
			if strings.HasPrefix(trimmed, "</") {
				depth--
			} else if !strings.HasSuffix(strings.TrimSpace(token), "/>") {
				depth++
			}
			if depth == 0 {
				blocks = append(blocks, htmlBlock{open: open, inner: input[loc[1]:locs[j][0]], full: input[loc[0]:locs[j][1]]})
				break
			}
		}
	}
	return blocks
}

func openingTags(input, tag string) []string {
	pattern := regexp.MustCompile(`(?is)<` + regexp.QuoteMeta(tag) + `\b[^>]*>`)
	return pattern.FindAllString(input, -1)
}

func blocksByClass(input, tag, class string) []htmlBlock {
	var out []htmlBlock
	for _, block := range htmlBlocks(input, tag) {
		if class == "" || hasClass(block.open, class) {
			out = append(out, block)
		}
	}
	return out
}

func firstBlock(input, tag, class string) htmlBlock {
	blocks := blocksByClass(input, tag, class)
	if len(blocks) == 0 {
		return htmlBlock{}
	}
	return blocks[0]
}

func firstBlockValue(block htmlBlock, tag, class string) htmlBlock {
	if block.inner == "" {
		return htmlBlock{}
	}
	return firstBlock(block.inner, tag, class)
}

func textOfFirstBlock(input, tag, class string) string {
	return blockText(firstBlock(input, tag, class).inner)
}

func textsOfAnchors(block htmlBlock, class string) []string {
	if block.inner == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, anchor := range blocksByClass(block.inner, "a", class) {
		value := blockText(anchor.inner)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func attrValue(openTag, key string) string {
	for _, match := range htmlAttribute.FindAllStringSubmatch(openTag, -1) {
		if !strings.EqualFold(match[1], key) {
			continue
		}
		if match[2] != "" {
			return htmlstd.UnescapeString(strings.TrimSpace(match[2]))
		}
		return htmlstd.UnescapeString(strings.TrimSpace(match[3]))
	}
	return ""
}

func hasClass(openTag, class string) bool {
	for _, token := range strings.Fields(attrValue(openTag, "class")) {
		if token == class {
			return true
		}
	}
	return false
}

func blockText(inner string) string {
	if inner == "" {
		return ""
	}
	plain := htmlTagStrip.ReplaceAllString(inner, " ")
	plain = htmlstd.UnescapeString(plain)
	return strings.TrimSpace(htmlSpace.ReplaceAllString(plain, " "))
}

func resolveHTMLURL(baseURL, href string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	rel, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return ""
	}
	return base.ResolveReference(rel).String()
}

func nonEmptySlice(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return []string{strings.TrimSpace(value)}
}
