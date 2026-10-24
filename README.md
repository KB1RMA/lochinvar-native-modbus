# lochinvar-serial-mqtt

Read-only MQTT bridge for Lochinvar boilers via the controller's built-in Modbus RTU serial port.
No MODB-I-O board required.

Unofficial and not affiliated with Lochinvar. Everything here was learned by observation and
comparison with the boiler's display; it has not been checked against Lochinvar documentation.

## Status

Register mapping is still in progress (see `docs/reverse-engineering-log.md`). The bridge is an
ESPHome package: an ESP32 polls the boiler and publishes to MQTT with Home Assistant discovery.

## ESPHome package

`esphome/packages/boiler.yaml` holds the Modbus setup and every entity. It is read-only: it defines
no switches, numbers, selects or outputs, and CI rejects any. It needs a UART with the id
`boiler_uart`, which a transport package provides:

- `esphome/packages/usb-ft232.yaml`: an ESP32-S3 as USB host for the boiler's USB port (tested).
- `esphome/packages/uart.yaml`: the ESP's own UART pins (untested; set `uart_tx_pin` and
  `uart_rx_pin`). The boiler's port is USB, so this only fits if you reach a UART some other way.

Use it from GitHub in your own device config, pinned to a release tag. Your config supplies the board,
network and the Home Assistant link (`mqtt:` or `api:`):

```yaml
esphome:
  name: lochinvar

esp32:
  board: esp32-s3-devkitc-1
  framework:
    type: esp-idf

wifi:
  ssid: !secret wifi_ssid
  password: !secret wifi_password

mqtt:
  broker: !secret mqtt_broker

packages:
  transport: github://KB1RMA/lochinvar-serial-mqtt/esphome/packages/usb-ft232.yaml@v0.0.0 # x-release-please-version
  boiler: github://KB1RMA/lochinvar-serial-mqtt/esphome/packages/boiler.yaml@v0.0.0 # x-release-please-version
```

`esphome/lochinvar.yaml` is the same device built from a copy of this repository. Copy
`esphome/secrets.example.yaml` to `esphome/secrets.yaml` first.

## Development

```
python3 -m venv .venv && .venv/bin/pip install -r requirements.txt
.venv/bin/python tools/check_registers.py
.venv/bin/python tools/check_esphome.py
.venv/bin/esphome compile esphome/lochinvar.yaml
```

CI (`.github/workflows/validate.yml`) runs both checks, validates every config and compiles
`esphome/lochinvar.yaml` and `esphome/tests/*.yaml`.

## Releases

Releases are made by [release-please](https://github.com/googleapis/release-please). PRs are
squash-merged and their titles must be [Conventional Commits](https://www.conventionalcommits.org/)
(`feat: ...`, `fix: ...`; checked by `.github/workflows/pr-title.yml`). release-please keeps a release
PR open with the next version and changelog. Merging it tags `vX.Y.Z`, which is what the `github://`
refs above point at, and bumps every line marked `x-release-please-version`.

## Hardware

- The boiler's serial port is a USB port: an FTDI FT232R (USB ID 0403:6001) sits inside the controller.
  Connect with a plain USB cable. A computer sees it as a USB-serial port (`/dev/cu.usbserial-*` on macOS).
- For the ESPHome bridge: an ESP32-S3 board with a native USB port (tested on an ESP32-S3-DevKitC-1
  clone). Plug the boiler into the S3's USB port and power the board from COM/UART. Bridge the USB-OTG
  solder pad so the USB port supplies 5 V; without it the boiler's FT232R never shows up.
- Link settings: 19200 baud, 8N1, slave ID 1, function 03 reads only.

## Not the MODB-I-O board

The MODB-I-O board has a documented map (input registers at 30001+, temperatures in tenths).
This project targets the controller's own port, which uses holding registers and a different
temperature encoding. For MODB-I-O owners see
[cwm12345/lochinvar-modbus](https://github.com/cwm12345/lochinvar-modbus).

## Layout

- `registers/controller.yaml`: the register map (single source of truth, read-only by design).
- `captures/`: raw scans and logs the map was derived from.
- `docs/discovery-notes.md`: working notes from the reverse engineering session.
- `tools/check_registers.py`: sanity-checks the map file.
- `esphome/packages/`: the ESPHome package (`boiler.yaml`) and its transports.
- `esphome/lochinvar.yaml`: a complete read-only device using the packages (Modbus RTU to MQTT).
- `esphome/tests/`: extra configs CI compiles to cover the other transport and framework.
- `tools/check_esphome.py`: checks every ESPHome file against the map, rejects write components and
  checks the package version matches the release manifest.

## Long captures for mapping

`tools/probe/08_long_capture.py` logs every readable register (function 03 only) from a laptop over
hours or days. It reads 0-255 each second and the rest of the map (256-955, 1006-1405) each minute,
and writes only changes to `captures/long/YYYY-MM-DD.csv` as `time,addr,value`. Each file starts with a full snapshot.

```
caffeinate -is python3 -I tools/probe/08_long_capture.py
```

Keep the Mac on power: `caffeinate -s` only prevents sleep on AC, and closing the lid on battery still
sleeps it. While it runs, type what the display shows (status text, firing rate) and press Enter. Each
line is logged as a timestamped `note` row. `--hours N` stops after N hours; `--port` picks the device.

## Tested hardware

| Controller firmware | Boiler model |
|---|---|
| SIT 547_00 May 15 2018 | TODO |
