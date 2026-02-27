package waybar

import (
	"testing"
	"time"

	"github.com/rbright/waybar-agent-usage/internal/domain"
)

func TestRender_PlacesIconAfterPercent(t *testing.T) {
	metrics := domain.Metrics{
		Provider:        domain.ProviderCodex,
		WeeklyRemaining: domain.Float64Ptr(44),
	}

	out := Render(
		metrics,
		time.Unix(0, 0).UTC(),
		IconConfig{Codex: "OPENAI", Claude: "CLAUDE"},
		"",
	)

	if out.Text != "44%  OPENAI" {
		t.Fatalf("unexpected text: %q", out.Text)
	}
}
