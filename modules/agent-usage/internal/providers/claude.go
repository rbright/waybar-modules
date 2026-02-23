package providers

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rbright/waybar-agent-usage/internal/config"
	"github.com/rbright/waybar-agent-usage/internal/domain"
	"github.com/rbright/waybar-agent-usage/internal/httpx"
	"github.com/rbright/waybar-agent-usage/internal/localusage"
)

const (
	claudeRefreshEndpoint = "https://platform.claude.com/v1/oauth/token"
	claudeUsageURL        = "https://api.anthropic.com/api/oauth/usage"
	claudeUsageBetaHeader = "oauth-2025-04-20"
)

type claudeUsageResponse struct {
	FiveHour   *claudeUsageWindow `json:"five_hour"`
	SevenDay   *claudeUsageWindow `json:"seven_day"`
	ExtraUsage *claudeExtraUsage  `json:"extra_usage"`
}

type claudeUsageWindow struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    string   `json:"resets_at"`
}

type claudeExtraUsage struct {
	UsedCredits  *float64 `json:"used_credits"`
	MonthlyLimit *float64 `json:"monthly_limit"`
	Currency     string   `json:"currency"`
}

func FetchClaude(ctx context.Context, cfg config.Runtime) (domain.Metrics, error) {
	credentials, err := readJSONMap(cfg.ClaudeCredentialsFile)
	if err != nil {
		return domain.Metrics{}, err
	}
	oauth := mapAtCreate(credentials, "claudeAiOauth")

	accessToken := strings.TrimSpace(cfg.ClaudeAccessToken)
	if accessToken == "" {
		accessToken = strings.TrimSpace(stringAt(oauth, "accessToken"))
	}
	if accessToken == "" {
		return domain.Metrics{}, fmt.Errorf("claude token missing. run `claude login`")
	}

	credentialsUpdated := false
	refreshFailed5xx := false
	if isClaudeTokenExpired(oauth) && strings.TrimSpace(cfg.ClaudeAccessToken) == "" {
		refreshedToken, refreshErr := claudeRefreshToken(ctx, oauth, cfg)
		if refreshErr != nil {
			if httpx.Is5xx(refreshErr) || strings.Contains(strings.ToLower(refreshErr.Error()), "temporarily unavailable") {
				refreshFailed5xx = true
			} else {
				return domain.Metrics{}, refreshErr
			}
		} else {
			credentialsUpdated = true
			accessToken = refreshedToken
		}
	}

	usage, err := claudeFetchUsage(ctx, cfg.Timeout, accessToken)
	if err != nil {
		statusErr := &httpx.StatusError{}
		if httpx.AsStatusError(err, &statusErr) && statusErr.StatusCode == 401 &&
			strings.Contains(strings.ToLower(statusErr.Body), "token_expired") && strings.TrimSpace(cfg.ClaudeAccessToken) == "" {
			refreshedToken, refreshErr := claudeRefreshToken(ctx, oauth, cfg)
			if refreshErr != nil {
				if httpx.Is5xx(refreshErr) {
					return domain.Metrics{}, fmt.Errorf("claude oauth refresh endpoint temporarily unavailable (http 5xx)")
				}
				return domain.Metrics{}, refreshErr
			}
			credentialsUpdated = true
			accessToken = refreshedToken
			usage, err = claudeFetchUsage(ctx, cfg.Timeout, accessToken)
		}
	}
	if err != nil {
		if httpx.Is5xx(err) {
			return domain.Metrics{}, fmt.Errorf("claude usage api temporarily unavailable (http 5xx)")
		}
		if refreshFailed5xx {
			return domain.Metrics{}, fmt.Errorf("claude oauth refresh endpoint temporarily unavailable (http 5xx)")
		}
		return domain.Metrics{}, err
	}

	if credentialsUpdated {
		credentials["claudeAiOauth"] = oauth
		if err := writeJSONMapAtomic(cfg.ClaudeCredentialsFile, credentials, 0o600); err != nil {
			return domain.Metrics{}, err
		}
	}

	metrics := domain.Metrics{
		Provider: domain.ProviderClaude,
		Plan: strings.TrimSpace(firstNonEmpty(
			stringAt(oauth, "rateLimitTier"),
			stringAt(oauth, "subscriptionType"),
		)),
	}

	if usage.FiveHour != nil && usage.FiveHour.Utilization != nil {
		metrics.SessionRemaining = domain.Float64Ptr(domain.RemainingPercent(*usage.FiveHour.Utilization))
	}
	if usage.SevenDay != nil && usage.SevenDay.Utilization != nil {
		metrics.WeeklyRemaining = domain.Float64Ptr(domain.RemainingPercent(*usage.SevenDay.Utilization))
	}
	if usage.FiveHour != nil {
		if parsed, ok := domain.ParseISO8601(strings.TrimSpace(usage.FiveHour.ResetsAt)); ok {
			metrics.SessionReset = parsed
		}
	}
	if usage.SevenDay != nil {
		if parsed, ok := domain.ParseISO8601(strings.TrimSpace(usage.SevenDay.ResetsAt)); ok {
			metrics.WeeklyReset = parsed
		}
	}

	if usage.ExtraUsage != nil {
		currency := strings.ToUpper(strings.TrimSpace(usage.ExtraUsage.Currency))
		if currency == "" {
			currency = "USD"
		}
		metrics.ExtraCurrency = currency
		if usage.ExtraUsage.UsedCredits != nil {
			metrics.ExtraUsed = domain.Float64Ptr(*usage.ExtraUsage.UsedCredits / 100.0)
		}
		if usage.ExtraUsage.MonthlyLimit != nil {
			metrics.ExtraLimit = domain.Float64Ptr(*usage.ExtraUsage.MonthlyLimit / 100.0)
		}
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	since := today.AddDate(0, 0, -29)
	home, _ := os.UserHomeDir()
	if summary, scanErr := localusage.ScanClaude(ctx, localusage.ClaudeProjectRoots(home), since, today); scanErr == nil {
		metrics.TodayTokens = summary.TodayTokens
		metrics.TodayCostUSD = summary.TodayCostUSD
		metrics.Last30Tokens = summary.Last30Tokens
		metrics.Last30CostUSD = summary.Last30CostUSD
	}

	return metrics, nil
}

func claudeFetchUsage(ctx context.Context, timeout time.Duration, accessToken string) (claudeUsageResponse, error) {
	return httpx.DoJSON[claudeUsageResponse](
		ctx,
		"GET",
		claudeUsageURL,
		map[string]string{
			"Authorization":  "Bearer " + strings.TrimSpace(accessToken),
			"Accept":         "application/json",
			"Content-Type":   "application/json",
			"anthropic-beta": claudeUsageBetaHeader,
			"User-Agent":     "waybar-agent-usage",
		},
		nil,
		timeout,
	)
}

func isClaudeTokenExpired(oauth map[string]any) bool {
	if oauth == nil {
		return true
	}
	raw := oauth["expiresAt"]
	if raw == nil {
		return true
	}
	expiresAtMs := domain.ParseInt64Any(raw)
	if expiresAtMs <= 0 {
		return true
	}
	expires := domain.ParseEpochMillis(float64(expiresAtMs))
	if expires == nil {
		return true
	}
	return time.Now().UTC().After(*expires)
}

func claudeRefreshToken(ctx context.Context, oauth map[string]any, cfg config.Runtime) (string, error) {
	refreshToken := strings.TrimSpace(stringAt(oauth, "refreshToken"))
	if refreshToken == "" {
		return "", fmt.Errorf("claude refresh token missing. run `claude login`")
	}

	body := []byte(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {cfg.ClaudeClientID},
	}.Encode())

	retries := cfg.ClaudeRetries
	if retries < 1 {
		retries = 1
	}

	var lastErr error
	for attempt := 0; attempt < retries; attempt++ {
		response, err := httpx.DoJSON[map[string]any](
			ctx,
			"POST",
			claudeRefreshEndpoint,
			map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
				"Accept":       "application/json",
				"User-Agent":   "waybar-agent-usage",
			},
			body,
			cfg.Timeout,
		)
		if err != nil {
			lastErr = err
			statusErr := &httpx.StatusError{}
			if httpx.AsStatusError(err, &statusErr) {
				lowerBody := strings.ToLower(statusErr.Body)
				if strings.Contains(lowerBody, "invalid_grant") {
					return "", fmt.Errorf("claude oauth refresh token invalid. run `claude login`")
				}
				if statusErr.StatusCode >= 500 && statusErr.StatusCode <= 599 {
					if attempt < retries-1 {
						backoff := 350 * time.Millisecond * time.Duration(1<<attempt)
						if backoff > 1500*time.Millisecond {
							backoff = 1500 * time.Millisecond
						}
						time.Sleep(backoff)
						continue
					}
					return "", fmt.Errorf("claude oauth refresh endpoint temporarily unavailable (http 5xx)")
				}
			}
			return "", err
		}

		accessToken := strings.TrimSpace(stringValue(response["access_token"]))
		if accessToken == "" {
			return "", fmt.Errorf("claude refresh returned no access token")
		}

		oauth["accessToken"] = accessToken
		if refreshedToken := strings.TrimSpace(stringValue(response["refresh_token"])); refreshedToken != "" {
			oauth["refreshToken"] = refreshedToken
		}

		expiresIn := domain.ParseInt64Any(response["expires_in"])
		if expiresIn > 0 {
			oauth["expiresAt"] = time.Now().UTC().Add(time.Duration(expiresIn) * time.Second).UnixMilli()
		}

		if scope := strings.TrimSpace(stringValue(response["scope"])); scope != "" {
			parts := strings.Fields(scope)
			scopes := make([]any, 0, len(parts))
			for _, part := range parts {
				scopes = append(scopes, part)
			}
			oauth["scopes"] = scopes
		}

		return accessToken, nil
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("claude oauth refresh failed")
}
