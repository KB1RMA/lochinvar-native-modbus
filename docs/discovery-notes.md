# Lochinvar boiler Modbus map (reverse-engineered, read-only)

Link: Modbus RTU, 19200 baud, 8N1, slave ID 1, function 03 (holding registers). Function 04 returns exception 1.

## Temperatures
raw has bit15 set as a flag (0x8000 alone = no sensor). F = ((raw & 0x7FFF) - 9451) / 25.5  (fit, about +/-2 F)

| Reg | Meaning | Confidence |
|---|---|---|
| 62, 64 (mirror 154) | Outlet | high (171 F read 170.8) |
| 63, 65 (mirror 156) | Exhaust (revised: falls fast as firing rate drops, 25 F dT vs display 5) | medium-high |
| 73 (mirror 155) | Inlet (revised: slow, rising; outlet-73 = 4 F vs display dT 5) | medium-high |
| 75 (mirror 158) | Outdoor | medium-high (43 F read ~43-46) |
| 74 | Unknown sensor, ~83 F | low |
| 142, 146, 147 | ~166 F, unknown (limit/target?) | low |
| 143, 144, 145 | ~124 / ~120 F, constant, likely setpoints | low |

## Other
| Reg | Meaning | Confidence |
|---|---|---|
| 110 | Fan speed RPM (read 7129 when display said 7129) | high |
| 89, 106, 107, 149 | ~Fan speed target/copy | medium |
| 80, 131 | Firing rate, hundredths of % (8516 when display said 85%) | medium |
| 81, 132 | ~100.00% maximum | low |
| 30-44 | ASCII (byte-swapped pairs): firmware string "SIT 547_00 May 15 2018 ..." | high |
| 172-176 | Looks like a date/time (20,5,18,h,m) | low |
| 66 | Flame signal in nA (12688 when display said 12.7 uA; 12559 when 12.6 uA) | high |
| 151, 150 | Fluctuating, unknown | unknown |

## Burner shutdown sequence (from flame_log.csv, 09:58:57-10:08:55)
Burner turned off ~10:02:17-10:02:30. Rate (r80/r131) and flame (r66) went to 0 and fan (r110) ramped down, with:
| Time | Change |
|---|---|
| 10:02:17 | r118 1->0, r136 8->0, r138 4->8, r139 16->0, r140 96->112 |
| 10:02:18 | r111 50->250, r115/r127/r130 1->0, r137 0->1, r95 pulse 64 |
| 10:02:27 | r116 1->0 |
| 10:02:30 | r84 10->4, r87 ->1440, r90/r105/r128 1->0 |
| 10:03:18-19 | r132, r133 -> 0, r137 1->0 |
Likely status/state flags: r84 (status code: 9/10 run, 4 idle), r115-r118/r127-r130 (output relays: pump/gas valve/blower/ignition?), r136-r140 (state bits). Meanings unconfirmed.

## Lifetime counters (found 10:15; confirmed against display)
32-bit values, high word at the even address (all high words 0 here), so read the low word at the odd address.
| Reg | Meaning | Value | Display |
|---|---|---|---|
| 1303 | Power-on hours | 24266 | 24266 |
| 1305 | Ignition attempts | 35036 | 35035 (one more attempt since) |
| 1307 | Ignitions (successful) | 34928 | 34928 |
| 1309 | SH (space heat) run time | 3828 | 3828 |
| 1311 | Unknown | 2055 | not displayed (maybe SH run time in other unit / another counter) |
| 1313 | SH cycles | 34920 | 34920 |
| 1315 | DHW cycles (probably) | 0 | 0 |

## Other regions
- 500-502: 6, 11, 2
- 1006-1100+: dense block of values, looks like configuration/parameters (many 0xFFFF separators, some temperatures in the 40000-48000 range)
- Exception 2 (illegal address) on the 50-register chunks starting 956, 1406, 1456 ... 1906; the rest of 256-2047 reads fine.
- Raw dump: regs256_2047.json

## Setpoints and refined temperature scale (from display: SH1 175, SH2 125, SH3 125, DHW 120)
| Reg | Raw | Meaning |
|---|---|---|
| 1286 | 45236 | DHW setpoint 120 F |
| 1287 | 46611 | SH1 setpoint 175 F |
| 1288 | 45361 | SH2 setpoint 125 F |
| 1289 | 45361 | SH3 setpoint 125 F |
| 1291 | 47236 | probably a max/limit, 200 F |
| 1292, 1293 | 47486 | probably a max/limit, 210 F |
| 143, 144 / 145 (live) | 45361 / 45236 | live copies of SH2,SH3 / DHW setpoints |

Parameter scale that fits all four exactly: F = ((raw & 0x7FFF) - 9468) / 25  (0.04 F per count, bit15 = flag).
The same formula gives outdoor (r75) = 43.0 F exactly. For the live outlet/inlet/exhaust sensors the earlier fit (9451, 25.5/F)
agreed within ~3 F; the 9468/25 formula reads them up to ~3 F higher, so treat sensor accuracy as +/-3 F until compared further.

## High limits (confirmed from display)
| Reg | Raw | Meaning |
|---|---|---|
| 1291 | 47236 | High limit, auto reset, 200 F (scale gives exactly 200.0) |
| 1292, 1293 | 47486 | High limit, manual reset, 210 F (scale gives exactly 210.0; two copies, one probably SH vs DHW or primary vs backup) |
