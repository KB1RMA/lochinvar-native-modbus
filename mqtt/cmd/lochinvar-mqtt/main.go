// Command lochinvar-mqtt is a read-only Modbus RTU to MQTT bridge, with Home Assistant discovery, for
// a Lochinvar boiler's built-in controller port.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/bridge"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/config"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/discovery"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/modbus"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/mqttout"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	cfg, err := config.Load(args, getenv, stderr)
	if errors.Is(err, config.ErrHelp) {
		return 0
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "lochinvar-mqtt:", err)
		return 2
	}
	if cfg.ShowVersion {
		_, _ = fmt.Fprintln(stdout, version.Version)
		return 0
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: cfg.LogLevel})))
	slog.Info("lochinvar-native-modbus MQTT bridge (read-only, function 03)", "version", version.Version)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	topics := discovery.Topics{Prefix: cfg.TopicPrefix, DiscoveryPrefix: cfg.DiscoveryPrefix, DeviceID: cfg.DeviceID}
	out := mqttout.New(mqttout.Settings{
		Host: cfg.MQTTHost, Port: cfg.MQTTPort, Username: cfg.MQTTUsername, Password: cfg.MQTTPassword,
		ClientID: cfg.MQTTClientID, TLS: cfg.MQTTTLS,
	}, topics)
	b := bridge.New(modbus.NewLink(cfg.SerialPort, cfg.SlaveID), out, topics, cfg.DeviceName, bridge.Options{
		FastInterval: cfg.FastInterval, SlowInterval: cfg.SlowInterval,
	})

	out.Connect()
	loop(ctx, b, out.Republish())
	b.Shutdown()
	out.Close(500 * time.Millisecond) // let the offline message go out
	return 0
}

// loop runs the bridge until ctx is done. All bridge calls happen on this goroutine.
func loop(ctx context.Context, b *bridge.Bridge, republish <-chan struct{}) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-republish:
			if err := b.RepublishAll(); err != nil {
				slog.Error("republish failed", "err", err)
			}
		case <-timer.C:
			timer.Reset(b.Step(time.Now()))
		}
	}
}
