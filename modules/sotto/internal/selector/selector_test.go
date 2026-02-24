package selector

import (
	"strings"
	"testing"

	"github.com/rbright/waybar-sotto/internal/audio"
)

func TestBuildMenuOptions(t *testing.T) {
	devices := []audio.Device{
		{ID: "alsa_input.wave3", Description: "Elgato Wave 3", Default: true},
		{ID: "bluez_input.headset", Description: "Sony WH-1000XM6", Muted: true},
	}

	options := buildMenuOptions(devices, "alsa_input.wave3")
	if len(options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(options))
	}

	if options[0].ID != "alsa_input.wave3" {
		t.Fatalf("unexpected first option ID: %q", options[0].ID)
	}
	if !strings.HasPrefix(options[0].Line, "01") {
		t.Fatalf("expected indexed option line, got %q", options[0].Line)
	}
	if !strings.Contains(options[0].Line, "selected") {
		t.Fatalf("expected selected marker in first option line, got %q", options[0].Line)
	}
	if !strings.Contains(options[1].Line, "muted") {
		t.Fatalf("expected muted marker in second option line, got %q", options[1].Line)
	}
}

func TestResolveSelectionByIndexPrefix(t *testing.T) {
	options := []menuOption{
		{Index: 1, ID: "input-a", Line: "01  Input A"},
		{Index: 2, ID: "input-b", Line: "02  Input B"},
	}

	id, ok := resolveSelection("02  Input B", options)
	if !ok {
		t.Fatal("expected to resolve selection")
	}
	if id != "input-b" {
		t.Fatalf("expected input-b, got %q", id)
	}
}

func TestResolveSelectionRejectsUnknownValue(t *testing.T) {
	options := []menuOption{{Index: 1, ID: "input-a", Line: "01  Input A"}}

	if id, ok := resolveSelection("99  Missing", options); ok {
		t.Fatalf("expected unresolved selection, got id %q", id)
	}
}
