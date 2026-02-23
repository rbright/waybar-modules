package schedule

import (
	"math"
	"sort"
	"strings"
	"time"
)

func SortOccurrences(items []Occurrence) {
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].Start.Equal(items[j].Start) {
			return items[i].Start.Before(items[j].Start)
		}
		if !strings.EqualFold(items[i].CalendarName, items[j].CalendarName) {
			return strings.ToLower(items[i].CalendarName) < strings.ToLower(items[j].CalendarName)
		}
		if !strings.EqualFold(items[i].Title, items[j].Title) {
			return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
		}
		return items[i].UID < items[j].UID
	})
}

func MeetingOnly(items []Occurrence) []Occurrence {
	if len(items) == 0 {
		return nil
	}

	filtered := make([]Occurrence, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.JoinURL) == "" {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func Upcoming(items []Occurrence, now time.Time, within time.Duration, maxItems int, includeAllDay bool) []Occurrence {
	if len(items) == 0 || maxItems <= 0 {
		return nil
	}

	windowEnd := now.Add(within)
	copyItems := make([]Occurrence, 0, len(items))
	for _, item := range items {
		if !includeAllDay && item.AllDay {
			continue
		}
		if !item.End.After(now) {
			continue
		}
		if item.Start.After(windowEnd) {
			continue
		}
		copyItems = append(copyItems, item)
	}

	SortOccurrences(copyItems)
	if len(copyItems) > maxItems {
		copyItems = copyItems[:maxItems]
	}
	return copyItems
}

func NextMeetingWithin(items []Occurrence, now time.Time, within time.Duration, includeAllDay bool) (Occurrence, bool) {
	if len(items) == 0 {
		return Occurrence{}, false
	}

	windowEnd := now.Add(within)
	candidates := make([]Occurrence, 0, len(items))
	for _, item := range items {
		if !includeAllDay && item.AllDay {
			continue
		}
		if !item.End.After(now) {
			continue
		}
		if item.Start.After(windowEnd) {
			continue
		}
		candidates = append(candidates, item)
	}

	if len(candidates) == 0 {
		return Occurrence{}, false
	}

	SortOccurrences(candidates)
	return candidates[0], true
}

func CountdownText(now time.Time, item Occurrence) string {
	if !item.Start.After(now) {
		return "now"
	}
	return HumanizeDuration(item.Start.Sub(now))
}

func HumanizeDuration(d time.Duration) string {
	if d <= 0 {
		return "now"
	}

	minutes := int(math.Ceil(d.Minutes()))
	if minutes < 0 {
		minutes = 0
	}

	days := minutes / (24 * 60)
	remaining := minutes % (24 * 60)
	hours := remaining / 60
	mins := remaining % 60

	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, strconvItoa(days)+"d")
	}
	if hours > 0 {
		parts = append(parts, strconvItoa(hours)+"h")
	}
	if mins > 0 {
		parts = append(parts, strconvItoa(mins)+"m")
	}
	if len(parts) == 0 {
		parts = append(parts, "0m")
	}
	return strings.Join(parts, " ")
}

func strconvItoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + strconvItoa(-n)
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + (n % 10))
		n /= 10
	}
	return string(buf[pos:])
}
