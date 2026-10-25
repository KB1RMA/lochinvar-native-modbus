# lochinvar-native-modbus: context for Claude

Goal: a read-only MQTT bridge (with Home Assistant discovery) for a Lochinvar boiler, using the
controller's built-in Modbus RTU serial port. There is no MODB-I-O board. Two bridges ship from this repo
(GitHub: KB1RMA/lochinvar-native-modbus, renamed from lochinvar-serial-mqtt; the local folder keeps the old name):
the ESPHome package (`esphome/`) and a Docker container (`mqtt/`). They must expose the same entities.

Start here: `registers/controller.yaml` (the map), `docs/reverse-engineering-log.md` (how we got it,
display readings used as ground truth, open questions), `docs/discovery-notes.md` (earlier notes).

## Hard rules

- **Read-only.** Only Modbus function 03 (and 04, which the device rejects) was ever sent. Never write
  function 05/06/15/16/22/23 or any diagnostic function to the boiler. The user confirmed this explicitly.
  The pollers must have no write path at all. The container (`mqtt/internal/modbus`) hand-rolls function 03
  frames; don't add a Modbus library. `mqtt/internal/bridge/readonly_test.go` enforces this.
- Ask before transmitting anything new to the boiler.
- Don't commit or push without being asked.

## Components and releases

release-please runs in manifest mode with two components (`release-please-config.json`, separate release PRs).
A commit belongs to a component by the paths it changes, not its scope; shared files (registers/, docs/, tools/,
root README) release neither.

- **esphome** (`esphome/`, tags `esphome-vX.Y.Z`): `packages/boiler.yaml` (Modbus + entities, needs a UART with
  id `boiler_uart`) plus a transport (`usb-ft232.yaml`, `uart.yaml`). Users pull
  `github://KB1RMA/lochinvar-native-modbus/esphome/packages/boiler.yaml@esphome-vX.Y.Z`, so the package paths must
  not move. `esphome/lochinvar.yaml` is the user's own device built from them and `esphome/tests/` holds extra
  configs CI compiles. Only `esphome/tests/` is excluded from the component: release-please's `exclude-paths`
  matches directories only, so a `fix:` to `lochinvar.yaml` still cuts an esphome release (use `chore:`). `tools/check_esphome.py` scans every ESPHome YAML
  file: no write components or write keys, every address in the map, and `project.version` and the
  `esphome/README.md` refs must equal the manifest's `esphome` entry.
- **mqtt** (`mqtt/`, tags `mqtt-vX.Y.Z`, image `ghcr.io/kb1rma/lochinvar-native-modbus`): a Go module
  (`cmd/lochinvar-mqtt`, `internal/...`; deps go.bug.st/serial and paho.mqtt.golang, mochi-mqtt only in tests),
  a distroless Dockerfile that cross-compiles for amd64/arm64/arm/v7, and tests. Version lives in
  `internal/version/version.go`. `docker-publish.yml` pushes on an mqtt release (called from
  `release-please.yml`, or run by hand with a tag) with a signed provenance attestation
  (`actions/attest-build-provenance`; the caller must grant `id-token` and `attestations: write`). CI (`validate.yml`): `go mod tidy -diff`, `go mod verify`,
  `go vet`, golangci-lint v2 (`mqtt/.golangci.yml`, pinned version in the workflow), `go test -race -shuffle=on`,
  a 15 s fuzz of the request builder, govulncheck, and a multi-arch image build. The tests check parity with
  `boiler.yaml` (names, addresses, units, decoding), every address against the map, that every frame sent is a
  function 03 read and only `internal/modbus/link.go` calls Write, and the version against the manifest. It
  reads only the blocks `08_long_capture.py` reads (50-99, 100-149, 1256-1305, 1306-1355). When `boiler.yaml`
  changes behavior, change `mqtt/` the same way (or the parity test fails). Intended differences: "Link
  Reconnects" instead of "Link Watchdog Restarts" (it reopens the port after 3 min down, backing off to 30 min),
  "Controller Online" from the MQTT last will. No Go on this Mac by default: a session fetched go and
  golangci-lint into its scratchpad to verify; Docker is not installed either.
