# Cross-reference with the Lochinvar manuals

Desk work only (2026-10-25). No new reads were sent to the boiler. The boiler is a Knight Fire Tube KHB155N,
Series 100, built 2023, so these manuals are the right ones for it. The map was compared against three Lochinvar
documents. None of them are in this repo; get them from Lochinvar.

| Document | What it covers |
|---|---|
| Knight Fire Tube Service Manual, 100305279 Rev E (03/23) | Models 55-285, Series 100-101, 110-111 and 120-121: SMART SYSTEM control, parameters, protections, faults, sensor tables |
| Knight Fire Tube Installation & Operation Manual, 100305277 Rev M | The same models: wiring, sequence of operation, display screens |
| SMART SYSTEM PC Program Instructions, SM-PCPII Rev D (05/16) | Lochinvar's Windows "WinPro" program for Knight, Knight XL, Knight Wall Mount, Knight Wall Hung and Armor. Its screenshots list every parameter in order with short names, defaults and ranges |

The third one matters most. The USB port we use is the display's "PC Connection" port. The I&O manual says "A
serial and LOCH-N-LINK USB Programming support additional communication with the control". WinPro talks
to the control over that port, and the PC manual calls the program "SMART SYSTEM 938". Our firmware string ends in
"938FT". So the registers we read are the data WinPro shows, and its screens are a good guide to what is in them.

## The parameter block is WinPro's parameter table, in order

WinPro's "Table 4 - Lochinvar" screenshot (factory, non-adjustable) lists parameters with defaults from another
model. Our 1068-1121 follow the same order. Where both are temperatures or rpm, the values line up one for one:

| Reg | Our value | WinPro name | Meaning (from the screenshot and the service manual) |
|---|---|---|---|
| 1070 | 10 F | S1AUTOLIMHYS | auto-reset high limit hysteresis (service manual: restarts 10 F below set point) |
| 1071 | 200 F | S1AUTOHILIMDWH | auto-reset high limit, direct water heater |
| 1072 / 1073 | 195 F / 10 F | CS1BLO / CS1HYS | outlet shutdown and its hysteresis (service manual: shuts down above 195 F) |
| 1074 / 1075 | 195 F / 185 F | FANMINS1C / FANMAXS1C | outlet fan limiting: starts at 185 F, min fan at 195 F |
| 1076 / 1077 | 225 F / 25 F | CS3BLO / CS3HYS | flue shutdown 225 F, restart after a 25 F drop |
| 1078 / 1079 | 225 F / 215 F | FANMINS3C / FANMAXS3C | flue fan limiting from 215 F |
| 1080 / 1081 | 60 F / 10 F | DS1S2BLO / DS1S2HYS | delta T (outlet - inlet) shutdown at 60 F |
| 1082 / 1083 | 60 F / 55 F | FANMINS12C / FANMAXS12C | delta T fan limiting from 55 F |
| 1088-1092 | 18, 158, 4, 9, 1 F | FLOGRAX/BX/AY/BY/HYS | outlet gradient limiting curve |
| 1093 / 1094 | 15 s / 4 min | FLUGRTSTEP / FLUGRT | flue gradient sampling |
| 1097 / 1098 | 1700 / 8200 rpm | MINFAN / MAXFAN | already mapped |
| 1099 / 1100 | 250 / 250 | INCFAN / DECFAN | max fan speed change per step |
| 1101 / 1102 | 50 / 50 | INCFAN_R / DECFAN_R | the same while running. This is why r111 reads 50 while running and 250 otherwise |
| 1103-1105 | 70, 25, 5 | FANKP / FANKI / FANKD | fan speed PID |
| 1106-1108 | 3500, 2500, 500 | MINION / FANINCION / FANDELTA | flame current support (raise min fan when the flame signal is weak) |
| 1109 / 1110 | 2900 / 2900 rpm | FANHIGH / FANIGN | purge and ignition fan speeds (was "post_purge_fan_speed") |
| 1111-1113 | 2 s, 2 min, 10 s | TPCIRCSWITCH / TPCIRCERR / TMINPCIRCHW | pump post-circulation timers |
| 1115-1117 | 350, 10, 0 | HWKP / HWKI / HWKD | DHW temperature PID |

