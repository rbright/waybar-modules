package selector

import (
	"testing"

	"github.com/rbright/waybar-sotto/internal/audio"
)

func TestParseSelectionOutput(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "", want: ""},
		{name: "newline separated", raw: "alsa_input.wave3\n", want: "alsa_input.wave3"},
		{name: "pipe separated", raw: "alsa_input.wave3|ignored", want: "alsa_input.wave3"},
		{name: "trimmed", raw: "  alsa_input.wave3  ", want: "alsa_input.wave3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseSelectionOutput(tt.raw); got != tt.want {
				t.Fatalf("parseSelectionOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildStatusLabel(t *testing.T) {
	tests := []struct {
		name   string
		device audio.Device
		want   string
	}{
		{name: "active", device: audio.Device{}, want: "active"},
		{name: "default", device: audio.Device{Default: true}, want: "default"},
		{name: "muted", device: audio.Device{Muted: true}, want: "muted"},
		{name: "default and muted", device: audio.Device{Default: true, Muted: true}, want: "default, muted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildStatusLabel(tt.device); got != tt.want {
				t.Fatalf("buildStatusLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}
