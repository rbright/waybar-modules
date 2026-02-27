package waybar

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rbright/waybar-agent-usage/internal/domain"
)

type Output struct {
	Text    string `json:"text"`
	Tooltip string `json:"tooltip"`
	Class   string `json:"class"`
}

type IconConfig struct {
	Codex  string
	Claude string
}

func Render(metrics domain.Metrics, fetchedAt time.Time, icons IconConfig, staleError string) Output {
	provider := metrics.Provider
	icon := iconFor(provider, icons)
	text := fmt.Sprintf("%s  %s", domain.FormatPercent(metrics.WeeklyRemaining), icon)

	classes := []string{string(provider), severityClass(metrics.WeeklyRemaining)}
	if strings.TrimSpace(staleError) != "" {
		classes = append(classes, "stale")
	}

	return Output{
		Text:    text,
		Tooltip: tooltip(metrics, fetchedAt, staleError),
		Class:   strings.Join(classes, " "),
	}
}

func RenderError(provider domain.Provider, icons IconConfig, message string) Output {
	return Output{
		Text:    fmt.Sprintf("%s --", iconFor(provider, icons)),
		Tooltip: strings.TrimSpace(message),
		Class:   fmt.Sprintf("%s error", provider),
	}
}

func Encode(output Output) ([]byte, error) {
	payload, err := json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("marshal waybar output: %w", err)
	}
	return payload, nil
}

func iconFor(provider domain.Provider, icons IconConfig) string {
	switch provider {
	case domain.ProviderCodex:
		if strings.TrimSpace(icons.Codex) != "" {
			return icons.Codex
		}
		return "⌬"
	case domain.ProviderClaude:
		if strings.TrimSpace(icons.Claude) != "" {
			return icons.Claude
		}
		return "✻"
	default:
		return "?"
	}
}

func severityClass(weeklyRemaining *float64) string {
	if weeklyRemaining == nil {
		return "unknown"
	}
	if *weeklyRemaining <= 10 {
		return "critical"
	}
	if *weeklyRemaining <= 20 {
		return "warning"
	}
	return "normal"
}

func tooltip(metrics domain.Metrics, fetchedAt time.Time, staleError string) string {
	title := "Claude usage"
	if metrics.Provider == domain.ProviderCodex {
		title = "Codex usage"
	}

	lines := []string{
		title,
		fmt.Sprintf("Session remaining: %s", domain.FormatPercent(metrics.SessionRemaining)),
		fmt.Sprintf("Session reset: %s", resetLine(metrics.SessionReset)),
		fmt.Sprintf("Weekly remaining: %s", domain.FormatPercent(metrics.WeeklyRemaining)),
		fmt.Sprintf("Weekly reset: %s", resetLine(metrics.WeeklyReset)),
		fmt.Sprintf("Today: %s · %s tokens", domain.FormatUSD(metrics.TodayCostUSD), domain.FormatTokens(metrics.TodayTokens)),
		fmt.Sprintf("Last 30 days: %s · %s tokens", domain.FormatUSD(metrics.Last30CostUSD), domain.FormatTokens(metrics.Last30Tokens)),
	}

	if metrics.Provider == domain.ProviderClaude && metrics.ExtraUsed != nil && metrics.ExtraLimit != nil {
		lines = append(lines, fmt.Sprintf(
			"Extra usage: %s / %s",
			domain.FormatMoney(metrics.ExtraUsed, metrics.ExtraCurrency),
			domain.FormatMoney(metrics.ExtraLimit, metrics.ExtraCurrency),
		))
	}

	if strings.TrimSpace(metrics.Plan) != "" {
		lines = append(lines, fmt.Sprintf("Plan: %s", strings.TrimSpace(metrics.Plan)))
	}
	lines = append(lines, fmt.Sprintf("Updated: %s", domain.RelativeAge(time.Now(), fetchedAt)))

	if strings.TrimSpace(staleError) != "" {
		lines = append(lines, "", fmt.Sprintf("Cached data (refresh failed): %s", strings.TrimSpace(staleError)))
	}

	return strings.Join(lines, "\n")
}

func resetLine(value *time.Time) string {
	if value == nil {
		return "—"
	}
	return fmt.Sprintf("%s (%s)", domain.ResetAbsolute(value), domain.ResetCountdown(time.Now(), value))
}
