package schedule

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/teambition/rrule-go"
)

func ExpandEvents(events []RawEvent, windowStart, windowEnd time.Time) []Occurrence {
	if len(events) == 0 {
		return nil
	}

	masters := make([]RawEvent, 0, len(events))
	singles := make([]RawEvent, 0, len(events))
	overrides := make(map[string]RawEvent)
	usedOverrides := make(map[string]bool)

	for _, event := range events {
		if strings.TrimSpace(event.UID) == "" {
			continue
		}

		if strings.TrimSpace(event.RRULE) != "" {
			masters = append(masters, event)
			continue
		}

		if strings.TrimSpace(event.RecurrenceID) != "" {
			key := overrideKeyForEvent(event)
			overrides[key] = event
			continue
		}

		singles = append(singles, event)
	}

	occurrences := make([]Occurrence, 0, len(events))

	for _, event := range singles {
		if !overlaps(event.Start, event.End, windowStart, windowEnd) {
			continue
		}
		occurrences = append(occurrences, occurrenceFromRaw(event, event.Start, event.End))
	}

	for _, master := range masters {
		duration := master.End.Sub(master.Start)
		if duration <= 0 {
			duration = 30 * time.Minute
		}

		starts := expandRRuleStarts(master, windowStart, windowEnd)
		for _, start := range starts {
			overrideKey := overrideKey(master.CalendarUID, master.UID, start)
			if override, ok := overrides[overrideKey]; ok {
				usedOverrides[overrideKey] = true
				overrideStart := override.Start
				overrideEnd := override.End
				if !overrideEnd.After(overrideStart) {
					overrideEnd = overrideStart.Add(duration)
				}
				if !overlaps(overrideStart, overrideEnd, windowStart, windowEnd) {
					continue
				}
				occurrences = append(occurrences, occurrenceFromRaw(override, overrideStart, overrideEnd))
				continue
			}

			end := start.Add(duration)
			if !overlaps(start, end, windowStart, windowEnd) {
				continue
			}
			occurrences = append(occurrences, occurrenceFromRaw(master, start, end))
		}
	}

	for key, override := range overrides {
		if usedOverrides[key] {
			continue
		}
		if !overlaps(override.Start, override.End, windowStart, windowEnd) {
			continue
		}
		occurrences = append(occurrences, occurrenceFromRaw(override, override.Start, override.End))
	}

	unique := dedupeOccurrences(occurrences)
	SortOccurrences(unique)
	return unique
}

func expandRRuleStarts(event RawEvent, windowStart, windowEnd time.Time) []time.Time {
	opt, err := rrule.StrToROption(event.RRULE)
	if err != nil {
		return fallbackStarts(event, windowStart, windowEnd, err)
	}

	opt.Dtstart = event.Start
	rule, err := rrule.NewRRule(*opt)
	if err != nil {
		return fallbackStarts(event, windowStart, windowEnd, err)
	}

	set := &rrule.Set{}
	set.RRule(rule)
	for _, exdate := range event.ExDates {
		set.ExDate(exdate)
	}
	for _, rdate := range event.RDates {
		set.RDate(rdate)
	}

	starts := set.Between(windowStart, windowEnd, true)
	if len(starts) == 0 && overlaps(event.Start, event.End, windowStart, windowEnd) {
		starts = append(starts, event.Start)
	}

	sort.Slice(starts, func(i, j int) bool {
		return starts[i].Before(starts[j])
	})
	return starts
}

func fallbackStarts(event RawEvent, windowStart, windowEnd time.Time, _ error) []time.Time {
	starts := make([]time.Time, 0, 1)
	if overlaps(event.Start, event.End, windowStart, windowEnd) {
		starts = append(starts, event.Start)
	}
	return starts
}

func overlaps(start, end, windowStart, windowEnd time.Time) bool {
	if end.IsZero() {
		end = start.Add(30 * time.Minute)
	}
	if !end.After(start) {
		end = start.Add(30 * time.Minute)
	}
	return start.Before(windowEnd) && end.After(windowStart)
}

func occurrenceFromRaw(event RawEvent, start, end time.Time) Occurrence {
	joinURL, eventURL, provider := DeriveLinks(event)
	title := sanitize(fallback(event.Summary, "Meeting"))
	return Occurrence{
		CalendarUID:     event.CalendarUID,
		CalendarName:    event.CalendarName,
		CalendarAccount: event.CalendarAccount,
		UID:             event.UID,
		Title:           title,
		Description:     sanitize(event.Description),
		Location:        sanitize(event.Location),
		Start:           start,
		End:             end,
		AllDay:          event.AllDay,
		JoinURL:         joinURL,
		EventURL:        eventURL,
		Provider:        provider,
	}
}

func dedupeOccurrences(items []Occurrence) []Occurrence {
	if len(items) == 0 {
		return nil
	}

	seen := make(map[string]Occurrence)
	for _, item := range items {
		key := occurrenceKey(item)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = item
	}

	results := make([]Occurrence, 0, len(seen))
	for _, item := range seen {
		results = append(results, item)
	}
	return results
}

func occurrenceKey(item Occurrence) string {
	return strings.Join([]string{
		item.CalendarUID,
		item.UID,
		item.Start.UTC().Format(time.RFC3339Nano),
		item.End.UTC().Format(time.RFC3339Nano),
		item.Title,
	}, "|")
}

func overrideKeyForEvent(event RawEvent) string {
	if event.RecurrenceAt != nil {
		return overrideKey(event.CalendarUID, event.UID, *event.RecurrenceAt)
	}
	return overrideKey(event.CalendarUID, event.UID, event.Start)
}

func overrideKey(calendarUID, uid string, start time.Time) string {
	return fmt.Sprintf("%s|%s|%s", calendarUID, uid, start.UTC().Format(time.RFC3339Nano))
}

func sanitize(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
