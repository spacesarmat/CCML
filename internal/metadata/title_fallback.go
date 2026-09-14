package metadata

import (
	"strings"

	"github.com/spacesarmat/CCML/internal/model"
)

const titleFallbackIssue = "title_search_fallback"

// titleFallbackQuery returns a safer provider query with only trailing version
// qualifiers removed from Title.
//
// Examples:
//
//	Track (Extended Mix)       -> Track
//	Track (Dirty)              -> Track
//	Track (John Doe Remix)     -> Track
//	Track - Radio Edit         -> Track
//	Track [Intro Clean]        -> Track
//
// Only trailing bracket groups or an explicit trailing separator segment are
// removed. Ordinary words in the middle of a title are left untouched.
func titleFallbackQuery(query model.MetadataQuery) (model.MetadataQuery, bool) {
	base, _, ok := splitTitleVersionSuffix(query.Title)
	if !ok || strings.TrimSpace(base) == "" || normalizeText(base) == normalizeText(query.Title) {
		return model.MetadataQuery{}, false
	}
	fallback := query
	fallback.Title = base
	return fallback, true
}

// PreserveLocalVersionTitle keeps the local track's trailing version qualifier
// when provider metadata is applied.
//
// The provider still supplies the base title and all other fields, but local
// identity such as "(Extended Mix)" or "(Dirty)" is not silently erased.
func PreserveLocalVersionTitle(localTitle, providerTitle string) string {
	localBase, localSuffix, ok := splitTitleVersionSuffix(localTitle)
	if !ok || strings.TrimSpace(localSuffix) == "" || strings.TrimSpace(providerTitle) == "" {
		return providerTitle
	}

	providerBase := strings.TrimSpace(providerTitle)
	if base, _, providerHasSuffix := splitTitleVersionSuffix(providerTitle); providerHasSuffix {
		providerBase = base
	}
	if providerBase == "" {
		return providerTitle
	}

	// Never graft a local version label onto an unrelated provider result.
	if textSimilarity(localBase, providerBase) < 0.60 {
		return providerTitle
	}
	return strings.TrimSpace(providerBase) + localSuffix
}

func metadataLookupNeedsTitleFallback(result model.MetadataLookupResult) bool {
	if len(result.Candidates) == 0 {
		return true
	}
	for _, candidate := range result.Candidates {
		if candidate.MatchClass != "rejected" && candidate.Confidence >= 0.55 {
			return false
		}
	}
	return true
}

func markTitleFallbackResult(originalTitle string, result model.MetadataLookupResult) model.MetadataLookupResult {
	for i := range result.Candidates {
		result.Candidates[i].Title = PreserveLocalVersionTitle(originalTitle, result.Candidates[i].Title)
		if !containsString(result.Candidates[i].MatchIssues, titleFallbackIssue) {
			result.Candidates[i].MatchIssues = append(result.Candidates[i].MatchIssues, titleFallbackIssue)
		}
	}
	result.Suggested = buildSuggested(result.Candidates)
	result.FieldOptions = buildFieldOptions(result.Candidates)
	return result
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func splitTitleVersionSuffix(value string) (base, suffix string, ok bool) {
	original := strings.TrimSpace(value)
	if original == "" {
		return "", "", false
	}

	working := original
	changed := false

	// Strip one or more recognized trailing (...) or [...] groups.
	for {
		trimmed := strings.TrimSpace(working)
		start, content, found := trailingBracketGroup(trimmed)
		if !found || !containsVersionQualifier(content) {
			break
		}
		working = strings.TrimSpace(trimmed[:start])
		changed = true
		if working == "" {
			return original, "", false
		}
	}

	// Also support explicit suffixes such as "Track - Extended Mix".
	for {
		trimmed := strings.TrimSpace(working)
		removed := false
		for _, separator := range []string{" - ", " – ", " — "} {
			index := strings.LastIndex(trimmed, separator)
			if index <= 0 {
				continue
			}
			content := strings.TrimSpace(trimmed[index+len(separator):])
			if !containsVersionQualifier(content) {
				continue
			}
			working = strings.TrimSpace(trimmed[:index])
			changed = true
			removed = true
			break
		}
		if !removed {
			break
		}
		if working == "" {
			return original, "", false
		}
	}

	if !changed {
		return original, "", false
	}

	base = strings.TrimSpace(working)
	if base == "" || !strings.HasPrefix(original, base) {
		return original, "", false
	}

	rawSuffix := strings.TrimSpace(original[len(base):])
	if rawSuffix == "" {
		return original, "", false
	}
	return base, " " + rawSuffix, true
}

func trailingBracketGroup(value string) (start int, content string, ok bool) {
	if len(value) < 3 {
		return 0, "", false
	}

	switch value[len(value)-1] {
	case ')':
		start = strings.LastIndex(value, "(")
		if start <= 0 {
			return 0, "", false
		}
		return start, strings.TrimSpace(value[start+1 : len(value)-1]), true
	case ']':
		start = strings.LastIndex(value, "[")
		if start <= 0 {
			return 0, "", false
		}
		return start, strings.TrimSpace(value[start+1 : len(value)-1]), true
	default:
		return 0, "", false
	}
}

func containsVersionQualifier(value string) bool {
	return len(versionSignatureFor(value)) > 0
}