Two encodings show up that the map did not have yet:

- **Temperature differences** (hysteresis, offsets, differentials) are raw / 25 in F with no flag bit: 250 = 10 F,
  625 = 25 F, 1500 = 60 F, 1375 = 55 F. Absolute temperatures still use `temp_f`.
- **Times** that WinPro shows as min'sec" are in 0.1 s. Its maximum of 54'37" is 32767 tenths. 600 = 60 s,
  900 = 90 s, 2400 = 4 min.

The user tables follow in other places in the block. These matched the manual's defaults, the display, or the
live registers:

| Reg | Value | Name | Evidence |
|---|---|---|---|
| 1144 | 5309 | installer password | the service manual prints 5309. Do not publish it |
| 1153-1156 | 13, 8760, 5000 | service notification months, run hours, cycles (x10?) | manual defaults 13 months, 8760 h, 50,000 cycles |
| 1163-1168 | 35 F, 180 F, 70 F, 130 F, 100 F, 10 F | SH1 outdoor reset: cold outdoor temp, set point there, warm outdoor temp, set point there, outdoor shutdown, shutdown differential | the curve predicts r142: 167.7 F at 43.6 F outdoor (logged 167.7) and 161.4 F at 48 F (logged 161.2) |
| 1169-1174 | 25, 180, 70, 70, 70, 10 | the same for SH2 | all factory defaults |
| 1175-1180 | 25, 180, 70, 70, 70, 10 | the same for SH3 | all factory defaults |
| 1187-1190 | 60, 140, 60, 185 F | tank min / tank max / SH min / SH max set point | manual defaults 60, 140, 60, 185 |
| 1191-1196 | 10, 20, 10, 20, 10, 20 F | SH1-SH3 offset and differential | manual defaults 10 and 20 |
| 1201 / 1202 | 10 / 20 F | DHW boiler offset / differential | manual defaults |
| 1221 / 1222 | 0 / 0 | SH and DHW night setback offset | 0 means night setback is off |
| 1223-1250 | minutes of the week | night setback triggers: SH on 1-7, SH off 1-7, DHW on 1-7, DHW off 1-7 | WinPro lists 28 triggers in this order. Ours are on daily at 20:00 and off daily at 06:00, and do nothing while the offset is 0 |
| 1253-1255 | 45 F, 37 F, 5 F | freeze protection: pumps on, burner on, burner differential | manual defaults 45, 37, 5 |
| 1256 | 60 s | boiler pump delay | r132 stays on about 60 s after the flame goes out |
| 1257 / 1258 | 60 s / 30 s | DHW pump delay / system pump delay | manual defaults 1 min and 30 s |
| 1259-1261 | 20 s x3 | pump anti-seize run times | manual default 20 s |
| 1263-1269 | 3.00, 2.00, 10.00, 2.00, 1.00 V, 20 %, 100 % | boiler pump min volts, BMS volts at min/max, on volts, off differential, rate at min/max volts | manual defaults (volts in 0.01 V, rates in 0.1 %) |
| 1270 / 1271 | 70 / 180 F | BMS set point at min / max volts | manual defaults |

The map stores the ones with more than one source as `likely` and the ones with only order evidence as
`guess`. Nothing here is published by the bridges yet. If someone wants them, the outdoor reset curve, the freeze
settings and the pump delay are the useful ones.

## Live registers the PC status screen explains

WinPro's status screen shows what the control exports live:

- **dSP set point.** In the screenshot dSP reads 32 F with no call for heat. That is our r146: exactly 32.0 F
  whenever r118 = 0. dSP is "the highest controlling set point for space heat that is active", or the
  outlet set point while heating an indirect tank.
- **Derivative Outlet and Derivative Flue.** These are rates of change. r150 (signed, about +/-7, steps of about 6.5)
  is a good candidate for one of them.
