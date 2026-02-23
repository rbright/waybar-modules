package sotto

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAudioSelectionDefaultsWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.jsonc")

	selection, err := ReadAudioSelection(path)
	if err != nil {
		t.Fatalf("ReadAudioSelection returned error: %v", err)
	}
	if selection.Input != "default" {
		t.Fatalf("expected default input, got %q", selection.Input)
	}
	if selection.Fallback != "default" {
		t.Fatalf("expected default fallback, got %q", selection.Fallback)
	}
}

func TestReadAudioSelectionParsesJSONC(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.jsonc")
	content := `{
  // audio selection
  "audio": {
    "input": "Elgato Wave 3 Mono",
    "fallback": "default",
  },
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	selection, err := ReadAudioSelection(path)
	if err != nil {
		t.Fatalf("ReadAudioSelection returned error: %v", err)
	}
	if selection.Input != "Elgato Wave 3 Mono" {
		t.Fatalf("unexpected input %q", selection.Input)
	}
}

func TestSetAudioInputWritesUpdatedConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.jsonc")
	content := `{
  "audio": {
    "input": "Old Mic",
    "fallback": "default"
  },
  "paste": {
    "enable": true
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := SetAudioInput(path, "alsa_input.wave3"); err != nil {
		t.Fatalf("SetAudioInput returned error: %v", err)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated config: %v", err)
	}
	text := string(updated)
	if !strings.Contains(text, "\"input\": \"alsa_input.wave3\"") {
		t.Fatalf("expected updated input in config, got: %s", text)
	}
	if !strings.Contains(text, "\"paste\": {") {
		t.Fatalf("expected existing keys retained in config, got: %s", text)
	}
}
