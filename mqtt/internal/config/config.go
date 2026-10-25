// Package config reads the settings. Every setting is an environment variable (which is how the Docker
// image is configured) and a command line option. Each one is taken from, in order:
//
//  1. the command line option
//  2. the environment variable, e.g. MQTT_PASSWORD
//  3. a file named by the variable with _FILE appended, e.g. MQTT_PASSWORD_FILE=/run/secrets/mqtt
//  4. the config file (CONFIG_FILE or --config): KEY=VALUE lines with the same names as the variables
//  5. the default
package config

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Config is the bridge's configuration.
type Config struct {
	SerialPort      string
	SlaveID         uint8
	ModbusTimeout   time.Duration
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
	ReopenAfter     time.Duration
	ReopenMax       time.Duration
	LogLevel        slog.Level
	ConfigFile      string
	ShowVersion     bool
}

// ErrHelp is returned for -h/--help after the usage was printed.
var ErrHelp = flag.ErrHelp

// Load parses args (without the program name). getenv supplies the environment (os.Getenv).
func Load(args []string, getenv func(string) string, out io.Writer) (Config, error) {
	c := Config{ConfigFile: configFlag(args)}
	if c.ConfigFile == "" {
		c.ConfigFile = getenv("CONFIG_FILE")
	}
	var file map[string]string
	if c.ConfigFile != "" {
		var err error
		if file, err = readConfigFile(c.ConfigFile); err != nil {
			return Config{}, err
		}
	}

	fs := flag.NewFlagSet("lochinvar-mqtt", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(out, "Read-only Modbus RTU to MQTT bridge for a Lochinvar boiler's built-in controller port.\n\n"+
			"Usage: lochinvar-mqtt [options]\n\n"+
			"Every option can also be set with the environment variable in brackets, with that variable plus\n"+
			"_FILE naming a file that holds the value, or as KEY=VALUE in the config file.\n\n")
		fs.PrintDefaults()
	}

	envErrs := map[string]error{} // flag name -> bad value from outside the command line; the command line wins
	given := map[string]bool{}    // flag name -> a value came from the environment or the config file
	known := map[string]bool{"CONFIG_FILE": true}
	// lookup returns the value of env from the environment, its _FILE, or the config file.
	lookup := func(name, env string) string {
		known[env], known[env+"_FILE"] = true, true
		get := func(k string) string {
			if v := getenv(k); v != "" {
				return v
			}
			return file[k]
		}
		v := get(env)
		if v == "" {
			if path := get(env + "_FILE"); path != "" {
				data, err := os.ReadFile(path) //nolint:gosec // the operator names the file
				if err != nil {
					envErrs[name] = fmt.Errorf("%s_FILE: %w", env, err)
					return ""
				}
				v = strings.TrimRight(string(data), "\r\n")
			}
		}
		given[name] = v != ""
		return v
	}
	str := func(p *string, name, env, def, usage string) {
		v := def
		if e := lookup(name, env); e != "" {
			v = e
		}
		fs.StringVar(p, name, v, fmt.Sprintf("%s [%s]", usage, env))
	}
	secret := func(p *string, name, env, usage string) {
		fs.StringVar(p, name, "", fmt.Sprintf("%s [%s]", usage, env))
		*p = lookup(name, env) // after StringVar, so --help never shows it
	}
	num := func(p *int, name, env string, def int, usage string) {
		v := def
		if e := lookup(name, env); e != "" {
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
		if e := lookup(name, env); e != "" {
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
		if e := lookup(name, env); e != "" {
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
	dur(&c.ModbusTimeout, "modbus-timeout", "MODBUS_TIMEOUT", 600*time.Millisecond, "how long to wait for each reply")
	str(&c.MQTTHost, "mqtt-host", "MQTT_HOST", "localhost", "MQTT broker host")
	num(&c.MQTTPort, "mqtt-port", "MQTT_PORT", 1883, "MQTT broker port (8883 by default with TLS)")
	str(&c.MQTTUsername, "mqtt-username", "MQTT_USERNAME", "", "MQTT username")
	secret(&c.MQTTPassword, "mqtt-password", "MQTT_PASSWORD", "MQTT password")
	boolean(&c.MQTTTLS, "mqtt-tls", "MQTT_TLS", "connect to the broker with TLS")
	str(&c.MQTTCAFile, "mqtt-ca-file", "MQTT_CA_FILE", "", "PEM CA certificate(s) to verify the broker with, instead of the system roots")
	str(&c.MQTTClientID, "mqtt-client-id", "MQTT_CLIENT_ID", "lochinvar-native-modbus", "MQTT client id")
	str(&c.TopicPrefix, "topic-prefix", "MQTT_TOPIC_PREFIX", "lochinvar", "base topic for state and availability")
	str(&c.DiscoveryPrefix, "discovery-prefix", "DISCOVERY_PREFIX", "homeassistant", "Home Assistant discovery prefix")
	str(&c.DeviceID, "device-id", "DEVICE_ID", "lochinvar", "unique id of the device in Home Assistant")
	str(&c.DeviceName, "device-name", "DEVICE_NAME", "Lochinvar Boiler", "device name in Home Assistant")
	dur(&c.FastInterval, "fast-interval", "FAST_INTERVAL", 5*time.Second, "time between live readings")
	dur(&c.SlowInterval, "slow-interval", "SLOW_INTERVAL", 5*time.Minute, "time between settings and counter reads")
	dur(&c.ReopenAfter, "reopen-after", "REOPEN_AFTER", 3*time.Minute, "reopen the serial port after the boiler is silent this long")
	dur(&c.ReopenMax, "reopen-max", "REOPEN_MAX", 30*time.Minute, "longest wait between reopens (each one that doesn't help doubles it)")
	str(&level, "log-level", "LOG_LEVEL", "info", "debug, info, warn or error")
	fs.StringVar(&c.ConfigFile, "config", c.ConfigFile, "file of KEY=VALUE settings, with the variable names [CONFIG_FILE]")
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
	for _, k := range slices.Sorted(maps.Keys(file)) {
		if !known[k] {
			errs = append(errs, fmt.Errorf("%s: unknown setting %s", c.ConfigFile, k))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	if c.MQTTTLS && !set["mqtt-port"] && !given["mqtt-port"] {
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
	if c.ModbusTimeout < 100*time.Millisecond || c.ModbusTimeout > 10*time.Second {
		return Config{}, fmt.Errorf("modbus timeout %s out of range 100ms-10s", c.ModbusTimeout)
	}
	if c.FastInterval < time.Second || c.SlowInterval < c.FastInterval {
		return Config{}, fmt.Errorf("intervals must be at least 1s, and slow >= fast (got %s, %s)",
			c.FastInterval, c.SlowInterval)
	}
	if c.ReopenAfter < 30*time.Second || c.ReopenMax < c.ReopenAfter {
		return Config{}, fmt.Errorf("reopen wait must be at least 30s, and max >= wait (got %s, %s)",
			c.ReopenAfter, c.ReopenMax)
	}
	if err := c.LogLevel.UnmarshalText([]byte(level)); err != nil {
		return Config{}, fmt.Errorf("log level: %w", err)
	}
	return c, nil
}

// configFlag finds --config in args. The flag package can only parse it once every option's default
// is known, and the config file supplies those defaults.
func configFlag(args []string) string {
	for i, a := range args {
		if a == "--" {
			break
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if !strings.HasPrefix(a, "-") || name != "config" {
			continue
		}
		if hasValue {
			return value
		}
		if i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// readConfigFile reads KEY=VALUE lines, like a Docker env file: blank lines and lines starting with #
// are skipped, "export " before a key is allowed, and a value may be in single or double quotes.
func readConfigFile(path string) (map[string]string, error) {
	f, err := os.Open(path) //nolint:gosec // the operator names the file
	if err != nil {
		return nil, fmt.Errorf("config file: %w", err)
	}
	defer func() { _ = f.Close() }()
	settings := map[string]string{}
	scanner := bufio.NewScanner(f)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimPrefix(line, "export "), "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || key == "" {
			return nil, fmt.Errorf("%s:%d: want KEY=VALUE", path, n)
		}
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		settings[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("config file: %w", err)
	}
	return settings, nil
}

// parseDuration accepts Go durations ("5s", "5m") and plain seconds ("5", "300").
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		if math.IsNaN(f) || math.Abs(f) > float64(math.MaxInt64/int64(time.Second)) {
			return 0, fmt.Errorf("duration %q out of range", s)
		}
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
