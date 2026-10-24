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

## Session 2: long capture of heat cycles (2026-10-24, 11:21 onwards)

Logged with `tools/probe/08_long_capture.py` (function 03 only; 0-255 every 1 s, 256-955 and 1006-1405 every
60 s, changes only) to `captures/long/2026-10-24.csv`. Display observations relayed by the user are in
`captures/long/notes-2026-10-24.csv`, with times approximate (typed into chat, display lags about 10 s).

### Link drops at ignition

The serial link went silent at the end of prepurge both times: 11:24:52 (r85 = 12) and 11:31:44 (r85 = 5,
same second r115 went to 1). The port stayed open with no error and the FTDI stayed on USB, but every read
timed out until the adapter was unplugged and replugged (the logger then reopens on its own). The boiler
answered fine while firing in session 1, so this is most likely ignition spark noise on the link. Gaps:
11:24:52-11:27:49 and 11:31:44-11:35:54. Check cable routing and grounding near the ignition lead.

### Cycle 1 timeline (zone call)

| Time | Registers | Display / meaning |
|---|---|---|
| 11:21-11:24 | r84 = 4 | standby, circulator running (user, ~11:24), outlet 151 F (r62 = 150.6 F) |
| 11:23:06 | r118, r138, r139, r146 on for 1 s, then back | demand blip only |
| 11:24:34 | r118 = 1, r138 4, r139 16, r140 96, r146 161.2 F, r132 10000, r133 1000 | zone called for heat (user) |
| 11:24:35 | r84 = 6, r105 = r128 = 1, r106/r107 = 2900, r85 = 177 counting down 10/s | prepurge, fan to ~2900 rpm |
| 11:24:52 | link lost (r85 = 12) | ignition |
| 11:27:49 | link back: r84 = 9, r80 85.2 %, r66 12.5 uA, r110 7129, r90 = r115 = r116 = r127 = r130 = 1 | running at 85 % (user) |
| 11:28:36-11:29:16 | r80/r131 and fan target ease down 85 -> 80 % as outlet reached ~164 F | modulating |
| 11:29:16 | r118 = 0, r138 8, r139 0, r140 112, r146 32 F | demand satisfied |
| 11:29:17 | r84 = 10, r80 = r131 = 0, r127 = r130 = 0, r137 = 1, r106 = 2900, r85 = 115 counting down | post-purge, flame out |
| 11:29:18-11:29:26 | r115 = 0 (11:29:18), r116 = 0 (11:29:26), fan coasting down | |
| 11:29:29 | r84 = 4, r105 = r128 = r90 = 0, r106 0, r107 1700, r87 back to 1440 | standby |
| 11:30:18-19 | r132 = r133 = 0, then r137 = 0 | anti-cycle ended (inferred, see below) |

### Cycle 2 (mostly inside the link gap)

Demand on at 11:30:51, but prepurge (r84 = 6) only started at 11:31:26, 35 s later. r137 had already cleared
at 11:30:19, so the delay is not the r137 anti-cycle; cause unknown. The link died at ignition (11:31:44).
Across the gap the counters moved: ignition attempts r1305 +2, ignitions r1307 +1, SH cycles r1313 +1. So the
first ignition attempt failed and a retry lit; cycle 1 moved each by exactly 1. On reconnect (11:35:54) the
burner was off, r84 = 4, r137 = 1. The display showed "clocked" and anti-cycling (user, ~11:36). r132 = r133 = 0
at 11:36:13 and r137 = 0 at 11:36:14. No other register changed after that. The display then went from
anti-cycling to standby in one step (user, ~11:36:40, allowing for display lag).

### Findings

- **r84 status codes:** 4 standby (also shown during anti-cycle, so anti-cycle is not in r84), 6 prepurge,
  9 running, 10 post-purge. The ignition state(s) fell inside the link gaps.
