package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_ConfigFileOverrides(t *testing.T) {
	tmp := t.TempDir()
	xdgConfig := filepath.Join(tmp, "config")
	xdgState := filepath.Join(tmp, "state")
	if err := os.MkdirAll(filepath.Join(xdgConfig, "waybar"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}

	configFile := filepath.Join(xdgConfig, "waybar", "schedule.env")
	content := "LOOKAHEAD_MINUTES=30\nMAX_ITEMS=99\nQUERY_AHEAD_DAYS=3\nINCLUDE_ALL_DAY=true\n"
	if err := os.WriteFile(configFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", xdgConfig)
	t.Setenv("XDG_STATE_HOME", xdgState)
	t.Setenv("HOME", tmp)
	t.Setenv("WAYBAR_SCHEDULE_CONFIG_FILE", configFile)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Lookahead != 30*time.Minute {
		t.Fatalf("lookahead mismatch: %v", cfg.Lookahead)
	}
	if cfg.MaxItems != maxActionItems {
		t.Fatalf("max items mismatch: %d", cfg.MaxItems)
	}
	if cfg.QueryAhead != 72*time.Hour {
		t.Fatalf("query ahead mismatch: %v", cfg.QueryAhead)
	}
	if !cfg.IncludeAllDay {
		t.Fatalf("expected include all day true")
	}

	expectedSelection := filepath.Join(xdgConfig, "waybar", "schedule-selected-calendars.json")
	if cfg.SelectionPath != expectedSelection {
		t.Fatalf("selection path mismatch: %s", cfg.SelectionPath)
	}
}
