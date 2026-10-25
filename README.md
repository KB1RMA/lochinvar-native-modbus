# lochinvar-native-modbus

See what your Lochinvar boiler is doing, right in Home Assistant.

This project reads live data from a Lochinvar boiler and sends it to Home Assistant over MQTT. It
uses the USB port on the boiler's controller. You do not need Lochinvar's MODB-I-O add-on board.

There are two ways to run it. Both give you the same sensors:

| | What runs it | Set it up |
|---|---|---|
| **ESPHome package** | A small ESP32-S3 board plugged into the boiler | [esphome/README.md](esphome/README.md) |
| **Docker container** | Any computer with Docker next to the boiler, like a Raspberry Pi | [mqtt/README.md](mqtt/README.md) |

Each one has its own releases: tags `esphome-vX.Y.Z` for the ESPHome package, and `mqtt-vX.Y.Z` plus the
image `ghcr.io/kb1rma/lochinvar-native-modbus` for the container.

> **This is not an official Lochinvar project.** Lochinvar did not make it and does not support it.
> We figured out everything here by watching the boiler and comparing it to the boiler's own screen.
> Use it at your own risk.

## What you get

Once it is running, Home Assistant shows things like:

- **Temperatures:** water going out, water coming back, exhaust, outdoor air, and the temperature the
  boiler is aiming for during a call for heat
- **Sensor health:** how far apart the paired outlet and flue sensors read (the boiler faults when they
  disagree, so a growing gap is an early warning)
- **Burner:** firing rate, flame signal, fan speed, and whether the flame is on
- **Status:** standby, prepurge, running, post-purge, heat demand, and anti-cycle wait
- **Settings:** the set points for space heat and hot water, the high limits, and the heating set point
  after outdoor reset (while heating)
- **Counters:** hours powered on, heating run time, cycles, and ignition attempts

## It only reads. It never writes.

A boiler is a gas appliance, so safety comes first. This bridge **only asks the boiler for
numbers**. It never sends a command that could change a setting, turn something on, or turn
something off.

