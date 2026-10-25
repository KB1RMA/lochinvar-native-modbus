# MQTT bridge (Docker)

A small program in a Docker container. It reads your boiler and sends the readings to your MQTT broker.
Home Assistant then finds the boiler by itself. You get the same sensors as the
[ESPHome package](../esphome/README.md).

For what you get and which boilers work, see the [main README](../README.md).

```
Boiler's PC Connection port  --USB cable-->  Linux computer running Docker  -->  MQTT broker  -->  Home Assistant
```

> [!NOTE]
> **Running Home Assistant OS?** It can't run your own containers, and this isn't a Home Assistant
> add-on yet. Use the [ESPHome package](../esphome/README.md) instead, or run this container on
> another computer near the boiler.

**It only reads.** It can only send one kind of Modbus request: function 03, which reads values. It has
no code that can write to the boiler or change a setting. The tests check this. They make sure every
message it sends is a function 03 read of one of four known blocks.

## What you need

- A Lochinvar boiler with the SMART SYSTEM display and a USB **PC Connection** port. See
  [Will it work on my boiler?](../README.md#will-it-work-on-my-boiler)
- A Linux computer near the boiler that runs Docker, like a Raspberry Pi or a home server. Images are
  built for amd64, arm64 and 32-bit ARM (arm/v7).
- A USB cable. The boiler end is USB-B (the square printer kind).
- An MQTT broker that Home Assistant uses, such as the Mosquitto add-on, with MQTT discovery on (the
  default).

Docker Desktop on a Mac or Windows PC can't easily pass a USB serial device into a container, so use
Linux.

## Running it

Plug the boiler's USB cable into the computer. The boiler shows up as a serial device, most often
`/dev/ttyUSB0`. That name can change when you unplug the cable. A better choice is the fixed name under
`/dev/serial/by-id/`. To find it, run:

```bash
ls /dev/serial/by-id/
```

Then start the container:

```bash
docker run -d --name lochinvar --restart unless-stopped \
  --device /dev/serial/by-id/usb-FTDI_FT232R_USB_UART_XXXXXXXX-if00-port0:/dev/ttyUSB0 \
  -e MQTT_HOST=192.168.1.10 -e MQTT_USERNAME=lochinvar -e MQTT_PASSWORD=secret \
  ghcr.io/kb1rma/lochinvar-native-modbus:0.1.1 # x-release-please-version
```

Or with Docker Compose:

```yaml
services:
  lochinvar:
    image: ghcr.io/kb1rma/lochinvar-native-modbus:0.1.1 # x-release-please-version
    restart: unless-stopped
    devices:
      - /dev/serial/by-id/usb-FTDI_FT232R_USB_UART_XXXXXXXX-if00-port0:/dev/ttyUSB0
    environment:
      MQTT_HOST: 192.168.1.10
      MQTT_USERNAME: lochinvar
      MQTT_PASSWORD: secret
```

### Other container platforms

Any platform that runs Docker images works, such as Unraid, TrueNAS, Portainer or Synology. Look for
these four things in its setup screen:

| What you set | Value |
|---|---|
| Image (or repository) | `ghcr.io/kb1rma/lochinvar-native-modbus` |
| Device | The boiler's serial device on the host, mapped to `/dev/ttyUSB0` in the container |
| Environment variables | `MQTT_HOST`, `MQTT_USERNAME`, `MQTT_PASSWORD`, and any others from [Settings](#settings) |
| Folder or volume | Not needed. Only add one if you use a config file or a password file (see below). |

### Image tags

Each release has its own tag, like `0.1.0`. The `0.1` tag always points to the newest `0.1.x` release.
The `latest` tag always points to the newest release. Images are built for 64-bit PCs (amd64), 64-bit
ARM (arm64) and 32-bit ARM (arm/v7). [CHANGELOG.md](CHANGELOG.md) lists the changes once the first
release is out.

Each image has a signed build provenance attestation, made by this repository's release workflow. To check
that an image was built here from the release tag, run:

```sh
gh attestation verify oci://ghcr.io/kb1rma/lochinvar-native-modbus:0.1.1 -R KB1RMA/lochinvar-native-modbus # x-release-please-version
```

### Serial port permissions

The container does not run as root. It runs as a user in group 20. On Debian, Ubuntu and Raspberry Pi
OS, group 20 (`dialout`) owns serial devices, so this just works. On other systems, check the group
number of the device:

```bash
ls -lnL /dev/serial/by-id/
```

The fourth column is the group number. If it is not 20, add `--group-add <that number>` to the
container. Many platforms call this "extra parameters".

## Settings

Every setting is an environment variable. You only need the MQTT ones. The rest have good defaults.

| Variable | Default | What it does |
|---|---|---|
| `SERIAL_PORT` | `/dev/ttyUSB0` | The serial device inside the container |
| `MODBUS_SLAVE_ID` | `1` | The boiler's Modbus address |
| `MODBUS_TIMEOUT` | `600ms` | How long to wait for each answer from the boiler. It tries 3 times. |
| `MQTT_HOST` | `localhost` | Your MQTT broker |
| `MQTT_PORT` | `1883`, or `8883` with TLS | The broker's port |
| `MQTT_USERNAME`, `MQTT_PASSWORD` | none | Your broker login |
| `MQTT_TLS` | `false` | Set to `true` to use an encrypted (TLS) connection |
| `MQTT_CA_FILE` | none | A certificate file to trust, for a broker with its own certificate |
| `MQTT_CLIENT_ID` | `lochinvar-native-modbus` | The name the bridge uses on the broker |
| `MQTT_TOPIC_PREFIX` | `lochinvar` | The start of every topic (see [Topics](#topics)) |
| `DISCOVERY_PREFIX` | `homeassistant` | Home Assistant's discovery prefix. Only change it if you changed it in Home Assistant. |
| `DEVICE_ID` | `lochinvar` | The device's ID in Home Assistant. Give each bridge its own if you run two. |
| `DEVICE_NAME` | `Lochinvar Boiler` | The device's name in Home Assistant |
| `FAST_INTERVAL` | `5s` | How often to read live values. Use `5s`, or just `5` for seconds. |
| `SLOW_INTERVAL` | `5m` | How often to read settings and counters |
| `REOPEN_AFTER` | `3m` | How long the boiler can stay silent before the bridge reopens the port (see below) |
| `REOPEN_MAX` | `30m` | The longest wait between reopens |
| `LOG_LEVEL` | `info` | How much to log: `debug`, `info`, `warn` or `error`. `debug` logs every failed read. |
| `CONFIG_FILE` | none | A file of settings (see below) |

### Other ways to give settings

You can also give any setting in these ways:

- **From a file.** Add `_FILE` to the name and give a file path. For example,
  `MQTT_PASSWORD_FILE=/run/secrets/mqtt_password` reads the password from that file. This works with
  Docker secrets. If you set both, the plain variable wins.
- **In a config file.** Put the file in a folder you mount into the container. Then set `CONFIG_FILE`
  to its path. Write one setting per line, using the same names as the variables. Lines that start with
  `#` are comments. A misspelled name stops the bridge with an error, so typos don't go unnoticed.

  ```
  # /config/lochinvar.env
  MQTT_HOST=192.168.1.10
  MQTT_USERNAME=lochinvar
  MQTT_PASSWORD=secret
  DEVICE_NAME="Basement Boiler"
  ```

- **On the command line.** Every setting is also an option, like `--mqtt-host`. Run the image with
  `--help` to see them all.

If you give the same setting twice, the command line wins first. The environment variable (or its
`_FILE`) comes next, and the config file comes last. The bridge saves nothing to disk. It only needs a
folder if you use a config file, a password file or a certificate.

## Topics

| Topic | What it holds |
|---|---|
| `lochinvar/status` | `online` or `offline`: whether the bridge is running. Shown as "Controller Online". |
| `lochinvar/link` | `online` or `offline`: whether the boiler answers. Shown as "Boiler Link OK". |
| `lochinvar/<component>/<entity>/state` | One reading, like `lochinvar/sensor/outlet_temp/state` = `150.2`. `None` means no reading. |
| `homeassistant/<component>/lochinvar/<entity>/config` | Discovery messages. They are sent again each time Home Assistant starts. |

The broker keeps the last message on every topic (they are "retained").

## When the link drops

This works like the ESPHome package. If the boiler stops answering:

- the live readings show as unavailable, along with Delta T, the sensor differences and Flame Present,
- "Boiler Link OK" turns off,
- Boiler Status reads "Link lost",
- settings and counters keep their last values,
- the bridge tries again every 30 seconds.

The ESP restarts itself after 3 minutes without the boiler. A container can't restart itself that
way. Instead, the bridge closes the serial port and opens it again. If that doesn't help, it waits twice
as long before the next try, up to 30 minutes. It goes slowly on purpose. On a Mac, the USB serial
driver stopped working after too many tries. "Link Reconnects" counts these reopens. Like the ESP, the
bridge only does this after the boiler has answered at least once. If the USB cable is unplugged, the
bridge tries to open the port every 10 seconds.

Sometimes the boiler answers with an error code (a Modbus exception) instead of the readings. That
still counts as an answer, so "Boiler Link OK" stays on and the port is not reopened. The live readings
are cleared, Boiler Status reads "Unavailable", and the bridge tries again every 30 seconds.

Sometimes only unplugging and replugging the USB cable brings the link back. That happened on the Mac
too. See "Limits and known problems" in the main README.

## Troubleshooting

| What you see | What to check |
|---|---|
| The log says the serial port can't be opened | Is the `--device` path right? Run `ls /dev/serial/by-id/` again. Check [Serial port permissions](#serial-port-permissions). |
| No device in Home Assistant | Check the MQTT login, and that MQTT discovery is on. Run with `LOG_LEVEL=debug`. |
| Boiler Link OK stays off | Is the cable in **PC Connection**, not USB Memory Drive? Try another cable. |
| Boiler Status reads "Unknown (n)" | A status we haven't seen yet, maybe a lockout. Please open an issue with the number and what the display says. |

To see what the bridge is doing, run `docker logs lochinvar`.

## Differences from the ESPHome package

- "Link Watchdog Restarts" is called "Link Reconnects" here (see above).
- "Controller Online" turns off when the container stops or loses the broker. It uses the MQTT "last
  will" message for this.
- Home Assistant shows the container as its own device, separate from an ESP.

## Developing

The bridge is written in Go (see `go.mod` for the version). It uses
[go.bug.st/serial](https://github.com/bugst/go-serial) and
[paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang).

| Path | What it is |
|---|---|
| `cmd/lochinvar-mqtt/` | The program's start: settings, signals and the main loop |
| `internal/modbus/` | Function 03 reads over the serial port. This is the only code that writes to the port. |
| `internal/decode/` | Turns raw values into readings: temperatures, status text, Delta T, the ignition check |
| `internal/entities/` | The list of sensors, matching `esphome/packages/boiler.yaml` |
| `internal/discovery/` | Topics and Home Assistant discovery messages |
| `internal/bridge/` | The read loop, link loss and reopening the port |
| `internal/mqttout/` | The MQTT connection, the last will, and when to send everything again |
| `internal/config/` | Command line options, environment variables and the config file |
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

The tests use a fake serial port and a test MQTT broker, so they never talk to a real boiler. They also
check that:

- every message sent is a function 03 read, and only `internal/modbus` writes to the port,
- the sensors match `esphome/packages/boiler.yaml` (names, addresses, units and decoding),
- every address is in `registers/controller.yaml`,
- the version matches `.release-please-manifest.json`.

CI also checks that `go.mod` is tidy. It runs `go vet`, golangci-lint (`.golangci.yml`), a short fuzz
test and govulncheck, and it builds the image for every platform.
