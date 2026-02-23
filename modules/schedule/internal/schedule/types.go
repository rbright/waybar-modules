package schedule

import "time"

type Calendar struct {
	UID         string `json:"uid"`
	Name        string `json:"name"`
	AccountName string `json:"accountName,omitempty"`
	ParentUID   string `json:"parentUid,omitempty"`
	Backend     string `json:"backend,omitempty"`
	Color       string `json:"color,omitempty"`
	Selected    bool   `json:"selected"`
	Enabled     bool   `json:"enabled"`
}

type RawEvent struct {
	CalendarUID     string
	CalendarName    string
	CalendarAccount string

	UID          string
	RecurrenceID string
	RecurrenceAt *time.Time

	Summary     string
	Description string
	Location    string
	Status      string
	Class       string

	Start  time.Time
	End    time.Time
	AllDay bool

	RRULE               string
	RDates              []time.Time
	ExDates             []time.Time
	URL                 string
	GoogleConferenceURL string
}

type Occurrence struct {
	CalendarUID     string    `json:"calendarUid"`
	CalendarName    string    `json:"calendarName"`
	CalendarAccount string    `json:"calendarAccount,omitempty"`
	UID             string    `json:"uid"`
	Title           string    `json:"title"`
	Description     string    `json:"description,omitempty"`
	Location        string    `json:"location,omitempty"`
	Start           time.Time `json:"start"`
	End             time.Time `json:"end"`
	AllDay          bool      `json:"allDay"`
	JoinURL         string    `json:"joinUrl,omitempty"`
	EventURL        string    `json:"eventUrl,omitempty"`
	Provider        string    `json:"provider,omitempty"`
}