- Both bridges only use Modbus "read" requests (function 03).
- The ESPHome package has no switches, buttons, number boxes, or other controls.
- The Docker container builds its own Modbus requests, and the only kind it can build is a read.
- Automatic checks run on every change (`tools/check_esphome.py` and the container's tests). They fail
  if anyone adds something that could write to the boiler.

## How it works

Inside the boiler's controller there is a USB-to-serial chip (an FTDI FT232R). It speaks a common
language for equipment called **Modbus RTU**. When you ask it for a "register" (a numbered slot),
it answers with a number. For example, register 62 holds the outlet water temperature.

Lochinvar does not publish a list of what each register means for this port. So we built one by
hand. We recorded the registers for hours while the boiler heated, then matched the changes to what
the boiler's screen showed at the same time. That list lives in
[`registers/controller.yaml`](registers/controller.yaml).

The path the data takes:

```
Boiler controller  --USB cable-->  ESP32-S3 or a computer running Docker  -->  MQTT broker  -->  Home Assistant
```

The link settings are 19200 baud, 8N1, device ID 1. Both bridges set these for you.

## What you need

- A Lochinvar boiler with a SMART SYSTEM control that has a **USB "PC Connection" port** next to its
  display. See [Which boilers this works on](#which-boilers-this-works-on).
- A regular USB cable.
- Either an ESP32-S3 board (see [esphome/README.md](esphome/README.md)) or a computer that runs
  Docker (see [mqtt/README.md](mqtt/README.md)).
- Home Assistant with an MQTT broker (such as the Mosquitto add-on).

## Known problems

- **The link can drop when the burner lights.** On our boiler, the USB link sometimes goes quiet
  right at ignition. We think electrical noise from the spark causes it. Unplugging and replugging
  the USB cable brings it back. The "Boiler Link OK" sensor shows when this happens. While the link
  is down, the readings show as unavailable and Boiler Status reads "Link lost". After 3 minutes
  down, the ESP restarts itself and the container reopens the serial port. Each component's README
  has the details.
- **The screen is a little behind.** The boiler's display lags the live data by about 10 seconds.
  So, if you compare them, expect small differences while the burner is changing.
- **Some registers are still guesses.** The register map marks each one as known, a guess, or unknown.
  Guesses are not shown as sensors unless the map says so.

## Help map more registers

There is still a lot left to learn, like what the lockout codes look like. If you have a Lochinvar
boiler, you can help:

1. Plug the boiler's USB into a laptop.
2. Run the logger. It reads every register once a second (or once a minute for the slow ones) and
   saves only the changes:

   ```
   caffeinate -is python3 -I tools/probe/08_long_capture.py
   ```

3. While it runs, type what the boiler's screen says (like "prepurge" or "firing 40%") and press
   Enter. Your notes get saved with a timestamp.

Logs go into `captures/`. That folder is **not** part of the repo, so your logs stay on your computer.
To share a finding, open an issue or pull request and describe what you saw. Please don't post
whole log files.

On a Mac, keep the laptop plugged in. `caffeinate -s` only stops sleep while on power. Use
`--hours N` to stop after N hours, and `--port` to pick the USB device.

After a capture, `tools/analysis/backtest.py` replays it and checks the register map against it.

The full story of how each register was figured out is in
[`docs/reverse-engineering-log.md`](docs/reverse-engineering-log.md).

## What's in this repo

| Path | What it is |
|---|---|
| `registers/controller.yaml` | The register map. This is the main source of truth. |
| `esphome/` | The ESPHome package (`packages/boiler.yaml`) and its transports, plus a complete device config. |
| `mqtt/` | The Docker container: a read-only Modbus to MQTT bridge in Go. |
| `docs/` | Notes and the reverse-engineering log. |
| `tools/probe/` | Scripts that read the boiler from a laptop (read-only). |
| `tools/analysis/` | Scripts that check the map against saved logs. |
| `tools/check_registers.py` | Checks the register map file for mistakes. |
| `tools/check_esphome.py` | Checks the ESPHome files: no writes, every register is in the map, and the version matches. |

## For developers

```
python3 -m venv .venv && .venv/bin/pip install -r requirements.txt
.venv/bin/python tools/check_registers.py
.venv/bin/python tools/check_esphome.py
.venv/bin/esphome compile esphome/lochinvar.yaml
```

The container has its own steps in [mqtt/README.md](mqtt/README.md#developing).

GitHub Actions runs these checks on every pull request. It also builds `esphome/lochinvar.yaml`
and everything in `esphome/tests/`, runs the container's lint and tests, and builds its image.

**Releases** are made by [release-please](https://github.com/googleapis/release-please), separately
for each part. Pull requests are squash-merged. Their titles must follow
[Conventional Commits](https://www.conventionalcommits.org/), like `feat: add a sensor` or
`fix: correct the outlet temperature`. Which part a change releases depends on the files it touches:
changes under `esphome/` go into the next `esphome-vX.Y.Z` tag (what the `github://` lines point to),
and changes under `mqtt/` go into the next `mqtt-vX.Y.Z` tag and container image. Changes to shared files
(the register map, docs, tools) release neither. When one changes what a bridge does, change that
bridge too.

## Not the MODB-I-O board

Lochinvar sells an add-on board called MODB-I-O, which has its own documented register list. This
project does **not** use that board. The controller's built-in port uses different registers and
stores temperatures in a different way. If you have a MODB-I-O board, see
[cwm12345/lochinvar-modbus](https://github.com/cwm12345/lochinvar-modbus).

## Which boilers this works on

This bridge talks to the port that Lochinvar's own Windows program for the SMART SYSTEM control
(called "WinPro" or "SMART SYSTEM 938") uses. Whether your boiler works depends on which control and display
it has. The model year alone doesn't tell you, because a control can be replaced with a newer one.

| Your boiler | Will it work? |
|---|---|
| **Knight Fire Tube** (floor KHB and wall WHB, models 55-285, Series 100-101, 110-111 and 120-121) with a large screen, a turning dial, and a USB "PC Connection" port | **Tested.** Firmware "SIT 547_00 May 15 2018 938FT". The Lochinvar manuals for these models match what we see |
| Other boilers with the same large-screen SMART SYSTEM display and USB "PC Connection" port: Knight floor fire tube (KHN), Knight Wall Hung fire tube (WHN), Knight XL, Armor | **Not tested.** WinPro covers all of them, so the port is probably the same. Some registers may sit at other addresses, or be missing on models without that feature. Try it, compare with the display, and tell us |
| Older Knight boilers (about 2005-2012: KBN, KBL, Knight Wall Mount WBN, early Knight XL) with a **2-line text display** and a **phone-style RJ11 jack** for the PC | **Not supported.** Their PC port is a serial jack (Lochinvar's kit used a USB adapter cable for it), not the built-in USB port. Its protocol and registers are unknown. Don't plug a phone line into it |
| Any boiler with Lochinvar's MODB-I-O board, or the optional ModBus/BACnet interface module on the low-voltage board | **Different project.** Those boards have their own documented RS-485 registers. See [Not the MODB-I-O board](#not-the-modb-i-o-board) |
| Crest and other boilers with the SMART TOUCH touchscreen control | **Not supported.** A different control |

**How to check yours:**

1. Look at the display. You want a large screen with a dial in the middle, two soft keys, and a USB port
   labeled **PC Connection** (separate from the full-size USB port for a memory stick). A 2-line text display
   with an RJ11 jack is the older control.
2. Read the software version in the installer menu (Software Version, read only). The logger below also
   saves it, in registers 30-44. Ours ends in **938FT**: 938 is the SMART SYSTEM platform, and FT means Fire Tube.
3. When the bridge starts, the readings should match the display: outlet and inlet temperature, flame current,
   fan speed, and the History screen counters. If they don't, please open an issue.

### Tested boilers

| Boiler (from the rating plate) | Controller firmware | Bridge | Notes |
|---|---|---|---|
| Knight Fire Tube KHB155N (155,000 BTU/hr, natural gas), Series 100, built 2023 | SIT 547_00 May 15 2018 938FT | ESPHome and the container | the boiler that this map comes from |

Got it working on a different boiler? Please open an issue with the model and series from the rating plate,
the firmware string, and whether the temperatures matched the display. We'll add it here.
