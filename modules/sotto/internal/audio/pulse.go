package audio

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jfreymuth/pulse"
	pulseproto "github.com/jfreymuth/pulse/proto"
)

type Device struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	State       string `json:"state"`
	Available   bool   `json:"available"`
	Muted       bool   `json:"muted"`
	Default     bool   `json:"default"`
}

func ListDevices(_ context.Context) ([]Device, error) {
	client, err := pulse.NewClient(
		pulse.ClientApplicationName("waybar-sotto"),
		pulse.ClientApplicationIconName("audio-input-microphone"),
	)
	if err != nil {
		return nil, fmt.Errorf("connect pulse server: %w", err)
	}
	defer client.Close()

	defaultSource, err := client.DefaultSource()
	if err != nil {
		return nil, fmt.Errorf("read default source: %w", err)
	}
	defaultID := defaultSource.ID()

	var sourceInfos pulseproto.GetSourceInfoListReply
	if err := client.RawRequest(&pulseproto.GetSourceInfoList{}, &sourceInfos); err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}

	devices := make([]Device, 0, len(sourceInfos))
	for _, source := range sourceInfos {
		if source == nil {
			continue
		}
		devices = append(devices, Device{
			ID:          source.SourceName,
			Description: source.Device,
			State:       sourceStateString(source.State),
			Available:   sourceAvailable(source),
			Muted:       source.Mute,
			Default:     source.SourceName == defaultID,
		})
	}

	return devices, nil
}

func FilterActiveInputDevices(devices []Device) []Device {
	active := make([]Device, 0, len(devices))
	for _, device := range devices {
		if !device.Available {
			continue
		}
		if isMonitorSource(device) {
			continue
		}
		active = append(active, device)
	}

	sort.SliceStable(active, func(i, j int) bool {
		left := strings.ToLower(DisplayName(active[i]))
		right := strings.ToLower(DisplayName(active[j]))
		if left == right {
			return strings.ToLower(active[i].ID) < strings.ToLower(active[j].ID)
		}
		return left < right
	})

	return active
}

func MatchConfiguredDevice(devices []Device, configuredInput string) (Device, bool) {
	input := strings.TrimSpace(strings.ToLower(configuredInput))

	var defaultDevice *Device
	var matchedByInput *Device

	for i := range devices {
		device := &devices[i]
		if device.Default {
			defaultDevice = device
		}
		if matchedByInput == nil && input != "" && input != "default" && deviceMatches(*device, input) {
			matchedByInput = device
		}
	}

	if input == "" || input == "default" {
		if defaultDevice != nil {
			return *defaultDevice, true
		}
		return Device{}, false
	}

	if matchedByInput != nil {
		return *matchedByInput, true
	}

	return Device{}, false
}

func DisplayName(device Device) string {
	description := strings.TrimSpace(device.Description)
	if description != "" {
		return description
	}
	return strings.TrimSpace(device.ID)
}

func deviceMatches(device Device, term string) bool {
	if term == "" {
		return false
	}
	id := strings.ToLower(device.ID)
	desc := strings.ToLower(device.Description)
	return strings.Contains(id, term) || strings.Contains(desc, term)
}

func isMonitorSource(device Device) bool {
	id := strings.ToLower(strings.TrimSpace(device.ID))
	description := strings.ToLower(strings.TrimSpace(device.Description))
	return strings.Contains(id, ".monitor") || strings.HasPrefix(description, "monitor of ")
}

func sourceStateString(state uint32) string {
	switch state {
	case 0:
		return "running"
	case 1:
		return "idle"
	case 2:
		return "suspended"
	default:
		return fmt.Sprintf("unknown(%d)", state)
	}
}

func sourceAvailable(source *pulseproto.GetSourceInfoReply) bool {
	if source == nil {
		return false
	}
	if len(source.Ports) == 0 {
		return true
	}
	for _, port := range source.Ports {
		if port.Name != source.ActivePortName {
			continue
		}
		// PulseAudio values: unknown=0, no=1, yes=2.
		return port.Available == 0 || port.Available == 2
	}
	return true
}
