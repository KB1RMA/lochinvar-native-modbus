# lochinvar-serial-mqtt: context for Claude

Goal: a read-only MQTT bridge (with Home Assistant discovery) for a Lochinvar boiler, using the
controller's built-in Modbus RTU serial port. There is no MODB-I-O board.

Start here: `registers/controller.yaml` (the map), `docs/reverse-engineering-log.md` (how we got it,
display readings used as ground truth, open questions), `docs/discovery-notes.md` (earlier notes).

## Hard rules

- **Read-only.** Only Modbus function 03 (and 04, which the device rejects) was ever sent. Never write
  function 05/06/15/16/22/23 or any diagnostic function to the boiler. The user confirmed this explicitly.
  The poller must have no write path at all.
- Ask before transmitting anything new to the boiler.
- Don't commit or push without being asked.

## ESPHome package and releases

- The published unit is an ESPHome package: `esphome/packages/boiler.yaml` (Modbus + entities, needs a UART
  with id `boiler_uart`) plus a transport (`usb-ft232.yaml`, `uart.yaml`). `esphome/lochinvar.yaml` is the
  user's own device built from them; `esphome/tests/` holds extra configs CI compiles.
- `tools/check_esphome.py` scans every ESPHome YAML file: no write components or write keys, every address in
  the map, and the package `project.version` must equal `.release-please-manifest.json`.
- release-please (`simple` type) owns versions: never hand-edit `version.txt`, the manifest, `CHANGELOG.md`
  or lines marked `x-release-please-version`. PR titles must be Conventional Commits (they are squash-merged).
- Remove `release-as` from `release-please-config.json` after the first (0.1.0) release.
- No GitHub remote yet; the GitHub owner `KB1RMA` in README refs and `project.name` is assumed.

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

- r84 status: 4 standby (also shown during anti-cycle, "clocked" and "set point met"), 6 prepurge, 9 running,
  10 post-purge. Ignition and lockout codes not seen yet (the link drops at ignition).
- r85 phase timer, 0.1 s, counts down (prepurge 17.7 s, post-purge 11.5 s).
- r118 = 1 heat demand (any zone; DHW is piped as a zone, so the boiler can't tell which one).
- r140 = 96 burner wanted, 112 held off (no demand, set point met, clocked). Changes 1 s before prepurge.
- r132 boiler pump output command, 0.01 % (100 % from demand until ~60 s post-pump); r81 is the measured output. Display "Rate Out" volts = r80 / 1000.
- r137 = 1 anti-cycle after flame-out; r129 igniter (guess), r115 gas valve (guess), r105/r128 blower.
- Demand with no flame for up to ~11 min is normal. A DHW call with its valve closed showed as demand held on
  for 20+ min with only short burns ending in "set point met".

## Link drops at ignition

The FTDI link goes silent the second ignition starts, every time (and once during post-purge, 12:26:46). Closing and reopening the port in software
does not help (tcsetattr EINVAL). Only an unplug and replug does. `tools/probe/08_long_capture.py` retries
every 10 s and resumes by itself after a replug. Suspect spark noise on the cable.

## Related

- `cwm12345/lochinvar-modbus` and `cwm12345/ha-lochinvar` on GitHub target the MODB-I-O board
  (different map). Compared in session 3 (docs/reverse-engineering-log.md): no address or encoding
  overlap and no MODB-style status/blocking/lockout register on the native port. Still useful: the lockout
  code table (161-240) if a lockout shows up, and their table order, which placed r157/r159 and r121-r125
  (guesses). Nothing from them has been run or copied.
- `tools/analysis/backtest.py` replays captures offline and checks the candidate mappings; rerun it on new captures.
