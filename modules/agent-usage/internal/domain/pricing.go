package domain

import (
	"regexp"
	"strings"
)

type codexPricing struct {
	inputPerToken     float64
	outputPerToken    float64
	cacheReadPerToken float64
}

type claudePricing struct {
	inputPerToken           float64
	outputPerToken          float64
	cacheCreatePerToken     float64
	cacheReadPerToken       float64
	thresholdTokens         *int
	inputPerTokenAbove      *float64
	outputPerTokenAbove     *float64
	cacheCreatePerTokenOver *float64
	cacheReadPerTokenOver   *float64
}

var codexPricingTable = map[string]codexPricing{
	"gpt-5":         {1.25e-6, 1e-5, 1.25e-7},
	"gpt-5-codex":   {1.25e-6, 1e-5, 1.25e-7},
	"gpt-5.1":       {1.25e-6, 1e-5, 1.25e-7},
	"gpt-5.2":       {1.75e-6, 1.4e-5, 1.75e-7},
	"gpt-5.2-codex": {1.75e-6, 1.4e-5, 1.75e-7},
}

var (
	threshold200k = 200_000
	in6e6         = 6e-6
	out225e5      = 2.25e-5
	cache75e6     = 7.5e-6
	cache6e7      = 6e-7
)

var claudePricingTable = map[string]claudePricing{
	"claude-haiku-4-5-20251001": {1e-6, 5e-6, 1.25e-6, 1e-7, nil, nil, nil, nil, nil},
	"claude-haiku-4-5":          {1e-6, 5e-6, 1.25e-6, 1e-7, nil, nil, nil, nil, nil},
	"claude-opus-4-5-20251101":  {5e-6, 2.5e-5, 6.25e-6, 5e-7, nil, nil, nil, nil, nil},
	"claude-opus-4-5":           {5e-6, 2.5e-5, 6.25e-6, 5e-7, nil, nil, nil, nil, nil},
	"claude-opus-4-6-20260205":  {5e-6, 2.5e-5, 6.25e-6, 5e-7, nil, nil, nil, nil, nil},
	"claude-opus-4-6":           {5e-6, 2.5e-5, 6.25e-6, 5e-7, nil, nil, nil, nil, nil},
	"claude-sonnet-4-5": {
		3e-6, 1.5e-5, 3.75e-6, 3e-7,
		&threshold200k, &in6e6, &out225e5, &cache75e6, &cache6e7,
	},
	"claude-sonnet-4-5-20250929": {
		3e-6, 1.5e-5, 3.75e-6, 3e-7,
		&threshold200k, &in6e6, &out225e5, &cache75e6, &cache6e7,
	},
	"claude-opus-4-20250514": {1.5e-5, 7.5e-5, 1.875e-5, 1.5e-6, nil, nil, nil, nil, nil},
	"claude-opus-4-1":        {1.5e-5, 7.5e-5, 1.875e-5, 1.5e-6, nil, nil, nil, nil, nil},
	"claude-sonnet-4-20250514": {
		3e-6, 1.5e-5, 3.75e-6, 3e-7,
		&threshold200k, &in6e6, &out225e5, &cache75e6, &cache6e7,
	},
}

var (
	claudeVersionSuffixPattern = regexp.MustCompile(`-v\d+:\d+$`)
	claudeDateSuffixPattern    = regexp.MustCompile(`-\d{8}$`)
)

func NormalizeCodexModel(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "openai/")
	if idx := strings.Index(trimmed, "-codex"); idx >= 0 {
		base := trimmed[:idx]
		if _, ok := codexPricingTable[base]; ok {
			return base
		}
	}
	return trimmed
}

func NormalizeClaudeModel(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "anthropic.")
	if strings.Contains(trimmed, "claude-") {
		if lastDot := strings.LastIndex(trimmed, "."); lastDot >= 0 {
			tail := trimmed[lastDot+1:]
			if strings.HasPrefix(tail, "claude-") {
				trimmed = tail
			}
		}
	}

	trimmed = claudeVersionSuffixPattern.ReplaceAllString(trimmed, "")
	base := claudeDateSuffixPattern.ReplaceAllString(trimmed, "")
	if _, ok := claudePricingTable[base]; ok {
		return base
	}
	return trimmed
}

func CodexCostUSD(model string, inputTokens, cachedInputTokens, outputTokens int64) (float64, bool) {
	key := NormalizeCodexModel(model)
	pricing, ok := codexPricingTable[key]
	if !ok {
		return 0, false
	}

	cached := maxInt64(0, cachedInputTokens)
	input := maxInt64(0, inputTokens)
	if cached > input {
		cached = input
	}
	nonCached := input - cached
	output := maxInt64(0, outputTokens)

	cost := float64(nonCached)*pricing.inputPerToken +
		float64(cached)*pricing.cacheReadPerToken +
		float64(output)*pricing.outputPerToken
	return cost, true
}

func ClaudeCostUSD(model string, inputTokens, cacheReadInputTokens, cacheCreationInputTokens, outputTokens int64) (float64, bool) {
	key := NormalizeClaudeModel(model)
	pricing, ok := claudePricingTable[key]
	if !ok {
		return 0, false
	}

	cost := tieredCost(maxInt64(0, inputTokens), pricing.inputPerToken, pricing.thresholdTokens, pricing.inputPerTokenAbove) +
		tieredCost(maxInt64(0, cacheReadInputTokens), pricing.cacheReadPerToken, pricing.thresholdTokens, pricing.cacheReadPerTokenOver) +
		tieredCost(maxInt64(0, cacheCreationInputTokens), pricing.cacheCreatePerToken, pricing.thresholdTokens, pricing.cacheCreatePerTokenOver) +
		tieredCost(maxInt64(0, outputTokens), pricing.outputPerToken, pricing.thresholdTokens, pricing.outputPerTokenAbove)
	return cost, true
}

func tieredCost(tokens int64, baseRate float64, threshold *int, overRate *float64) float64 {
	if threshold == nil || overRate == nil {
		return float64(tokens) * baseRate
	}
	th := int64(*threshold)
	below := minInt64(tokens, th)
	over := maxInt64(0, tokens-th)
	return float64(below)*baseRate + float64(over)*(*overRate)
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
