// Package entities lists what the bridge publishes, matching esphome/packages/boiler.yaml (names,
// units, classes, precision). The tests compare the two and check every address against
// registers/controller.yaml.
package entities

import "github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/decode"

// Component is the Home Assistant entity platform.
type Component string

// Components used.
const (
	Sensor       Component = "sensor"
	BinarySensor Component = "binary_sensor"
	TextSensor   Component = "text_sensor"
)

// Group is when an entity is read and how it behaves while the boiler link is down.
type Group string

// Groups.
const (
	// Fast entities are read every 5 s and cleared (and unavailable) while the link is down.
	Fast Group = "fast"
	// Slow entities (settings and counters) are read every 5 min and keep their value while the link is down.
	Slow Group = "slow"
	// Derived entities are computed from fast readings and cleared with them.
	Derived Group = "derived"
	// Bridge entities describe the bridge itself and are available while it runs.
	Bridge Group = "bridge"
)

// Keys of entities the bridge computes or refers to by name.
const (
	OutletTemp       = "outlet_temp"
	InletTemp        = "inlet_temp"
	ExhaustTemp      = "exhaust_temp"
	OutletSensorDiff = "outlet_sensor_diff"
	FlueSensorDiff   = "flue_sensor_diff"
	SH1ResetTarget   = "sh1_reset_target"
	FlameSignal      = "flame_signal"
	BoilerPumpOutput = "boiler_pump_output"
	StatusCode       = "status_code"
	HeatDemand       = "heat_demand"
	BurnerRequest    = "burner_request"
	AntiCycle        = "anti_cycle"
	DeltaT           = "delta_t"
	FlamePresent     = "flame_present"
	IgnitionAttempts = "ignition_attempts"
	Ignitions        = "ignitions"
	FailedIgnitions  = "failed_ignitions"
	BoilerStatus     = "boiler_status"
	ControllerOnline = "controller_online"
	LinkOK           = "link_ok"
	LinkReconnects   = "link_reconnects"
)

// Entity is one Home Assistant entity.
type Entity struct {
	Key       string
	Name      string
	Component Component
	Group     Group
	// Address is the register read (the high word for 32-bit counters); 0 for computed entities.
	Address uint16
	// Words is 2 for 32-bit counters (decoded with decode.U32), else 1.
	Words  int
	Decode decode.Func
	// Combine, if set, joins the decoded reading with the With entity's in the same poll (a plain
	// fast entity), the way the boiler.yaml lambda does with id(With).
	Combine Combine
	With    string

	Unit              string
	DeviceClass       string
	StateClass        string
	Precision         *int
	EntityCategory    string
	Icon              string
	DisabledByDefault bool
}

// IsModbus reports whether the entity is read from a register.
func (e Entity) IsModbus() bool { return e.Address != 0 }

// Combine is how a register entity's reading is joined with another one's.
type Combine string

// Combinations.
const (
	// AbsDiff is |With - this|: the two thermistors of a dual sensor.
	AbsDiff Combine = "abs_diff"
	// WhileOn is this reading while With reads on, else no reading.
	WhileOn Combine = "while_on"
)

// Apply joins v with w, the With entity's reading.
func (c Combine) Apply(w, v decode.Value) decode.Value {
	switch c {
	case AbsDiff:
		return decode.SensorDiff(w, v)
	case WhileOn:
		return decode.While(w, v)
	}
	return v
}

func prec(n int) *int { return &n }

func disabled(e Entity) Entity {
	e.DisabledByDefault = true
	return e
}

const (
	degF        = "°F"
	measurement = "measurement"
	total       = "total_increasing"
	diagnostic  = "diagnostic"
	temperature = "temperature"
)

func liveTemp(key, name string, address uint16) Entity {
	return Entity{
		Key: key, Name: name, Component: Sensor, Group: Fast, Address: address, Words: 1,
		Decode: decode.TempF, Unit: degF, DeviceClass: temperature, StateClass: measurement, Precision: prec(1),
	}
}

func setpoint(key, name string, address uint16) Entity {
	return Entity{
		Key: key, Name: name, Component: Sensor, Group: Slow, Address: address, Words: 1,
		Decode: decode.SetpointF, Unit: degF, DeviceClass: temperature, Precision: prec(0), EntityCategory: diagnostic,
	}
}

