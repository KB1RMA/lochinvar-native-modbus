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

## Link facts

- Adapter: FTDI FT232R, shows up as `/dev/cu.usbserial-*` on macOS.
- Modbus RTU, 19200 baud, 8N1, slave 1. The device is silent until polled (passive listening heard nothing).
- Function 04 -> exception 1. Unmapped addresses -> exception 2 (e.g. 956-1005, 1406+).
- Electrical level (RS-232/RS-485/TTL) of the boiler port is still unknown.

## Known gotchas

- On macOS the FTDI port twice stopped accepting `tcsetattr` (EINVAL; ioctl "Device error") after many
  open/close cycles. Unplug and replug the adapter. Prefer one open per run.
- The boiler display lags the registers by roughly 10 s (firing rate, delta T), so compare with timestamps.
- 32-bit counters store the high word at the even address, the number you want is usually in the odd one.
- Temperature scale differs slightly between parameters (exact) and live sensors (about 3 F error).
- A boiler USB menu entry reading "waiting device..." exists on the controller; it is unrelated to this serial link.

## Related

- `cwm12345/lochinvar-modbus` and `cwm12345/ha-lochinvar` on GitHub target the MODB-I-O board
  (different map). Their status/blocking/lockout code tables may help decode registers 84 and 138-140.
  Nothing from them has been run or copied.
