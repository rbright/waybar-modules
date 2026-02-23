package eds

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/rbright/waybar-schedule/internal/schedule"
	"gopkg.in/ini.v1"
)

type sourceEntry struct {
	UID string

	DisplayName string
	ParentUID   string
	Enabled     bool

	HasCalendar      bool
	CalendarEnabled  bool
	CalendarSelected bool
	CalendarBackend  string
	CalendarColor    string
}

func (c *Client) ListCalendars(ctx context.Context) ([]schedule.Calendar, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sourceObj := c.conn.Object(c.sourceService, dbus.ObjectPath("/org/gnome/evolution/dataserver/SourceManager"))

	managed := make(map[dbus.ObjectPath]map[string]map[string]dbus.Variant)
	if err := sourceObj.Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&managed); err != nil {
		return nil, fmt.Errorf("eds GetManagedObjects: %w", err)
	}

	entries := make(map[string]sourceEntry)
	for _, ifaceMap := range managed {
		sourceProps, ok := ifaceMap["org.gnome.evolution.dataserver.Source"]
		if !ok {
			continue
		}

		uid := variantString(sourceProps, "UID")
		data := variantString(sourceProps, "Data")
		if strings.TrimSpace(uid) == "" || strings.TrimSpace(data) == "" {
			continue
		}

		entry, err := parseSourceEntry(uid, data)
		if err != nil {
			continue
		}
		entries[uid] = entry
	}

	calendars := make([]schedule.Calendar, 0, len(entries))
	for _, entry := range entries {
		if !entry.HasCalendar {
			continue
		}

		calendar := schedule.Calendar{
			UID:         entry.UID,
			Name:        fallback(entry.DisplayName, entry.UID),
			ParentUID:   strings.TrimSpace(entry.ParentUID),
			Backend:     strings.TrimSpace(entry.CalendarBackend),
			Color:       strings.TrimSpace(entry.CalendarColor),
			Selected:    entry.CalendarSelected,
			Enabled:     entry.Enabled && entry.CalendarEnabled,
			AccountName: accountName(entry, entries),
		}
		calendars = append(calendars, calendar)
	}

	sort.SliceStable(calendars, func(i, j int) bool {
		nameI := strings.ToLower(calendars[i].Name)
		nameJ := strings.ToLower(calendars[j].Name)
		if nameI != nameJ {
			return nameI < nameJ
		}
		return calendars[i].UID < calendars[j].UID
	})

	return calendars, nil
}

func parseSourceEntry(uid, data string) (sourceEntry, error) {
	cfg, err := ini.LoadSources(ini.LoadOptions{
		IgnoreInlineComment: true,
		AllowShadows:        true,
	}, []byte(data))
	if err != nil {
		return sourceEntry{}, err
	}

	entry := sourceEntry{UID: uid}

	dataSection := cfg.Section("Data Source")
	entry.DisplayName = strings.TrimSpace(dataSection.Key("DisplayName").String())
	entry.ParentUID = strings.TrimSpace(dataSection.Key("Parent").String())
	entry.Enabled = parseBoolWithDefault(dataSection.Key("Enabled").String(), true)

	calendarSection, err := cfg.GetSection("Calendar")
	if err == nil {
		entry.HasCalendar = true
		entry.CalendarEnabled = parseBoolWithDefault(calendarSection.Key("Enabled").String(), true)
		entry.CalendarSelected = parseBoolWithDefault(calendarSection.Key("Selected").String(), false)
		entry.CalendarBackend = strings.TrimSpace(calendarSection.Key("BackendName").String())
		entry.CalendarColor = strings.TrimSpace(calendarSection.Key("Color").String())
	}

	return entry, nil
}

func variantString(props map[string]dbus.Variant, key string) string {
	value, ok := props[key]
	if !ok {
		return ""
	}
	asString, ok := value.Value().(string)
	if !ok {
		return ""
	}
	return asString
}

func parseBoolWithDefault(value string, fallback bool) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(trimmed)
	if err != nil {
		return fallback
	}
	return parsed
}

func accountName(entry sourceEntry, entries map[string]sourceEntry) string {
	parentUID := strings.TrimSpace(entry.ParentUID)
	if parentUID == "" {
		return ""
	}

	if strings.HasSuffix(parentUID, "-stub") {
		return ""
	}

	parent, ok := entries[parentUID]
	if !ok {
		return ""
	}

	name := strings.TrimSpace(parent.DisplayName)
	if name == "" {
		return ""
	}
	if strings.HasSuffix(strings.ToLower(name), "stub") {
		return ""
	}
	return name
}

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackValue
	}
	return value
}
