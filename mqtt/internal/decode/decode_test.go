package decode_test

import (
	"math"
	"testing"
	"time"

	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/decode"
)

func near(t *testing.T, name string, v decode.Value, want float64) {
	t.Helper()
	got, ok := v.Float()
	if !ok || math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", name, v, want)
	}
}

func none(t *testing.T, name string, v decode.Value) {
	t.Helper()
	if v.Valid() {
		t.Errorf("%s = %v, want no reading", name, v)
	}
}

func TestTempF(t *testing.T) {
	t.Parallel()
	// F = ((raw & 0x7FFF) - 9468) / 25; bit 15 is a flag.
	near(t, "12760", decode.TempF(12760), 131.68)
	near(t, "0x8000|12760", decode.TempF(0x8000|12760), 131.68)
	near(t, "43036", decode.TempF(43036), 32.0) // r146's "no setpoint" value
	none(t, "0x8000 (no sensor)", decode.TempF(0x8000))
	none(t, "0 (bogus first read after a reboot)", decode.TempF(0))
}

func TestTargetF(t *testing.T) {
	t.Parallel()
	near(t, "172 flagged", decode.TargetF(0x8000|(172*25+9468)), 172)
	none(t, "43036 (32.0 °F, no set point)", decode.TargetF(43036))
	none(t, "0x8000", decode.TargetF(0x8000))
	none(t, "0", decode.TargetF(0))
}

func TestSensorDiffAndWhile(t *testing.T) {
	t.Parallel()
	near(t, "diff", decode.SensorDiff(decode.Num(150), decode.Num(150.6)), 0.6)
	near(t, "diff reversed", decode.SensorDiff(decode.Num(150.6), decode.Num(150)), 0.6)
	none(t, "diff without second sensor", decode.SensorDiff(decode.Num(150), decode.None))
	near(t, "while on", decode.While(decode.Bool(true), decode.Num(172.1)), 172.1)
	none(t, "while off", decode.While(decode.Bool(false), decode.Num(172.1)))
	none(t, "while unknown", decode.While(decode.None, decode.Num(172.1)))
}

func TestSetpointOnlyDropsZero(t *testing.T) {
	t.Parallel()
	raw := uint16(180*25 + 9468)
	near(t, "180", decode.SetpointF(raw), 180)
	near(t, "180 flagged", decode.SetpointF(raw|0x8000), 180)
	none(t, "0", decode.SetpointF(0))
}

func TestScalesBitsU32(t *testing.T) {
	t.Parallel()
	near(t, "flame", decode.Scaled(0.001)(12688), 12.688)
	if !decode.Bit(0x10, true)(96).On() {
		t.Error("r140 = 96 should be burner wanted")
	}
	if on, ok := decode.Bit(0x10, true)(112).IsOn(); on || !ok {
		t.Error("r140 = 112 should be held off")
	}
	if !decode.Bit(0x01, false)(1).On() {
		t.Error("bit 0 set should be on")
	}
	near(t, "u32 low", decode.U32(0, 5109), 5109)
	near(t, "u32 high", decode.U32(1, 2), 65538)
}

func TestDerived(t *testing.T) {
	t.Parallel()
	near(t, "delta T", decode.DeltaT(decode.Num(150), decode.Num(130)), 20)
	none(t, "delta T without inlet", decode.DeltaT(decode.Num(150), decode.None))
	if !decode.FlamePresent(decode.Num(2.1)).On() {
		t.Error("2.1 µA should be a flame")
	}
	if on, ok := decode.FlamePresent(decode.Num(0)).IsOn(); on || !ok {
		t.Error("0 µA should be no flame")
	}
	none(t, "flame present without signal", decode.FlamePresent(decode.None))
}

func TestBoilerStatus(t *testing.T) {
	t.Parallel()
	n, b, x := decode.Num, decode.Bool, decode.None
	tests := []struct {
		want                               string
		link                               bool
		status, anti, demand, burner, pump decode.Value
	}{
		{"Link lost", false, n(9), b(false), b(true), b(true), n(100)},
		{"Unavailable", true, x, x, x, x, x},
		{"Anti-cycle", true, n(4), b(true), b(true), b(true), n(100)},
		{"Demand, held off", true, n(4), b(false), b(true), b(false), n(100)},
		{"Demand", true, n(4), b(false), b(true), b(true), n(100)},
		{"Pump delay", true, n(4), b(false), b(false), b(false), n(100)},
		{"Standby", true, n(4), b(false), b(false), b(false), n(0)},
		{"Standby", true, n(4), b(false), b(false), b(false), x},
		{"Prepurge", true, n(6), b(false), b(true), b(true), n(100)},
		{"Ignition", true, n(7), b(false), b(true), b(true), n(100)},
		{"Running", true, n(9), b(false), b(true), b(true), n(100)},
		{"Post-purge", true, n(10), b(false), b(false), b(false), n(100)},
		{"Unknown (233)", true, n(233), b(false), b(false), b(false), n(0)},
	}
	for _, tt := range tests {
		got := decode.BoilerStatus(tt.link, tt.status, tt.anti, tt.demand, tt.burner, tt.pump)
		if got != tt.want {
			t.Errorf("BoilerStatus(%v, %v, ...) = %q, want %q", tt.link, tt.status, got, tt.want)
		}
	}
}

func TestIgnitionGuard(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 10, 24, 20, 0, 0, 0, time.UTC)
	at := func(s float64) time.Time { return t0.Add(time.Duration(s * float64(time.Second))) }
	var g decode.IgnitionGuard
	if !g.CountersClean(decode.Num(4), at(0)) {
		t.Error("clean before any ignition")
	}
	if g.CountersClean(decode.None, at(0)) {
		t.Error("unknown status must not be clean")
	}
	g.NoteStatus(decode.Num(6), at(100))
	if g.CountersClean(decode.Num(6), at(100)) {
		t.Error("prepurge must not be clean")
	}
	g.NoteStatus(decode.Num(7), at(110))
	for _, s := range []float64{120, 139.9} {
		if g.CountersClean(decode.Num(9), at(s)) {
			t.Errorf("t=%v within 30 s of ignition must not be clean", s)
		}
	}
	if !g.CountersClean(decode.Num(9), at(140)) {
		t.Error("30 s after ignition should be clean")
	}
	near(t, "failed", decode.FailedIgnitions(decode.Num(5109), decode.Num(5000)), 109)
	none(t, "failed without attempts", decode.FailedIgnitions(decode.None, decode.Num(5000)))
}
