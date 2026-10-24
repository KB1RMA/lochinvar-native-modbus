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

## Tested hardware

| Controller firmware | Boiler model |
|---|---|
| SIT 547_00 May 15 2018 | TODO |
