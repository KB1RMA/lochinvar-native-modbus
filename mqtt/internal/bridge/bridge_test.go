package bridge_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/bridge"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/discovery"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/modbustest"
)

type message struct {
	topic, payload string
	retain         bool
}

type recorder struct {
	messages []message
	last     map[string]string
}

func (r *recorder) Publish(topic, payload string, retain bool) {
	r.messages = append(r.messages, message{topic, payload, retain})
	r.last[topic] = payload
}

func (r *recorder) state(key string) string { return r.last["lochinvar/sensor/"+key+"/state"] }

func (r *recorder) binary(key string) string { return r.last["lochinvar/binary_sensor/"+key+"/state"] }

func (r *recorder) status() string { return r.last["lochinvar/text_sensor/boiler_status/state"] }

var t0 = time.Date(2026, 10, 24, 20, 0, 0, 0, time.UTC)

func at(seconds float64) time.Time { return t0.Add(time.Duration(seconds * float64(time.Second))) }

type fixture struct {
	bridge *bridge.Bridge
	boiler *modbustest.Boiler
	out    *recorder
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	return newFixtureWith(t, bridge.Options{})
}

func newFixtureWith(t *testing.T, opt bridge.Options) fixture {
	t.Helper()
	boiler := modbustest.NewBoiler(modbustest.Running())
	out := &recorder{last: map[string]string{}}
	b := bridge.New(boiler.Link(), out, discovery.DefaultTopics(), "Lochinvar Boiler", opt)
	return fixture{b, boiler, out}
}

func expect(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", name, got, want)
	}
}

func TestFirstPollPublishesEverything(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if wait := f.bridge.Step(at(0)); wait != 5*time.Second {
		t.Errorf("next step in %v, want 5s", wait)
	}
	o := f.out
	expect(t, "link", o.last["lochinvar/link"], "online")
	expect(t, "outlet", o.state("outlet_temp"), "150.0")
	expect(t, "inlet", o.state("inlet_temp"), "130.0")
	expect(t, "delta T", o.state("delta_t"), "20.0")
	expect(t, "flame", o.state("flame_signal"), "2.1")
	expect(t, "firing rate", o.state("firing_rate"), "40.0")
	expect(t, "pump", o.state("boiler_pump_output"), "100")
	expect(t, "status code", o.state("status_code"), "9")
	expect(t, "heat demand", o.binary("heat_demand"), "ON")
	expect(t, "burner request", o.binary("burner_request"), "ON")
	expect(t, "anti-cycle", o.binary("anti_cycle"), "OFF")
	expect(t, "flame present", o.binary("flame_present"), "ON")
	expect(t, "boiler status", o.status(), "Running")
	expect(t, "SH1", o.state("setpoint_sh1"), "180")
	expect(t, "attempts", o.state("ignition_attempts"), "5109")
	expect(t, "SH run time", o.state("sh_run_time"), "3828")
	expect(t, "failed", o.state("failed_ignitions"), "109")
	expect(t, "reconnects", o.state("link_reconnects"), "0")
	for _, m := range o.messages {
		if !m.retain {
			t.Errorf("%s not retained", m.topic)
		}
	}
}

func TestPublishesOnlyChanges(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.bridge.Step(at(0))
	n := len(f.out.messages)
	f.bridge.Step(at(5))
	if len(f.out.messages) != n {
		t.Fatalf("unchanged poll published %v", f.out.messages[n:])
	}
	f.boiler.Set(62, f.boiler.Get(62)+25) // +1 °F
	f.bridge.Step(at(10))
	want := []message{
		{"lochinvar/sensor/outlet_temp/state", "151.0", true},
		{"lochinvar/sensor/delta_t/state", "21.0", true},
	}
	if got := f.out.messages[n:]; !slices.Equal(got, want) {
		t.Fatalf("published %v, want %v", got, want)
	}
}

func slowReads(b *modbustest.Boiler) int {
	n := 0
	for _, w := range b.Written() {
		if w[2] == 0x04 && w[3] == 0xE8 { // 1256
			n++
		}
	}
	return n
}

func TestSlowGroupEveryFiveMinutes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.bridge.Step(at(0))
	for s := 5; s < 300; s += 5 {
		f.bridge.Step(at(float64(s)))
	}
	if n := slowReads(f.boiler); n != 1 {
		t.Fatalf("%d slow reads in the first 5 min, want 1", n)
	}
	f.bridge.Step(at(300))
	if n := slowReads(f.boiler); n != 2 {
		t.Fatalf("%d slow reads after 5 min, want 2", n)
	}
}

