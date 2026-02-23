package eds

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/rbright/waybar-schedule/internal/schedule"
)

func (c *Client) FetchRawEvents(ctx context.Context, calendars []schedule.Calendar, windowStart, windowEnd time.Time) ([]schedule.RawEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if len(calendars) == 0 {
		return nil, nil
	}

	query := buildTimeRangeQuery(windowStart, windowEnd)
	factory := c.conn.Object(c.calendarService, dbus.ObjectPath("/org/gnome/evolution/dataserver/CalendarFactory"))

	events := make([]schedule.RawEvent, 0, 64)
	errors := make([]string, 0)

	for _, calendar := range calendars {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if strings.TrimSpace(calendar.UID) == "" {
			continue
		}

		objectPath, busName, err := openCalendar(factory, calendar.UID)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", calendar.Name, err.Error()))
			continue
		}

		calendarObj := c.conn.Object(busName, dbus.ObjectPath(objectPath))

		var properties []string
		if err := calendarObj.Call("org.gnome.evolution.dataserver.Calendar.Open", 0).Store(&properties); err != nil {
			errors = append(errors, fmt.Sprintf("%s: open backend: %s", calendar.Name, err.Error()))
			continue
		}

		var payloads []string
		if err := calendarObj.Call("org.gnome.evolution.dataserver.Calendar.GetObjectList", 0, query).Store(&payloads); err != nil {
			errors = append(errors, fmt.Sprintf("%s: query: %s", calendar.Name, err.Error()))
			continue
		}

		for _, payload := range payloads {
			mapped, parseErr := parseEventPayload(calendar, payload)
			if parseErr != nil {
				continue
			}
			events = append(events, mapped...)
		}

		_ = calendarObj.Call("org.gnome.evolution.dataserver.Calendar.Close", 0)
	}

	if len(events) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("failed to query calendars: %s", strings.Join(errors, "; "))
	}

	return events, nil
}

func openCalendar(factory dbus.BusObject, sourceUID string) (objectPath string, busName string, err error) {
	if callErr := factory.Call("org.gnome.evolution.dataserver.CalendarFactory.OpenCalendar", 0, strings.TrimSpace(sourceUID)).Store(&objectPath, &busName); callErr != nil {
		return "", "", fmt.Errorf("OpenCalendar: %w", callErr)
	}
	if strings.TrimSpace(objectPath) == "" {
		return "", "", fmt.Errorf("OpenCalendar returned empty object path")
	}
	if strings.TrimSpace(busName) == "" {
		return "", "", fmt.Errorf("OpenCalendar returned empty bus name")
	}
	return objectPath, busName, nil
}

func buildTimeRangeQuery(windowStart, windowEnd time.Time) string {
	start := windowStart.UTC().Format("20060102T150405Z")
	end := windowEnd.UTC().Format("20060102T150405Z")
	return fmt.Sprintf("(occur-in-time-range? (make-time \"%s\") (make-time \"%s\"))", start, end)
}
