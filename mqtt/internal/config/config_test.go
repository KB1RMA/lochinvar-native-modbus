package config_test

import (
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/config"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDefaults(t *testing.T) {
	t.Parallel()
	c, err := config.Load(nil, env(nil), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	want := config.Config{
		SerialPort: "/dev/ttyUSB0", SlaveID: 1, MQTTHost: "localhost", MQTTPort: 1883,
		MQTTClientID: "lochinvar-native-modbus", TopicPrefix: "lochinvar", DiscoveryPrefix: "homeassistant",
		DeviceID: "lochinvar", DeviceName: "Lochinvar Boiler", FastInterval: 5 * time.Second,
		SlowInterval: 5 * time.Minute, LogLevel: slog.LevelInfo,
	}
	if c != want {
		t.Errorf("defaults\n got %+v\nwant %+v", c, want)
	}
}

func TestEnvironmentAndFlags(t *testing.T) {
	t.Parallel()
	e := env(map[string]string{
		"SERIAL_PORT": "/dev/serial/by-id/ftdi", "MQTT_HOST": "broker", "MQTT_PORT": "8883", "MQTT_TLS": "true",
		"FAST_INTERVAL": "10", "SLOW_INTERVAL": "10m", "LOG_LEVEL": "debug", "DEVICE_NAME": "Boiler",
	})
	c, err := config.Load([]string{"--mqtt-host", "other", "-device-id", "boiler2"}, e, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if c.SerialPort != "/dev/serial/by-id/ftdi" || c.MQTTPort != 8883 || !c.MQTTTLS || c.DeviceName != "Boiler" {
		t.Errorf("environment not applied: %+v", c)
	}
	if c.MQTTHost != "other" || c.DeviceID != "boiler2" {
		t.Errorf("flags should win over the environment: %+v", c)
	}
	if c.FastInterval != 10*time.Second || c.SlowInterval != 10*time.Minute || c.LogLevel != slog.LevelDebug {
		t.Errorf("intervals or level wrong: %+v", c)
	}
}

func TestInvalid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		env  map[string]string
	}{
		{"bad port env", nil, map[string]string{"MQTT_PORT": "x"}},
		{"bad bool env", nil, map[string]string{"MQTT_TLS": "maybe"}},
		{"bad duration", nil, map[string]string{"FAST_INTERVAL": "soon"}},
		{"port range", []string{"--mqtt-port", "70000"}, nil},
		{"slave range", []string{"--slave-id", "0"}, nil},
		{"fast too short", []string{"--fast-interval", "100ms"}, nil},
		{"slow below fast", []string{"--slow-interval", "1s"}, nil},
		{"log level", []string{"--log-level", "loud"}, nil},
		{"stray argument", []string{"extra"}, nil},
		{"unknown flag", []string{"--write"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := config.Load(tt.args, env(tt.env), io.Discard); err == nil {
				t.Error("Load succeeded, want an error")
			}
		})
	}
}

func TestHelp(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	_, err := config.Load([]string{"-h"}, env(nil), &out)
	if !errors.Is(err, config.ErrHelp) {
		t.Fatalf("err = %v, want ErrHelp", err)
	}
	for _, want := range []string{"MQTT_HOST", "SERIAL_PORT", "-fast-interval", "(default 5s)", "(default 5m0s)"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help does not mention %s", want)
		}
	}
}

func TestFlagOverridesBadEnvironment(t *testing.T) {
	t.Parallel()
	e := env(map[string]string{"MQTT_PORT": "abc", "FAST_INTERVAL": "soon"})
	c, err := config.Load([]string{"--mqtt-port", "1884", "--fast-interval", "2s"}, e, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if c.MQTTPort != 1884 || c.FastInterval != 2*time.Second {
		t.Errorf("flags not applied: %+v", c)
	}
}

func TestVersionIgnoresBadEnvironment(t *testing.T) {
	t.Parallel()
	c, err := config.Load([]string{"--version"}, env(map[string]string{"MQTT_PORT": "abc"}), io.Discard)
	if err != nil || !c.ShowVersion {
		t.Fatalf("--version: %+v, %v", c, err)
	}
}

func TestTLSPort(t *testing.T) {
	t.Parallel()
	tests := []struct {
		args []string
		env  map[string]string
		want int
	}{
		{[]string{"--mqtt-tls"}, nil, 8883},
		{nil, map[string]string{"MQTT_TLS": "true"}, 8883},
		{[]string{"--mqtt-tls", "--mqtt-port", "1883"}, nil, 1883},
		{nil, map[string]string{"MQTT_TLS": "true", "MQTT_PORT": "8884"}, 8884},
		{nil, nil, 1883},
	}
	for _, tt := range tests {
		c, err := config.Load(tt.args, env(tt.env), io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		if c.MQTTPort != tt.want {
			t.Errorf("Load(%v, %v) port %d, want %d", tt.args, tt.env, c.MQTTPort, tt.want)
		}
	}
}
