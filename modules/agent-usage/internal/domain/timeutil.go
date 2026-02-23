package domain

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func ParseISO8601(raw string) (*time.Time, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, false
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z0700",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, text); err == nil {
			utc := parsed.UTC()
			return &utc, true
		}
	}

	if strings.HasSuffix(text, "Z") {
		trimmed := strings.TrimSuffix(text, "Z")
		if parsed, err := time.Parse("2006-01-02T15:04:05.999999999", trimmed); err == nil {
			utc := parsed.UTC()
			return &utc, true
		}
	}

	return nil, false
}

func ParseEpochSeconds(value float64) *time.Time {
	parsed := time.Unix(int64(value), 0).UTC()
	return &parsed
}

func ParseEpochMillis(value float64) *time.Time {
	seconds := int64(value / 1000)
	nanos := int64(value*1_000_000) % int64(time.Second)
	parsed := time.Unix(seconds, nanos).UTC()
	return &parsed
}

func ToISOZ(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func DayKeyFromTimestamp(raw string) (string, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", false
	}
	if len(text) >= 10 && text[4] == '-' && text[7] == '-' {
		return text[:10], true
	}
	if parsed, ok := ParseISO8601(text); ok {
		return parsed.Local().Format("2006-01-02"), true
	}
	return "", false
}

func ParseFloat(raw string, fallback float64) float64 {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return fallback
	}
	return value
}

func ParseInt(raw string, fallback int) int {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return fallback
	}
	return value
}

func ParseInt64Any(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return int64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		return int64(v)
	case float32:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i
		}
		if f, err := v.Float64(); err == nil {
			return int64(f)
		}
		return 0
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return int64(f)
		}
		return 0
	default:
		return 0
	}
}

func ToFloat64Any(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func MustDayKey(date time.Time) string {
	return date.Format("2006-01-02")
}

func ParseDayKey(raw string) (time.Time, error) {
	parsed, err := time.ParseInLocation("2006-01-02", raw, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse day key %q: %w", raw, err)
	}
	return parsed, nil
}
