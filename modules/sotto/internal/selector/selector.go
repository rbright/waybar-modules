package selector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rbright/waybar-sotto/internal/audio"
)

var ErrSelectionCancelled = errors.New("input selection cancelled")

func SelectInput(ctx context.Context, devices []audio.Device, currentID string) (string, error) {
	if len(devices) == 0 {
		return "", fmt.Errorf("no active microphone inputs available")
	}

	if !hasGraphicalSession() {
		return "", fmt.Errorf("input selection requires a graphical session")
	}

	if _, err := exec.LookPath("zenity"); err != nil {
		return "", fmt.Errorf("zenity is required for input selection")
	}

	return selectWithZenity(ctx, devices, currentID)
}

func hasGraphicalSession() bool {
	return strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != "" || strings.TrimSpace(os.Getenv("DISPLAY")) != ""
}

func selectWithZenity(ctx context.Context, devices []audio.Device, currentID string) (string, error) {
	selectedID := strings.TrimSpace(currentID)

	args := []string{
		"--list",
		"--radiolist",
		"--title=Waybar Sotto Input",
		"--text=Select the active microphone input",
		"--modal",
		"--width=960",
		"--height=680",
		"--separator=\n",
		"--print-column=4",
		"--column=Use",
		"--column=Input",
		"--column=Status",
		"--column=ID",
		"--hide-column=4",
	}

	for _, device := range devices {
		checked := "FALSE"
		if strings.TrimSpace(device.ID) == selectedID && selectedID != "" {
			checked = "TRUE"
		}

		args = append(args,
			checked,
			audio.DisplayName(device),
			buildStatusLabel(device),
			strings.TrimSpace(device.ID),
		)
	}

	cmd := exec.CommandContext(ctx, "zenity", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return "", ErrSelectionCancelled
			}
		}
		return "", fmt.Errorf("zenity selector failed: %w", err)
	}

	selection := parseSelectionOutput(string(out))
	if selection == "" {
		return "", ErrSelectionCancelled
	}
	return selection, nil
}

func buildStatusLabel(device audio.Device) string {
	parts := make([]string, 0, 2)
	if device.Default {
		parts = append(parts, "default")
	}
	if device.Muted {
		parts = append(parts, "muted")
	}

	if len(parts) == 0 {
		return "active"
	}
	return strings.Join(parts, ", ")
}

func parseSelectionOutput(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '\n' || r == '|'
	})
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			return value
		}
	}

	return ""
}
