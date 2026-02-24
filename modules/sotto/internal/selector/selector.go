package selector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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

type cursorPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type monitorGeometry struct {
	Name   string `json:"name"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type pointerPlacement struct {
	Output  string
	XMargin int
	YMargin int
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

	if _, err := exec.LookPath("fuzzel"); err != nil {
		return "", fmt.Errorf("fuzzel is required for anchored input selection")
	}

	fuzzelArgs, err := buildFuzzelArgs(ctx, linesCount)
	if err != nil {
		return "", err
	}

	return runMenuCommand(ctx, "fuzzel", fuzzelArgs, input)
}

func buildFuzzelArgs(ctx context.Context, linesCount int) ([]string, error) {
	args := []string{
		"--dmenu",
		"--prompt", "Mic> ",
		"--lines", strconv.Itoa(linesCount),
		"--layer", "overlay",
	}

	placement, err := detectPointerPlacement(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to anchor selector near pointer: %w", err)
	}

	args = append(args,
		"--output", placement.Output,
		"--anchor", "top-left",
		"--x-margin", strconv.Itoa(placement.XMargin),
		"--y-margin", strconv.Itoa(placement.YMargin),
	)

	return args, nil
}

func detectPointerPlacement(ctx context.Context) (pointerPlacement, error) {
	if _, err := exec.LookPath("hyprctl"); err != nil {
		return pointerPlacement{}, fmt.Errorf("hyprctl not available")
	}

	cursorRaw, err := exec.CommandContext(ctx, "hyprctl", "-j", "cursorpos").Output()
	if err != nil {
		return pointerPlacement{}, fmt.Errorf("read cursor position: %w", err)
	}

	var cursor cursorPosition
	if err := json.Unmarshal(cursorRaw, &cursor); err != nil {
		return pointerPlacement{}, fmt.Errorf("decode cursor position: %w", err)
	}

	monitorsRaw, err := exec.CommandContext(ctx, "hyprctl", "-j", "monitors").Output()
	if err != nil {
		return pointerPlacement{}, fmt.Errorf("read monitors: %w", err)
	}

	var monitors []monitorGeometry
	if err := json.Unmarshal(monitorsRaw, &monitors); err != nil {
		return pointerPlacement{}, fmt.Errorf("decode monitors: %w", err)
	}

	placement, ok := placementFromCursor(cursor, monitors)
	if !ok {
		return pointerPlacement{}, fmt.Errorf("cursor monitor not found")
	}

	return placement, nil
}

func placementFromCursor(cursor cursorPosition, monitors []monitorGeometry) (pointerPlacement, bool) {
	cursorX := int(math.Round(cursor.X))
	cursorY := int(math.Round(cursor.Y))

	for _, monitor := range monitors {
		if strings.TrimSpace(monitor.Name) == "" {
			continue
		}
		if monitor.Width <= 0 || monitor.Height <= 0 {
			continue
		}

		inside := cursorX >= monitor.X && cursorX < monitor.X+monitor.Width &&
			cursorY >= monitor.Y && cursorY < monitor.Y+monitor.Height
		if !inside {
			continue
		}

		localX := cursorX - monitor.X
		localY := cursorY - monitor.Y

		placement := pointerPlacement{
			Output:  monitor.Name,
			XMargin: clampInt(localX-24, 0, monitor.Width-20),
			YMargin: clampInt(localY+8, 0, monitor.Height-20),
		}
		return placement, true
	}

	return pointerPlacement{}, false
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

func clampInt(value, minValue, maxValue int) int {
	if maxValue < minValue {
		maxValue = minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
