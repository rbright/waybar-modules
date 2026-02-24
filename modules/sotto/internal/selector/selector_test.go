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

func TestPlacementFromCursor(t *testing.T) {
	cursor := cursorPosition{X: 1502, Y: 20}
	monitors := []monitorGeometry{
		{Name: "DP-5", X: 0, Y: 0, Width: 3840, Height: 2160},
	}

	placement, ok := placementFromCursor(cursor, monitors)
	if !ok {
		t.Fatal("expected cursor placement to resolve")
	}
	if placement.Output != "DP-5" {
		t.Fatalf("expected output DP-5, got %q", placement.Output)
	}
	if placement.XMargin != 1478 {
		t.Fatalf("expected x margin 1478, got %d", placement.XMargin)
	}
	if placement.YMargin != 28 {
		t.Fatalf("expected y margin 28, got %d", placement.YMargin)
	}
}

func TestPlacementFromCursorOutsideKnownMonitors(t *testing.T) {
	cursor := cursorPosition{X: 99999, Y: 99999}
	monitors := []monitorGeometry{{Name: "DP-5", X: 0, Y: 0, Width: 3840, Height: 2160}}

	if _, ok := placementFromCursor(cursor, monitors); ok {
		t.Fatal("expected placement lookup to fail for cursor outside monitors")
	}
}
