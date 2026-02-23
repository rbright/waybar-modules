package selector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/rbright/waybar-schedule/internal/schedule"
)

var ErrSelectionCancelled = errors.New("calendar selection cancelled")

func SelectCalendars(ctx context.Context, calendars []schedule.Calendar, currentSelected map[string]bool) ([]string, error) {
	if len(calendars) == 0 {
		return nil, fmt.Errorf("no calendars available")
	}

	if !hasGraphicalSession() {
		return nil, fmt.Errorf("calendar selection requires a graphical session")
	}

	if _, err := exec.LookPath("zenity"); err != nil {
		return nil, fmt.Errorf("zenity is required for calendar selection")
	}

	return selectWithZenity(ctx, calendars, currentSelected)
}

func hasGraphicalSession() bool {
	return strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != "" || strings.TrimSpace(os.Getenv("DISPLAY")) != ""
}

func selectWithZenity(ctx context.Context, calendars []schedule.Calendar, currentSelected map[string]bool) ([]string, error) {
	args := []string{
		"--list",
		"--checklist",
		"--title=Waybar Schedule Calendars",
		"--text=Select calendars to include in the schedule module",
		"--modal",
		"--width=960",
		"--height=680",
		"--separator=\n",
		"--print-column=4",
		"--column=Use",
		"--column=Calendar",
		"--column=Account",
		"--column=UID",
		"--hide-column=4",
	}

	for _, calendar := range calendars {
		checked := "FALSE"
		if currentSelected[calendar.UID] {
			checked = "TRUE"
		}

		account := strings.TrimSpace(calendar.AccountName)
		if account == "" {
			account = "-"
		}

		args = append(args,
			checked,
			calendarLabel(calendar),
			account,
			calendar.UID,
		)
	}

	cmd := exec.CommandContext(ctx, "zenity", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return nil, ErrSelectionCancelled
			}
		}
		return nil, fmt.Errorf("zenity selector failed: %w", err)
	}

	selected := parseSelectionOutput(string(out))
	return normalizeUIDs(selected), nil
}

func parseSelectionOutput(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '\n' || r == '|'
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}

func calendarLabel(calendar schedule.Calendar) string {
	name := strings.TrimSpace(calendar.Name)
	if name == "" {
		name = calendar.UID
	}
	if strings.TrimSpace(calendar.AccountName) != "" {
		return fmt.Sprintf("%s (%s)", name, calendar.AccountName)
	}
	return name
}

func normalizeUIDs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	sort.Strings(result)
	return result
}