- **Dual sensors.** WinPro shows Outlet 1, Outlet 2, Flue 1, Flue 2 (check / limit). The service manual
  calls them S1a/S1b and S3a/S3b, two thermistors in one housing. That confirms r62/r64 and r63/r65. The
  "Outlet Temp Diff" and "Flue sens diff" faults trip when the two disagree.
- **Inputs:** SH1, SH2, SH3 Tstat, HW Tstat, 0-10 V system pump, Louver, Flow Sw/LWCO, Gas Press, Air Switch,
  Drain Switch (closed when OK). Five switches closed is what r121-r125 = 1 shows. If they follow the WinPro order,
  r121-r125 are louver, flow, gas pressure, air, drain. That is not the MODB-I-O order the map uses now.
- **Outputs:** boiler pump, system pump, DHW pump, louver relay, gas valve, 0-10 V rate, 0-10 V boiler pump.
- **Limitations** flags: outlet temp limit, delta T limit, flue temp limit, flame current low. These were never
  active in our captures. They are the four protections in the service manual (outlet 185/195 F, flue 215/225 F,
  delta T 55/60 F, flame current support).
- **Fan speeds:** ignition, maximum, minimum, target and actual. All of these are now mapped.

The display adds DHW RECIRC TEMP (fits r77 = 0x8000, no sensor) and LOOP 1-3 TEMP from the optional MTLC
board (not found yet).

## History and fault log

The display's History screen has SH run time, SH cycles, DHW run time, DHW cycles, power time, ignitions and
ignition attempts. WinPro's fault log has power hours, SH hours, **SH hours <50 %**, DHW hours, successful and
failed ignitions. It also keeps per-fault counts (current and total) for flame out sequence, air switch, fan
speed, MRHL, ARHL, ignition failure, flame failure, outlet sensor error, outlet diff, inlet sensor error, flue
sensor error, flue sensor diff and flue high limit. On top of those it holds the last 10 lockouts, the control's
serial number and its production date.

- **r1311 = 2055 is probably SH hours below 50 % rate.** It is 54 % of the 3828 SH hours. It can't be DHW run time,
  which the display showed as 0.
- **1316-1332 (all 0) and 1335-1365 (all 0xFFFF)** look like the fault counters and an empty last-10-lockouts
  table. This boiler has had no lockouts that we know of. A lockout should fill them; capture 1256-1405
  before and after one.

## Status codes

The display's status bar has STANDBY, START, PREPURGE, IGNITION, firing %, POSTPURGE, SHUTDOWN, SETPOINT MET and
BLOCKED. We have seen r84 = 4, 6, 7, 9 and 10. The gaps fit the display's list: 5 is probably START (the safety
check before prepurge, too short to catch so far), and 8 may be the step between ignition and modulation (the
manual says the rate is held for a few seconds to let the flame settle). SHUTDOWN (the boiler turned off from the
display) and lockout have not been seen. None of this is confirmed.

Behaviour from the sequence of operation that matches the logs:

- The control tries to light **4 times** before it locks out with "Flame Fail Ign". The failed-ignition counter
  is the one to watch.
- **Anti-cycle** is 60 s by default (1 min to 10 min), and is cut short when the inlet drops 10 F
  below where it was at shutdown. That explains the shorter anti-cycle periods.
- **Too Many Recyclings:** more than 5 limit switch openings in one call for heat is a fault.
- Freeze protection runs the pumps below 45 F inlet and fires below 37 F.

## The controller clock

The manual says the clock must be set at install and after the power has been off for more than 4 hours, and
that it does not adjust for daylight saving time. WinPro sets it from the PC when it connects. That explains
why 172-176 runs but is about 54 minutes slow with the wrong date. Setting it from our side would be a write, so
we don't.

## Ideas for Home Assistant (not built)

- Alert when delta T passes 50 F or the flue passes 210 F, before the control starts limiting at 55 F and 215 F.
- Alert on a rising failed-ignition count. The fourth failure in one call is a lockout.
- Read the outdoor reset curve and show the calculated set point next to r142.