- **r85 phase timer**, tenths of a second, counts down: prepurge 17.7 s, post-purge 11.5 s.
- **r137 anti-cycle flag (likely):** 1 from flame-out until the off-delay ends (62 s after cycle 1), clears
  together with the display leaving anti-cycle. r132/r133 drop to 0 one second before it clears, and return to
  10000/1000 with the next demand.
- **Demand flags:** r118 (1 = call for heat), r138 (4 demand / 8 none), r139 (16 / 0), r140 (96 / 112), r146 and
  r147 (active target 161.2 F during demand, 32 F with none). 161 F with SH1 at 175 F and outdoor 48 F
  suggests an outdoor-reset target.
- **Blower flags:** r105 and r128 are 1 from prepurge start to post-purge end, so they track the blower.
- **Burner flags:** r115, r116, r127, r130 and r90 were on while running. r127 and r130 went off at flame-out,
  r115 1 s later, r116 9 s later and r90 at the end of post-purge. In cycle 2 r115 went to 1 the same second the
  link died, so it may be the igniter or gas valve.
- **Fan:** r106 and r107 follow the fan target (2900 rpm purge, 1700 idle in r107); r110 is the measured speed.
- **r87** counted down about 1 per minute while running and reset to 1440 at standby: a minutes timer, meaning
  unknown. **r176** goes up by 1 each minute; it is probably a clock.
- The pump has no flag identified yet: it was already running when the log started and its stop was not seen.

### Cycle 3 (11:54) and the software reopen test

- Prepurge 11:54:14 (r84 = 6, r140 96). At 11:54:31 r129 = 1 (r85 = 10), at 11:54:32 r115 = 1 and the flame
  signal r66 = 2150 (r85 = 1). The link died at 11:54:33, a second after flame appeared. So r129 is likely the
  igniter and r115 the gas valve. In cycle 2 both rose in the same second the link died.
- Closing and reopening the port in software (11:54:50) did not recover the link. tcsetattr failed with EINVAL
  (the FTDI wedge), and only a replug fixed it (logging resumed 11:57:43). The logger also crashed then,
  because termios.error is not an OSError. That is now fixed, and startup retries every 10 s until a replug.
- Across the gap: r1305 +1, r1307 +1, r1313 +1, so one clean ignition and a short burn that ended inside the gap.
- After the burn the demand stayed on (r118 = 1, r138 4, r139 16, target r146 159.3 F) with the burner off. r140
  was 112 and r137 was 0. The display showed "clocked", then "set point met" (user, ~11:56 and ~11:58), with
  outlet 151 F. No register except the r176 minute counter changed between those two display texts.
- **r140 looks like the burner request:** 96 when the burner is wanted, 112 when not. It went to 96 one second
  before each prepurge (11:24:34, 11:31:25, 11:54:14), not when demand came on. In cycle 2 demand began
  at 11:30:51 while r140 stayed 112 until 11:31:25, which explains that 35 s delay as a held-off burner request
  (setpoint met or clocked), not r137 anti-cycle.
- Cycle 4: prepurge 12:05:44, link lost at ignition 12:06:02, back after a replug at 12:14:31. Across the gap:
  r1305, r1307 and r1313 each +1 (one clean ignition, burn ended inside the gap), r87 1439 -> 1440. On return
  demand was still on with the burner off (r140 112), outlet 165.4 F above the 158.2 F target.

### Demand without firing (basis for a "calling but not heating" alert)

Intervals today where demand was on (r118 = 1) but the burner was not firing (r80 = 0), excluding link gaps:

