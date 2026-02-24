package app

import "testing"

func TestParseArgsDefaultsToStatus(t *testing.T) {
	cmd, index, err := parseArgs(nil)
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if cmd != "status" {
		t.Fatalf("expected status command, got %q", cmd)
	}
	if index != 0 {
		t.Fatalf("expected zero index, got %d", index)
	}
}

func TestParseArgsSelectItem(t *testing.T) {
	cmd, index, err := parseArgs([]string{"select-item", "3"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if cmd != "select-item" {
		t.Fatalf("expected select-item command, got %q", cmd)
	}
	if index != 3 {
		t.Fatalf("expected index 3, got %d", index)
	}
}

func TestParseArgsSelectInput(t *testing.T) {
	cmd, index, err := parseArgs([]string{"select-input"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if cmd != "select-input" {
		t.Fatalf("expected select-input command, got %q", cmd)
	}
	if index != 0 {
		t.Fatalf("expected zero index, got %d", index)
	}
}

func TestParseArgsRejectsInvalidIndex(t *testing.T) {
	_, _, err := parseArgs([]string{"select-item", "0"})
	if err == nil {
		t.Fatal("expected error for invalid index")
	}
}
