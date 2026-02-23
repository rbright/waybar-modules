package localusage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rbright/waybar-agent-usage/internal/domain"
)

func ClaudeProjectRoots(homeDir string) []string {
	env := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR"))
	roots := make([]string, 0)

	if env != "" {
		for _, raw := range strings.Split(env, ",") {
			path := strings.TrimSpace(raw)
			if path == "" {
				continue
			}
			if filepath.Base(path) == "projects" {
				roots = append(roots, path)
			} else {
				roots = append(roots, filepath.Join(path, "projects"))
			}
		}
	} else {
		roots = append(roots,
			filepath.Join(homeDir, ".config", "claude", "projects"),
			filepath.Join(homeDir, ".claude", "projects"),
		)
	}

	unique := make([]string, 0, len(roots))
	seen := map[string]struct{}{}
	for _, root := range roots {
		if _, ok := seen[root]; ok {
			continue
		}
		seen[root] = struct{}{}
		unique = append(unique, root)
	}
	return unique
}

func ScanClaude(ctx context.Context, roots []string, sinceDay, untilDay time.Time) (domain.LocalUsageSummary, error) {
	sinceKey := sinceDay.Format("2006-01-02")
	untilKey := untilDay.Format("2006-01-02")
	minMTime := sinceDay.AddDate(0, 0, -1)

	days := map[string]*dayBucket{}

	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return domain.LocalUsageSummary{}, err
		}
		if err := walkClaudeRoot(root, minMTime, func(path string) error {
			return scanClaudeFile(path, sinceKey, untilKey, days)
		}); err != nil {
			return domain.LocalUsageSummary{}, err
		}
	}

	return summarizeDays(days), nil
}

func walkClaudeRoot(root string, minMTime time.Time, onFile func(path string) error) error {
	_, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat claude root %s: %w", root, err)
	}

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().Before(minMTime) {
			return nil
		}

		return onFile(path)
	})
}

func scanClaudeFile(path, sinceKey, untilKey string, days map[string]*dayBucket) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open claude log %s: %w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)

	seenPairs := map[string]struct{}{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		if stringValue(obj["type"]) != "assistant" {
			continue
		}

		dayKey, ok := domain.DayKeyFromTimestamp(stringValue(obj["timestamp"]))
		if !ok || dayKey < sinceKey || dayKey > untilKey {
			continue
		}

		message := mapValue(obj["message"])
		usage := mapValue(message["usage"])
		if len(usage) == 0 {
			continue
		}

		messageID := strings.TrimSpace(stringValue(message["id"]))
		requestID := strings.TrimSpace(stringValue(obj["requestId"]))
		if messageID != "" && requestID != "" {
			pair := messageID + ":" + requestID
			if _, seen := seenPairs[pair]; seen {
				continue
			}
			seenPairs[pair] = struct{}{}
		}

		model := stringValue(message["model"])
		input := nonNegative(domain.ParseInt64Any(usage["input_tokens"]))
		cacheRead := nonNegative(domain.ParseInt64Any(usage["cache_read_input_tokens"]))
		cacheCreate := nonNegative(domain.ParseInt64Any(usage["cache_creation_input_tokens"]))
		output := nonNegative(domain.ParseInt64Any(usage["output_tokens"]))
		if input == 0 && cacheRead == 0 && cacheCreate == 0 && output == 0 {
			continue
		}

		totalTokens := input + cacheRead + cacheCreate + output
		bucket := ensureBucket(days, dayKey)
		bucket.tokens += totalTokens

		if cost, ok := domain.ClaudeCostUSD(model, input, cacheRead, cacheCreate, output); ok {
			bucket.costUSD += cost
			bucket.costSeen = true
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan claude log %s: %w", path, err)
	}

	return nil
}
