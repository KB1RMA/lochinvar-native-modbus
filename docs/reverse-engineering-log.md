# Reverse-engineering log

Date of work: 2026-10-24 (about 09:30 to 10:20 local). Boiler: Lochinvar Knight Fire Tube KHB155N, Series 100,
natural gas, 155,000 BTU/hr input (15,500 minimum, 10:1), built 2023 (rating plate, see session 6).

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
- **Boiler pump (found later, ~12:25):** r132 = boiler pump output in 0.01 % (r133 is the same in 0.1 %). It is 100 %
  from the start of demand until about 60 s after flame-out (post-pump: 11:29:17 -> 11:30:18), then 0. The 11:24 "pump
  running in standby" fits too: r132 was 10000 from the 11:23:06 blip until 11:24:09. Display at ~12:24:
  "BLR Pump Out 99 % cycling to 100 %" while r132 read 10000 steadily.
- **Other system readings from the display, ~12:24 (burner running, 28-33 %):**
  - BMS IN 0.0 V and SYS PUMP SPEED 0 %. Not identified; too many registers read 0 to tell.
  - Rate Out 2.8 V (r80 = 28.6 % at 12:24:20), then 3.2 V (~12:25:30, r80 = 32.8 %). So Rate Out is the firing
    rate on a 0-10 V scale (V = r80 / 1000), not a separate register.
  - r135 reads 1000 with demand and drops to 0 about 30 s after flame-out. It is not the system pump, which read
    0 % at the time.

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

The DHW call ended at 12:26:36 (r118 0, r140 112, r146 32 F). The burner went into post-purge at 12:26:37 after
about 10 min at 28-33 %, with the outlet at 157 F. **The link then dropped at 12:26:46, during post-purge, not at
ignition.** That is the first drop seen outside ignition, possibly from a relay or the gas valve switching off.
Only a replug brought it back (12:42:48). No burner start happened in the gap: r1305, r1307 and r1313 were
unchanged.

A rule like "r118 = 1 for more than 20 min and burner duty under ~30 %" (time with r66 > 0 divided by demand time)
would have caught this. A real load like the one after 12:16 keeps the burner lit.

DHW note: the DHW cycle counter (r1315) is 0 for life, so this boiler has never run a boiler-controlled DHW
cycle. The indirect tank is most likely piped as a zone, and its call looks like any other space-heat call
(r118). The boiler cannot tell which zone is calling, so a DHW-specific alert needs the zone relay or tank
thermostat state from Home Assistant, combined with the boiler-side "not heating" condition above.

## Audit of the map against the session 1 captures (2026-10-24)

Re-checked `registers/controller.yaml` against the idle scan, all 597 rows of `flame_log.csv` and
`regs256_2047.json`. No new reads were sent to the boiler. Where session 2 above has display ground truth
(r84 codes, r118, r137, r140, r132), its names are used in the map; this section only adds to it.

