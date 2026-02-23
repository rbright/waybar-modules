package providers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rbright/waybar-agent-usage/internal/config"
	"github.com/rbright/waybar-agent-usage/internal/domain"
	"github.com/rbright/waybar-agent-usage/internal/httpx"
	"github.com/rbright/waybar-agent-usage/internal/localusage"
)

const (
	codexRefreshEndpoint = "https://auth.openai.com/oauth/token"
	codexRefreshClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexUsageURL        = "https://chatgpt.com/backend-api/wham/usage"
)

type codexUsageResponse struct {
	PlanType  string             `json:"plan_type"`
	RateLimit codexRateLimitBody `json:"rate_limit"`
}

type codexRateLimitBody struct {
	PrimaryWindow   *codexWindow `json:"primary_window"`
	SecondaryWindow *codexWindow `json:"secondary_window"`
}

type codexWindow struct {
	UsedPercent *float64 `json:"used_percent"`
	ResetAt     *int64   `json:"reset_at"`
}

func FetchCodex(ctx context.Context, cfg config.Runtime) (domain.Metrics, error) {
	auth, err := readJSONMap(cfg.CodexAuthFile)
	if err != nil {
		return domain.Metrics{}, err
	}

	tokens := mapAtCreate(auth, "tokens")
	accessToken := strings.TrimSpace(cfg.CodexAccessToken)
	if accessToken == "" {
		accessToken = strings.TrimSpace(stringAt(tokens, "access_token"))
		if accessToken == "" {
			accessToken = strings.TrimSpace(stringAt(auth, "OPENAI_API_KEY"))
		}
	}
	accountID := strings.TrimSpace(cfg.CodexAccountID)
	if accountID == "" {
		accountID = strings.TrimSpace(stringAt(tokens, "account_id"))
	}
	if accessToken == "" {
		return domain.Metrics{}, fmt.Errorf("codex token missing. run `codex login`")
	}

	authUpdated := false
	if strings.TrimSpace(cfg.CodexAccessToken) == "" {
		refreshed, refreshedToken, err := codexRefreshIfNeeded(ctx, auth, cfg.Timeout)
		if err != nil {
			return domain.Metrics{}, err
		}
		if refreshed {
			authUpdated = true
			accessToken = refreshedToken
		}
	}

	usage, err := codexFetchUsage(ctx, cfg.Timeout, accessToken, accountID)
	if err != nil {
		if (httpx.IsStatus(err, 401) || httpx.IsStatus(err, 403)) && strings.TrimSpace(cfg.CodexAccessToken) == "" {
			refreshedToken, refreshErr := codexForceRefresh(ctx, auth, cfg.Timeout)
			if refreshErr != nil {
				return domain.Metrics{}, refreshErr
			}
			authUpdated = true
			accessToken = refreshedToken
			usage, err = codexFetchUsage(ctx, cfg.Timeout, accessToken, accountID)
		}
	}
	if err != nil {
		return domain.Metrics{}, err
	}

	if authUpdated {
		if err := writeJSONMapAtomic(cfg.CodexAuthFile, auth, 0o600); err != nil {
			return domain.Metrics{}, err
		}
	}

	metrics := domain.Metrics{
		Provider: domain.ProviderCodex,
		Plan:     strings.TrimSpace(usage.PlanType),
	}
	if usage.RateLimit.PrimaryWindow != nil && usage.RateLimit.PrimaryWindow.UsedPercent != nil {
		metrics.SessionRemaining = domain.Float64Ptr(domain.RemainingPercent(*usage.RateLimit.PrimaryWindow.UsedPercent))
	}
	if usage.RateLimit.SecondaryWindow != nil && usage.RateLimit.SecondaryWindow.UsedPercent != nil {
		metrics.WeeklyRemaining = domain.Float64Ptr(domain.RemainingPercent(*usage.RateLimit.SecondaryWindow.UsedPercent))
	}
	if usage.RateLimit.PrimaryWindow != nil && usage.RateLimit.PrimaryWindow.ResetAt != nil {
		metrics.SessionReset = domain.ParseEpochSeconds(float64(*usage.RateLimit.PrimaryWindow.ResetAt))
	}
	if usage.RateLimit.SecondaryWindow != nil && usage.RateLimit.SecondaryWindow.ResetAt != nil {
		metrics.WeeklyReset = domain.ParseEpochSeconds(float64(*usage.RateLimit.SecondaryWindow.ResetAt))
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	since := today.AddDate(0, 0, -29)
	if summary, scanErr := localusage.ScanCodex(ctx, cfg.CodexHome, since, today); scanErr == nil {
		metrics.TodayTokens = summary.TodayTokens
		metrics.TodayCostUSD = summary.TodayCostUSD
		metrics.Last30Tokens = summary.Last30Tokens
		metrics.Last30CostUSD = summary.Last30CostUSD
	}

	return metrics, nil
}

func codexFetchUsage(ctx context.Context, timeout time.Duration, accessToken, accountID string) (codexUsageResponse, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + strings.TrimSpace(accessToken),
		"Accept":        "application/json",
		"User-Agent":    "waybar-agent-usage",
	}
	if strings.TrimSpace(accountID) != "" {
		headers["ChatGPT-Account-Id"] = strings.TrimSpace(accountID)
	}
	return httpx.DoJSON[codexUsageResponse](ctx, "GET", codexUsageURL, headers, nil, timeout)
}

