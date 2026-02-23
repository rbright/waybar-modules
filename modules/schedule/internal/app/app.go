package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rbright/waybar-schedule/internal/config"
	"github.com/rbright/waybar-schedule/internal/eds"
	"github.com/rbright/waybar-schedule/internal/schedule"
	"github.com/rbright/waybar-schedule/internal/selector"
	"github.com/rbright/waybar-schedule/internal/state"
	"github.com/rbright/waybar-schedule/internal/waybar"
)

func Run(ctx context.Context, args []string, cfg config.Runtime, stdout io.Writer) error {
	cmd, index, err := parseArgs(args)
	if err != nil {
		return err
	}

	switch cmd {
	case "status":
		out, statusErr := buildStatus(ctx, cfg)
		if statusErr != nil {
			return statusErr
		}
		return writeOutput(stdout, out)
	case "refresh":
		_, statusErr := buildStatus(ctx, cfg)
		return statusErr
	case "join-next":
		return joinNext(ctx, cfg)
	case "join-item":
		return joinItem(ctx, cfg, index)
	case "select-calendars":
		return selectCalendars(ctx, cfg, stdout)
	default:
		return fmt.Errorf("unsupported command %q", cmd)
	}
}

func parseArgs(args []string) (command string, index int, err error) {
	if len(args) == 0 {
		return "status", 0, nil
	}

	switch strings.TrimSpace(args[0]) {
	case "status", "refresh", "join-next", "select-calendars":
		if len(args) > 1 {
			return "", 0, fmt.Errorf("unexpected argument %q", args[1])
		}
		return strings.TrimSpace(args[0]), 0, nil
	case "join-item":
		if len(args) != 2 {
			return "", 0, fmt.Errorf("usage: waybar-schedule join-item <index>")
		}
		n, convErr := strconv.Atoi(strings.TrimSpace(args[1]))
		if convErr != nil || n < 1 {
			return "", 0, fmt.Errorf("invalid item index %q", args[1])
		}
		return "join-item", n, nil
	default:
		return "", 0, fmt.Errorf("usage: waybar-schedule <status|refresh|join-next|join-item N|select-calendars>")
	}
}

func buildStatus(ctx context.Context, cfg config.Runtime) (waybar.Output, error) {
	if err := state.EnsureDirs(cfg.StateDir, cfg.MenuDir, cfg.SelectionPath); err != nil {
		return waybar.Output{}, err
	}

	client, err := eds.New(ctx)
	if err != nil {
		return renderUnknownState(cfg, "EDS is not available")
	}
	defer func() {
		_ = client.Close()
	}()

	calendars, err := client.ListCalendars(ctx)
	if err != nil {
		return renderErrorState(cfg, fmt.Sprintf("Failed to list calendars: %s", err.Error()))
	}

	if err := state.SaveCalendars(cfg.CalendarsPath, calendars); err != nil {
		return waybar.Output{}, err
	}

	selection, err := state.LoadSelection(cfg.SelectionPath)
	if err != nil {
		return waybar.Output{}, err
	}

	selectedUIDs := resolveSelectedUIDs(calendars, selection)
	selectedCalendars := filterCalendarsByUID(calendars, selectedUIDs)
	if len(selectedCalendars) == 0 {
		statusLine := "No calendars selected"
		if err := state.SaveMeetings(cfg.ItemsPath, []schedule.Occurrence{}); err != nil {
			return waybar.Output{}, err
		}
		if err := state.WriteMenu(cfg.MenuPath, state.MenuData{StatusLine: statusLine}); err != nil {
			return waybar.Output{}, err
		}
		return waybar.Output{
			Text:    "∅",
			Tooltip: statusLine + "\nRun select-calendars to choose calendars",
			Class:   "unknown",
		}, nil
	}

	now := time.Now()
	windowStart := now.Add(-cfg.QueryLookback)
	windowEnd := now.Add(cfg.QueryAhead)

	rawEvents, err := client.FetchRawEvents(ctx, selectedCalendars, windowStart, windowEnd)
	if err != nil {
		return renderErrorState(cfg, fmt.Sprintf("Calendar query failed: %s", err.Error()))
	}

	occurrences := schedule.ExpandEvents(rawEvents, windowStart, windowEnd)
	meetingOccurrences := schedule.MeetingOnly(occurrences)
	upcoming := schedule.Upcoming(meetingOccurrences, now, cfg.Lookahead, cfg.MaxItems, cfg.IncludeAllDay)
	if err := state.SaveMeetings(cfg.ItemsPath, upcoming); err != nil {
		return waybar.Output{}, err
	}

	hasNext := len(upcoming) > 0
	var next schedule.Occurrence
	if hasNext {
		next = upcoming[0]
	}

	statusLine := fmt.Sprintf("No meeting link in next %d minutes", int(cfg.Lookahead.Minutes()))
	menuData := state.MenuData{StatusLine: statusLine, Items: upcoming}
	if hasNext {
		menuData.Next = &next
		statusLine = "Upcoming meeting"
		menuData.StatusLine = statusLine
	}

	if err := state.WriteMenu(cfg.MenuPath, menuData); err != nil {
		return waybar.Output{}, err
	}

	tooltip := buildTooltip(now, cfg.Lookahead, hasNext, next, upcoming)
	if hasNext {
		return waybar.Output{
			Text:    schedule.CountdownText(now, next),
			Tooltip: tooltip,
			Class:   "normal",
		}, nil
	}

	return waybar.Output{
		Text:    "—",
		Tooltip: tooltip,
		Class:   "clear",
	}, nil
}

