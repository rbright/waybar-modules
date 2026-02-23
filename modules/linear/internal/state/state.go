package state

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/rbright/waybar-linear/internal/linear"
)

type Meta struct {
	URLKey string `json:"urlKey"`
}

type MenuData struct {
	StatusLine   string
	Items        []linear.Notification
	AllowMarkAll bool
}

func EnsureDirs(stateDir, menuDir string) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	if err := os.MkdirAll(menuDir, 0o755); err != nil {
		return fmt.Errorf("create menu dir: %w", err)
	}
	return nil
}

func SaveItems(path string, items []linear.Notification) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create items dir: %w", err)
	}

	payload, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal items: %w", err)
	}

	return writeFileAtomically(path, append(payload, '\n'))
}

func LoadItems(path string) ([]linear.Notification, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read items file: %w", err)
	}

	var items []linear.Notification
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("decode items file: %w", err)
	}
	return items, nil
}

func SaveMeta(path string, meta Meta) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create meta dir: %w", err)
	}

	payload, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}

	return writeFileAtomically(path, append(payload, '\n'))
}

func LoadMeta(path string) (Meta, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Meta{}, nil
		}
		return Meta{}, fmt.Errorf("read meta file: %w", err)
	}

	var meta Meta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return Meta{}, fmt.Errorf("decode meta file: %w", err)
	}
	return meta, nil
}

func WriteMenu(path string, data MenuData) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create menu dir: %w", err)
	}

	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<interface>\n")
	b.WriteString("  <object class=\"GtkMenu\" id=\"menu\">\n")

	writeMenuItem(&b, "open_inbox", "Open Linear Inbox")
	if data.AllowMarkAll {
		writeMenuItem(&b, "mark_all_read", "Mark All as Read")
	}
	writeSeparator(&b, "separator_primary")

	if len(data.Items) > 0 {
		for idx, item := range data.Items {
			label := fallback(item.Subtitle, "Notification")
			if strings.TrimSpace(item.Title) != "" && strings.TrimSpace(item.Title) != strings.TrimSpace(item.Subtitle) {
				label += ": " + item.Title
			}
			writeMenuItem(&b, fmt.Sprintf("open_%d", idx+1), label)
		}
	} else {
		writeMenuItem(&b, "noop", fallback(data.StatusLine, "No unread notifications"))
	}

	writeSeparator(&b, "separator_footer")
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

func writeFileAtomically(path string, content []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
