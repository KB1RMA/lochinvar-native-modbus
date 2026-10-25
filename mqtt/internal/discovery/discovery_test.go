package discovery_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/discovery"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/entities"
)

var topics = discovery.DefaultTopics()

func payload(t *testing.T, key string) discovery.Payload {
	t.Helper()
	e, ok := entities.ByKey(key)
	if !ok {
		t.Fatalf("no entity %q", key)
	}
	return discovery.NewPayload(e, topics, "Boiler")
}

func TestOneConfigPerEntityWithUniqueIDs(t *testing.T) {
	t.Parallel()
	msgs, err := discovery.Messages(topics, "Lochinvar Boiler")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != len(entities.All) {
		t.Fatalf("%d messages for %d entities", len(msgs), len(entities.All))
	}
	ids, seenTopics := map[string]bool{}, map[string]bool{}
	for _, m := range msgs {
		var p map[string]any
		if err := json.Unmarshal([]byte(m.Payload), &p); err != nil {
			t.Fatalf("%s: %v", m.Topic, err)
		}
		id, _ := p["unique_id"].(string)
		if ids[id] || seenTopics[m.Topic] {
			t.Errorf("duplicate unique_id %q or topic %q", id, m.Topic)
		}
		ids[id], seenTopics[m.Topic] = true, true
	}
}

func TestFastEntitiesNeedTheLink(t *testing.T) {
	t.Parallel()
	p := payload(t, "outlet_temp")
	want := []discovery.Availability{{Topic: "lochinvar/status"}, {Topic: "lochinvar/link"}}
	if !slices.Equal(p.Availability, want) || p.AvailabilityMode != "all" {
		t.Errorf("availability %v mode %q", p.Availability, p.AvailabilityMode)
	}
	if p.Unit != "°F" || p.DeviceClass != "temperature" || p.Precision == nil || *p.Precision != 1 {
		t.Errorf("unexpected payload %+v", p)
	}
	if !slices.Equal(p.Device.Identifiers, []string{"lochinvar"}) {
		t.Errorf("device identifiers %v", p.Device.Identifiers)
	}
	if p.StateTopic != "lochinvar/sensor/outlet_temp/state" {
		t.Errorf("state topic %s", p.StateTopic)
	}
}

func TestSlowAndStatusEntitiesOnlyNeedTheBridge(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"setpoint_sh1", "failed_ignitions", "boiler_status", "link_ok", "link_reconnects"} {
		p := payload(t, key)
		if !slices.Equal(p.Availability, []discovery.Availability{{Topic: "lochinvar/status"}}) || p.AvailabilityMode != "" {
			t.Errorf("%s: availability %v mode %q", key, p.Availability, p.AvailabilityMode)
		}
	}
}

func TestLinkAndControllerSensorsFollowAvailabilityTopics(t *testing.T) {
	t.Parallel()
	online := payload(t, "controller_online")
	if online.StateTopic != "lochinvar/status" || online.Availability != nil ||
		online.PayloadOn != "online" || online.PayloadOff != "offline" {
		t.Errorf("controller online %+v", online)
	}
	if link := payload(t, "link_ok"); link.StateTopic != "lochinvar/link" || link.PayloadOn != "online" {
		t.Errorf("link ok %+v", link)
	}
}

func TestRawRegistersDisabledAndBinaryPayloads(t *testing.T) {
	t.Parallel()
	if p := payload(t, "raw_r138"); p.EnabledByDefault == nil || *p.EnabledByDefault {
		t.Error("raw_r138 should be disabled by default")
	}
	p := payload(t, "heat_demand")
	if p.PayloadOn != "ON" || p.PayloadOff != "OFF" || p.Precision != nil {
		t.Errorf("heat demand %+v", p)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"suggested_display_precision", "enabled_by_default", "unit_of_measurement"} {
		if _, ok := m[absent]; ok {
			t.Errorf("heat demand payload has %s", absent)
		}
	}
}
