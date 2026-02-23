package schedule

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var (
	urlRegex = regexp.MustCompile(`https?://[^\s<>"]+`)
)

type urlCandidate struct {
	Value      string
	SourceRank int
	JoinRank   int
	Provider   string
}

func DeriveLinks(event RawEvent) (joinURL, eventURL, provider string) {
	candidates := make([]urlCandidate, 0, 8)

	if value := strings.TrimSpace(event.GoogleConferenceURL); value != "" {
		providerName, rank := providerRank(value)
		candidates = append(candidates, urlCandidate{Value: value, SourceRank: 0, JoinRank: rank, Provider: providerName})
	}

	if value := strings.TrimSpace(event.URL); value != "" {
		providerName, rank := providerRank(value)
		candidates = append(candidates, urlCandidate{Value: value, SourceRank: 1, JoinRank: rank, Provider: providerName})
	}

	for _, source := range []struct {
		rank int
		text string
	}{
		{rank: 2, text: event.Location},
		{rank: 3, text: event.Description},
	} {
		for _, found := range extractURLs(source.text) {
			providerName, rank := providerRank(found)
			candidates = append(candidates, urlCandidate{Value: found, SourceRank: source.rank, JoinRank: rank, Provider: providerName})
		}
	}

	if len(candidates) == 0 {
		return "", "", ""
	}

	unique := dedupeCandidates(candidates)
	if len(unique) == 0 {
		return "", "", ""
	}

	sort.SliceStable(unique, func(i, j int) bool {
		if unique[i].JoinRank != unique[j].JoinRank {
			return unique[i].JoinRank < unique[j].JoinRank
		}
		if unique[i].SourceRank != unique[j].SourceRank {
			return unique[i].SourceRank < unique[j].SourceRank
		}
		return unique[i].Value < unique[j].Value
	})

	first := unique[0]
	eventURL = first.Value

	for _, candidate := range unique {
		if candidate.JoinRank <= 10 {
			joinURL = candidate.Value
			provider = candidate.Provider
			break
		}
	}

	return joinURL, eventURL, provider
}

func extractURLs(text string) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}

	found := urlRegex.FindAllString(trimmed, -1)
	if len(found) == 0 {
		return nil
	}

	results := make([]string, 0, len(found))
	for _, item := range found {
		normalized := normalizeURL(item)
		if normalized == "" {
			continue
		}
		results = append(results, normalized)
	}
	return results
}

func dedupeCandidates(candidates []urlCandidate) []urlCandidate {
	seen := make(map[string]urlCandidate)
	for _, candidate := range candidates {
		normalized := normalizeURL(candidate.Value)
		if normalized == "" {
			continue
		}
		candidate.Value = normalized

		if existing, ok := seen[normalized]; ok {
			if candidate.JoinRank < existing.JoinRank ||
				(candidate.JoinRank == existing.JoinRank && candidate.SourceRank < existing.SourceRank) {
				seen[normalized] = candidate
			}
			continue
		}
		seen[normalized] = candidate
	}

	results := make([]urlCandidate, 0, len(seen))
	for _, candidate := range seen {
		results = append(results, candidate)
	}
	return results
}

func normalizeURL(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.TrimRight(value, ".,;)")
	if value == "" {
		return ""
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return ""
	}
	return parsed.String()
}

func providerRank(value string) (string, int) {
	host := hostOf(value)
	switch {
	case strings.HasSuffix(host, "meet.google.com"):
		return "google_meet", 0
	case strings.HasSuffix(host, "zoom.us"), strings.HasSuffix(host, "zoomgov.com"):
		if strings.Contains(value, "/j/") || strings.Contains(value, "/wc/") {
			return "zoom", 1
		}
		return "zoom", 5
	case strings.HasSuffix(host, "tel.meet"):
		return "google_meet", 4
	case strings.Contains(host, "google.com") && strings.Contains(value, "meet"):
		return "google_meet", 6
	default:
		return "", 50
	}
}

func hostOf(value string) string {
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}
