package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rbright/waybar-agent-usage/internal/domain"
)

type Runtime struct {
	Timeout       time.Duration
	CacheTTL      time.Duration
	StateDir      string
	ConfigDir     string
	EnvFile       string
	ClaudeRetries int

	CodexHome        string
	CodexAuthFile    string
	CodexAccessToken string
	CodexAccountID   string
	CodexIcon        string

	ClaudeCredentialsFile string
	ClaudeAccessToken     string
	ClaudeClientID        string
	ClaudeIcon            string
}

func Load() (Runtime, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Runtime{}, fmt.Errorf("resolve home dir: %w", err)
	}

	xdgConfig := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}
	envFile := filepath.Join(xdgConfig, "waybar", "ai-usage.env")
	_ = LoadEnvFile(envFile)

	xdgState := strings.TrimSpace(os.Getenv("XDG_STATE_HOME"))
	if xdgState == "" {
		xdgState = filepath.Join(home, ".local", "state")
	}

	codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME"))
	if codexHome == "" {
		codexHome = filepath.Join(home, ".codex")
	}

	stateDir := strings.TrimSpace(os.Getenv("WAYBAR_AI_STATE_DIR"))
	if stateDir == "" {
		stateDir = filepath.Join(xdgState, "waybar", "ai-usage")
	}

	cfg := Runtime{
		Timeout:   time.Duration(domain.ParseFloat(os.Getenv("WAYBAR_AI_TIMEOUT_SECONDS"), 15) * float64(time.Second)),
		CacheTTL:  time.Duration(domain.ParseInt(os.Getenv("WAYBAR_AI_CACHE_TTL_SECONDS"), 75)) * time.Second,
		StateDir:  stateDir,
		ConfigDir: filepath.Dir(envFile),
		EnvFile:   envFile,
		ClaudeRetries: maxInt(
			1,
			domain.ParseInt(os.Getenv("WAYBAR_AI_CLAUDE_REFRESH_RETRIES"), 3),
		),

		CodexHome:        codexHome,
		CodexAuthFile:    firstNonEmpty(os.Getenv("WAYBAR_AI_CODEX_AUTH_FILE"), filepath.Join(codexHome, "auth.json")),
		CodexAccessToken: strings.TrimSpace(os.Getenv("WAYBAR_AI_CODEX_ACCESS_TOKEN")),
		CodexAccountID:   strings.TrimSpace(os.Getenv("WAYBAR_AI_CODEX_ACCOUNT_ID")),
		CodexIcon:        firstNonEmpty(os.Getenv("WAYBAR_AI_CODEX_ICON"), "\ue7cf"),

		ClaudeCredentialsFile: firstNonEmpty(os.Getenv("WAYBAR_AI_CLAUDE_CREDENTIALS_FILE"), filepath.Join(home, ".claude", ".credentials.json")),
		ClaudeAccessToken:     strings.TrimSpace(os.Getenv("WAYBAR_AI_CLAUDE_ACCESS_TOKEN")),
		ClaudeClientID: firstNonEmpty(
			os.Getenv("WAYBAR_AI_CLAUDE_CLIENT_ID"),
			"9d1c250a-e61b-44d9-88ed-5944d1962f5e",
		),
		ClaudeIcon: firstNonEmpty(os.Getenv("WAYBAR_AI_CLAUDE_ICON"), "\ue861"),
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.CacheTTL < 0 {
		cfg.CacheTTL = 0
	}

	return cfg, nil
}

func LoadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open env file %s: %w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if len(value) >= 2 {
			if (value[0] == '\'' && value[len(value)-1] == '\'') ||
				(value[0] == '"' && value[len(value)-1] == '"') {
				value = value[1 : len(value)-1]
			}
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan env file %s: %w", path, err)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