- Never hand-edit `version.txt`, `CHANGELOG.md` (per component, created by release-please), the manifest, or lines
  marked `x-release-please-version`. Each component's marked lines live only in its own folder, because the
  generic updater writes one version into every marked line of a file. PR titles must be Conventional Commits
  (they are squash-merged).
- Remove `release-as` from both packages in `release-please-config.json` after the first (0.1.0) releases.

## Link facts

- The boiler's port is USB: an FTDI FT232R (0403:6001) is inside the controller, connected with a plain USB
  cable. It shows up as `/dev/cu.usbserial-*` on macOS. There is no external adapter and no RS-232/RS-485 level to work out.
- ESP32-S3 bridge: boiler cable in the S3's USB (OTG) port, power in COM. The USB-OTG pad must be bridged so
  that port supplies 5 V; without it the FT232R never enumerates and the log shows "Channel not initialised".
- Modbus RTU, 19200 baud, 8N1, slave 1. The device is silent until polled (passive listening heard nothing).
- Function 04 -> exception 1. Unmapped addresses -> exception 2 (e.g. 956-1005, 1406+).

## Known gotchas

- On macOS the FTDI port twice stopped accepting `tcsetattr` (EINVAL; ioctl "Device error") after many
  open/close cycles. Unplug and replug the boiler's USB cable. Prefer one open per run.
- The boiler display lags the registers by roughly 10 s (firing rate, delta T), so compare with timestamps.
- 32-bit counters store the high word at the even address, the number you want is usually in the odd one.
- Temperature formula is exact for parameters; live sensors differed from the display by up to ~3 F while
  firing, which matches display lag (idle readings match within the display's truncation).
- Strap-on pipe probes on the boiler outlet and return (in Home Assistant) read
  7-12 F low and lag by ~1 min: good for telling sensors apart, not for calibration.
- A boiler USB menu entry reading "waiting device..." exists on the controller; it is unrelated to this serial link.

## Boiler states (from the 2026-10-24 long capture; details in docs/reverse-engineering-log.md, session 2)

- r84 status: 4 standby (also shown during anti-cycle, "clocked" and "set point met"), 6 prepurge, 7 ignition
  (5-10 s, starts with the flame signal; session 4), 9 running, 10 post-purge. Lockout codes not seen yet.
- r85 phase timer, 0.1 s, counts down (prepurge 17.7 s, post-purge 11.5 s).
- r118 = 1 heat demand (any zone; DHW is piped as a zone, so the boiler can't tell which one).
- r140 = 96 burner wanted, 112 held off (no demand, set point met, clocked). Changes 1 s before prepurge.
- r132 boiler pump output command, 0.01 % (100 % from demand until ~60 s post-pump); r81 is the measured output. Display "Rate Out" volts = r80 / 1000.
- r137 = 1 anti-cycle after flame-out; r129 igniter (guess), r115 gas valve (guess), r105/r128 blower.
- Demand with no flame for up to ~11 min is normal. A DHW call with its valve closed showed as demand held on
  for 20+ min with only short burns ending in "set point met".

## Link drops at ignition (Mac only so far)

On the Mac, the FTDI link went silent the second ignition started, every time (and once during post-purge, 12:26:46). Closing and reopening the port in software
does not help (tcsetattr EINVAL). Only an unplug and replug does. `tools/probe/08_long_capture.py` retries
every 10 s and resumes by itself after a replug. Suspect spark noise on the cable.

On the ESP32-S3 bridge the link survived 24 ignitions, but it died once mid-burn (2026-10-24 20:24:33, 3.5 min
into running) and stayed dead 17 min until the ESP restarted (log session 5). The package now clears the readings
while the link is down and restarts the ESP after 3 min (Link Watchdog Restarts counts them).
The ESP rebooted 3 times (14:22, 14:42, 14:48) that no OTA flash explains; worth watching too.

## Related

- `cwm12345/lochinvar-modbus` and `cwm12345/ha-lochinvar` on GitHub target the MODB-I-O board
  (different map). Compared in session 3 (docs/reverse-engineering-log.md): no address or encoding
  overlap and no MODB-style status/blocking/lockout register on the native port. Still useful: the lockout
  code table (161-240) if a lockout shows up, and their table order, which placed r157/r159 and r121-r125
  (guesses). Nothing from them has been run or copied.
- `tools/analysis/backtest.py` replays captures offline and checks the candidate mappings; rerun it on new captures.
