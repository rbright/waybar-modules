package selector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/rbright/waybar-sotto/internal/audio"
)

var ErrSelectionCancelled = errors.New("input selection cancelled")

type menuOption struct {
	Index int
	ID    string
	Line  string
}

func SelectInput(ctx context.Context, devices []audio.Device, currentID string) (string, error) {
	if len(devices) == 0 {
		return "", fmt.Errorf("no active microphone inputs available")
	}

	if !hasGraphicalSession() {
		return "", fmt.Errorf("input selection requires a graphical session")
	}

	options := buildMenuOptions(devices, currentID)
	if len(options) == 0 {
		return "", fmt.Errorf("no selectable microphone inputs available")
	}

	selection, err := runSelector(ctx, options)
	if err != nil {
		return "", err
	}

	selectedID, ok := resolveSelection(selection, options)
	if !ok {
		return "", fmt.Errorf("unable to resolve selected input %q", selection)
	}

	return selectedID, nil
}

func hasGraphicalSession() bool {
	return strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != "" || strings.TrimSpace(os.Getenv("DISPLAY")) != ""
}

func buildMenuOptions(devices []audio.Device, currentID string) []menuOption {
	normalizedCurrent := strings.TrimSpace(currentID)
	options := make([]menuOption, 0, len(devices))
	labelCounts := map[string]int{}

	idx := 1
	for _, device := range devices {
		id := strings.TrimSpace(device.ID)
		if id == "" {
			continue
		}

		label := audio.DisplayName(device)
		if strings.TrimSpace(label) == "" {
			label = id
		}

		status := make([]string, 0, 3)
		if normalizedCurrent != "" && id == normalizedCurrent {
			status = append(status, "selected")
		}
		if device.Default {
			status = append(status, "default")
		}
		if device.Muted {
			status = append(status, "muted")
		}
		if len(status) > 0 {
			label = fmt.Sprintf("%s (%s)", label, strings.Join(status, ", "))
		}

		labelCounts[label] = labelCounts[label] + 1
		if labelCounts[label] > 1 {
			label = fmt.Sprintf("%s — %s", label, id)
		}

		options = append(options, menuOption{
			Index: idx,
			ID:    id,
			Line:  fmt.Sprintf("%02d  %s", idx, label),
		})
		idx++
	}

	return options
}

func runSelector(ctx context.Context, options []menuOption) (string, error) {
	lines := make([]string, 0, len(options))
	for _, option := range options {
		lines = append(lines, option.Line)
	}
	input := strings.Join(lines, "\n") + "\n"

	linesCount := minInt(len(lines), 10)
	if linesCount < 1 {
		linesCount = 1
	}

	candidates := []struct {
		command string
		args    []string
	}{
		{command: "fuzzel", args: []string{"--dmenu", "--prompt", "Mic> ", "--lines", strconv.Itoa(linesCount)}},
		{command: "wofi", args: []string{"--dmenu", "--prompt", "Mic", "--lines", strconv.Itoa(linesCount)}},
		{command: "bemenu", args: []string{"-p", "Mic>"}},
		{command: "rofi", args: []string{"-dmenu", "-i", "-p", "Mic", "-lines", strconv.Itoa(linesCount)}},
	}

	var lastErr error
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate.command); err != nil {
			continue
		}

		selection, err := runMenuCommand(ctx, candidate.command, candidate.args, input)
		if err != nil {
			if errors.Is(err, ErrSelectionCancelled) {
				return "", err
			}
			lastErr = err
			continue
		}

		return selection, nil
	}

	if lastErr != nil {
		return "", lastErr
	}

	return "", fmt.Errorf("no supported selector found (install fuzzel, wofi, bemenu, or rofi)")
}

func runMenuCommand(ctx context.Context, command string, args []string, input string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = strings.NewReader(input)

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return "", ErrSelectionCancelled
			}
		}
		return "", fmt.Errorf("%s selector failed: %w", command, err)
	}

	selection := strings.TrimSpace(string(out))
	if selection == "" {
		return "", ErrSelectionCancelled
	}

	return selection, nil
}

func resolveSelection(selection string, options []menuOption) (string, bool) {
	trimmed := strings.TrimSpace(selection)
	if trimmed == "" {
		return "", false
	}

	fields := strings.Fields(trimmed)
	if len(fields) > 0 {
		if idx, err := strconv.Atoi(fields[0]); err == nil {
			for _, option := range options {
				if option.Index == idx {
					return option.ID, true
				}
			}
		}
	}

	for _, option := range options {
		if trimmed == strings.TrimSpace(option.Line) {
			return option.ID, true
		}
	}

	return "", false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
