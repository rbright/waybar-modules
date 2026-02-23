package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func readJSONMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("missing file: %s", path)
		}
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode json %s: %w", path, err)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, nil
}

func writeJSONMapAtomic(path string, payload map[string]any, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", path, err)
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json %s: %w", path, err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), "tmp-*.json")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file for %s: %w", path, err)
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("chmod temp file for %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace file %s: %w", path, err)
	}
	return nil
}

func mapAt(input map[string]any, key string) map[string]any {
	value, ok := input[key]
	if !ok {
		return map[string]any{}
	}
	mapped, ok := value.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return mapped
}

func mapAtCreate(input map[string]any, key string) map[string]any {
	if existing, ok := input[key].(map[string]any); ok {
		return existing
	}
	mapped := map[string]any{}
	input[key] = mapped
	return mapped
}

func stringAt(input map[string]any, key string) string {
	return stringValue(input[key])
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
