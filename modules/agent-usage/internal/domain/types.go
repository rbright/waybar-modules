package domain

import (
	"fmt"
	"time"
)

type Provider string

const (
	ProviderCodex  Provider = "codex"
	ProviderClaude Provider = "claude"
)

func ParseProvider(raw string) (Provider, error) {
	switch Provider(raw) {
	case ProviderCodex:
		return ProviderCodex, nil
	case ProviderClaude:
		return ProviderClaude, nil
	default:
		return "", fmt.Errorf("unsupported provider %q", raw)
	}
}

type Metrics struct {
	Provider Provider `json:"provider"`
	Plan     string   `json:"plan,omitempty"`

	SessionRemaining *float64   `json:"session_remaining,omitempty"`
	WeeklyRemaining  *float64   `json:"weekly_remaining,omitempty"`
	SessionReset     *time.Time `json:"session_reset,omitempty"`
	WeeklyReset      *time.Time `json:"weekly_reset,omitempty"`

	TodayTokens   *int64   `json:"today_tokens,omitempty"`
	TodayCostUSD  *float64 `json:"today_cost_usd,omitempty"`
	Last30Tokens  *int64   `json:"last30_tokens,omitempty"`
	Last30CostUSD *float64 `json:"last30_cost_usd,omitempty"`

	ExtraUsed     *float64 `json:"extra_used,omitempty"`
	ExtraLimit    *float64 `json:"extra_limit,omitempty"`
	ExtraCurrency string   `json:"extra_currency,omitempty"`
}

type LocalUsageSummary struct {
	TodayTokens   *int64
	TodayCostUSD  *float64
	Last30Tokens  *int64
	Last30CostUSD *float64
}
