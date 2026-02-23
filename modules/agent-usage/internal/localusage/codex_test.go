package localusage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanCodex(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	logDir := filepath.Join(root, "sessions", "2026", "02", "19")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	logPath := filepath.Join(logDir, "rollout-test.jsonl")

	content := "" +
		"{\"type\":\"turn_context\",\"timestamp\":\"2026-02-19T12:00:00Z\",\"payload\":{\"model\":\"gpt-5-codex\"}}\n" +
		"{\"type\":\"event_msg\",\"timestamp\":\"2026-02-19T12:00:02Z\",\"payload\":{\"type\":\"token_count\",\"info\":{\"total_token_usage\":{\"input_tokens\":100,\"cached_input_tokens\":20,\"output_tokens\":30},\"last_token_usage\":{\"input_tokens\":100,\"cached_input_tokens\":20,\"output_tokens\":30}}}}\n" +
		"{\"type\":\"event_msg\",\"timestamp\":\"2026-02-19T12:00:03Z\",\"payload\":{\"type\":\"token_count\",\"info\":{\"total_token_usage\":{\"input_tokens\":150,\"cached_input_tokens\":20,\"output_tokens\":50}}}}\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	since := time.Date(2026, 2, 19, 0, 0, 0, 0, time.Local)
	until := since

	summary, err := ScanCodex(context.Background(), root, since, until)
	if err != nil {
		t.Fatalf("scan codex: %v", err)
	}

	if summary.TodayTokens == nil || *summary.TodayTokens != 200 {
		t.Fatalf("unexpected today tokens: %#v", summary.TodayTokens)
	}
	if summary.Last30Tokens == nil || *summary.Last30Tokens != 200 {
		t.Fatalf("unexpected last30 tokens: %#v", summary.Last30Tokens)
	}
	if summary.TodayCostUSD == nil || *summary.TodayCostUSD <= 0 {
		t.Fatalf("expected positive today cost, got %#v", summary.TodayCostUSD)
	}
}
