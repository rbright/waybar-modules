package app

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/rbright/waybar-sotto/internal/audio"
	"github.com/rbright/waybar-sotto/internal/config"
	"github.com/rbright/waybar-sotto/internal/selector"
	"github.com/rbright/waybar-sotto/internal/sotto"
	"github.com/rbright/waybar-sotto/internal/state"
	"github.com/rbright/waybar-sotto/internal/waybar"
)

func Run(ctx context.Context, args []string, cfg config.Runtime, stdout io.Writer) error {
	cmd, index, err := parseArgs(args)
	if err != nil {
		return err
	}

	switch cmd {
	case "status":
		out, err := buildStatus(ctx, cfg)
		if err != nil {
			return err
		}
		return writeOutput(stdout, out)
	case "refresh":
		_, err := buildStatus(ctx, cfg)
		return err
	case "select-item":
		return selectItem(ctx, cfg, index)
	case "select-input":
		return selectInput(ctx, cfg)
	default:
		return fmt.Errorf("unsupported command %q", cmd)
	}
}

func parseArgs(args []string) (command string, index int, err error) {
	if len(args) == 0 {
		return "status", 0, nil
	}

	switch strings.TrimSpace(args[0]) {
	case "status", "refresh", "select-input":
		if len(args) > 1 {
			return "", 0, fmt.Errorf("unexpected argument %q", args[1])
		}
		return strings.TrimSpace(args[0]), 0, nil
	case "select-item":
		if len(args) != 2 {
			return "", 0, fmt.Errorf("usage: waybar-sotto select-item <index>")
		}
		n, convErr := strconv.Atoi(strings.TrimSpace(args[1]))
		if convErr != nil || n < 1 {
			return "", 0, fmt.Errorf("invalid item index %q", args[1])
		}
		return "select-item", n, nil
	default:
		return "", 0, fmt.Errorf("usage: waybar-sotto <status|refresh|select-item N|select-input>")
	}
}

func buildStatus(ctx context.Context, cfg config.Runtime) (waybar.Output, error) {
	if err := state.EnsureDirs(cfg.StateDir, cfg.MenuDir); err != nil {
		return waybar.Output{}, err
	}

	devices, err := audio.ListDevices(ctx)
	if err != nil {
		statusLine := "Audio input discovery failed"
		if saveErr := state.SaveItems(cfg.ItemsPath, []state.Item{}); saveErr != nil {
			return waybar.Output{}, saveErr
		}
		if menuErr := state.WriteMenu(cfg.MenuPath, state.MenuData{StatusLine: statusLine}); menuErr != nil {
			return waybar.Output{}, menuErr
		}
		return waybar.Output{
			Text:    cfg.Icon,
			Tooltip: fmt.Sprintf("Sotto microphone: %s", err.Error()),
			Class:   "error",
		}, nil
	}

	activeDevices := audio.FilterActiveInputDevices(devices)

	selection, selectionErr := sotto.ReadAudioSelection(cfg.SottoConfigFile)
	if selectionErr != nil {
		selection = sotto.AudioSelection{Input: "default", Fallback: "default"}
	}
	configuredInput := selection.Input
	if strings.TrimSpace(configuredInput) == "" {
		configuredInput = "default"
	}

	matchedConfiguredDevice, matched := audio.MatchConfiguredDevice(devices, configuredInput)

	items := make([]state.Item, 0, minInt(len(activeDevices), cfg.MaxItems))
	truncatedCount := 0
	activeConfigured := false

	for _, device := range activeDevices {
		item := state.Item{
			ID:      strings.TrimSpace(device.ID),
			Label:   audio.DisplayName(device),
			Active:  matched && device.ID == matchedConfiguredDevice.ID,
			Muted:   device.Muted,
			Default: device.Default,
		}
		if item.Active {
			activeConfigured = true
		}

		if len(items) < cfg.MaxItems {
			items = append(items, item)
		} else {
			truncatedCount++
		}
	}

	if err := state.SaveItems(cfg.ItemsPath, items); err != nil {
		return waybar.Output{}, err
	}

	currentLine := buildCurrentLine(configuredInput, matched, matchedConfiguredDevice, activeConfigured)
	statusLine := buildStatusLine(len(activeDevices), truncatedCount)

	if err := state.WriteMenu(cfg.MenuPath, state.MenuData{
		CurrentLine:    currentLine,
		StatusLine:     statusLine,
		Items:          items,
		TruncatedCount: truncatedCount,
	}); err != nil {
		return waybar.Output{}, err
	}

	className := "normal"
	switch {
	case selectionErr != nil:
		className = "error"
	case len(activeDevices) == 0:
		className = "unknown"
	case !activeConfigured:
		className = "warning"
	}

	tooltip := buildTooltip(configuredInput, matched, matchedConfiguredDevice, activeConfigured, len(activeDevices), truncatedCount, selectionErr)

	return waybar.Output{
		Text:    cfg.Icon,
		Tooltip: tooltip,
		Class:   className,
	}, nil
}

