# lochinvar-serial-mqtt

See what your Lochinvar boiler is doing, right in Home Assistant.

This project reads live data from a Lochinvar boiler and sends it to Home Assistant over MQTT. It
uses a small ESP32 board plugged into the USB port on the boiler's controller. You do not need
Lochinvar's MODB-I-O add-on board.

> **This is not an official Lochinvar project.** Lochinvar did not make it and does not support it.
> We figured out everything here by watching the boiler and comparing it to the boiler's own screen.
> Use it at your own risk.

## What you get

Once it is running, Home Assistant shows things like:

- **Temperatures:** water going out, water coming back, exhaust, and outdoor air
- **Burner:** firing rate, flame signal, fan speed, and whether the flame is on
- **Status:** standby, prepurge, running, post-purge, heat demand, and anti-cycle wait
- **Settings:** the set points for space heat and hot water, and the high limits
- **Counters:** hours powered on, heating run time, cycles, and ignition attempts

## It only reads. It never writes.

A boiler is a gas appliance, so safety comes first. This bridge **only asks the boiler for
numbers**. It never sends a command that could change a setting, turn something on, or turn
something off.

- The bridge only uses Modbus "read" requests (function 03).
- The ESPHome package has no switches, buttons, number boxes, or other controls.
- An automatic check (`tools/check_esphome.py`) runs on every change. It fails if anyone adds
  something that could write to the boiler.

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
Boiler controller  --USB cable-->  ESP32-S3  --Wi-Fi-->  MQTT broker  -->  Home Assistant
```

## What you need

- A Lochinvar boiler with a USB port on its controller. We tested firmware
  "SIT 547_00 May 15 2018". Other models may work, but we have not tried them yet.
- An **ESP32-S3** board with a USB port that can act as a host. We used an ESP32-S3-DevKitC-1 clone.
- A regular USB cable to connect the boiler to the ESP32-S3.
- Home Assistant with an MQTT broker (such as the Mosquitto add-on). ESPHome's own API also works.

### Wiring it up

1. Plug the boiler's USB cable into the ESP32-S3's **USB** port (sometimes labeled "OTG").
2. Power the board through its other port (labeled **COM** or **UART**).
3. Bridge the **USB-OTG** solder pad on the board. This lets the USB port send 5 volts to the
   boiler's chip. If you skip this, the boiler never shows up, and the log says
   "Channel not initialised".

The link settings are 19200 baud, 8N1, device ID 1. The package sets these for you.

## Setting it up with ESPHome

The bridge is an [ESPHome](https://esphome.io) package. You write a short config for your own
board, and it pulls in this project from GitHub. Here is a full example:

```yaml
esphome:
  name: lochinvar

esp32:
  board: esp32-s3-devkitc-1
  framework:
    type: esp-idf

wifi:
  ssid: !secret wifi_ssid
  password: !secret wifi_password

mqtt:
  broker: !secret mqtt_broker

packages:
  transport: github://KB1RMA/lochinvar-serial-mqtt/esphome/packages/usb-ft232.yaml@v0.0.0 # x-release-please-version
  boiler: github://KB1RMA/lochinvar-serial-mqtt/esphome/packages/boiler.yaml@v0.0.0 # x-release-please-version
```

The `@v...` at the end pins a release, so your boiler setup won't change by surprise.

There are two parts:

- `boiler.yaml` sets up Modbus and all the sensors.
- A **transport** picks how the ESP32 reaches the boiler:
  - `usb-ft232.yaml`: the ESP32-S3 talks to the boiler's USB port. **This is the tested way.**
  - `uart.yaml`: uses the ESP32's own serial pins. This has not been tested. It only helps if you
    reach the boiler's serial line some other way.

Want to build from a copy of this repo instead? Use [`esphome/lochinvar.yaml`](esphome/lochinvar.yaml).
Copy `esphome/secrets.example.yaml` to `esphome/secrets.yaml` and fill in your Wi-Fi and MQTT details.

## Known problems

- **The link can drop when the burner lights.** On our boiler, the USB link sometimes goes quiet
  right at ignition. We think electrical noise from the spark causes it. Unplugging and replugging
  the USB cable brings it back. The "Boiler Link OK" sensor shows when this happens. While the link
  is down, the readings show as unavailable and Boiler Status reads "Link lost".
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
| `esphome/packages/` | The ESPHome package (`boiler.yaml`) and its transports. |
| `esphome/lochinvar.yaml` | A complete device config built from the packages. |
| `esphome/tests/` | Extra configs that the automatic checks build. |
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

GitHub Actions runs these checks on every pull request. It also builds `esphome/lochinvar.yaml`
and everything in `esphome/tests/`.

**Releases** are made by [release-please](https://github.com/googleapis/release-please).
Pull requests are squash-merged. Their titles must follow
[Conventional Commits](https://www.conventionalcommits.org/), like `feat: add a sensor` or
`fix: correct the outlet temperature`. When a release is merged, it creates a `vX.Y.Z` tag. That
tag is what the `github://` lines above point to.

## Not the MODB-I-O board

Lochinvar sells an add-on board called MODB-I-O, which has its own documented register list. This
project does **not** use that board. The controller's built-in port uses different registers and
stores temperatures in a different way. If you have a MODB-I-O board, see
[cwm12345/lochinvar-modbus](https://github.com/cwm12345/lochinvar-modbus).

## Tested hardware

| Controller firmware | Boiler model |
|---|---|
| SIT 547_00 May 15 2018 | TODO |

Got it working on a different boiler? Please open an issue so we can add it to this table.