func codexRefreshIfNeeded(ctx context.Context, auth map[string]any, timeout time.Duration) (bool, string, error) {
	refreshToken := strings.TrimSpace(stringAt(mapAt(auth, "tokens"), "refresh_token"))
	if refreshToken == "" {
		return false, strings.TrimSpace(stringAt(mapAt(auth, "tokens"), "access_token")), nil
	}

	lastRefreshText := strings.TrimSpace(stringAt(auth, "last_refresh"))
	if lastRefreshText != "" {
		if lastRefresh, ok := domain.ParseISO8601(lastRefreshText); ok {
			if time.Since(*lastRefresh) <= 8*24*time.Hour {
				return false, strings.TrimSpace(stringAt(mapAt(auth, "tokens"), "access_token")), nil
			}
		}
	}

	accessToken, err := codexRefresh(ctx, auth, timeout)
	if err != nil {
		return false, "", err
	}
	return true, accessToken, nil
}

func codexForceRefresh(ctx context.Context, auth map[string]any, timeout time.Duration) (string, error) {
	return codexRefresh(ctx, auth, timeout)
}

func codexRefresh(ctx context.Context, auth map[string]any, timeout time.Duration) (string, error) {
	tokens := mapAtCreate(auth, "tokens")
	refreshToken := strings.TrimSpace(stringAt(tokens, "refresh_token"))
	if refreshToken == "" {
		return "", fmt.Errorf("codex auth expired. run `codex login`")
	}

	body, err := httpx.EncodeJSON(map[string]string{
		"client_id":     codexRefreshClientID,
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"scope":         "openid profile email",
	})
	if err != nil {
		return "", err
	}

	response, err := httpx.DoJSON[map[string]any](
		ctx,
		"POST",
		codexRefreshEndpoint,
		map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
			"User-Agent":   "waybar-agent-usage",
		},
		body,
		timeout,
	)
	if err != nil {
		if httpx.Is5xx(err) {
			return "", fmt.Errorf("codex oauth refresh endpoint temporarily unavailable (http 5xx)")
		}
		return "", err
	}

	access := strings.TrimSpace(stringValue(response["access_token"]))
	if access == "" {
		return "", fmt.Errorf("codex refresh returned no access token")
	}

	tokens["access_token"] = access
	if refresh := strings.TrimSpace(stringValue(response["refresh_token"])); refresh != "" {
		tokens["refresh_token"] = refresh
	}
	if idToken := strings.TrimSpace(stringValue(response["id_token"])); idToken != "" {
		tokens["id_token"] = idToken
	}
	auth["last_refresh"] = time.Now().UTC().Format(time.RFC3339)

	return access, nil
}
