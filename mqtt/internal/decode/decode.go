// Package decode turns raw register values into readings. It mirrors esphome/packages/boiler.yaml;
// see registers/controller.yaml for the encodings.
package decode

import (
	"fmt"
	"strconv"
	"time"
)

// NoSensor is the raw value of a temperature input with nothing connected.
const NoSensor uint16 = 0x8000

// Status codes seen in r84. Lockout codes have not been observed yet.
const (
	StatusStandby   = 4
	StatusPrepurge  = 6
	StatusIgnition  = 7
	StatusRunning   = 9
	StatusPostPurge = 10
)

// IgnitionSettle is how long after prepurge/ignition counter reads are still skipped.
const IgnitionSettle = 30 * time.Second

type kind uint8

const (
	kindNone kind = iota
	kindNum
	kindBool
	kindText
)

// Value is one entity's state. The zero Value is "no reading" (ESPHome's NAN), published as "None".
type Value struct {
	kind kind
	num  float64
	on   bool
	text string
}

// None is no reading.
var None = Value{}

// Num is a numeric reading.
func Num(f float64) Value { return Value{kind: kindNum, num: f} }

// Bool is an on/off reading.
func Bool(b bool) Value { return Value{kind: kindBool, on: b} }

// Text is a text reading.
func Text(s string) Value { return Value{kind: kindText, text: s} }

// Valid reports whether v holds a reading.
func (v Value) Valid() bool { return v.kind != kindNone }

// Float returns a numeric reading.
func (v Value) Float() (float64, bool) { return v.num, v.kind == kindNum }

// IsOn returns an on/off reading.
func (v Value) IsOn() (on, ok bool) { return v.on, v.kind == kindBool }

// On is true only for an "on" reading.
func (v Value) On() bool { return v.kind == kindBool && v.on }

// String returns a text reading.
func (v Value) String() string {
	switch v.kind {
	case kindNum:
		return strconv.FormatFloat(v.num, 'g', -1, 64)
	case kindBool:
		return strconv.FormatBool(v.on)
	case kindText:
		return v.text
	default:
		return "None"
	}
}

// Func decodes one register.
type Func func(raw uint16) Value

func fromTemp(raw uint16) float64 {
	return (float64(raw&0x7FFF) - 9468) / 25
}

// TempF decodes live temperature sensors. 0x8000 means no sensor; 0 (-378.7 °F) came through once
// after a reboot and is never a real reading.
func TempF(raw uint16) Value {
	if raw == NoSensor || raw == 0 {
		return None
	}
	return Num(fromTemp(raw))
}

// SetpointF decodes parameters: the same formula, but boiler.yaml only drops 0 for them.
func SetpointF(raw uint16) Value {
	if raw == 0 {
		return None
	}
	return Num(fromTemp(raw))
}

// Scaled multiplies by factor.
func Scaled(factor float64) Func {
	return func(raw uint16) Value { return Num(float64(raw) * factor) }
}

// U16 is the raw value.
func U16(raw uint16) Value { return Num(float64(raw)) }

// Bit is on when any bit of mask is set (off, if inverted).
func Bit(mask uint16, invert bool) Func {
	return func(raw uint16) Value { return Bool((raw&mask != 0) != invert) }
}

// U32 decodes 32-bit counters, high word at the lower address.
func U32(hi, lo uint16) Value {
	return Num(float64(uint32(hi)<<16 | uint32(lo)))
}

// DeltaT is the display's Delta T: outlet - inlet (r62 - r73).
func DeltaT(outlet, inlet Value) Value {
	o, ok1 := outlet.Float()
	i, ok2 := inlet.Float()
	if !ok1 || !ok2 {
		return None
	}
	return Num(o - i)
}

// FlamePresent is on above 0.5 µA of flame signal.
func FlamePresent(flameSignal Value) Value {
	f, ok := flameSignal.Float()
	if !ok {
		return None
	}
	return Bool(f > 0.5)
}

// Status is the latest r84 reading as an int, if any.
func Status(v Value) (int, bool) {
	f, ok := v.Float()
	return int(f), ok
}

// BoilerStatus is the Boiler Status text. r84 = 4 covers several display states; they are split the
// way tools/analysis/backtest.py does.
func BoilerStatus(linkOK bool, status, antiCycle, heatDemand, burnerRequest, pumpOutput Value) string {
	if !linkOK {
		return "Link lost"
	}
	code, ok := Status(status)
	if !ok {
		return "Unavailable"
	}
	switch code {
	case StatusStandby:
		pump, _ := pumpOutput.Float()
		switch {
		case antiCycle.On():
			return "Anti-cycle"
		case heatDemand.On() && !burnerRequest.On():
			return "Demand, held off"
		case heatDemand.On():
			return "Demand"
		case pump > 0:
			return "Pump delay"
		default:
			return "Standby"
		}
	case StatusPrepurge:
		return "Prepurge"
	case StatusIgnition:
		return "Ignition"
	case StatusRunning:
		return "Running"
	case StatusPostPurge:
		return "Post-purge"
	default:
		return fmt.Sprintf("Unknown (%d)", code)
	}
}

// IgnitionGuard protects Failed Ignitions (attempts - ignitions). The controller counts the attempt
// during prepurge/ignition and the ignition a few seconds later, once the flame is proven; a read in
// between would make Failed Ignitions go up by one until the next slow poll. Such reads are skipped,
// and the last value held.
type IgnitionGuard struct {
	lastIgnitionPhase time.Time
}

func inIgnition(code int) bool { return code == StatusPrepurge || code == StatusIgnition }

// NoteStatus records each Status Code reading.
func (g *IgnitionGuard) NoteStatus(status Value, now time.Time) {
	if code, ok := Status(status); ok && inIgnition(code) {
		g.lastIgnitionPhase = now
	}
}

// CountersClean reports whether counters read now, with this latest status, can be trusted.
func (g *IgnitionGuard) CountersClean(status Value, now time.Time) bool {
	code, ok := Status(status)
	if !ok || inIgnition(code) {
		return false
	}
	return g.lastIgnitionPhase.IsZero() || now.Sub(g.lastIgnitionPhase) >= IgnitionSettle
}

// FailedIgnitions is lifetime attempts minus successes.
func FailedIgnitions(attempts, ignitions Value) Value {
	a, ok1 := attempts.Float()
	i, ok2 := ignitions.Float()
	if !ok1 || !ok2 {
		return None
	}
	return Num(a - i)
}