| Interval | Length | Registers | Display / reason |
|---|---|---|---|
| 11:24:34-11:24:52 | 18 s | r84 6, r140 96 | prepurge (normal) |
| 11:30:51-11:31:26 | 35 s | r84 4, r140 112, outlet 157 / target 161 F | held off, then prepurge |
| 11:54:14-11:54:33 | 19 s | r84 6, r140 96 | prepurge (normal) |
| ?-12:05:44 (seen from 11:57:43) | at least 8 min, burn ended inside the 11:54:33-11:57:43 gap, so up to about 11 min | r84 4, r140 112, r137 0, outlet 153 / target 159 F | "clocked", then "set point met" |
| ?-12:15:52 (seen from 12:14:31) | at least 80 s | r84 4, r140 112, outlet 165 / target 158 F | set point met |

So demand with no flame is normal for several minutes at a time. The longest held-off period was 8-11 minutes, even
with the outlet below the target. An alert on demand alone would fire on every cycle.

Suggested alert logic (not yet built; the ESPHome config does not expose r118 or r140 yet):

- **Calling but not heating:** r118 = 1 and flame r66 = 0 continuously for more than 20 min (above the ~11 min
  seen today; tune it once more cycles are logged).
- **Burner wanted but not lighting (faster, more specific):** r140 = 96 (burner request) and r66 = 0 for more than
  about 2 min. Prepurge plus ignition took about 20 s today.
- **Failed ignitions:** ignition attempts (r1305) rising faster than ignitions (r1307). Cycle 2 had one failed
  attempt and a successful retry. More than one failed attempt in a day is worth a notice.
- **Unknown status:** r84 outside {4, 6, 9, 10} (plus the ignition codes once seen) for more than a minute. This
  probably catches lockouts; their codes are not seen yet.

**Real case caught at 12:15 (user):** the DHW zone was calling for heat with its valve closed. Opening the valve
triggered the 12:15:52 prepurge; from 12:16:41 the burner ran steadily at 85 % while the outlet fell to ~148 F
(real load). Before that, demand (r118 = 1) had been on continuously since 11:54:14. The burner ran two short
burns (cycles 3 and 4, each ending inside a link gap) with long "clocked" / "set point met" holds in between
(r140 112), and the outlet reached 165 F, above the 158 F target. So the "calling but not heating" signature on
the boiler side is:
- **Demand on for a long time with short burns:** r118 stays 1 across several burner cycles, and each burn ends
  on set point met (r140 back to 112) while r118 stays 1.
- **Held off most of the time:** the burner is held off (r140 112, r84 4) for most of the demand time, even though
  demand never drops.
- **Outlet near or above target with no firing:** heat isn't leaving the boiler loop.

A rule like "r118 = 1 for more than 20 min and burner duty under ~30 %" (time with r66 > 0 divided by demand time)
would have caught this. A real load like the one after 12:16 keeps the burner lit.

DHW note: the DHW cycle counter (r1315) is 0 for life, so this boiler has never run a boiler-controlled DHW
cycle. The indirect tank is most likely piped as a zone, and its call looks like any other space-heat call
(r118). The boiler cannot tell which zone is calling, so a DHW-specific alert needs the zone relay or tank
thermostat state from Home Assistant, combined with the boiler-side "not heating" condition above.

## Open questions / next steps

1. Electrical level and connector of the boiler port; boiler model; fill the README TODOs.
2. Status registers: partly done in session 2 (r84 codes, r85 timer, r137 anti-cycle, r140 burner request, demand and blower
   flags). Still open: the ignition state(s), which needs the link to survive ignition; the pump flag (note when
   the circulator stops); the 35 s demand-to-prepurge delay in cycle 2; r87, r132, r133 and r136.
3. Parameter block 1006-1295: read more settings from the installer menu (min/max setpoint, differentials,
   fan speeds, pump delay, outdoor reset) and match them.
4. Register 1311 (2055) and the weekly schedule at 1223-1250.
5. Check `cwm12345/lochinvar-modbus` status/lockout code tables against our status registers.
6. Build the poller: read-only, blocks of at most 50, fast poll of 62-75/80/110/84, slow poll of counters
   and settings, `lochinvar/status` availability topic, reconnect handling for the adapter wedge.
