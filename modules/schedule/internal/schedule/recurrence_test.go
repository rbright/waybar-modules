package schedule

import (
	"testing"
	"time"
)

func TestExpandEvents_WeeklyWithExdate(t *testing.T) {
	t.Parallel()

	loc := time.UTC
	start := time.Date(2026, 2, 2, 10, 0, 0, 0, loc)
	end := start.Add(30 * time.Minute)
	exdate := start.Add(7 * 24 * time.Hour)

	events := []RawEvent{
		{
			CalendarUID: "cal",
			UID:         "event-1",
			Summary:     "Weekly",
			Start:       start,
			End:         end,
			RRULE:       "FREQ=WEEKLY;COUNT=3;BYDAY=MO",
			ExDates:     []time.Time{exdate},
		},
	}

	windowStart := start.Add(-time.Hour)
	windowEnd := start.Add(22 * 24 * time.Hour)
	occurrences := ExpandEvents(events, windowStart, windowEnd)

	if len(occurrences) != 2 {
		t.Fatalf("expected 2 occurrences, got %d", len(occurrences))
	}
	if !occurrences[0].Start.Equal(start) {
		t.Fatalf("unexpected first occurrence start: %v", occurrences[0].Start)
	}
	third := start.Add(14 * 24 * time.Hour)
	if !occurrences[1].Start.Equal(third) {
		t.Fatalf("unexpected second occurrence start: %v", occurrences[1].Start)
	}
}

func TestExpandEvents_RecurrenceOverride(t *testing.T) {
	t.Parallel()

	loc := time.UTC
	start := time.Date(2026, 3, 2, 9, 0, 0, 0, loc)
	second := start.Add(7 * 24 * time.Hour)

	secondRecurrence := second
	events := []RawEvent{
		{
			CalendarUID: "cal",
			UID:         "event-2",
			Summary:     "Sync",
			Start:       start,
			End:         start.Add(30 * time.Minute),
			RRULE:       "FREQ=WEEKLY;COUNT=2;BYDAY=MO",
		},
		{
			CalendarUID:  "cal",
			UID:          "event-2",
			Summary:      "Sync (moved)",
			RecurrenceID: second.Format("20060102T150405Z"),
			RecurrenceAt: &secondRecurrence,
			Start:        second.Add(15 * time.Minute),
			End:          second.Add(45 * time.Minute),
		},
	}

	windowStart := start.Add(-time.Hour)
	windowEnd := start.Add(14 * 24 * time.Hour)
	occurrences := ExpandEvents(events, windowStart, windowEnd)

	if len(occurrences) != 2 {
		t.Fatalf("expected 2 occurrences, got %d", len(occurrences))
	}
	if occurrences[1].Title != "Sync (moved)" {
		t.Fatalf("expected override title, got %q", occurrences[1].Title)
	}
}