func counter(key, name string, address uint16, unit string) Entity {
	return Entity{
		Key: key, Name: name, Component: Sensor, Group: Slow, Address: address, Words: 2,
		Unit: unit, StateClass: total, Precision: prec(0), EntityCategory: diagnostic,
	}
}

func raw(key, name string, address uint16) Entity {
	return Entity{
		Key: key, Name: name, Component: Sensor, Group: Fast, Address: address, Words: 1,
		Decode: decode.U16, Precision: prec(0), EntityCategory: diagnostic, DisabledByDefault: true,
	}
}

// All is every entity, in publishing order.
var All = []Entity{
	// ---- fast group ----
	liveTemp(OutletTemp, "Outlet Temperature", 62),
	liveTemp(InletTemp, "Inlet Temperature", 73),
	liveTemp(ExhaustTemp, "Exhaust Temperature", 63),
	liveTemp("outdoor_temp", "Outdoor Temperature", 75),
	// The second thermistor of each dual sensor (S1b, S3b), published as the difference from the first
	// one (r62, r63).
	{
		Key: OutletSensorDiff, Name: "Outlet Sensor Difference", Component: Sensor, Group: Fast, Address: 64, Words: 1,
		Decode: decode.TempF, Combine: AbsDiff, With: OutletTemp,
		Unit: degF, DeviceClass: "temperature_delta", StateClass: measurement, Precision: prec(1),
		EntityCategory: diagnostic, Icon: "mdi:thermometer-alert",
	},
	{
		Key: FlueSensorDiff, Name: "Flue Sensor Difference", Component: Sensor, Group: Fast, Address: 65, Words: 1,
		Decode: decode.TempF, Combine: AbsDiff, With: ExhaustTemp,
		Unit: degF, DeviceClass: "temperature_delta", StateClass: measurement, Precision: prec(1),
		EntityCategory: diagnostic, Icon: "mdi:thermometer-alert",
	},
	{
		Key: "target_temp", Name: "Target Temperature", Component: Sensor, Group: Fast, Address: 146, Words: 1,
		Decode: decode.TargetF, Unit: degF, DeviceClass: temperature, StateClass: measurement, Precision: prec(1),
		Icon: "mdi:thermometer-check",
	},
	// Published only while Heat Demand is on: the control stops updating it with no call.
	{
		Key: SH1ResetTarget, Name: "SH1 Reset Target", Component: Sensor, Group: Fast, Address: 142, Words: 1,
		Decode: decode.TempF, Combine: WhileOn, With: HeatDemand,
		Unit: degF, DeviceClass: temperature, StateClass: measurement, Precision: prec(1), EntityCategory: diagnostic,
	},
	{
		Key: FlameSignal, Name: "Flame Signal", Component: Sensor, Group: Fast, Address: 66, Words: 1,
		Decode: decode.Scaled(0.001), Unit: "µA", StateClass: measurement, Precision: prec(1),
	},
	{
		Key: "fan_speed", Name: "Fan Speed", Component: Sensor, Group: Fast, Address: 110, Words: 1,
		Decode: decode.U16, Unit: "rpm", StateClass: measurement, Precision: prec(0),
	},
	{
		Key: "firing_rate", Name: "Firing Rate", Component: Sensor, Group: Fast, Address: 80, Words: 1,
		Decode: decode.Scaled(0.01), Unit: "%", StateClass: measurement, Precision: prec(1),
	},
	{
		Key: BoilerPumpOutput, Name: "Boiler Pump Output", Component: Sensor, Group: Fast, Address: 132, Words: 1,
		Decode: decode.Scaled(0.01), Unit: "%", StateClass: measurement, Precision: prec(0),
	},
	{
		Key: StatusCode, Name: "Status Code", Component: Sensor, Group: Fast, Address: 84, Words: 1,
		Decode: decode.U16, Precision: prec(0), EntityCategory: diagnostic,
	},
	raw("raw_r138", "Raw State r138", 138),
	raw("raw_r139", "Raw State r139", 139),
	raw("raw_r140", "Raw State r140", 140),
	{
		Key: HeatDemand, Name: "Heat Demand", Component: BinarySensor, Group: Fast, Address: 118, Words: 1,
		Decode: decode.Bit(0x0001, false), Icon: "mdi:thermostat",
	},
	// r140 = 96 when the burner is wanted, 112 when held off: bit 4 set = held off, so inverted.
	{
		Key: BurnerRequest, Name: "Burner Request", Component: BinarySensor, Group: Fast, Address: 140, Words: 1,
		Decode: decode.Bit(0x0010, true),
	},
	{
		Key: AntiCycle, Name: "Anti-Cycle", Component: BinarySensor, Group: Fast, Address: 137, Words: 1,
		Decode: decode.Bit(0x0001, false), Icon: "mdi:timer-sand",
	},

	// ---- derived from the fast group ----
	{
		Key: DeltaT, Name: "Delta T", Component: Sensor, Group: Derived, Unit: degF, DeviceClass: "temperature_delta",
		StateClass: measurement, Precision: prec(1), Icon: "mdi:thermometer-lines",
	},
	{Key: FlamePresent, Name: "Flame Present", Component: BinarySensor, Group: Derived, Icon: "mdi:fire"},

	// ---- slow group: settings ----
	setpoint("setpoint_sh1", "SH1 Setpoint", 1287),
	setpoint("setpoint_sh2", "SH2 Setpoint", 1288),
	setpoint("setpoint_sh3", "SH3 Setpoint", 1289),
	setpoint("setpoint_dhw", "DHW Setpoint", 1286),
	setpoint("high_limit_auto_reset", "High Limit Auto Reset", 1291),
	setpoint("high_limit_manual_reset", "High Limit Manual Reset", 1292),

	// ---- slow group: lifetime counters ----
	counter("power_on_hours", "Power On Hours", 1302, "h"),
	counter(IgnitionAttempts, "Ignition Attempts", 1304, ""),
	counter(Ignitions, "Ignitions", 1306, ""),
	counter("sh_run_time", "SH Run Time", 1308, "h"),
	// Map confidence "guess" (WinPro "SH hours <50 %").
	disabled(counter("sh_run_time_low_fire", "SH Run Time Low Fire", 1310, "h")),
	counter("sh_cycles", "SH Cycles", 1312, ""),
	// Lifetime attempts minus successes. Held while the counters were read during an ignition.
	{
		Key: FailedIgnitions, Name: "Failed Ignitions", Component: Sensor, Group: Slow, StateClass: total,
		Precision: prec(0), EntityCategory: diagnostic,
	},

	// ---- status and the bridge ----
	{Key: BoilerStatus, Name: "Boiler Status", Component: TextSensor, Group: Bridge},
	{
		Key: ControllerOnline, Name: "Controller Online", Component: BinarySensor, Group: Bridge,
		DeviceClass: "connectivity", EntityCategory: diagnostic,
	},
	{
		Key: LinkOK, Name: "Boiler Link OK", Component: BinarySensor, Group: Bridge, DeviceClass: "connectivity",
		EntityCategory: diagnostic,
	},
	// The ESP restarts itself after 3 min of a dead link ("Link Watchdog Restarts"); the bridge reopens
	// the serial port instead and counts that here (since it started).
	{
		Key: LinkReconnects, Name: "Link Reconnects", Component: Sensor, Group: Bridge, StateClass: total,
		Precision: prec(0), EntityCategory: diagnostic, Icon: "mdi:restart-alert",
	},
}

var byKey = func() map[string]Entity {
	m := make(map[string]Entity, len(All))
	for _, e := range All {
		m[e.Key] = e
	}
	return m
}()

// ByKey returns the entity with this key.
func ByKey(key string) (Entity, bool) {
	e, ok := byKey[key]
	return e, ok
}

var byGroup = func() map[Group][]Entity {
	m := map[Group][]Entity{}
	for _, e := range All {
		m[e.Group] = append(m[e.Group], e)
	}
	return m
}()

// InGroup returns the entities of one group, in order. The slice is shared; don't modify it.
func InGroup(g Group) []Entity { return byGroup[g] }
