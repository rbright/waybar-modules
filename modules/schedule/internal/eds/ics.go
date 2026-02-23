package eds

import (
	"fmt"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/rbright/waybar-schedule/internal/schedule"
)

func parseEventPayload(calendar schedule.Calendar, payload string) ([]schedule.RawEvent, error) {
	wrapped := "BEGIN:VCALENDAR\n" + strings.TrimSpace(payload) + "\nEND:VCALENDAR\n"
	parsed, err := ics.ParseCalendar(strings.NewReader(wrapped))
	if err != nil {
		return nil, fmt.Errorf("parse ics payload: %w", err)
	}

	events := parsed.Events()
	if len(events) == 0 {
		return nil, nil
	}

	results := make([]schedule.RawEvent, 0, len(events))
	for _, event := range events {
		raw, mapErr := mapEvent(calendar, event)
		if mapErr != nil {
			continue
		}
		results = append(results, raw)
	}

	return results, nil
}

func mapEvent(calendar schedule.Calendar, event *ics.VEvent) (schedule.RawEvent, error) {
	start, err := event.GetStartAt()
	if err != nil {
		return schedule.RawEvent{}, err
	}

	end, err := event.GetEndAt()
	if err != nil || !end.After(start) {
		end = start.Add(30 * time.Minute)
	}

	uid := propertyValue(event.GetProperty(ics.ComponentPropertyUniqueId))
	if strings.TrimSpace(uid) == "" {
		uid = strings.TrimSpace(propertyValue(event.GetProperty(ics.ComponentPropertySummary)))
	}

	allDay := isAllDay(event.GetProperty(ics.ComponentPropertyDtStart))

	recurrenceIDProp := event.GetProperty(ics.ComponentPropertyRecurrenceId)
	recurrenceID := propertyValue(recurrenceIDProp)
	var recurrenceAt *time.Time
	if recurrenceIDProp != nil {
		if parsedRecurrence, parseErr := parseICSTimeValue(recurrenceIDProp.Value, recurrenceIDProp.ICalParameters); parseErr == nil {
			recurrenceAt = &parsedRecurrence
		}
	}

	rdates := collectDateTimes(event.GetProperties(ics.ComponentPropertyRdate))
	exdates := collectDateTimes(event.GetProperties(ics.ComponentPropertyExdate))

	googleConference := propertyValue(event.GetProperty(ics.ComponentProperty("X-GOOGLE-CONFERENCE")))

	return schedule.RawEvent{
		CalendarUID:         calendar.UID,
		CalendarName:        calendar.Name,
		CalendarAccount:     calendar.AccountName,
		UID:                 sanitize(uid),
		RecurrenceID:        strings.TrimSpace(recurrenceID),
		RecurrenceAt:        recurrenceAt,
		Summary:             sanitize(propertyValue(event.GetProperty(ics.ComponentPropertySummary))),
		Description:         strings.TrimSpace(propertyValue(event.GetProperty(ics.ComponentPropertyDescription))),
		Location:            strings.TrimSpace(propertyValue(event.GetProperty(ics.ComponentPropertyLocation))),
		Status:              sanitize(propertyValue(event.GetProperty(ics.ComponentPropertyStatus))),
		Class:               sanitize(propertyValue(event.GetProperty(ics.ComponentPropertyClass))),
		Start:               start,
		End:                 end,
		AllDay:              allDay,
		RRULE:               strings.TrimSpace(propertyValue(event.GetProperty(ics.ComponentPropertyRrule))),
		RDates:              rdates,
		ExDates:             exdates,
		URL:                 strings.TrimSpace(propertyValue(event.GetProperty(ics.ComponentPropertyUrl))),
		GoogleConferenceURL: strings.TrimSpace(googleConference),
	}, nil
}

func collectDateTimes(properties []*ics.IANAProperty) []time.Time {
	if len(properties) == 0 {
		return nil
	}

	results := make([]time.Time, 0, len(properties))
	for _, property := range properties {
		if property == nil {
			continue
		}
		values := strings.Split(property.Value, ",")
		for _, value := range values {
			parsed, err := parseICSTimeValue(strings.TrimSpace(value), property.ICalParameters)
			if err != nil {
				continue
			}
			results = append(results, parsed)
		}
	}
	return results
}

func parseICSTimeValue(value string, params map[string][]string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("empty time value")
	}

	var location *time.Location
	if tzIDs, ok := params["TZID"]; ok && len(tzIDs) > 0 && strings.TrimSpace(tzIDs[0]) != "" {
		loaded, err := time.LoadLocation(strings.TrimSpace(tzIDs[0]))
		if err == nil {
			location = loaded
		}
	}

	layouts := []string{
		"20060102T150405Z",
		"20060102T1504Z",
		"20060102T150405",
		"20060102T1504",
		"20060102",
	}

	for _, layout := range layouts {
		if strings.HasSuffix(layout, "Z") {
			parsed, err := time.Parse(layout, trimmed)
			if err == nil {
				return parsed, nil
			}
			continue
		}

		loc := time.Local
		if location != nil {
			loc = location
		}

		parsed, err := time.ParseInLocation(layout, trimmed, loc)
		if err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time value %q", trimmed)
}

func isAllDay(property *ics.IANAProperty) bool {
	if property == nil {
		return false
	}
	if values, ok := property.ICalParameters["VALUE"]; ok && len(values) > 0 {
		for _, value := range values {
			if strings.EqualFold(strings.TrimSpace(value), "DATE") {
				return true
			}
		}
	}
	trimmed := strings.TrimSpace(property.Value)
	return len(trimmed) == 8
}

func propertyValue(property *ics.IANAProperty) string {
	if property == nil {
		return ""
	}
	return property.Value
}

func sanitize(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}
