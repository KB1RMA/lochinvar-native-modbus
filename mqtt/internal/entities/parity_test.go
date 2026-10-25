package entities_test

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/entities"
)

// The bridge exposes the same entities, with the same decoding, as esphome/packages/boiler.yaml.

// renamed lists entities that differ on purpose (see entities.go).
var renamed = map[string]string{"Link Watchdog Restarts": "Link Reconnects"}

// espFilter is one ESPHome filter, e.g. {multiply: 0.01} or {invert: null}.
type espFilter map[string]any

type espEntity struct {
	Platform          string      `yaml:"platform"`
	Name              string      `yaml:"name"`
	ControllerID      string      `yaml:"modbus_controller_id"`
	Address           uint16      `yaml:"address"`
	ValueType         string      `yaml:"value_type"`
	Bitmask           uint16      `yaml:"bitmask"`
	Lambda            string      `yaml:"lambda"`
	Filters           []espFilter `yaml:"filters"`
	Unit              string      `yaml:"unit_of_measurement"`
	DeviceClass       string      `yaml:"device_class"`
	StateClass        string      `yaml:"state_class"`
	AccuracyDecimals  *int        `yaml:"accuracy_decimals"`
	EntityCategory    string      `yaml:"entity_category"`
	Icon              string      `yaml:"icon"`
	DisabledByDefault bool        `yaml:"disabled_by_default"`
	component         entities.Component
}

func loadESPHome(t *testing.T) []espEntity {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "esphome", "packages", "boiler.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	var out []espEntity
	for _, c := range []entities.Component{entities.Sensor, entities.BinarySensor, entities.TextSensor} {
		node, ok := raw[string(c)]
		if !ok {
			continue
		}
		var list []espEntity
		if err := node.Decode(&list); err != nil {
			t.Fatalf("%s: %v", c, err)
		}
		for i := range list {
			list[i].component = c
		}
		out = append(out, list...)
	}
	return out
}

func TestSameSetOfEntities(t *testing.T) {
	t.Parallel()
	esp := map[string]bool{}
	for _, e := range loadESPHome(t) {
		name := e.Name
		if r, ok := renamed[name]; ok {
			name = r
		}
		esp[name] = true
	}
	ours := map[string]bool{}
	for _, e := range entities.All {
		ours[e.Name] = true
		if !esp[e.Name] {
			t.Errorf("%q is not in boiler.yaml", e.Name)
		}
	}
	for name := range esp {
		if !ours[name] {
			t.Errorf("%q from boiler.yaml is missing", name)
		}
	}
}

func byName(t *testing.T, name string) entities.Entity {
	t.Helper()
	if r, ok := renamed[name]; ok {
		name = r
	}
	for _, e := range entities.All {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("no entity %q", name)
	return entities.Entity{}
}

func TestEntitiesMatchESPHome(t *testing.T) {
	t.Parallel()
	for _, esp := range loadESPHome(t) {
		t.Run(esp.Name, func(t *testing.T) {
			t.Parallel()
			checkEntity(t, esp, byName(t, esp.Name))
		})
	}
}

func checkEntity(t *testing.T, esp espEntity, ours entities.Entity) {
	t.Helper()
	// ESPHome's status platform is a diagnostic connectivity sensor.
	if esp.Platform == "status" {
		esp.DeviceClass, esp.EntityCategory = "connectivity", "diagnostic"
	}
	eq := func(field string, got, want any) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %v, boiler.yaml has %v", field, got, want)
		}
	}
	eq("component", ours.Component, esp.component)
	eq("unit", ours.Unit, esp.Unit)
	eq("device_class", ours.DeviceClass, esp.DeviceClass)
	eq("state_class", ours.StateClass, esp.StateClass)
	eq("entity_category", ours.EntityCategory, esp.EntityCategory)
	eq("icon", ours.Icon, esp.Icon)
	eq("disabled_by_default", ours.DisabledByDefault, esp.DisabledByDefault)
	if esp.component == entities.Sensor {
		if (ours.Precision == nil) != (esp.AccuracyDecimals == nil) ||
			(ours.Precision != nil && *ours.Precision != *esp.AccuracyDecimals) {
			t.Errorf("precision = %v, boiler.yaml accuracy_decimals = %v", ours.Precision, esp.AccuracyDecimals)
		}
	}
	if esp.Platform != "modbus_controller" {
		eq("modbus", ours.IsModbus(), false)
		return
	}

	eq("address", ours.Address, esp.Address)
	group := map[string]entities.Group{"boiler": entities.Fast, "boiler_slow": entities.Slow}[esp.ControllerID]
	eq("group", ours.Group, group)
	words := 1
	if esp.ValueType == "U_DWORD" {
		words = 2
	}
	eq("words", ours.Words, words)
	if words == 2 {
		return
	}

	switch {
	case esp.Bitmask != 0:
		inverted := false
		for _, f := range esp.Filters {
			_, inv := f["invert"]
			inverted = inverted || inv
		}
		eq("bit set", ours.Decode(esp.Bitmask).On(), !inverted)
		eq("bit clear", ours.Decode(0).On(), inverted)
	case esp.Lambda != "":
		// Temperature lambdas: live sensors drop 0x8000 and 0, setpoints only 0.
		if !strings.Contains(esp.Lambda, "- 9468) / 25") {
			t.Fatalf("unexpected lambda %q", esp.Lambda)
		}
		got, ok := ours.Decode(150*25 + 9468).Float()
		eq("150 °F decodes", ok && math.Abs(got-150) < 1e-9, true)
		eq("0 dropped", ours.Decode(0).Valid(), false)
		eq("0x8000 dropped", !ours.Decode(0x8000).Valid(), strings.Contains(esp.Lambda, "32768"))
	default:
		factor := 1.0
		for _, f := range esp.Filters {
			switch m := f["multiply"].(type) {
			case float64:
				factor *= m
			case int:
				factor *= float64(m)
			case nil:
			default:
				t.Fatalf("unexpected multiply %v", m)
			}
		}
		got, ok := ours.Decode(1234).Float()
		eq("scale", ok && math.Abs(got-1234*factor) < 1e-9, true)
	}
}