func buildTooltip(now time.Time, lookahead time.Duration, hasNext bool, next schedule.Occurrence, upcoming []schedule.Occurrence) string {
	var b strings.Builder

	if hasNext {
		if next.Start.After(now) {
			_, _ = fmt.Fprintf(&b, "Next in %s: %s\n", schedule.HumanizeDuration(next.Start.Sub(now)), next.Title)
		} else {
			_, _ = fmt.Fprintf(&b, "In progress: %s\n", next.Title)
		}

		_, _ = fmt.Fprintf(&b, "Starts: %s\n", next.Start.Format("Mon 15:04"))
		if strings.TrimSpace(next.CalendarName) != "" {
			_, _ = fmt.Fprintf(&b, "Calendar: %s\n", next.CalendarName)
		}
		if strings.TrimSpace(next.Provider) != "" {
			_, _ = fmt.Fprintf(&b, "Provider: %s\n", providerLabel(next.Provider))
		}
		if strings.TrimSpace(next.JoinURL) != "" {
			_, _ = fmt.Fprint(&b, "Join available\n")
		}
	} else {
		_, _ = fmt.Fprintf(&b, "No meeting link in next %d minutes\n", int(lookahead.Minutes()))
	}

	if len(upcoming) > 0 {
		_, _ = fmt.Fprint(&b, "\nUpcoming:\n")
		max := len(upcoming)
		if max > 4 {
			max = 4
		}
		for i := 0; i < max; i++ {
			item := upcoming[i]
			_, _ = fmt.Fprintf(&b, "%s — %s\n", item.Start.Format("Mon 15:04"), item.Title)
		}
	}

	_, _ = fmt.Fprint(&b, "Click to open dropdown")
	return strings.TrimSpace(b.String())
}

func providerLabel(provider string) string {
	switch provider {
	case "google_meet":
		return "Google Meet"
	case "zoom":
		return "Zoom"
	default:
		return provider
	}
}

func resolveSelectedUIDs(calendars []schedule.Calendar, selection state.Selection) []string {
	available := make(map[string]schedule.Calendar, len(calendars))
	for _, calendar := range calendars {
		available[calendar.UID] = calendar
	}

	resolved := make([]string, 0, len(calendars))
	if selection.Exists {
		for _, uid := range selection.SelectedUIDs {
			calendar, ok := available[uid]
			if !ok || !calendar.Enabled {
				continue
			}
			resolved = append(resolved, uid)
		}
		sort.Strings(resolved)
		return resolved
	}

	for _, calendar := range calendars {
		if calendar.Enabled && calendar.Selected {
			resolved = append(resolved, calendar.UID)
		}
	}
	if len(resolved) == 0 {
		for _, calendar := range calendars {
			if calendar.Enabled {
				resolved = append(resolved, calendar.UID)
			}
		}
	}

	sort.Strings(resolved)
	return resolved
}