func selectItem(ctx context.Context, cfg config.Runtime, index int) error {
	items, err := state.LoadItems(cfg.ItemsPath)
	if err != nil {
		return err
	}
	if len(items) == 0 || index > len(items) {
		return nil
	}

	item := items[index-1]
	if strings.TrimSpace(item.ID) == "" {
		return nil
	}

	if err := sotto.SetAudioInput(cfg.SottoConfigFile, item.ID); err != nil {
		return err
	}

	_, err = buildStatus(ctx, cfg)
	return err
}

func selectInput(ctx context.Context, cfg config.Runtime) error {
	devices, err := audio.ListDevices(ctx)
	if err != nil {
		return err
	}

	activeDevices := audio.FilterActiveInputDevices(devices)
	if len(activeDevices) == 0 {
		return nil
	}

	selection, selectionErr := sotto.ReadAudioSelection(cfg.SottoConfigFile)
	if selectionErr != nil {
		selection = sotto.AudioSelection{Input: "default", Fallback: "default"}
	}
	configuredInput := strings.TrimSpace(selection.Input)
	if configuredInput == "" {
		configuredInput = "default"
	}

	preselectedID := ""
	if matchedDevice, matched := audio.MatchConfiguredDevice(devices, configuredInput); matched {
		preselectedID = strings.TrimSpace(matchedDevice.ID)
	}

	selectedID, err := selector.SelectInput(ctx, activeDevices, preselectedID)
	if err != nil {
		if err == selector.ErrSelectionCancelled {
			return nil
		}
		return err
	}
	if strings.TrimSpace(selectedID) == "" {
		return nil
	}

	if err := sotto.SetAudioInput(cfg.SottoConfigFile, selectedID); err != nil {
		return err
	}

	_, err = buildStatus(ctx, cfg)
	return err
}

func writeOutput(w io.Writer, output waybar.Output) error {
	payload, err := waybar.Encode(output)
	if err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		return fmt.Errorf("write trailing newline: %w", err)
	}
	return nil
}

func buildCurrentLine(configuredInput string, matched bool, matchedDevice audio.Device, activeConfigured bool) string {
	if activeConfigured {
		return fmt.Sprintf("Current input: %s", audio.DisplayName(matchedDevice))
	}
	if matched {
		return fmt.Sprintf("Current input: %s (inactive)", audio.DisplayName(matchedDevice))
	}
	return fmt.Sprintf("Current input: %s (not found)", configuredInput)
}

func buildStatusLine(activeCount int, truncatedCount int) string {
	if activeCount == 0 {
		return "No active microphone inputs"
	}
	if truncatedCount > 0 {
		return fmt.Sprintf("%d active microphones (%d hidden)", activeCount, truncatedCount)
	}
	if activeCount == 1 {
		return "1 active microphone"
	}
	return fmt.Sprintf("%d active microphones", activeCount)
}

func buildTooltip(
	configuredInput string,
	matched bool,
	matchedDevice audio.Device,
	activeConfigured bool,
	activeCount int,
	truncatedCount int,
	selectionErr error,
) string {
	lines := []string{"Sotto microphone input"}
	lines = append(lines, fmt.Sprintf("Configured input: %s", configuredInput))

	switch {
	case activeConfigured:
		lines = append(lines, fmt.Sprintf("Selected active input: %s", audio.DisplayName(matchedDevice)))
	case matched:
		lines = append(lines, fmt.Sprintf("Selected input inactive: %s", audio.DisplayName(matchedDevice)))
	default:
		lines = append(lines, "Selected input not found")
	}

	if activeCount == 1 {
		lines = append(lines, "Active inputs: 1")
	} else {
		lines = append(lines, fmt.Sprintf("Active inputs: %d", activeCount))
	}

	if truncatedCount > 0 {
		lines = append(lines, fmt.Sprintf("Hidden inputs: %d (MAX_ITEMS limit)", truncatedCount))
	}

	if selectionErr != nil {
		lines = append(lines, fmt.Sprintf("Config error: %s", selectionErr.Error()))
	}

	lines = append(lines, "Click to choose input")
	return strings.Join(lines, "\n")
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
