package main

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/bridge"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/discovery"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/modbustest"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/version"
)

func TestRunVersionHelpAndBadConfig(t *testing.T) {
	t.Parallel()
	noenv := func(string) string { return "" }
	tests := []struct {
		args     []string
		code     int
		stdout   string
		stderrIn string
	}{
		{[]string{"--version"}, 0, version.Version + "\n", ""},
		{[]string{"--help"}, 0, "", "MQTT_HOST"},
		{[]string{"--mqtt-port", "0"}, 2, "", "out of range"},
	}
	for _, tt := range tests {
		var stdout, stderr strings.Builder
		if code := run(tt.args, noenv, &stdout, &stderr); code != tt.code {
			t.Errorf("run(%v) = %d, want %d (stderr %q)", tt.args, code, tt.code, stderr.String())
		}
		if stdout.String() != tt.stdout {
			t.Errorf("run(%v) stdout %q, want %q", tt.args, stdout.String(), tt.stdout)
		}
		if !strings.Contains(stderr.String(), tt.stderrIn) {
			t.Errorf("run(%v) stderr %q, want it to contain %q", tt.args, stderr.String(), tt.stderrIn)
		}
	}
}

type recorder struct {
	mu   sync.Mutex
	last map[string]string
}

func (r *recorder) Publish(topic, payload string, _ bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.last[topic] = payload
}

func (r *recorder) get(topic string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.last[topic]
}

func TestLoopPollsRepublishesAndStops(t *testing.T) {
	t.Parallel()
	boiler := modbustest.NewBoiler(modbustest.Running())
	out := &recorder{last: map[string]string{}}
	b := bridge.New(boiler.Link(), out, discovery.DefaultTopics(), "Boiler", bridge.Options{})
	republish := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		loop(ctx, b, republish)
		close(done)
	}()

	waitFor := func(what, topic, want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for out.get(topic) != want {
			if time.Now().After(deadline) {
				t.Fatalf("%s: %s = %q, want %q", what, topic, out.get(topic), want)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	waitFor("first poll", "lochinvar/text_sensor/boiler_status/state", "Running")
	republish <- struct{}{}
	waitFor("republish", "lochinvar/status", discovery.Online)
	if out.get("homeassistant/sensor/lochinvar/outlet_temp/config") == "" {
		t.Error("republish did not send discovery")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("loop did not stop after cancel")
	}
}
