// Package config reads the command line, with every option also settable by an environment variable
// (which is how the Docker image is configured). Flags win over the environment.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Config is the bridge's configuration.
type Config struct {
	SerialPort      string
	SlaveID         uint8
	MQTTHost        string
	MQTTPort        int
	MQTTUsername    string
	MQTTPassword    string
	MQTTTLS         bool
	MQTTCAFile      string
	MQTTClientID    string
	TopicPrefix     string
	DiscoveryPrefix string
	DeviceID        string
	DeviceName      string
	FastInterval    time.Duration
	SlowInterval    time.Duration
	LogLevel        slog.Level
	ShowVersion     bool
}

// ErrHelp is returned for -h/--help after the usage was printed.
var ErrHelp = flag.ErrHelp

// Load parses args (without the program name). getenv supplies the environment (os.Getenv).
func Load(args []string, getenv func(string) string, out io.Writer) (Config, error) {
	c := Config{}
	fs := flag.NewFlagSet("lochinvar-mqtt", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(out, "Read-only Modbus RTU to MQTT bridge for a Lochinvar boiler's built-in controller port.\n\n"+
			"Usage: lochinvar-mqtt [options]\n\nEvery option can also be set with the environment variable in brackets.\n\n")
		fs.PrintDefaults()
	}

	envErrs := map[string]error{} // flag name -> bad environment value; a flag given on the command line wins
	str := func(p *string, name, env, def, usage string) {
		v := def
		if e := getenv(env); e != "" {
			v = e
		}
		fs.StringVar(p, name, v, fmt.Sprintf("%s [%s]", usage, env))
	}
	num := func(p *int, name, env string, def int, usage string) {
		v := def
		if e := getenv(env); e != "" {
			n, err := strconv.Atoi(e)
			if err != nil {
				envErrs[name] = fmt.Errorf("%s: %w", env, err)
			}
			v = n
		}
		fs.IntVar(p, name, v, fmt.Sprintf("%s [%s]", usage, env))
	}
	boolean := func(p *bool, name, env string, usage string) {
		v := false
		if e := getenv(env); e != "" {
			b, err := strconv.ParseBool(e)
			if err != nil {
				envErrs[name] = fmt.Errorf("%s: %w", env, err)
			}
			v = b
		}
		fs.BoolVar(p, name, v, fmt.Sprintf("%s [%s]", usage, env))
	}
	dur := func(p *time.Duration, name, env string, def time.Duration, usage string) {
		v := def
		if e := getenv(env); e != "" {
			d, err := parseDuration(e)
			if err != nil {
				envErrs[name] = fmt.Errorf("%s: %w", env, err)
			}
			v = d
		}
		*p = v // before Var, which takes the default shown in the help from it
		fs.Var(durationValue{p}, name, fmt.Sprintf("%s, e.g. 5s or 5 (seconds) [%s]", usage, env))
	}

	var slave int
	var level string
	str(&c.SerialPort, "serial-port", "SERIAL_PORT", "/dev/ttyUSB0", "serial device of the boiler's FT232R")
	num(&slave, "slave-id", "MODBUS_SLAVE_ID", 1, "Modbus slave address")
	str(&c.MQTTHost, "mqtt-host", "MQTT_HOST", "localhost", "MQTT broker host")
	num(&c.MQTTPort, "mqtt-port", "MQTT_PORT", 1883, "MQTT broker port (8883 by default with TLS)")
	str(&c.MQTTUsername, "mqtt-username", "MQTT_USERNAME", "", "MQTT username")
	str(&c.MQTTPassword, "mqtt-password", "MQTT_PASSWORD", "", "MQTT password")
	boolean(&c.MQTTTLS, "mqtt-tls", "MQTT_TLS", "connect to the broker with TLS")
	str(&c.MQTTCAFile, "mqtt-ca-file", "MQTT_CA_FILE", "", "PEM CA certificate(s) to verify the broker with, instead of the system roots")
	str(&c.MQTTClientID, "mqtt-client-id", "MQTT_CLIENT_ID", "lochinvar-native-modbus", "MQTT client id")
	str(&c.TopicPrefix, "topic-prefix", "MQTT_TOPIC_PREFIX", "lochinvar", "base topic for state and availability")
	str(&c.DiscoveryPrefix, "discovery-prefix", "DISCOVERY_PREFIX", "homeassistant", "Home Assistant discovery prefix")
	str(&c.DeviceID, "device-id", "DEVICE_ID", "lochinvar", "unique id of the device in Home Assistant")
	str(&c.DeviceName, "device-name", "DEVICE_NAME", "Lochinvar Boiler", "device name in Home Assistant")
	dur(&c.FastInterval, "fast-interval", "FAST_INTERVAL", 5*time.Second, "time between live readings")
	dur(&c.SlowInterval, "slow-interval", "SLOW_INTERVAL", 5*time.Minute, "time between settings and counter reads")
	str(&level, "log-level", "LOG_LEVEL", "info", "debug, info, warn or error")
	fs.BoolVar(&c.ShowVersion, "version", false, "print the version and exit")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if c.ShowVersion {
		return c, nil
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(envErrs)) {
		if !set[name] {
			errs = append(errs, envErrs[name])
		}
	}
	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	if c.MQTTTLS && !set["mqtt-port"] && getenv("MQTT_PORT") == "" {
		c.MQTTPort = 8883
	}
	if fs.NArg() > 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if slave < 1 || slave > 247 {
		return Config{}, fmt.Errorf("slave id %d out of range 1-247", slave)
	}
	c.SlaveID = uint8(slave)
	if c.MQTTPort < 1 || c.MQTTPort > 65535 {
		return Config{}, fmt.Errorf("MQTT port %d out of range", c.MQTTPort)
	}
	if c.FastInterval < time.Second || c.SlowInterval < c.FastInterval {
		return Config{}, fmt.Errorf("intervals must be at least 1s, and slow >= fast (got %s, %s)",
			c.FastInterval, c.SlowInterval)
	}
	if err := c.LogLevel.UnmarshalText([]byte(level)); err != nil {
		return Config{}, fmt.Errorf("log level: %w", err)
	}
	return c, nil
}

// parseDuration accepts Go durations ("5s", "5m") and plain seconds ("5", "300").
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return time.Duration(f * float64(time.Second)), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	return d, nil
}

type durationValue struct{ p *time.Duration }

func (d durationValue) String() string {
	if d.p == nil {
		return ""
	}
	return d.p.String()
}

func (d durationValue) Set(s string) error {
	v, err := parseDuration(s)
	if err != nil {
		return err
	}
	*d.p = v
	return nil
}
