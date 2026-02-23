package audio

import "testing"

func TestFilterActiveInputDevices(t *testing.T) {
	devices := []Device{
		{ID: "alsa_input.wave3", Description: "Elgato Wave 3 Mono", Available: true},
		{ID: "bluez_input.headset", Description: "Sony WH-1000XM6", Available: true},
		{ID: "alsa_output.sink.monitor", Description: "Monitor of Speakers", Available: true},
		{ID: "alsa_input.usb_mic", Description: "USB Audio Microphone", Available: false},
	}

	active := FilterActiveInputDevices(devices)
	if len(active) != 2 {
		t.Fatalf("expected 2 active inputs, got %d", len(active))
	}
	if active[0].Description != "Elgato Wave 3 Mono" {
		t.Fatalf("expected first item to be Elgato, got %q", active[0].Description)
	}
	if active[1].Description != "Sony WH-1000XM6" {
		t.Fatalf("expected second item to be Sony headset, got %q", active[1].Description)
	}
}

func TestMatchConfiguredDeviceDefault(t *testing.T) {
	devices := []Device{
		{ID: "alsa_input.wave3", Description: "Elgato Wave 3 Mono", Default: true},
		{ID: "bluez_input.headset", Description: "Sony WH-1000XM6"},
	}

	matched, ok := MatchConfiguredDevice(devices, "default")
	if !ok {
		t.Fatal("expected default device match")
	}
	if matched.ID != "alsa_input.wave3" {
		t.Fatalf("expected default ID alsa_input.wave3, got %q", matched.ID)
	}
}

func TestMatchConfiguredDeviceByDescription(t *testing.T) {
	devices := []Device{
		{ID: "alsa_input.wave3", Description: "Elgato Wave 3 Mono"},
		{ID: "bluez_input.headset", Description: "Sony WH-1000XM6"},
	}

	matched, ok := MatchConfiguredDevice(devices, "sony")
	if !ok {
		t.Fatal("expected configured device match")
	}
	if matched.ID != "bluez_input.headset" {
		t.Fatalf("expected Sony device ID, got %q", matched.ID)
	}
}