func TestLinkLossClearsReadings(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.bridge.Step(at(0))
	f.boiler.SetSilent(true)
	if wait := f.bridge.Step(at(5)); wait != 30*time.Second {
		t.Errorf("backoff %v, want 30s (6 updates, like the ESP)", wait)
	}
	o := f.out
	expect(t, "link", o.last["lochinvar/link"], "offline")
	expect(t, "status", o.status(), "Link lost")
	expect(t, "outlet", o.state("outlet_temp"), "None")
	expect(t, "delta T", o.state("delta_t"), "None")
	expect(t, "heat demand", o.binary("heat_demand"), "None")
	expect(t, "flame present", o.binary("flame_present"), "None")
	expect(t, "SH1 keeps its value", o.state("setpoint_sh1"), "180")
	expect(t, "failed keeps its value", o.state("failed_ignitions"), "109")

	f.boiler.SetSilent(false)
	f.bridge.Step(at(35))
	expect(t, "link", o.last["lochinvar/link"], "online")
	expect(t, "outlet", o.state("outlet_temp"), "150.0")
	expect(t, "status", o.status(), "Running")
}

func TestReopenAfterThreeMinutesWithBackoff(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.bridge.Step(at(0))
	f.boiler.SetSilent(true)
	f.bridge.Step(at(5)) // down since 5
	for s := 35.0; s < 185; s += 30 {
		f.bridge.Step(at(s))
		if n := f.boiler.Opened(); n != 1 {
			t.Fatalf("reopened after %v s, want 180 s", s-5)
		}
	}
	f.bridge.Step(at(185)) // 180 s down: close, reopen 2 s later
	f.bridge.Step(at(187))
	if n := f.boiler.Opened(); n != 2 {
		t.Fatalf("opened %d times, want 2", n)
	}
	expect(t, "reconnects", f.out.state("link_reconnects"), "1")
	if w := f.bridge.ReopenWait(); w != 6*time.Minute {
		t.Errorf("next reopen wait %v, want 6m", w)
	}
	f.bridge.Step(at(487))
	if n := f.boiler.Opened(); n != 2 {
		t.Fatalf("reopened again after 5 min, want 6 min")
	}
	f.boiler.SetSilent(false)
	f.bridge.Step(at(517))
	if w := f.bridge.ReopenWait(); w != 3*time.Minute {
		t.Errorf("reopen wait %v after the boiler answered, want 3m", w)
	}
}

func TestReopenWaitCapped(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.bridge.Step(at(0))
	f.boiler.SetSilent(true)
	for s := 5.0; s < 6*3600; s += 30 {
		f.bridge.Step(at(s))
	}
	if w := f.bridge.ReopenWait(); w != 30*time.Minute {
		t.Errorf("reopen wait %v, want capped at 30m", w)
	}
}

func TestNoReopenBeforeFirstAnswer(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.boiler.SetSilent(true)
	for s := 0.0; s < 3600; s += 30 {
		f.bridge.Step(at(s))
	}
	if n := f.boiler.Opened(); n != 1 {
		t.Fatalf("opened %d times, want 1", n)
	}
	expect(t, "status", f.out.status(), "Link lost")
}

func TestExceptionKeepsLinkUpWithoutReopen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.bridge.Step(at(0))
	f.boiler.SetException(6)
	if wait := f.bridge.Step(at(5)); wait != 30*time.Second {
		t.Errorf("backoff %v, want 30s", wait)
	}
	o := f.out
	expect(t, "link", o.last["lochinvar/link"], "online")
	expect(t, "status", o.status(), "Unavailable")
	expect(t, "outlet", o.state("outlet_temp"), "None")
	expect(t, "SH1 keeps its value", o.state("setpoint_sh1"), "180")
	for s := 35.0; s < 3600; s += 30 {
		f.bridge.Step(at(s))
	}
	if n := f.boiler.Opened(); n != 1 {
		t.Fatalf("opened %d times, want 1: an exception is an answer, not a dead link", n)
	}
	f.boiler.SetException(0)
	f.bridge.Step(at(3605)) // polls back off to every 30 s, like a silent link
	expect(t, "outlet", o.state("outlet_temp"), "150.0")
	expect(t, "status", o.status(), "Running")
}

func TestPortErrorRetriesEveryTenSeconds(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.bridge.Step(at(0))
	f.boiler.SetFail(true)
	for _, step := range []struct {
		at   float64
		want time.Duration
	}{
		{5, 10 * time.Second},  // read fails
		{10, 5 * time.Second},  // not yet
		{15, 10 * time.Second}, // open fails, try again later
	} {
		if got := f.bridge.Step(at(step.at)); got != step.want {
			t.Errorf("Step(%v) = %v, want %v", step.at, got, step.want)
		}
	}
	expect(t, "status", f.out.status(), "Link lost")
	f.boiler.SetFail(false)
	f.bridge.Step(at(25))
	expect(t, "link", f.out.last["lochinvar/link"], "online")
	expect(t, "reconnects", f.out.state("link_reconnects"), "0") // only silent-link reopens count
}