func filterCalendarsByUID(calendars []schedule.Calendar, selectedUIDs []string) []schedule.Calendar {
	if len(selectedUIDs) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(selectedUIDs))
	for _, uid := range selectedUIDs {
		set[uid] = struct{}{}
	}

	selected := make([]schedule.Calendar, 0, len(selectedUIDs))
	for _, calendar := range calendars {
		if _, ok := set[calendar.UID]; !ok {
			continue
		}
		if !calendar.Enabled {
			continue
		}
		selected = append(selected, calendar)
	}
	return selected
}

func joinNext(ctx context.Context, cfg config.Runtime) error {
	items, err := state.LoadMeetings(cfg.ItemsPath)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	return openMeeting(ctx, items[0])
}

func joinItem(ctx context.Context, cfg config.Runtime, index int) error {
	items, err := state.LoadMeetings(cfg.ItemsPath)
	if err != nil {
		return err
	}
	if len(items) == 0 || index > len(items) {
		return nil
	}
	return openMeeting(ctx, items[index-1])
}

func openMeeting(ctx context.Context, item schedule.Occurrence) error {
	url := strings.TrimSpace(item.JoinURL)
	if url == "" {
		return nil
	}
	return openURL(ctx, url)
}

func selectCalendars(ctx context.Context, cfg config.Runtime, stdout io.Writer) error {
	if err := state.EnsureDirs(cfg.StateDir, cfg.MenuDir, cfg.SelectionPath); err != nil {
		return err
	}

	client, err := eds.New(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	calendars, err := client.ListCalendars(ctx)
	if err != nil {
		return err
	}

	selectionState, err := state.LoadSelection(cfg.SelectionPath)
	if err != nil {
		return err
	}

	currentUIDs := resolveSelectedUIDs(calendars, selectionState)
	currentSet := make(map[string]bool, len(currentUIDs))
	for _, uid := range currentUIDs {
		currentSet[uid] = true
	}

	selected, err := selector.SelectCalendars(ctx, calendars, currentSet)
	if err != nil {
		if errors.Is(err, selector.ErrSelectionCancelled) {
			return nil
		}
		notifySelectionError(ctx, err.Error())
		return err
	}

	if err := state.SaveSelection(cfg.SelectionPath, selected); err != nil {
		return err
	}

	if _, err := buildStatus(ctx, cfg); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(stdout, "Saved %d selected calendar(s)\n", len(selected))
	return nil
}

func openURL(ctx context.Context, url string) error {
	if _, err := exec.LookPath("xdg-open"); err != nil {
		return fmt.Errorf("xdg-open not found")
	}

	cmd := exec.CommandContext(ctx, "xdg-open", url)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open url: %w", err)
	}
	return nil
}

func notifySelectionError(ctx context.Context, message string) {
	if _, err := exec.LookPath("notify-send"); err != nil {
		return
	}
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		trimmed = "Calendar selection failed"
	}
	cmd := exec.CommandContext(ctx, "notify-send", "Waybar Schedule", trimmed)
	_ = cmd.Start()
}

func renderUnknownState(cfg config.Runtime, tooltip string) (waybar.Output, error) {
	if err := state.SaveMeetings(cfg.ItemsPath, []schedule.Occurrence{}); err != nil {
		return waybar.Output{}, err
	}
	if err := state.WriteMenu(cfg.MenuPath, state.MenuData{StatusLine: tooltip}); err != nil {
		return waybar.Output{}, err
	}
	return waybar.Output{Text: "?", Tooltip: tooltip, Class: "unknown"}, nil
}

func renderErrorState(cfg config.Runtime, tooltip string) (waybar.Output, error) {
	if err := state.SaveMeetings(cfg.ItemsPath, []schedule.Occurrence{}); err != nil {
		return waybar.Output{}, err
	}
	if err := state.WriteMenu(cfg.MenuPath, state.MenuData{StatusLine: "Calendar query failed"}); err != nil {
		return waybar.Output{}, err
	}
	return waybar.Output{Text: "!", Tooltip: tooltip, Class: "error"}, nil
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
