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

## Audit of the map against the captures (2026-10-24, after the session)

Re-checked `registers/controller.yaml` against the idle scan, all 597 rows of `flame_log.csv` and
`regs256_2047.json`. No new reads were sent to the boiler.

Corrected:
- 1097/1146 = 1700 and 1098/1150 = 8200 are fan speeds in rpm, not percents. Over 201 firing samples,
  r89 = 1700 + (rate - 10) * 6500/90 within 50 rpm, so 10 percent rate = 1700 rpm and 100 percent = 8200 rpm
  (it also predicts the display's 85 percent -> 7129 rpm). 1146-1150 is a curve 1700/3500/5300/6750/8200,
  r107 sits at 1700 at idle, and 1109/1110 = 2900 is the fan speed held during post-purge. r113/r114 are the
  live max/min.
- r81 is not a firing-rate maximum. It holds about 100.00 percent while firing and for 60 s after the
  flame goes out, then drops together with r132, r133 and r137 at 10:03:18. Most likely boiler pump
  post-circulation (speed or a 0-10 V output).

Mirrors that are not copies:
- r131 is the requested rate (always a multiple of 10, about 1 s ahead of r80).
- r107 is the commanded fan speed from r131 (within 9 rpm of the same curve, about 1 s ahead of r89).
- r64 and r65 are second thermistors (within 0.7 / 0.9 F of r62 / r63). r156 = max(r63, r65) in 92 percent
  of samples. True copies: 154 = 62, 155 = 73, 158 = 75, 106/149 = 89, 147 = 146, 144 = 143 (for now).

Temperature accuracy: at idle the formula matches the display within its truncation (131.7 -> 131,
129.7 -> 129, 41.3 -> 41). The hot-side gaps (for example display 171 vs register 173.6 at 09:57:49) match
register values 10-20 s earlier, the same lag seen on firing rate. So the "about 3 F error" is most likely
display lag. One display reading at steady high fire would confirm it.

Home Assistant pipe probes (outlet/return probes, strap-on): over the
log window the return probe tracks r73 with a spread of about 1 F (mean 7 F low, best at about 30-60 s lag),
while the difference to r63 swings 0-47 F. This independently confirms r73 = inlet. The outlet probe reads
about 12 F below r62. The probes lag and read low, so they confirm which register is which but cannot
calibrate the formula. the HA boiler-firing sensor stayed "on" through the 10:02:17 shutdown, so it is not
a usable burner-state reference.

New decodes (see the map for confidence):
- r84: 9 = running, 10 = post-purge (flame already 0), 4 = standby. r85 counts the post-purge down
  11.3 -> 0 s in tenths and reaches 0 exactly as r84 goes to 4.
- r146/r147: active target temperature. Exactly 32.0 F (raw 43036) with no demand. It fell to 32 at
  10:02:17 while the rate was rising, so that shutdown was a satisfied call for heat, not a fault.
- r142: SH1 setpoint after outdoor reset (172.1 F at 41.3 F outdoor, 167.7 F at 43.6 F; frozen with no
  call). So r143/r144 are probably the SH2/SH3 targets rather than copies of each other.
- r108: fan tach period, r108 * r110 = 3.75e6.
- r87: minutes counting down from 1440 while firing (one step every ~61 s), reset at standby; puts the
  burner start at about 09:48.
- Shutdown order: r118/r136 at gas off (10:02:17), r115/r127/r130 at flame out (10:02:18-19), r116 at
  10:02:27, r90/r105/r128 at the end of post-purge (10:02:30).

Traps:
- r72 = 3828 in the idle scan equals SH run hours by coincidence; it wanders 3553-3887 in the log.
- Clock 172-176 runs but is about 54 min slow and the date (20, 5, 18) is wrong.
- The firmware string continues past the date: "SIT 547_00 May 15 2018 938FT". "938FT" may identify the model.
- r63 (exhaust) reads 15-25 F below the inlet at low fire, which is odd for a flue sensor. The display agreed
  at the time, but a simultaneous display reading of inlet and exhaust would settle it.
- `tools/probe/07_decode_temps.py` decodes every value with bit 15 set, so counters above 32767 (for
  example ignitions 34928) show up as bogus temperatures.

## Open questions / next steps

1. Electrical level and connector of the boiler port; boiler model; fill the README TODOs.
2. Identify the remaining status/state registers (r115-r118, r127-r130, r136-r140). Needs a log of a full
   startup with the display status text and times (purge, ignition, run, post-purge, standby). This would
   also give the r84 codes for pre-purge and ignition.
3. Parameter block 1006-1295: read more settings from the installer menu (min/max setpoint, differentials,
   fan speeds, pump delay, outdoor reset) and match them. Check whether the display's pump delay is 60 s
   (r81/r132) and min/max fan is 1700/8200.
4. Register 1311 (2055) and the weekly schedule at 1223-1250.
5. Check `cwm12345/lochinvar-modbus` status/lockout code tables against our status registers.
6. Build the poller: read-only, blocks of at most 50, fast poll of 62-75/80/110/84, slow poll of counters
   and settings, `lochinvar/status` availability topic, reconnect handling for the adapter wedge.
