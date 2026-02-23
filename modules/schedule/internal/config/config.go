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

	Lookahead     time.Duration
	QueryLookback time.Duration
	QueryAhead    time.Duration
	IncludeAllDay bool
	MaxItems      int
	Timeout       time.Duration

	StateDir      string
	MenuDir       string
	MenuPath      string
	ItemsPath     string
	CalendarsPath string
	SelectionPath string
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

	defaultConfig := filepath.Join(xdgConfig, "waybar", "schedule.env")
	configFile := strings.TrimSpace(os.Getenv("WAYBAR_SCHEDULE_CONFIG_FILE"))
	if configFile == "" {
		configFile = defaultConfig
	}

	_ = loadEnvFile(configFile)

	v := viper.New()
	v.SetEnvPrefix("WAYBAR_SCHEDULE")
	v.AutomaticEnv()

	_ = v.BindEnv("lookahead_minutes", "WAYBAR_SCHEDULE_LOOKAHEAD_MINUTES", "LOOKAHEAD_MINUTES")
	_ = v.BindEnv("query_lookback_minutes", "WAYBAR_SCHEDULE_QUERY_LOOKBACK_MINUTES", "QUERY_LOOKBACK_MINUTES")
	_ = v.BindEnv("query_ahead_days", "WAYBAR_SCHEDULE_QUERY_AHEAD_DAYS", "QUERY_AHEAD_DAYS")
	_ = v.BindEnv("include_all_day", "WAYBAR_SCHEDULE_INCLUDE_ALL_DAY", "INCLUDE_ALL_DAY")
	_ = v.BindEnv("max_items", "WAYBAR_SCHEDULE_MAX_ITEMS", "MAX_ITEMS")
	_ = v.BindEnv("timeout_seconds", "WAYBAR_SCHEDULE_TIMEOUT_SECONDS")
	_ = v.BindEnv("state_dir", "WAYBAR_SCHEDULE_STATE_DIR")
	_ = v.BindEnv("menu_dir", "WAYBAR_SCHEDULE_MENU_DIR")
	_ = v.BindEnv("selection_file", "WAYBAR_SCHEDULE_SELECTION_FILE")

	v.SetDefault("lookahead_minutes", 60)
	v.SetDefault("query_lookback_minutes", 120)
	v.SetDefault("query_ahead_days", 14)
	v.SetDefault("include_all_day", false)
	v.SetDefault("max_items", 8)
	v.SetDefault("timeout_seconds", 20)
	v.SetDefault("state_dir", filepath.Join(xdgState, "waybar", "schedule"))
	v.SetDefault("menu_dir", filepath.Join(xdgState, "waybar", "menus"))
	v.SetDefault("selection_file", filepath.Join(xdgConfig, "waybar", "schedule-selected-calendars.json"))

	maxItems := v.GetInt("max_items")
	if maxItems < 1 {
		maxItems = 1
	}
	if maxItems > maxActionItems {
		maxItems = maxActionItems
	}

	timeoutSeconds := v.GetInt("timeout_seconds")
	if timeoutSeconds <= 0 {
		timeoutSeconds = 20
	}

	lookaheadMinutes := v.GetInt("lookahead_minutes")
	if lookaheadMinutes < 0 {
		lookaheadMinutes = 0
	}

	queryLookbackMinutes := v.GetInt("query_lookback_minutes")
	if queryLookbackMinutes < 0 {
		queryLookbackMinutes = 0
	}

	queryAheadDays := v.GetInt("query_ahead_days")
	if queryAheadDays <= 0 {
		queryAheadDays = 14
	}

	stateDir := strings.TrimSpace(v.GetString("state_dir"))
	if stateDir == "" {
		stateDir = filepath.Join(xdgState, "waybar", "schedule")
	}

	menuDir := strings.TrimSpace(v.GetString("menu_dir"))
	if menuDir == "" {
		menuDir = filepath.Join(xdgState, "waybar", "menus")
	}

	selectionPath := strings.TrimSpace(v.GetString("selection_file"))
	if selectionPath == "" {
		selectionPath = filepath.Join(xdgConfig, "waybar", "schedule-selected-calendars.json")
	}

	return Runtime{
		ConfigFile: configFile,
		Lookahead:  time.Duration(lookaheadMinutes) * time.Minute,
		QueryLookback: time.Duration(queryLookbackMinutes) *
			time.Minute,
		QueryAhead:    time.Duration(queryAheadDays) * 24 * time.Hour,
		IncludeAllDay: v.GetBool("include_all_day"),
		MaxItems:      maxItems,
		Timeout:       time.Duration(timeoutSeconds) * time.Second,
		StateDir:      stateDir,
		MenuDir:       menuDir,
		MenuPath:      filepath.Join(menuDir, "schedule.xml"),
		ItemsPath:     filepath.Join(stateDir, "meetings.json"),
		CalendarsPath: filepath.Join(stateDir, "calendars.json"),
		SelectionPath: selectionPath,
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
