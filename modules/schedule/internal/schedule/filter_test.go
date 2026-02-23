package schedule

import (
	"testing"
	"time"
)

func TestMeetingOnly_FiltersNonMeetingEvents(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	items := []Occurrence{
		{Title: "No Link", Start: now.Add(10 * time.Minute), End: now.Add(40 * time.Minute), JoinURL: ""},
		{Title: "Meet", Start: now.Add(20 * time.Minute), End: now.Add(50 * time.Minute), JoinURL: "https://meet.google.com/abc-defg-hij"},
	}

	filtered := MeetingOnly(items)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 meeting occurrence, got %d", len(filtered))
	}
	if filtered[0].Title != "Meet" {
		t.Fatalf("unexpected filtered title: %q", filtered[0].Title)
	}
}

func TestUpcoming_RespectsWithinWindow(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	items := []Occurrence{
		{Title: "Soon", Start: now.Add(2 * time.Hour), End: now.Add(3 * time.Hour), JoinURL: "https://zoom.us/j/1"},
		{Title: "Tomorrow", Start: now.Add(26 * time.Hour), End: now.Add(27 * time.Hour), JoinURL: "https://zoom.us/j/2"},
	}

	upcoming := Upcoming(items, now, 24*time.Hour, 8, false)
	if len(upcoming) != 1 {
		t.Fatalf("expected 1 upcoming item in 24h, got %d", len(upcoming))
	}
	if upcoming[0].Title != "Soon" {
		t.Fatalf("unexpected upcoming title: %q", upcoming[0].Title)
	}
}

func TestHumanizeDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Duration
		out  string
	}{
		{name: "minutes", in: 24 * time.Minute, out: "24m"},
		{name: "hours_minutes", in: 4*time.Hour + 24*time.Minute, out: "4h 24m"},
		{name: "days_hours_minutes", in: 2*24*time.Hour + 3*time.Hour + 5*time.Minute, out: "2d 3h 5m"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := HumanizeDuration(tc.in); got != tc.out {
				t.Fatalf("HumanizeDuration() = %q, want %q", got, tc.out)
			}
		})
	}
}