Corrected:
- 1097/1146 = 1700 and 1098/1150 = 8200 are fan speeds in rpm, not percents. Over 201 firing samples,
  r89 = 1700 + (rate - 10) * 6500/90 within 50 rpm, so 10 percent rate = 1700 rpm and 100 percent = 8200 rpm
  (it also predicts the display's 85 percent -> 7129 rpm). 1146-1150 is a curve 1700/3500/5300/6750/8200,
  r107 sits at 1700 at idle, and 1109/1110 = 2900 is the fan speed held during post-purge. r113/r114 are the
  live max/min.
- r81 is not a firing-rate maximum. It holds about 100.00 percent while firing and for 60 s after the
  flame goes out, then drops together with r132, r133 and r137 at 10:03:18. Most likely boiler pump
  post-circulation (speed or a 0-10 V output). The session 2 capture confirms it: r81 read 3917, 1243 and
  6674 mid-ramp in the same seconds r132 switched, so r81 is the measured boiler pump output and r132 the command.

Mirrors that are not copies:
- r131 is the requested rate (always a multiple of 10, about 1 s ahead of r80).
- r107 is the commanded fan speed from r131 (within 9 rpm of the same curve, about 1 s ahead of r89).
- r64 and r65 are second thermistors (within 0.7 / 0.9 F of r62 / r63). r156 = max(r63, r65) in 92 percent
  of samples. True copies: 154 = 62, 155 = 73, 158 = 75, 106/149 = 89, 147 = 146, 144 = 143 (for now).

Temperature accuracy: at idle the formula matches the display within its truncation (131.7 -> 131,
129.7 -> 129, 41.3 -> 41). The hot-side gaps (for example display 171 vs register 173.6 at 09:57:49) match
register values 10-20 s earlier, the same lag seen on firing rate. So the "about 3 F error" is most likely
display lag. One display reading at steady high fire would confirm it.

Home Assistant pipe probes (strap-on, on the boiler outlet and return): over the
log window the return probe tracks r73 with a spread of about 1 F (mean 7 F low, best at about 30-60 s lag),
while the difference to r63 swings 0-47 F. This independently confirms r73 = inlet. The outlet probe reads
about 12 F below r62. The probes lag and read low, so they confirm which register is which but cannot
calibrate the formula. A separate Home Assistant "boiler firing" sensor stayed "on" through the 10:02:17 shutdown, so it is not
a usable burner-state reference.

New decodes (see the map for confidence):
- r84 = 10 is post-purge with the flame already 0 (session 2 adds 6 = prepurge). r85 counts the post-purge
  down 11.3 -> 0 s in tenths and reaches 0 exactly as r84 goes to 4.
- r146/r147: active target temperature. Exactly 32.0 F (raw 43036) with no demand. It fell to 32 at
  10:02:17 while the rate was rising, so that shutdown was a satisfied call for heat, not a fault.
- r142: SH1 setpoint after outdoor reset (172.1 F at 41.3 F outdoor, 167.7 F at 43.6 F; frozen with no
  call). So r143/r144 are probably the SH2/SH3 targets rather than copies of each other.
- r108: fan tach period, r108 * r110 = 3.75e6.
- r87: minutes counting down from 1440 while firing (one step every ~61 s), reset at standby; puts the
  burner start at about 09:48.
- Shutdown order: r118 (demand) and r136 at demand end (10:02:17), r115/r127/r130 at flame out (10:02:18-19), r116 at
  10:02:27, r90/r105/r128 at the end of post-purge (10:02:30).

Traps:
- r72 = 3828 in the idle scan equals SH run hours by coincidence; it wanders 3553-3887 in the log.
- Clock 172-176 runs but is about 54 min slow and the date (20, 5, 18) is wrong.
- The firmware string continues past the date: "SIT 547_00 May 15 2018 938FT". "938FT" may identify the model.
- r63 (exhaust) reads 15-25 F below the inlet at low fire, which is odd for a flue sensor. The display agreed
  at the time, but a simultaneous display reading of inlet and exhaust would settle it.
- `tools/probe/07_decode_temps.py` decodes every value with bit 15 set, so counters above 32767 (for
  example ignitions 34928) show up as bogus temperatures.

## Session 3: comparison with the MODB-I-O map and backtest (2026-10-24, desk only)

No reads were sent to the boiler. `cwm12345/lochinvar-modbus` (and `ha-lochinvar`, which vendors it) was
cloned to a scratch directory and read; nothing from it was run or copied. Its map is for the MODB-I-O
board, which presents a 16-register summary (input registers 30001-30016, holding 40001-40008, tenths of a
degree F when the display is in F). No address or encoding carries over to the controller's own port. What
does carry over is the order of its tables, its live sample values (a Knight Wall Hung at idle, in its
`tests/conftest.py`) and its code tables from the MODB-I-O manual (status, blocking, lockout 161-240).
Matches based on order or sample values are weaker evidence than a display reading; the map says which is which.

Each candidate was then backtested with `tools/analysis/backtest.py` against `flame_log.csv` and
`long/2026-10-24.csv`. Counts are seconds of boiler time with the link up (each state weighted by how long
it held), not rows. The replay pauses on `link` drop rows and on gaps over 8 s (the 11:24:52 and 11:31:44
drops have no `link` row), and resumes at the next `snapshot` or `link,ok`. An earlier ad-hoc replay
skipped 11:57-12:06 and counted the 11:31:44 drop as 250 s of stale state; the numbers here are from the
fixed one.

Held up:
- r146 = 32.0 F if and only if r118 = 0 (all 600 s of session 1, all 3383 s of session 2). The MODB-I-O
  outlet setpoint uses the same "32 = no setpoint" convention. r147 = r146 throughout, so whether r147 is the
  system setpoint (equal with no system sensor) can't be told here.
- A MODB-style status built from native registers (r84, then r137 anti-cycle, r118 with r140 = 112 "demand
  held off", r118 "demand", r132 > 0 "pump delay", else idle) matches every display note that has data: the
  11:24 "standby" with the circulator running is pump delay (their status 33), and anti-cycle ends 20 s
  before the display leaves it. The 11:57 "clocked" and 11:58 "set point met" notes are both "demand, held
  off"; the registers do not separate them. This is now the ESPHome Boiler Status.

Relabelled:
- r127 (and r130, identical) = 1 exactly when r84 = 9. It is the run state, not a separate flame input.
- r67 reads 1512 only with the pump on and 1509 otherwise, flickering from the second r135 drops until the
  pump stops. An analog value nudged by relay load; meaning unknown.
- r104 correlates 0.98-0.998 with r110 and r80 while running, with a non-constant ratio: maybe fan PWM.
- r111 = 50 while running and 250 otherwise, r112 = -r111 always: probably a fan ramp limit.

Untestable with these captures (nothing changed): r121-r125 = 1 and r120/r126 = 0 throughout, which fits
the MODB-I-O idle input word 62 (flow, gas pressure, louver, air pressure, blocked drain on; high limit off),
r157/r159 (tank, system supply by table order), r68/r70/r77 (no sensor), r134 (system pump speed by position).

New observation: in all four drops that happened during prepurge (11:31:44, 11:54:32, 12:06:01, 12:16:10)
the last poll before the drop already shows a flame signal of 1.5-2.2 uA with r84 still 6. So the link dies
at the moment the flame rod first sees current, which is when the spark is running. That fits the
spark-noise suspicion; it does not tell spark noise on the flame rod apart from a real early flame.

## Session 4: first cycles on the ESP32-S3 bridge (2026-10-24, 13:50 onwards)

The ESPHome package (`esphome/lochinvar.yaml`) went live at 13:50 and published to Home Assistant. Data is
from the Home Assistant recorder, not a capture file. Live readings and status are polled every 5 s, the
counters every 60 s, so the times below are good to about 5 s and are when Home Assistant received each value.

### The link survives ignition

Both burns went through ignition with no gap, the Boiler Link OK sensor stayed on and no replug was needed.
On the Mac with the same FT232R (session 2) the link died at every ignition. Possible reasons: the S3's
USB host recovers differently, or the cable or grounding is different now. That isn't known yet. Keep
watching Boiler Link OK over more cycles before calling it fixed.

### Timelines

| | Burn 1 | Burn 2 |
|---|---|---|
| Heat demand (r118) | 14:01:54-14:03:59 | 15:38:48-15:54:58 |
| Burner request (r140 = 96) | 14:02:04 | 15:38:48 |
| Prepurge (r84 = 6) | 14:02:09, 35 s | 15:38:53, 15 s |
| Ignition (r84 = 7) | 14:02:44, about 5 s | 15:39:08, about 10 s |
| Flame signal first > 0 | 14:02:43 | 15:39:07 |
| Running (r84 = 9) | 14:02:49-14:03:59 (70 s) | 15:39:18-15:54:58 (15.7 min) |
| Post-purge (r84 = 10) | 14:03:59, 10 s | 15:54:58, 15 s |
| Anti-cycle (r137) | not seen | 15:54:58-15:55:58 |
| Pump output (r132) 100 % | 14:01:54-14:04:59 | 15:38:48-15:55:58 |
| Counters +1 | 14:05:05 | 15:40:41 |

### Findings

- r84 = 7 is ignition. It sat between prepurge and running in both burns. The flame signal came up within
  a second of it starting, and ignition attempts and ignitions both went up by one each time with failed
  ignitions unchanged at 109. Boiler Status now shows "Ignition".
- Both burns ended because the call for heat ended, not "set point met". In each one r118 and r140 dropped
  in the same poll, and post-purge followed. Burn 1 was a 2-minute zone call.
- Burn 1's prepurge took 35 s, twice the 15-18 s seen everywhere else. There was no failed ignition, so it
  wasn't a retry. The cause is not known; it may be a longer purge after a long idle.
- Peak firing rate 85 %, peak fan 7142 rpm (burn 2). The fan started at about 3000 rpm in prepurge.
- The counters (ignitions, attempts, SH cycles) updated 1-2 minutes after each burn. That is the 60 s poll
  plus however late the controller writes them.
- Delta T was 3-7 F while firing. With the pump off it climbs to 27 F within 45 minutes, because the
  stagnant water at the inlet sensor cools faster than at the outlet. It means nothing without flow.
- The strap-on probes read 7-12 F low and lagged, as before.
- The ESP dropped off Home Assistant for about 5 s six times: 13:56, 14:00, 14:22, 14:42, 14:48, 15:15. None
  happened during a burn. The slow counters went unknown for about a minute after each drop, so each was a
  reboot. 14:00 and 15:15 line up with firmware commits (OTA). 14:22, 14:42 and 14:48 are not explained.

## Session 5: overnight review of the ESP32-S3 bridge (2026-10-24 13:50 to 2026-10-25 09:23)

From the Home Assistant recorder and a live ESPHome log. 24 burn cycles, 21 of them after 04:00.

### What checked out

- Every burn followed prepurge, 7, running, post-purge, anti-cycle, standby. "Demand, held off" and "Pump
  delay" showed where expected. No other status code appeared.
- Since the 20:41 restart, ignitions and ignition attempts both rose by 21, matching the 21 ignitions in
  the status history. SH cycles rose by 21 as well. SH run time rose 2 h against about 1.3 h of burning (whole hours).
- The live log showed every 5 s poll succeeding, with no retries.

### Problems found (fixed in the package the same day)

- **The link died mid-burn on the ESP too.** At 20:24:33, about 3.5 min into running and not at ignition,
  Boiler Link OK went off and stayed off until the ESP restarted at 20:41:34 (same firmware, so not an OTA).
  Who or what restarted it is not known. Meanwhile every reading held its last value, so Home Assistant
  showed "Running" and a 12.6 uA flame for 17 minutes. Readings now clear when the link drops, and a
  watchdog restarts the ESP after 3 minutes. Whether a software restart clears the wedge, as it seemed to here,
  shows in Link Watchdog Restarts and Boiler Link OK.
- **False failed ignition.** The 5-minute counter poll at 20:20:41 came during prepurge: attempts had
  gone up, ignitions had not yet, so Failed Ignitions read 110 for 20 minutes. Counter reads taken during
  prepurge/ignition or in the 30 s after are now skipped.
- **Firmware behind the repo.** The dashboard's device config is a pasted copy of the package, built
  2026-10-24 15:13, so Home Assistant showed "Unknown (7)" at every ignition.
- **No long-term statistics** for the four temperatures and the flame signal. An early firmware sent the
  units garbled (`¬∞F`, `¬µA`) and the recorder kept those as the units. This needs fixing in Home Assistant
  (Developer Tools, Statistics), not in the package.

### Boiler behaviour

From 04:05 to 09:13 the boiler fired 21 times. Many burns were 1-3 minutes, with about 50 s of anti-cycle in
between. The outlet reached 168-173 F against a 175 F SH1 setpoint just before each burn ended.

## Session 6: the Lochinvar manuals (2026-10-25, desk only)

No reads were sent to the boiler. The Knight Fire Tube service manual and I&O manual (Series 100-121) and Lochinvar's
SMART SYSTEM PC Program Instructions were compared with the map. Details are in
[`manual-cross-reference.md`](manual-cross-reference.md). In short:

- The USB port is the display's "PC Connection" port for Lochinvar's WinPro program ("SMART SYSTEM 938"). The
  firmware string's "938FT" is that platform, Fire Tube variant. That settles the electrical level (USB) and
  sets which boilers the README says this covers.
- 1068-1121 is WinPro's factory parameter table in order. The outlet, flue and delta T limits match the service
  manual exactly (185/195 F, 215/225 F, 55/60 F). Temperature differences are raw / 25 F and times are 0.1 s.
- Labeled from manual defaults and the logs: the SH1 outdoor reset curve (35 F -> 180 F, 70 F -> 130 F; it predicts
  r142 within 0.2 F), SH2/SH3 curves, freeze protection, pump delays (boiler 60 s, matching the post-pump), night
  setback triggers (1223-1250), BMS settings, service reminders, and the installer password at 1144.
- r146 is WinPro's dSP. r111 shows the INCFAN / INCFAN_R step limits. 1109/1110 are the purge and ignition fan speeds.
  r1311 is probably SH hours below 50 % rate.
- From the manuals, not seen yet: status START (maybe r84 = 5), 4 ignition tries before lockout, a last-10-lockouts
  table and per-fault counters (probably 1316-1384), and the Derivative Outlet / Flue rates (r150?).

The boiler was thought to be a 2008 install. The Knight boilers of that time had a 2-line display and an RJ11 PC
jack, and this one has the large display, a USB port and 2018 Fire Tube firmware. The rating plate settled it:
**KHB155N, Series 100**, serial prefix 2332 (most likely 2023, week 32), heat exchanger stamped 2022. That fits
the 24,266 power-on hours (about 2.8 years) and puts this boiler in the range the manuals above cover.

## Open questions / next steps

1. Done in session 6: KHB155N Series 100, USB port. Still open: whether other SMART SYSTEM 938 boilers (KHN, WHN,
   WHB, Knight XL, Armor) use the same addresses.
2. Status registers: partly done in session 2 (r84 codes, r85 timer, r137 anti-cycle, r140 burner request, r132
   boiler pump, demand and blower flags). Ignition (r84 = 7) seen in session 4. Still open: the long
   35 s prepurge in session 4; whether r129/r115 really are igniter and gas valve, now that the link stays
   up through ignition (needs a capture of the 0-255 block at 1 s); why "set point met" is shown with the outlet below the r146 target; BMS IN and SYS PUMP SPEED
   (both 0 so far); r87, r135 and r136.
3. Parameter block 1006-1295: read more settings from the installer menu (min/max setpoint, differentials,
   fan speeds, pump delay, outdoor reset) and match them. Check whether the display's pump delay is 60 s
   (r81/r132) and min/max fan is 1700/8200.
4. Register 1311 (2055) and the weekly schedule at 1223-1250.
5. Done in session 3: no MODB-I-O code register exists on the native port; see that section.
   If a lockout ever happens, check whether r84 shows a value from the MODB-I-O lockout table (161-240).
6. Build the poller: read-only, blocks of at most 50, fast poll of 62-75/80/110/84, slow poll of counters
   and settings, `lochinvar/status` availability topic, reconnect handling for the adapter wedge.
7. Safety inputs: catch r120-r126 when a switch trips (or during a lockout) to confirm or drop the
   MODB-I-O order. Rerun `tools/analysis/backtest.py` on any new capture.
8. r147 vs r146: only separable with a system supply sensor fitted.
