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

const (
	fuzzelWidthChars       = 42
	fuzzelEstimatedWidthPx = 420
	fuzzelLineHeightPx     = 22
)

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
	Output        string
	LocalX        int
	LocalY        int
	MonitorWidth  int
	MonitorHeight int
}

type fuzzelPlacement struct {
	Anchor  string
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
	pointer, err := detectPointerPlacement(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to anchor selector near pointer: %w", err)
	}

	estimatedHeightPx := linesCount*fuzzelLineHeightPx + 64
	placement := computeFuzzelPlacement(pointer, fuzzelEstimatedWidthPx, estimatedHeightPx)

	args := []string{
		"--dmenu",
		"--layer", "overlay",
		"--output", pointer.Output,
		"--anchor", placement.Anchor,
		"--x-margin", strconv.Itoa(placement.XMargin),
		"--y-margin", strconv.Itoa(placement.YMargin),
		"--prompt", "Mic> ",
		"--lines", strconv.Itoa(linesCount),
		"--width", strconv.Itoa(fuzzelWidthChars),
		"--font", "IBM Plex Sans:size=12",
		"--line-height", "20",
		"--horizontal-pad", "12",
		"--vertical-pad", "8",
		"--inner-pad", "4",
		"--background-color", "#1e1e2eff",
		"--text-color", "#cdd6f4ff",
		"--prompt-color", "#89b4faff",
		"--input-color", "#cdd6f4ff",
		"--match-color", "#89b4faff",
		"--selection-color", "#313244ff",
		"--selection-text-color", "#cdd6f4ff",
		"--selection-match-color", "#89b4faff",
		"--border-width", "2",
		"--border-color", "#89b4faff",
		"--border-radius", "8",
	}

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
			Output:        monitor.Name,
			LocalX:        localX,
			LocalY:        localY,
			MonitorWidth:  monitor.Width,
			MonitorHeight: monitor.Height,
		}
		return placement, true
	}

	return pointerPlacement{}, false
}

func computeFuzzelPlacement(pointer pointerPlacement, estimatedWidthPx, estimatedHeightPx int) fuzzelPlacement {
	horizontal := "left"
	spaceRight := pointer.MonitorWidth - pointer.LocalX
	if spaceRight < estimatedWidthPx && pointer.LocalX > pointer.MonitorWidth/2 {
		horizontal = "right"
	}

	vertical := "top"
	spaceBottom := pointer.MonitorHeight - pointer.LocalY
	if spaceBottom < estimatedHeightPx && pointer.LocalY > pointer.MonitorHeight/2 {
		vertical = "bottom"
	}

	anchor := fmt.Sprintf("%s-%s", vertical, horizontal)

	xMargin := 0
	if horizontal == "left" {
		xMargin = clampInt(pointer.LocalX-18, 0, pointer.MonitorWidth-8)
	} else {
		xMargin = clampInt(pointer.MonitorWidth-pointer.LocalX-18, 0, pointer.MonitorWidth-8)
	}

	yMargin := 0
	if vertical == "top" {
		yMargin = clampInt(pointer.LocalY+6, 0, pointer.MonitorHeight-8)
	} else {
		yMargin = clampInt(pointer.MonitorHeight-pointer.LocalY+6, 0, pointer.MonitorHeight-8)
	}

	return fuzzelPlacement{
		Anchor:  anchor,
		XMargin: xMargin,
		YMargin: yMargin,
	}
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
