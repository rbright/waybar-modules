package localusage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanClaude(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	projects := filepath.Join(root, "projects", "example")
	if err := os.MkdirAll(projects, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	logPath := filepath.Join(projects, "session.jsonl")

	content := "" +
		"{\"type\":\"assistant\",\"timestamp\":\"2026-02-19T10:00:00Z\",\"requestId\":\"req-1\",\"message\":{\"id\":\"msg-1\",\"model\":\"claude-sonnet-4-5\",\"usage\":{\"input_tokens\":100,\"cache_read_input_tokens\":20,\"cache_creation_input_tokens\":10,\"output_tokens\":30}}}\n" +
		"{\"type\":\"assistant\",\"timestamp\":\"2026-02-19T10:01:00Z\",\"requestId\":\"req-1\",\"message\":{\"id\":\"msg-1\",\"model\":\"claude-sonnet-4-5\",\"usage\":{\"input_tokens\":999,\"cache_read_input_tokens\":999,\"cache_creation_input_tokens\":999,\"output_tokens\":999}}}\n" +
		"{\"type\":\"assistant\",\"timestamp\":\"2026-02-19T10:02:00Z\",\"requestId\":\"req-2\",\"message\":{\"id\":\"msg-2\",\"model\":\"claude-sonnet-4-5\",\"usage\":{\"input_tokens\":50,\"cache_read_input_tokens\":0,\"cache_creation_input_tokens\":0,\"output_tokens\":25}}}\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	now := time.Now()
	if err := os.Chtimes(logPath, now, now); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	since := time.Date(2026, 2, 19, 0, 0, 0, 0, time.Local)
	until := since

	summary, err := ScanClaude(context.Background(), []string{filepath.Join(root, "projects")}, since, until)
	if err != nil {
		t.Fatalf("scan claude: %v", err)
	}

	// deduped totals: first line (160) + third line (75) = 235
	if summary.TodayTokens == nil || *summary.TodayTokens != 235 {
		t.Fatalf("unexpected today tokens: %#v", summary.TodayTokens)
	}
	if summary.Last30Tokens == nil || *summary.Last30Tokens != 235 {
		t.Fatalf("unexpected last30 tokens: %#v", summary.Last30Tokens)
	}
	if summary.TodayCostUSD == nil || *summary.TodayCostUSD <= 0 {
		t.Fatalf("expected positive today cost, got %#v", summary.TodayCostUSD)
	}
}
