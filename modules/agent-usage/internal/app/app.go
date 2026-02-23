package app

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rbright/waybar-agent-usage/internal/config"
	"github.com/rbright/waybar-agent-usage/internal/domain"
	"github.com/rbright/waybar-agent-usage/internal/providers"
	"github.com/rbright/waybar-agent-usage/internal/state"
	"github.com/rbright/waybar-agent-usage/internal/waybar"
)

func Run(ctx context.Context, args []string, cfg config.Runtime, stdout io.Writer) error {
	provider, refresh, err := parseArgs(args)
	if err != nil {
		return err
	}

	icons := waybar.IconConfig{Codex: cfg.CodexIcon, Claude: cfg.ClaudeIcon}
	cacheStore := state.NewStore(cfg.StateDir)

	cached, _ := cacheStore.Load(provider)
	if cached != nil && !refresh {
		if cfg.CacheTTL <= 0 || time.Since(cached.FetchedAt) < cfg.CacheTTL {
			output := waybar.Render(cached.Metrics, cached.FetchedAt, icons, "")
			return writeOutput(stdout, output)
		}
	}

	metrics, fetchErr := fetchProvider(ctx, provider, cfg)
	if fetchErr == nil {
		now := time.Now().UTC()
		_ = cacheStore.Save(provider, metrics, now) // Best-effort cache persistence.
		output := waybar.Render(metrics, now, icons, "")
		return writeOutput(stdout, output)
	}

	if cached != nil {
		output := waybar.Render(cached.Metrics, cached.FetchedAt, icons, fetchErr.Error())
		return writeOutput(stdout, output)
	}

	output := waybar.RenderError(provider, icons, fetchErr.Error())
	return writeOutput(stdout, output)
}

func parseArgs(args []string) (domain.Provider, bool, error) {
	var providerArg string
	refresh := false

	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		switch trimmed {
		case "", "--":
			continue
		case "--refresh":
			refresh = true
		default:
			if strings.HasPrefix(trimmed, "-") {
				return "", false, fmt.Errorf("unsupported flag %q", trimmed)
			}
			if providerArg == "" {
				providerArg = trimmed
				continue
			}
			return "", false, fmt.Errorf("unexpected argument %q", trimmed)
		}
	}

	if providerArg == "" {
		return "", false, fmt.Errorf("usage: waybar-agent-usage <codex|claude> [--refresh]")
	}

	provider, err := domain.ParseProvider(providerArg)
	if err != nil {
		return "", false, err
	}
	return provider, refresh, nil
}

func fetchProvider(ctx context.Context, provider domain.Provider, cfg config.Runtime) (domain.Metrics, error) {
	switch provider {
	case domain.ProviderCodex:
		return providers.FetchCodex(ctx, cfg)
	case domain.ProviderClaude:
		return providers.FetchClaude(ctx, cfg)
	default:
		return domain.Metrics{}, fmt.Errorf("unsupported provider %q", provider)
	}
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
