# Reverse-engineering log

Date of work: 2026-10-24 (about 09:30 to 10:20 local). Boiler model: not recorded yet.

## Method (scripts in `tools/probe/`, run with `python3 -I`, no pyserial needed, uses termios)

1. `01_passive_baud_scan.py`: listened 3 s each at 1200-115200 baud, 8N1/8E1/8O1. Zero bytes everywhere.
   The device never transmits on its own.
2. `02_modbus_probe.py`: sent function 03/04 reads of register 0 to slave IDs 1-16 and 247 across bauds and
   parities. Only 19200 8N1, slave 1 answered. fn03 -> `01 03 02 00 0f f8 40`; fn04 -> exception 1.
3. `03_scan_holding_0_255.py`: read holding registers 0-255 one at a time. All readable. Output in
   `captures/holding_registers_slave1.csv`.
4. `04_snapshot_and_watch.py`, `05_timestamped_logger.py`: repeated snapshots and a 10 minute, ~1 s log
   (`captures/flame_log.csv`, 597 rows, 09:58:57-10:08:55), during which the burner went from 85 percent to off.
5. `06_scan_256_2047.py`: scanned 256-2047 for the lifetime counters and the parameter block
   (`captures/regs256_2047.json`).
6. `07_decode_temps.py`: decodes all flag-bit (0x8000) values with the temperature formula.

## Ground truth: values read off the boiler display

| When | Display | Register finding |
|---|---|---|
| idle, early | outdoor 41 F, inlet 129, outlet 131 | r62 about 12755 (15-bit) at outlet 131 |
| ~09:55 | flame 12.7 uA, fan 7129, delta T 16 | r66=12688, r110=7129 exactly |
| ~09:56 | boiler 85 percent | r80=8516, r131=8520 |
| ~09:57:31 | inlet 152, outlet 168, exhaust "152 rising" earlier | r62 13735 (outlet), r63 13316, r73 about 154 |
| ~09:57:49 | inlet "156", outlet 171 | r62 13808 -> 170.8 F with first fit |
| ~09:58 | outdoor 43, exhaust 156 | r75 about 43; r73 156.9 (r73 later matched better as inlet) |
| 09:59:43 | firing rate 35 percent | r80=3108 (display lags) |
| ~09:59:50 | rate 32 percent, falling | r80=2791 |
| ~10:00:18 | delta T 5 F | outlet - r73 = 4.2 F, so r73 is inlet, r63/65 exhaust |
| ~10:00:41 | delta T 4 F | outlet - r73 = 3.1 F |
| ~10:14 | standby; SH run 3828, SH cycles 34920, DHW run 0, DHW cycles 0, power hours 24266, ignitions 34928, ign attempts 35035 | r1309, r1313, r1315, r1303, r1307, r1305 (35036) |
| ~10:16 | SH1 175, SH2 125, SH3 125, DHW 120 | r1287, r1288, r1289, r1286 |
| ~10:18 | high limit auto reset 200, manual reset 210 | r1291, r1292/r1293 |

Burner shutdown observed in the log at 10:02:17-10:02:30 (see `docs/discovery-notes.md`), but the user did not
note the display state at that time.

## Findings that changed along the way

- First temperature fit: F = ((raw-32768) - 9451) / 25.5. Refined once parameters matched exactly:
  F = ((raw & 0x7FFF) - 9468) / 25 (0.04 F per count). Live sensors still disagree by up to about 3 F.
- Inlet and exhaust were first assigned the other way round; the delta T readings corrected that.
- Firing rate and fan: r80/r131 both track the firing rate; r89/r106/r107/r149 follow the fan target.

## Open questions / next steps

1. Electrical level and connector of the boiler port; boiler model; fill the README TODOs.
2. Identify status/state registers (r84, r115-r118, r127-r130, r136-r140). Needs a log of a full startup
   with the display status text and times (purge, ignition, run, post-purge, standby).
3. Parameter block 1006-1295: read more settings from the installer menu (min/max setpoint, differentials,
   fan speeds, pump delay, outdoor reset) and match them.
4. Register 1311 (2055) and the weekly schedule at 1223-1250.
5. Check `cwm12345/lochinvar-modbus` status/lockout code tables against our status registers.
6. Build the poller: read-only, blocks of at most 50, fast poll of 62-75/80/110/84, slow poll of counters
   and settings, `lochinvar/status` availability topic, reconnect handling for the adapter wedge.
