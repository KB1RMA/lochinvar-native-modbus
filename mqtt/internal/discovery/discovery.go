// Package discovery builds the MQTT topics and Home Assistant discovery payloads.
package discovery

import (
	"encoding/json"
	"fmt"

	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/entities"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/version"
)

// Availability payloads.
const (
	Online  = "online"
	Offline = "offline"
)

// Topics names every topic from the configured prefixes.
type Topics struct {
	Prefix          string
	DiscoveryPrefix string
	DeviceID        string
}

// DefaultTopics are the defaults of the command line options.
func DefaultTopics() Topics {
	return Topics{Prefix: "lochinvar", DiscoveryPrefix: "homeassistant", DeviceID: "lochinvar"}
}

// Status is the bridge's availability (MQTT last will).
func (t Topics) Status() string { return t.Prefix + "/status" }

// Link is whether the boiler answers.
func (t Topics) Link() string { return t.Prefix + "/link" }

// HAStatus is Home Assistant's birth topic; discovery is republished when it comes online.
func (t Topics) HAStatus() string { return t.DiscoveryPrefix + "/status" }

// State is an entity's state topic.
func (t Topics) State(e entities.Entity) string {
	switch e.Key {
	case entities.ControllerOnline:
		return t.Status()
	case entities.LinkOK:
		return t.Link()
	default:
		return fmt.Sprintf("%s/%s/%s/state", t.Prefix, e.Component, e.Key)
	}
}

// Config is an entity's discovery topic.
func (t Topics) Config(e entities.Entity) string {
	return fmt.Sprintf("%s/%s/%s/%s/config", t.DiscoveryPrefix, e.Component, t.DeviceID, e.Key)
}

// Device is the discovery "device" block.
type Device struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model"`
	SWVersion    string   `json:"sw_version"`
}

// Origin is the discovery "origin" block.
type Origin struct {
	Name       string `json:"name"`
	SWVersion  string `json:"sw_version"`
	SupportURL string `json:"support_url"`
}

// Availability is one availability topic.
type Availability struct {
	Topic string `json:"topic"`
}

// Payload is a discovery config message.
type Payload struct {
	Name             string         `json:"name"`
	UniqueID         string         `json:"unique_id"`
	StateTopic       string         `json:"state_topic"`
	Device           Device         `json:"device"`
	Origin           Origin         `json:"origin"`
	PayloadOn        string         `json:"payload_on,omitempty"`
	PayloadOff       string         `json:"payload_off,omitempty"`
	Availability     []Availability `json:"availability,omitempty"`
	AvailabilityMode string         `json:"availability_mode,omitempty"`
	Unit             string         `json:"unit_of_measurement,omitempty"`
	DeviceClass      string         `json:"device_class,omitempty"`
	StateClass       string         `json:"state_class,omitempty"`
	Precision        *int           `json:"suggested_display_precision,omitempty"`
	EntityCategory   string         `json:"entity_category,omitempty"`
	Icon             string         `json:"icon,omitempty"`
	EnabledByDefault *bool          `json:"enabled_by_default,omitempty"`
}

// NewPayload builds the discovery config of one entity.
func NewPayload(e entities.Entity, t Topics, deviceName string) Payload {
	p := Payload{
		Name:       e.Name,
		UniqueID:   t.DeviceID + "_" + e.Key,
		StateTopic: t.State(e),
		Device: Device{
			Identifiers:  []string{t.DeviceID},
			Name:         deviceName,
			Manufacturer: "Lochinvar",
			Model:        "Controller native Modbus port",
			SWVersion:    version.Version,
		},
		Origin: Origin{
			Name:       "lochinvar-native-modbus-mqtt",
			SWVersion:  version.Version,
			SupportURL: "https://github.com/KB1RMA/lochinvar-native-modbus",
		},
		Unit:           e.Unit,
		DeviceClass:    e.DeviceClass,
		StateClass:     e.StateClass,
		EntityCategory: e.EntityCategory,
		Icon:           e.Icon,
	}
	switch {
	case e.Key == entities.ControllerOnline || e.Key == entities.LinkOK:
		p.PayloadOn, p.PayloadOff = Online, Offline
	case e.Component == entities.BinarySensor:
		p.PayloadOn, p.PayloadOff = "ON", "OFF"
	}
	if e.Key != entities.ControllerOnline {
		p.Availability = []Availability{{Topic: t.Status()}}
		if e.Group == entities.Fast || e.Group == entities.Derived {
			p.Availability = append(p.Availability, Availability{Topic: t.Link()})
			p.AvailabilityMode = "all"
		}
	}
	if e.Component == entities.Sensor {
		p.Precision = e.Precision
	}
	if e.DisabledByDefault {
		disabled := false
		p.EnabledByDefault = &disabled
	}
	return p
}

// Message is one MQTT message.
type Message struct {
	Topic   string
	Payload string
}

// Messages returns the discovery config of every entity.
func Messages(t Topics, deviceName string) ([]Message, error) {
	out := make([]Message, 0, len(entities.All))
	for _, e := range entities.All {
		b, err := json.Marshal(NewPayload(e, t, deviceName))
		if err != nil {
			return nil, fmt.Errorf("discovery payload for %s: %w", e.Key, err)
		}
		out = append(out, Message{Topic: t.Config(e), Payload: string(b)})
	}
	return out, nil
}
