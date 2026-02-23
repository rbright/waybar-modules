package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rbright/waybar-schedule/internal/schedule"
)

type Selection struct {
	SelectedUIDs []string `json:"selectedUIDs"`
	UpdatedAt    string   `json:"updatedAt,omitempty"`

	Exists bool `json:"-"`
}

func EnsureDirs(stateDir, menuDir, selectionPath string) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	if err := os.MkdirAll(menuDir, 0o755); err != nil {
		return fmt.Errorf("create menu dir: %w", err)
	}
	if strings.TrimSpace(selectionPath) != "" {
		if err := os.MkdirAll(filepath.Dir(selectionPath), 0o755); err != nil {
			return fmt.Errorf("create selection dir: %w", err)
		}
	}
	return nil
}

func SaveMeetings(path string, meetings []schedule.Occurrence) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create meetings dir: %w", err)
	}

	payload, err := json.MarshalIndent(meetings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meetings: %w", err)
	}

	return writeFileAtomically(path, append(payload, '\n'))
}

func LoadMeetings(path string) ([]schedule.Occurrence, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read meetings file: %w", err)
	}

	var meetings []schedule.Occurrence
	if err := json.Unmarshal(raw, &meetings); err != nil {
		return nil, fmt.Errorf("decode meetings file: %w", err)
	}
	return meetings, nil
}

func SaveCalendars(path string, calendars []schedule.Calendar) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create calendars dir: %w", err)
	}

	payload, err := json.MarshalIndent(calendars, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal calendars: %w", err)
	}

	return writeFileAtomically(path, append(payload, '\n'))
}

func SaveSelection(path string, selectedUIDs []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create selection dir: %w", err)
	}

	normalized := normalizeUIDs(selectedUIDs)
	selection := Selection{
		SelectedUIDs: normalized,
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
		Exists:       true,
	}

	payload, err := json.MarshalIndent(selection, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal selection: %w", err)
	}

	return writeFileAtomically(path, append(payload, '\n'))
}

func LoadSelection(path string) (Selection, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Selection{Exists: false}, nil
		}
		return Selection{}, fmt.Errorf("read selection file: %w", err)
	}

	if len(strings.TrimSpace(string(raw))) == 0 {
		return Selection{Exists: true}, nil
	}

	var selection Selection
	if err := json.Unmarshal(raw, &selection); err != nil {
		return Selection{}, fmt.Errorf("decode selection file: %w", err)
	}
	selection.Exists = true
	selection.SelectedUIDs = normalizeUIDs(selection.SelectedUIDs)
	return selection, nil
}

func normalizeUIDs(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}

	sort.Strings(normalized)
	return normalized
}

func writeFileAtomically(path string, content []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
