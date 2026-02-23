package domain

import "testing"

func TestNormalizeCodexModel(t *testing.T) {
	if got := NormalizeCodexModel("openai/gpt-5-codex"); got != "gpt-5" {
		t.Fatalf("expected gpt-5, got %q", got)
	}
	if got := NormalizeCodexModel("gpt-5.2-codex"); got != "gpt-5.2" {
		t.Fatalf("expected gpt-5.2, got %q", got)
	}
}

func TestCodexCostUSD(t *testing.T) {
	cost, ok := CodexCostUSD("gpt-5", 1_000_000, 200_000, 500_000)
	if !ok {
		t.Fatal("expected pricing to resolve")
	}
	if cost <= 0 {
		t.Fatalf("expected positive cost, got %f", cost)
	}
}

func TestNormalizeClaudeModel(t *testing.T) {
	if got := NormalizeClaudeModel("anthropic.claude-sonnet-4-5-20250929"); got != "claude-sonnet-4-5" {
		t.Fatalf("expected claude-sonnet-4-5, got %q", got)
	}
	if got := NormalizeClaudeModel("provider.foo.claude-opus-4-6-v1:2"); got != "claude-opus-4-6" {
		t.Fatalf("expected claude-opus-4-6, got %q", got)
	}
}

func TestClaudeCostUSDTiered(t *testing.T) {
	cost, ok := ClaudeCostUSD("claude-sonnet-4-5", 250_000, 0, 0, 0)
	if !ok {
		t.Fatal("expected pricing to resolve")
	}
	if cost <= 0 {
		t.Fatalf("expected positive cost, got %f", cost)
	}

	baseCost, _ := ClaudeCostUSD("claude-sonnet-4-5", 150_000, 0, 0, 0)
	if cost <= baseCost {
		t.Fatalf("expected tiered cost (%f) to exceed base cost (%f)", cost, baseCost)
	}
}
