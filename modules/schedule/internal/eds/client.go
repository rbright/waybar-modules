package eds

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/godbus/dbus/v5"
)

const (
	sourceServicePrefix   = "org.gnome.evolution.dataserver.Sources"
	calendarServicePrefix = "org.gnome.evolution.dataserver.Calendar"
)

type Client struct {
	conn            *dbus.Conn
	sourceService   string
	calendarService string
}

func New(ctx context.Context) (*Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("connect session bus: %w", err)
	}

	sourceService, err := findServiceName(conn, sourceServicePrefix)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	calendarService, err := findServiceName(conn, calendarServicePrefix)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	return &Client{
		conn:            conn,
		sourceService:   sourceService,
		calendarService: calendarService,
	}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func findServiceName(conn *dbus.Conn, prefix string) (string, error) {
	dbusObj := conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus")

	var names []string
	if err := dbusObj.Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err == nil {
		if best := bestMatchingService(names, prefix); best != "" {
			return best, nil
		}
	}

	var activatable []string
	if err := dbusObj.Call("org.freedesktop.DBus.ListActivatableNames", 0).Store(&activatable); err == nil {
		if best := bestMatchingService(activatable, prefix); best != "" {
			return best, nil
		}
	}

	return "", fmt.Errorf("dbus service with prefix %q not found", prefix)
}

func bestMatchingService(names []string, prefix string) string {
	matches := make([]string, 0, len(names))
	for _, name := range names {
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, name)
		}
	}
	if len(matches) == 0 {
		return ""
	}

	sort.SliceStable(matches, func(i, j int) bool {
		versionI := serviceVersion(matches[i], prefix)
		versionJ := serviceVersion(matches[j], prefix)
		if versionI != versionJ {
			return versionI > versionJ
		}
		return matches[i] < matches[j]
	})

	return matches[0]
}

func serviceVersion(name, prefix string) int {
	versionPart := strings.TrimPrefix(name, prefix)
	versionPart = strings.TrimSpace(versionPart)
	if versionPart == "" {
		return 0
	}
	version, err := strconv.Atoi(versionPart)
	if err != nil {
		return 0
	}
	return version
}
