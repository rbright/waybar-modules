package app

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/rbright/waybar-linear/internal/config"
	"github.com/rbright/waybar-linear/internal/linear"
	"github.com/rbright/waybar-linear/internal/state"
	"github.com/rbright/waybar-linear/internal/waybar"
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
	case "open-inbox":
		return openInbox(ctx, cfg)
	case "mark-all-read":
		return markAllRead(ctx, cfg)
	case "open-item":
		return openItem(ctx, cfg, index)
	default:
		return fmt.Errorf("unsupported command %q", cmd)
	}
}

func parseArgs(args []string) (command string, index int, err error) {
	if len(args) == 0 {
		return "status", 0, nil
	}

	switch strings.TrimSpace(args[0]) {
	case "status", "refresh", "open-inbox", "mark-all-read":
		if len(args) > 1 {
			return "", 0, fmt.Errorf("unexpected argument %q", args[1])
		}
		return strings.TrimSpace(args[0]), 0, nil
	case "open-item":
		if len(args) != 2 {
			return "", 0, fmt.Errorf("usage: waybar-linear open-item <index>")
		}
		n, convErr := strconv.Atoi(strings.TrimSpace(args[1]))
		if convErr != nil || n < 1 {
			return "", 0, fmt.Errorf("invalid item index %q", args[1])
		}
		return "open-item", n, nil
	default:
		return "", 0, fmt.Errorf("usage: waybar-linear <status|refresh|open-inbox|mark-all-read|open-item N>")
	}
}

func buildStatus(ctx context.Context, cfg config.Runtime) (waybar.Output, error) {
	if err := state.EnsureDirs(cfg.StateDir, cfg.MenuDir); err != nil {
		return waybar.Output{}, err
	}

	if strings.TrimSpace(cfg.APIKey) == "" {
		statusLine := "Set LINEAR_API_KEY in ~/.config/waybar/linear-notifications.env"
		if err := state.SaveItems(cfg.ItemsPath, []linear.Notification{}); err != nil {
			return waybar.Output{}, err
		}
		if err := state.SaveMeta(cfg.MetaPath, state.Meta{}); err != nil {
			return waybar.Output{}, err
		}
		if err := state.WriteMenu(cfg.MenuPath, state.MenuData{StatusLine: statusLine}); err != nil {
			return waybar.Output{}, err
		}
		return waybar.Output{
			Text:    "?",
			Tooltip: statusLine,
			Class:   "unknown",
		}, nil
	}

	result, err := linear.FetchNotifications(ctx, cfg)
	if err != nil {
		statusLine := "Linear API request failed"
		if saveErr := state.SaveItems(cfg.ItemsPath, []linear.Notification{}); saveErr != nil {
			return waybar.Output{}, saveErr
		}
		if metaErr := state.SaveMeta(cfg.MetaPath, state.Meta{}); metaErr != nil {
			return waybar.Output{}, metaErr
		}
		if menuErr := state.WriteMenu(cfg.MenuPath, state.MenuData{StatusLine: statusLine}); menuErr != nil {
			return waybar.Output{}, menuErr
		}
		return waybar.Output{
			Text:    "!",
			Tooltip: fmt.Sprintf("Linear notifications: %s", err.Error()),
			Class:   "error",
		}, nil
	}

	if err := state.SaveItems(cfg.ItemsPath, result.Items); err != nil {
		return waybar.Output{}, err
	}
	if err := state.SaveMeta(cfg.MetaPath, state.Meta{URLKey: result.URLKey}); err != nil {
		return waybar.Output{}, err
	}

	statusLine := "No unread notifications"
	if result.UnreadCount > 0 {
		statusLine = "Unread notifications"
	}

	if err := state.WriteMenu(cfg.MenuPath, state.MenuData{
		StatusLine:   statusLine,
		Items:        result.Items,
		AllowMarkAll: result.UnreadCount > 0,
	}); err != nil {
		return waybar.Output{}, err
	}

	tooltip := fmt.Sprintf("Linear unread notifications: %d", result.UnreadCount)
	if len(result.Items) > 0 {
		lines := make([]string, 0, len(result.Items))
		for _, item := range result.Items {
			line := fallback(item.Subtitle, "Notification")
			if strings.TrimSpace(item.Title) != "" && strings.TrimSpace(item.Title) != strings.TrimSpace(item.Subtitle) {
				line += ": " + item.Title
			}
			lines = append(lines, line)
		}
		tooltip += "\n" + strings.Join(lines, "\n")
	}
	tooltip += "\nClick to open dropdown"

	className := "clear"
	if result.UnreadCount > 0 {
		className = "normal"
	}

	return waybar.Output{
		Text:    strconv.Itoa(result.UnreadCount),
		Tooltip: tooltip,
		Class:   className,
	}, nil
}

func openInbox(ctx context.Context, cfg config.Runtime) error {
	meta, err := state.LoadMeta(cfg.MetaPath)
	if err != nil {
		return err
	}

	url := "https://linear.app/inbox"
	if strings.TrimSpace(meta.URLKey) != "" {
		url = fmt.Sprintf("https://linear.app/%s/inbox", strings.TrimSpace(meta.URLKey))
	}
	return openURL(ctx, url)
}

func markAllRead(ctx context.Context, cfg config.Runtime) error {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil
	}

	result, err := linear.FetchNotifications(ctx, cfg)
	if err != nil {
		return err
	}

	for _, id := range result.UnreadIDs {
		if err := linear.MarkRead(ctx, cfg, id); err != nil {
			return err
		}
	}

	_, err = buildStatus(ctx, cfg)
	return err
}

func openItem(ctx context.Context, cfg config.Runtime, index int) error {
	items, err := state.LoadItems(cfg.ItemsPath)
	if err != nil {
		return err
	}
	if len(items) == 0 || index > len(items) {
		return nil
	}

	item := items[index-1]
	url := strings.TrimSpace(item.URL)
	if url == "" {
		return nil
	}

	if err := openURL(ctx, url); err != nil {
		return err
	}

	if strings.TrimSpace(cfg.APIKey) != "" && strings.TrimSpace(item.ID) != "" {
		if err := linear.MarkRead(ctx, cfg, item.ID); err == nil {
			_, _ = buildStatus(ctx, cfg)
		}
	}

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

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
