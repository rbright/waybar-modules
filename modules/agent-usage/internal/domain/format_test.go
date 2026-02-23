package domain

import (
	"testing"
	"time"
)

func TestFormatTokens(t *testing.T) {
	value := int64(7_300_000_000)
	if got := FormatTokens(&value); got != "7.3B" {
		t.Fatalf("expected 7.3B, got %q", got)
	}
}

func TestFormatPercent(t *testing.T) {
	value := 57.6
	if got := FormatPercent(&value); got != "58%" {
		t.Fatalf("expected 58%%, got %q", got)
	}
}

func TestResetCountdown(t *testing.T) {
	now := time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)
	reset := now.Add(95 * time.Minute)
	if got := ResetCountdown(now, &reset); got != "in 1h 35m" {
		t.Fatalf("unexpected countdown %q", got)
	}
}
