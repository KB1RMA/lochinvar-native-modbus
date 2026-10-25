# MQTT bridge (Docker)

Run the bridge as a Docker container on a computer next to the boiler, such as a Raspberry Pi. The
boiler's USB cable plugs into that computer. The container reads the boiler and publishes to your MQTT
broker, with Home Assistant discovery. You get the same entities as the [ESPHome package](../esphome/README.md).
See the [main README](../README.md) for what you get and how the project works.

```
Boiler controller  --USB cable-->  computer running Docker  -->  MQTT broker  -->  Home Assistant
```

**It only reads.** The only Modbus request it can build is function 03 (read holding registers). It has
no Modbus library, so there is no write code inside it at all. It is a single static Go binary in a
small image with no shell. Its tests check that every frame it sends is
a function 03 read of one of four blocks, the same blocks the project's logger has always read.

## Running it

Plug the boiler's USB cable into the computer. The boiler's FTDI chip shows up as a serial device,
usually `/dev/ttyUSB0`. Use the stable name under `/dev/serial/by-id/` if you can, so the device
keeps its name after a replug:

```bash
ls /dev/serial/by-id/
```

Then start the container:

```bash
docker run -d --name lochinvar --restart unless-stopped \
  --device /dev/serial/by-id/usb-FTDI_FT232R_USB_UART_XXXXXXXX-if00-port0:/dev/ttyUSB0 \
  -e MQTT_HOST=192.168.1.10 -e MQTT_USERNAME=lochinvar -e MQTT_PASSWORD=secret \
  ghcr.io/kb1rma/lochinvar-native-modbus:0.0.0 # x-release-please-version
```

Or with Docker Compose:

```yaml
services:
  lochinvar:
    image: ghcr.io/kb1rma/lochinvar-native-modbus:0.0.0 # x-release-please-version
    restart: unless-stopped
    devices:
      - /dev/serial/by-id/usb-FTDI_FT232R_USB_UART_XXXXXXXX-if00-port0:/dev/ttyUSB0
    environment:
      MQTT_HOST: 192.168.1.10
      MQTT_USERNAME: lochinvar
      MQTT_PASSWORD: secret
```

The image tag pins a release. `X.Y` follows the newest patch release, and `latest` the newest release.
Images are built for amd64, arm64 and 32-bit ARM (arm/v7). Changes are listed in
[CHANGELOG.md](CHANGELOG.md) once the first release is out.

The container runs as an unprivileged user in group `dialout` (gid 20), which owns serial devices on
Debian, Ubuntu and Raspberry Pi OS. If your host's device belongs to another group, add `--group-add <gid>`.

## Settings

| Variable | Default | What it does |
|---|---|---|
| `SERIAL_PORT` | `/dev/ttyUSB0` | Serial device inside the container |
| `MODBUS_SLAVE_ID` | `1` | The boiler's Modbus address |
| `MQTT_HOST` | `localhost` | MQTT broker |
| `MQTT_PORT` | `1883`, or `8883` with TLS | |
| `MQTT_USERNAME`, `MQTT_PASSWORD` | none | |
| `MQTT_TLS` | `false` | `true` to connect with TLS (system CA certificates unless `MQTT_CA_FILE` is set) |
| `MQTT_CA_FILE` | none | PEM CA certificate(s) to verify the broker with, e.g. a self-signed broker's CA; mount it with `-v` |
| `MQTT_CLIENT_ID` | `lochinvar-native-modbus` | |
| `MQTT_TOPIC_PREFIX` | `lochinvar` | Base topic for state and availability |
| `DISCOVERY_PREFIX` | `homeassistant` | Home Assistant's MQTT discovery prefix |
| `DEVICE_ID` | `lochinvar` | Unique id of the device in Home Assistant; change it to run two bridges |
| `DEVICE_NAME` | `Lochinvar Boiler` | Device name in Home Assistant |
| `FAST_INTERVAL` | `5s` | Time between live readings (`5s`, or plain seconds like `5`) |
| `SLOW_INTERVAL` | `5m` | Time between settings and counter reads |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`; `debug` logs every failed read |

The same settings are command line options, which win over the environment; run the image with
`--help` to list them.

## Topics

| Topic | Payload |
|---|---|
| `lochinvar/status` | `online` / `offline` (MQTT last will). Drives "Controller Online". |
| `lochinvar/link` | `online` / `offline`: whether the boiler answers. Drives "Boiler Link OK". |
| `lochinvar/<component>/<entity>/state` | The reading, e.g. `lochinvar/sensor/outlet_temp/state` = `150.2`. `None` = no reading. |
| `homeassistant/<component>/lochinvar/<entity>/config` | Discovery. Sent again whenever Home Assistant comes online. |

Everything is retained.

## When the link drops

This works like the ESP. The live readings and the values computed from them (Delta T, Flame Present)
show as unavailable, "Boiler Link OK" turns off, and Boiler Status reads "Link lost". Settings and
counters keep their last values. While the link is down, the bridge tries again every 30 seconds.

The ESP restarts itself after 3 minutes down. A container can't do that, so the bridge closes and reopens
the serial port instead. Each reopen that doesn't help doubles the wait, up to 30 minutes, because on a Mac
the FTDI driver stopped working after many open/close cycles. "Link Reconnects" counts reopens since the
container started. Like the ESP, this only starts once the boiler has answered at least once. If the
adapter disappears (cable unplugged), the bridge tries to open it again every 10 seconds.

A Modbus exception reply is not a dropped link: the boiler answered, so "Boiler Link OK" stays on and the
port is never reopened for it. The live readings are cleared, Boiler Status reads "Unavailable", and the
bridge tries again every 30 seconds.

If only unplugging and replugging the USB cable brings the link back, that is what happened on the Mac too.
See "Known problems" in the main README.

## Differences from the ESPHome package

- "Link Watchdog Restarts" is "Link Reconnects" here (see above).
- "Controller Online" turns off when the container stops or loses the broker, from its MQTT last will.
- Home Assistant shows the container as its own device, separate from an ESP.

## Developing

The bridge is Go (see `go.mod` for the version), using
[go.bug.st/serial](https://github.com/bugst/go-serial) and
[paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang).

| Path | What it is |
|---|---|
| `cmd/lochinvar-mqtt/` | `main`: configuration, signals, the main loop |
| `internal/modbus/` | Function 03 reads over the serial port. The only code that writes to it. |
| `internal/decode/` | Raw registers to readings: temperatures, status text, Delta T, the ignition guard |
| `internal/entities/` | The entity list, matching `esphome/packages/boiler.yaml` |
| `internal/discovery/` | Topics and Home Assistant discovery payloads |
| `internal/bridge/` | The poll loop, link loss and reopening |
| `internal/mqttout/` | The MQTT connection, last will, and republish signals |
| `internal/config/` | Flags and environment variables |
| `internal/modbustest/` | A fake boiler for the tests |

From `mqtt/`:

```bash
go test -race ./...
```

```bash
golangci-lint run
```

```bash
docker build -t lochinvar-mqtt .
```

The tests use a fake serial port and an in-process MQTT broker, so they never talk to a boiler. They also
check that:

- every frame sent is a function 03 read, and only `internal/modbus` writes to the port;
- the entities match `esphome/packages/boiler.yaml` (names, addresses, units, decoding);
- every address is in `registers/controller.yaml`;
- the version matches `.release-please-manifest.json`.

CI also checks that `go.mod` is tidy, runs `go vet`, golangci-lint (`.golangci.yml`), a short fuzz run and
govulncheck, and builds the image for every platform.
