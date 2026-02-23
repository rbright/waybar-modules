package state

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rbright/waybar-schedule/internal/schedule"
)

type MenuData struct {
	StatusLine string
	Next       *schedule.Occurrence
	Items      []schedule.Occurrence
}

func WriteMenu(path string, data MenuData) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create menu dir: %w", err)
	}

	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<interface>\n")
	b.WriteString("  <object class=\"GtkMenu\" id=\"menu\">\n")

	if data.Next != nil {
		prefix := "Open next"
		if strings.TrimSpace(data.Next.JoinURL) != "" {
			prefix = "Join call"
		}
		writeMenuItem(&b, "join_next", fmt.Sprintf("%s: %s", prefix, fallback(data.Next.Title, "Meeting")))
		writeSeparator(&b, "separator_next")
	}

	if len(data.Items) > 0 {
		for idx, item := range data.Items {
			label := fmt.Sprintf("%s — %s", formatStart(item.Start), fallback(item.Title, "Meeting"))
			writeMenuItem(&b, fmt.Sprintf("join_%d", idx+1), label)
		}
	} else {
		writeMenuItem(&b, "noop", fallback(data.StatusLine, "No upcoming meetings"))
	}

	writeSeparator(&b, "separator_actions")
	writeMenuItem(&b, "select_calendars", "Select Calendars…")
	writeMenuItem(&b, "refresh", "Refresh")

	b.WriteString("  </object>\n")
	b.WriteString("</interface>\n")

	return writeFileAtomically(path, []byte(b.String()))
}

func writeMenuItem(b *strings.Builder, id, label string) {
	b.WriteString("    <child>\n")
	_, _ = fmt.Fprintf(b, "      <object class=\"GtkMenuItem\" id=\"%s\">\n", html.EscapeString(id))
	_, _ = fmt.Fprintf(b, "        <property name=\"label\">%s</property>\n", html.EscapeString(label))
	b.WriteString("      </object>\n")
	b.WriteString("    </child>\n")
}

func writeSeparator(b *strings.Builder, id string) {
	b.WriteString("    <child>\n")
	_, _ = fmt.Fprintf(b, "      <object class=\"GtkSeparatorMenuItem\" id=\"%s\" />\n", html.EscapeString(id))
	b.WriteString("    </child>\n")
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func formatStart(value time.Time) string {
	now := time.Now()
	if now.Year() == value.Year() && now.YearDay() == value.YearDay() {
		return value.Format("15:04")
	}
	return value.Format("Mon 15:04")
}
