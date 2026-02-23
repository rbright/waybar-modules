package schedule

import "testing"

func TestDeriveLinks_PrefersGoogleConference(t *testing.T) {
	t.Parallel()

	event := RawEvent{
		GoogleConferenceURL: "https://meet.google.com/abc-defg-hij",
		Description:         "Join here: https://zoom.us/j/123456",
		URL:                 "https://calendar.google.com/event?eid=123",
	}

	joinURL, eventURL, provider := DeriveLinks(event)
	if joinURL != "https://meet.google.com/abc-defg-hij" {
		t.Fatalf("joinURL mismatch: %s", joinURL)
	}
	if eventURL == "" {
		t.Fatalf("expected eventURL")
	}
	if provider != "google_meet" {
		t.Fatalf("provider mismatch: %s", provider)
	}
}

func TestDeriveLinks_PrefersZoomMeetingURL(t *testing.T) {
	t.Parallel()

	event := RawEvent{
		Description: "Resources: https://support.google.com/a/users/answer/9282720\nJoin Zoom https://us02web.zoom.us/j/555123",
	}

	joinURL, _, provider := DeriveLinks(event)
	if joinURL != "https://us02web.zoom.us/j/555123" {
		t.Fatalf("joinURL mismatch: %s", joinURL)
	}
	if provider != "zoom" {
		t.Fatalf("provider mismatch: %s", provider)
	}
}

func TestDeriveLinks_DoesNotTreatCalendarPageAsMeetingLink(t *testing.T) {
	t.Parallel()

	event := RawEvent{URL: "https://calendar.google.com/event?eid=abc"}
	joinURL, eventURL, provider := DeriveLinks(event)

	if joinURL != "" {
		t.Fatalf("expected no joinURL, got %q", joinURL)
	}
	if eventURL == "" {
		t.Fatalf("expected eventURL")
	}
	if provider != "" {
		t.Fatalf("expected empty provider, got %q", provider)
	}
}
