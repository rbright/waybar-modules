package localusage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rbright/waybar-agent-usage/internal/domain"
)

type codexTotals struct {
	input  int64
	cached int64
	output int64
}

func ScanCodex(ctx context.Context, codexHome string, sinceDay, untilDay time.Time) (domain.LocalUsageSummary, error) {
	sinceKey := sinceDay.Format("2006-01-02")
	untilKey := untilDay.Format("2006-01-02")

	files, err := listCodexFiles(codexHome, sinceDay, untilDay)
	if err != nil {
		return domain.LocalUsageSummary{}, err
	}

	days := map[string]*dayBucket{}

	for _, filePath := range files {
		if err := ctx.Err(); err != nil {
			return domain.LocalUsageSummary{}, err
		}
		if err := scanCodexFile(filePath, sinceKey, untilKey, days); err != nil {
			return domain.LocalUsageSummary{}, err
		}
	}

	return summarizeDays(days), nil
}

func listCodexFiles(codexHome string, sinceDay, untilDay time.Time) ([]string, error) {
	roots := []string{
		filepath.Join(codexHome, "sessions"),
		filepath.Join(codexHome, "archived_sessions"),
	}

	seen := map[string]struct{}{}
	files := make([]string, 0)

	for day := sinceDay; !day.After(untilDay); day = day.AddDate(0, 0, 1) {
		y, m, d := day.Date()
		for _, root := range roots {
			partitionDir := filepath.Join(root, fmt.Sprintf("%04d", y), fmt.Sprintf("%02d", m), fmt.Sprintf("%02d", d))
			entries, err := os.ReadDir(partitionDir)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("read codex partition dir %s: %w", partitionDir, err)
			}
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
					continue
				}
				path := filepath.Join(partitionDir, entry.Name())
				if _, ok := seen[path]; ok {
					continue
				}
				seen[path] = struct{}{}
				files = append(files, path)
			}
		}
	}

	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read codex root dir %s: %w", root, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
				continue
			}
			path := filepath.Join(root, entry.Name())
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			files = append(files, path)
		}
	}

	sort.Strings(files)
	return files, nil
}

func scanCodexFile(path, sinceKey, untilKey string, days map[string]*dayBucket) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open codex log %s: %w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)

	currentModel := ""
	var previous *codexTotals

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}

		typ := stringValue(obj["type"])
		switch typ {
		case "turn_context":
			payload := mapValue(obj["payload"])
			if model := stringValue(payload["model"]); strings.TrimSpace(model) != "" {
				currentModel = strings.TrimSpace(model)
			}
			continue
		case "event_msg":
			// Continue below.
		default:
			continue
		}

		payload := mapValue(obj["payload"])
		if stringValue(payload["type"]) != "token_count" {
			continue
		}

		dayKey, ok := domain.DayKeyFromTimestamp(stringValue(obj["timestamp"]))
		if !ok || dayKey < sinceKey || dayKey > untilKey {
			continue
		}

		info := mapValue(payload["info"])
		model := stringValue(info["model"])
		if model == "" {
			model = stringValue(info["model_name"])
		}
		if model == "" {
			model = stringValue(payload["model"])
		}
		if model == "" {
			model = currentModel
		}
		if model == "" {
			model = "gpt-5"
		}

		deltaInput, deltaCached, deltaOutput := int64(0), int64(0), int64(0)

		totals := mapValue(info["total_token_usage"])
		if len(totals) > 0 {
			tInput := nonNegative(domain.ParseInt64Any(totals["input_tokens"]))
			tCached := nonNegative(domain.ParseInt64Any(firstNonNil(totals["cached_input_tokens"], totals["cache_read_input_tokens"])))
			tOutput := nonNegative(domain.ParseInt64Any(totals["output_tokens"]))

			if previous == nil {
				lastUsage := mapValue(info["last_token_usage"])
				if len(lastUsage) > 0 {
					deltaInput = nonNegative(domain.ParseInt64Any(lastUsage["input_tokens"]))
					deltaCached = nonNegative(domain.ParseInt64Any(firstNonNil(lastUsage["cached_input_tokens"], lastUsage["cache_read_input_tokens"])))
					deltaOutput = nonNegative(domain.ParseInt64Any(lastUsage["output_tokens"]))
				}
			} else {
				deltaInput = nonNegative(tInput - previous.input)
				deltaCached = nonNegative(tCached - previous.cached)
				deltaOutput = nonNegative(tOutput - previous.output)
			}
			previous = &codexTotals{input: tInput, cached: tCached, output: tOutput}
		} else {
			lastUsage := mapValue(info["last_token_usage"])
			if len(lastUsage) > 0 {
				deltaInput = nonNegative(domain.ParseInt64Any(lastUsage["input_tokens"]))
				deltaCached = nonNegative(domain.ParseInt64Any(firstNonNil(lastUsage["cached_input_tokens"], lastUsage["cache_read_input_tokens"])))
				deltaOutput = nonNegative(domain.ParseInt64Any(lastUsage["output_tokens"]))
			}
		}

		if deltaInput == 0 && deltaCached == 0 && deltaOutput == 0 {
			continue
		}

		cachedClamped := deltaCached
		if cachedClamped > deltaInput {
			cachedClamped = deltaInput
		}

		bucket := ensureBucket(days, dayKey)
		bucket.tokens += deltaInput + deltaOutput
		if cost, ok := domain.CodexCostUSD(model, deltaInput, cachedClamped, deltaOutput); ok {
			bucket.costUSD += cost
			bucket.costSeen = true
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan codex log %s: %w", path, err)
	}
	return nil
}

func ensureBucket(days map[string]*dayBucket, dayKey string) *dayBucket {
	bucket, ok := days[dayKey]
	if ok {
		return bucket
	}
	bucket = &dayBucket{}
	days[dayKey] = bucket
	return bucket
}

func mapValue(value any) map[string]any {
	mapped, ok := value.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return mapped
}

func stringValue(value any) string {
	s, ok := value.(string)
	if !ok {
		return ""
	}
	return s
}

func nonNegative(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
