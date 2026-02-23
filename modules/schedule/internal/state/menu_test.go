package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rbright/waybar-schedule/internal/schedule"
)

func TestWriteMenu_ContainsExpectedActions(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	menuPath := filepath.Join(tmp, "schedule.xml")

	next := schedule.Occurrence{Title: "Daily Standup", Start: time.Now().Add(10 * time.Minute)}
	items := []schedule.Occurrence{
		{Title: "Daily Standup", Start: time.Now().Add(10 * time.Minute)},
	}

	if err := WriteMenu(menuPath, MenuData{StatusLine: "Upcoming meeting", Next: &next, Items: items}); err != nil {
		t.Fatalf("write menu: %v", err)
	}

	raw, err := os.ReadFile(menuPath)
	if err != nil {
		t.Fatalf("read menu: %v", err)
	}
	text := string(raw)

	for _, expected := range []string{"join_next", "join_1", "select_calendars", "refresh"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing menu action %q", expected)
		}
	}
}