func TestNoReopenRightAfterUnplug(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.bridge.Step(at(0))
	f.boiler.SetFail(true) // cable unplugged for 10 min
	for s := 5.0; s < 600; s += 10 {
		f.bridge.Step(at(s))
	}
	f.boiler.SetFail(false)
	f.boiler.SetSilent(true) // replugged, but the boiler doesn't answer yet
	f.bridge.Step(at(605))
	opened := f.boiler.Opened()
	for s := 635.0; s < 605+180; s += 30 {
		f.bridge.Step(at(s))
	}
	if n := f.boiler.Opened(); n != opened {
		t.Fatalf("port reopened %d times within 3 min of opening it", n-opened)
	}
	expect(t, "reconnects", f.out.state("link_reconnects"), "0")
	f.bridge.Step(at(605 + 180)) // silent 3 min since the open: now it reopens
	expect(t, "reconnects", f.out.state("link_reconnects"), "1")
}

func TestFailedIgnitionsRetriedSoonAfterAStartDuringIgnition(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.boiler.Set(84, 6) // prepurge
	f.bridge.Step(at(0))
	expect(t, "counters still published", f.out.state("ignitions"), "5000")
	expect(t, "failed held", f.out.state("failed_ignitions"), "")
	f.boiler.Set(84, 9)
	for s := 5.0; s <= 30; s += 5 {
		f.bridge.Step(at(s))
	}
	expect(t, "failed 30 s later", f.out.state("failed_ignitions"), "109")
}

func TestFailedIgnitionsSkipsReadsDuringIgnition(t *testing.T) {
	t.Parallel()
	// Slow reads every 10 s, so each step below reads the counters.
	f := newFixtureWith(t, bridge.Options{SlowInterval: 10 * time.Second})
	f.bridge.Step(at(0))
	expect(t, "failed", f.out.state("failed_ignitions"), "109")

	steps := []struct {
		at                  float64
		status              uint16
		attempts, successes uint16 // added to the counters before the step
		want                string
	}{
		{10, 6, 1, 0, "109"}, // prepurge: attempt counted first, held
		{20, 9, 0, 1, "109"}, // running 10 s after the ignition phase: held
		{30, 9, 0, 0, "109"}, // 20 s after: held
		{40, 9, 0, 0, "109"}, // 30 s after: clean, same value
		{50, 9, 1, 0, "110"}, // a real failure shows on the next clean read
	}
	for _, st := range steps {
		f.boiler.Set(84, st.status)
		f.boiler.Set(1305, f.boiler.Get(1305)+st.attempts)
		f.boiler.Set(1307, f.boiler.Get(1307)+st.successes)
		f.bridge.Step(at(st.at))
		expect(t, fmt.Sprintf("failed at %v s", st.at), f.out.state("failed_ignitions"), st.want)
	}
	expect(t, "attempts", f.out.state("ignition_attempts"), "5111")
}

func TestRepublishAll(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.bridge.Step(at(0))
	f.out.messages = nil
	if err := f.bridge.RepublishAll(); err != nil {
		t.Fatal(err)
	}
	topics := make([]string, 0, len(f.out.messages))
	for _, m := range f.out.messages {
		topics = append(topics, m.topic)
	}
	if topics[0] != "lochinvar/status" || f.out.last["lochinvar/status"] != "online" {
		t.Errorf("first message %s=%s, want lochinvar/status=online", topics[0], f.out.last[topics[0]])
	}
	for _, want := range []string{
		"homeassistant/sensor/lochinvar/outlet_temp/config", "lochinvar/link",
		"lochinvar/sensor/outlet_temp/state",
	} {
		if !slices.Contains(topics, want) {
			t.Errorf("RepublishAll did not send %s", want)
		}
	}
	var p discovery.Payload
	if err := json.Unmarshal([]byte(f.out.last["homeassistant/sensor/lochinvar/outlet_temp/config"]), &p); err != nil {
		t.Fatal(err)
	}
	expect(t, "discovery name", p.Name, "Outlet Temperature")
}

func TestShutdownMarksOffline(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.bridge.Step(at(0))
	f.bridge.Shutdown()
	expect(t, "status", f.out.last["lochinvar/status"], "offline")
}
