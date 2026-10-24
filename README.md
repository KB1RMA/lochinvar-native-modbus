# lochinvar-serial-mqtt

Read-only MQTT bridge for Lochinvar boilers via the controller's built-in Modbus RTU serial port.
No MODB-I-O board required.

Unofficial and not affiliated with Lochinvar. Everything here was learned by observation and
comparison with the boiler's display; it has not been checked against Lochinvar documentation.

## Status

Register mapping only. The MQTT client is not written yet.

## Hardware

- A USB-serial adapter (an FTDI FT232R worked).
- Link settings: 19200 baud, 8N1, slave ID 1, function 03 reads only.
- Port electrical level and wiring: TODO (RS-232 / RS-485 / TTL).

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
- `esphome/lochinvar.yaml`: read-only ESP32 config (Modbus RTU to MQTT). Copy `esphome/secrets.example.yaml`
  to `secrets.yaml`. `tools/check_esphome.py` checks it against the map and rejects write components.

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
