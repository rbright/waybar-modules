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

	SottoConfigFile string
	Icon            string
	MaxItems        int
	Timeout         time.Duration

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

	defaultConfig := filepath.Join(xdgConfig, "waybar", "sotto-input.env")
	configFile := strings.TrimSpace(os.Getenv("WAYBAR_SOTTO_CONFIG_FILE"))
	if configFile == "" {
		configFile = defaultConfig
	}

	// Load dotenv-style config if present so viper can consume as env-backed config.
	_ = loadEnvFile(configFile)

	v := viper.New()
	v.SetEnvPrefix("WAYBAR_SOTTO")
	v.AutomaticEnv()

	_ = v.BindEnv("sotto_config_file", "WAYBAR_SOTTO_SOTTO_CONFIG_FILE", "SOTTO_CONFIG_FILE")
	_ = v.BindEnv("icon", "WAYBAR_SOTTO_ICON", "ICON")
	_ = v.BindEnv("max_items", "WAYBAR_SOTTO_MAX_ITEMS", "MAX_ITEMS")
	_ = v.BindEnv("timeout_seconds", "WAYBAR_SOTTO_TIMEOUT_SECONDS")
	_ = v.BindEnv("state_dir", "WAYBAR_SOTTO_STATE_DIR")
	_ = v.BindEnv("menu_dir", "WAYBAR_SOTTO_MENU_DIR")

	v.SetDefault("sotto_config_file", filepath.Join(xdgConfig, "sotto", "config.jsonc"))
	v.SetDefault("icon", "󰍬")
	v.SetDefault("max_items", 12)
	v.SetDefault("timeout_seconds", 10)
	v.SetDefault("state_dir", filepath.Join(xdgState, "waybar", "sotto-input"))
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
		timeoutSeconds = 10
	}

	sottoConfigFile := strings.TrimSpace(v.GetString("sotto_config_file"))
	if sottoConfigFile == "" {
		sottoConfigFile = filepath.Join(xdgConfig, "sotto", "config.jsonc")
	}

	icon := strings.TrimSpace(v.GetString("icon"))
	if icon == "" {
		icon = "󰍬"
	}

	stateDir := strings.TrimSpace(v.GetString("state_dir"))
	if stateDir == "" {
		stateDir = filepath.Join(xdgState, "waybar", "sotto-input")
	}

	menuDir := strings.TrimSpace(v.GetString("menu_dir"))
	if menuDir == "" {
		menuDir = filepath.Join(xdgState, "waybar", "menus")
	}

	return Runtime{
		ConfigFile:      configFile,
		SottoConfigFile: sottoConfigFile,
		Icon:            icon,
		MaxItems:        maxItems,
		Timeout:         time.Duration(timeoutSeconds) * time.Second,
		StateDir:        stateDir,
		MenuDir:         menuDir,
		MenuPath:        filepath.Join(menuDir, "sotto-input.xml"),
		ItemsPath:       filepath.Join(stateDir, "items.json"),
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
