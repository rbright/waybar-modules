package domain

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func ClampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func RemainingPercent(used float64) float64 {
	return ClampPercent(100 - used)
}

func FormatPercent(value *float64) string {
	if value == nil {
		return "—"
	}
	return fmt.Sprintf("%d%%", int(math.Round(ClampPercent(*value))))
}

func FormatTokens(value *int64) string {
	if value == nil {
		return "—"
	}

	n := *value
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}

	type unit struct {
		threshold int64
		suffix    string
	}

	for _, u := range []unit{{1_000_000_000, "B"}, {1_000_000, "M"}, {1_000, "K"}} {
		if n >= u.threshold {
			scaled := float64(n) / float64(u.threshold)
			if scaled >= 10 {
				return fmt.Sprintf("%s%.0f%s", sign, scaled, u.suffix)
			}
			text := fmt.Sprintf("%.1f", scaled)
			text = strings.TrimSuffix(strings.TrimSuffix(text, "0"), ".")
			return sign + text + u.suffix
		}
	}

	return sign + withCommasInt(n)
}

func FormatUSD(value *float64) string {
	if value == nil {
		return "—"
	}
	return fmt.Sprintf("$%s", withCommasFloat(*value, 2))
}

func FormatMoney(value *float64, currency string) string {
	code := strings.ToUpper(strings.TrimSpace(currency))
	if code == "" {
		code = "USD"
	}
	if code == "USD" {
		return FormatUSD(value)
	}
	if value == nil {
		return "—"
	}
	return fmt.Sprintf("%s %s", withCommasFloat(*value, 2), code)
}

func RelativeAge(now time.Time, fetchedAt time.Time) string {
	age := now.Sub(fetchedAt)
	if age < 0 {
		age = 0
	}
	seconds := int(age.Seconds())
	if seconds < 60 {
		return "just now"
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm ago", minutes)
	}
	hours := minutes / 60
	if hours < 24 {
		return fmt.Sprintf("%dh ago", hours)
	}
	days := hours / 24
	return fmt.Sprintf("%dd ago", days)
}

func ResetCountdown(now time.Time, resetAt *time.Time) string {
	if resetAt == nil {
		return "—"
	}
	delta := resetAt.Sub(now)
	if delta <= 0 {
		return "now"
	}

	minutes := int(math.Ceil(delta.Minutes()))
	if minutes < 1 {
		minutes = 1
	}

	days := minutes / (24 * 60)
	hours := (minutes / 60) % 24
	mins := minutes % 60

	if days > 0 {
		if hours > 0 {
			return fmt.Sprintf("in %dd %dh", days, hours)
		}
		return fmt.Sprintf("in %dd", days)
	}

	if hours > 0 {
		if mins > 0 {
			return fmt.Sprintf("in %dh %dm", hours, mins)
		}
		return fmt.Sprintf("in %dh", hours)
	}

	return fmt.Sprintf("in %dm", mins)
}

func ResetAbsolute(resetAt *time.Time) string {
	if resetAt == nil {
		return "—"
	}
	return resetAt.Local().Format("Jan 2, 3:04 PM")
}

func withCommasInt(v int64) string {
	negative := v < 0
	if negative {
		v = -v
	}
	raw := strconv.FormatInt(v, 10)
	n := len(raw)
	if n <= 3 {
		if negative {
			return "-" + raw
		}
		return raw
	}

	var b strings.Builder
	if negative {
		b.WriteByte('-')
	}
	prefix := n % 3
	if prefix == 0 {
		prefix = 3
	}
	b.WriteString(raw[:prefix])
	for i := prefix; i < n; i += 3 {
		b.WriteByte(',')
		b.WriteString(raw[i : i+3])
	}
	return b.String()
}

func withCommasFloat(v float64, decimals int) string {
	format := fmt.Sprintf("%%.%df", decimals)
	raw := fmt.Sprintf(format, v)
	parts := strings.SplitN(raw, ".", 2)
	intPart, _ := strconv.ParseInt(parts[0], 10, 64)
	if len(parts) == 1 {
		return withCommasInt(intPart)
	}
	return withCommasInt(intPart) + "." + parts[1]
}
