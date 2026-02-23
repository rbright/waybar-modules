package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const maxActionItems = 12

type Runtime struct {
	ConfigFile string

	Host       string
	APIURL     string
	GraphQLURL string
	Token      string
	PRQuery    string
	MaxItems   int
	Timeout    time.Duration

	StateDir  string
	MenuDir   string
	MenuPath  string
	ItemsPath string
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

	xdgState := strings.TrimSpace(os.Getenv("XDG_STATE_HOME"))
	if xdgState == "" {
		xdgState = filepath.Join(home, ".local", "state")
	}

	defaultConfig := filepath.Join(xdgConfig, "waybar", "github-pull-requests.env")
	configFile := strings.TrimSpace(os.Getenv("WAYBAR_GITHUB_CONFIG_FILE"))
	if configFile == "" {
		configFile = defaultConfig
	}

	// Load dotenv-style config if present so viper can consume as env-backed config.
	_ = loadEnvFile(configFile)

	v := viper.New()
	v.SetEnvPrefix("WAYBAR_GITHUB")
	v.AutomaticEnv()

	_ = v.BindEnv("host", "WAYBAR_GITHUB_HOST", "GITHUB_HOST")
	_ = v.BindEnv("api_url", "WAYBAR_GITHUB_API_URL", "GITHUB_API_URL")
	_ = v.BindEnv("graphql_url", "WAYBAR_GITHUB_GRAPHQL_URL", "GITHUB_GRAPHQL_URL")
	_ = v.BindEnv("token", "WAYBAR_GITHUB_TOKEN", "GITHUB_TOKEN")
	_ = v.BindEnv("pr_query", "WAYBAR_GITHUB_PR_QUERY", "PR_QUERY")
	_ = v.BindEnv("max_items", "WAYBAR_GITHUB_MAX_ITEMS", "MAX_ITEMS")
	_ = v.BindEnv("timeout_seconds", "WAYBAR_GITHUB_TIMEOUT_SECONDS")
	_ = v.BindEnv("state_dir", "WAYBAR_GITHUB_STATE_DIR")
	_ = v.BindEnv("menu_dir", "WAYBAR_GITHUB_MENU_DIR")

	v.SetDefault("host", "github.com")
	v.SetDefault("api_url", "https://api.github.com")
	v.SetDefault("graphql_url", "")
	v.SetDefault("pr_query", "is:open is:pr involves:@me archived:false sort:updated-desc")
	v.SetDefault("max_items", 8)
	v.SetDefault("timeout_seconds", 15)
	v.SetDefault("state_dir", filepath.Join(xdgState, "waybar", "github-pull-requests"))
	v.SetDefault("menu_dir", filepath.Join(xdgState, "waybar", "menus"))

	maxItems := v.GetInt("max_items")
	if maxItems < 1 {
		maxItems = 1
	}
	if maxItems > maxActionItems {
		maxItems = maxActionItems
	}

	timeoutSeconds := v.GetInt("timeout_seconds")
	if timeoutSeconds <= 0 {
		timeoutSeconds = 15
	}

	host := strings.TrimSpace(v.GetString("host"))
	if host == "" {
		host = "github.com"
	}

	apiURL := strings.TrimSpace(v.GetString("api_url"))
	if apiURL == "" {
		apiURL = "https://api.github.com"
	}

	graphqlURL := strings.TrimSpace(v.GetString("graphql_url"))
	if graphqlURL == "" {
		graphqlURL = strings.TrimRight(apiURL, "/") + "/graphql"
	}

	stateDir := strings.TrimSpace(v.GetString("state_dir"))
	if stateDir == "" {
		stateDir = filepath.Join(xdgState, "waybar", "github-pull-requests")
	}

	menuDir := strings.TrimSpace(v.GetString("menu_dir"))
	if menuDir == "" {
		menuDir = filepath.Join(xdgState, "waybar", "menus")
	}

	return Runtime{
		ConfigFile: configFile,
		Host:       host,
		APIURL:     apiURL,
		GraphQLURL: graphqlURL,
		Token:      strings.TrimSpace(v.GetString("token")),
		PRQuery:    strings.TrimSpace(v.GetString("pr_query")),
		MaxItems:   maxItems,
		Timeout:    time.Duration(timeoutSeconds) * time.Second,
		StateDir:   stateDir,
		MenuDir:    menuDir,
		MenuPath:   filepath.Join(menuDir, "github-pull-requests.xml"),
		ItemsPath:  filepath.Join(stateDir, "items.json"),
	}, nil
}

func loadEnvFile(path string) error {
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
