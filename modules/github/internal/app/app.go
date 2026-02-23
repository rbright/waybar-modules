package app

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/rbright/waybar-github/internal/config"
	"github.com/rbright/waybar-github/internal/github"
	"github.com/rbright/waybar-github/internal/state"
	"github.com/rbright/waybar-github/internal/waybar"
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
	case "open-dashboard":
		return openURL(ctx, fmt.Sprintf("https://%s/pulls", cfg.Host))
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
	case "status", "refresh", "open-dashboard":
		if len(args) > 1 {
			return "", 0, fmt.Errorf("unexpected argument %q", args[1])
		}
		return strings.TrimSpace(args[0]), 0, nil
	case "open-item":
		if len(args) != 2 {
			return "", 0, fmt.Errorf("usage: waybar-github open-item <index>")
		}
		n, convErr := strconv.Atoi(strings.TrimSpace(args[1]))
		if convErr != nil || n < 1 {
			return "", 0, fmt.Errorf("invalid item index %q", args[1])
		}
		return "open-item", n, nil
	default:
		return "", 0, fmt.Errorf("usage: waybar-github <status|refresh|open-dashboard|open-item N>")
	}
}

func buildStatus(ctx context.Context, cfg config.Runtime) (waybar.Output, error) {
	if err := state.EnsureDirs(cfg.StateDir, cfg.MenuDir); err != nil {
		return waybar.Output{}, err
	}

	authMode := github.DetectAuth(ctx, cfg)
	if authMode == github.AuthNone {
		statusLine := "Run 'gh auth login' or set GITHUB_TOKEN"
		if err := state.SaveItems(cfg.ItemsPath, []github.PullRequest{}); err != nil {
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

	result, err := github.FetchPullRequests(ctx, cfg, authMode)
	if err != nil {
		statusLine := "GitHub API request failed"
		if saveErr := state.SaveItems(cfg.ItemsPath, []github.PullRequest{}); saveErr != nil {
			return waybar.Output{}, saveErr
		}
		if menuErr := state.WriteMenu(cfg.MenuPath, state.MenuData{StatusLine: statusLine}); menuErr != nil {
			return waybar.Output{}, menuErr
		}
		return waybar.Output{
			Text:    "!",
			Tooltip: fmt.Sprintf("GitHub pull requests: %s", err.Error()),
			Class:   "error",
		}, nil
	}

	if err := state.SaveItems(cfg.ItemsPath, result.Items); err != nil {
		return waybar.Output{}, err
	}

	statusLine := "No matching pull requests"
	if result.Count > 0 {
		statusLine = "Open pull requests"
	}

	if err := state.WriteMenu(cfg.MenuPath, state.MenuData{StatusLine: statusLine, Items: result.Items}); err != nil {
		return waybar.Output{}, err
	}

	tooltip := fmt.Sprintf("GitHub pull requests: %d", result.Count)
	if len(result.Items) > 0 {
		lines := make([]string, 0, len(result.Items))
		for _, item := range result.Items {
			lines = append(lines, fmt.Sprintf("%s #%d: %s", item.Repository, item.Number, item.Title))
		}
		tooltip += "\n" + strings.Join(lines, "\n")
	}
	tooltip += "\nClick to open dropdown"

	className := "clear"
	if result.Count > 0 {
		className = "normal"
	}

	return waybar.Output{
		Text:    strconv.Itoa(result.Count),
		Tooltip: tooltip,
		Class:   className,
	}, nil
}

func openItem(ctx context.Context, cfg config.Runtime, index int) error {
	items, err := state.LoadItems(cfg.ItemsPath)
	if err != nil {
		return err
	}
	if len(items) == 0 || index > len(items) {
		return nil
	}

	url := strings.TrimSpace(items[index-1].URL)
	if url == "" {
		return nil
	}
	return openURL(ctx, url)
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
