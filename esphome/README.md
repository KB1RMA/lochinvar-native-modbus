# ESPHome package

Run the bridge on a small ESP32-S3 board plugged into the boiler's USB port. It sends the readings to
Home Assistant over MQTT (or ESPHome's own API). See the [main README](../README.md) for what you get
and how the project works. If you would rather run a Docker container on a computer next to the
boiler, see [`mqtt/`](../mqtt/README.md).

```
Boiler controller  --USB cable-->  ESP32-S3  --Wi-Fi-->  MQTT broker  -->  Home Assistant
```

## What you need

- A Lochinvar boiler with a USB port on its controller (see "Tested hardware" in the main README).
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

## Setting it up

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
  transport: github://KB1RMA/lochinvar-native-modbus/esphome/packages/usb-ft232.yaml@esphome-v0.1.0 # x-release-please-version
  boiler: github://KB1RMA/lochinvar-native-modbus/esphome/packages/boiler.yaml@esphome-v0.1.0 # x-release-please-version
```

The `@esphome-v...` at the end pins a release, so your boiler setup won't change by surprise. The
package's releases and changes are listed in [CHANGELOG.md](CHANGELOG.md) once the first one is out.

There are two parts:

- `boiler.yaml` sets up Modbus and all the sensors.
- A **transport** picks how the ESP32 reaches the boiler:
  - `usb-ft232.yaml`: the ESP32-S3 talks to the boiler's USB port. **This is the tested way.**
  - `uart.yaml`: uses the ESP32's own serial pins. This has not been tested. It only helps if you
    reach the boiler's serial line some other way.

Want to build from a copy of this repo instead? Use [`lochinvar.yaml`](lochinvar.yaml).
Copy `secrets.example.yaml` to `secrets.yaml` and fill in your Wi-Fi and MQTT details.

## When the link drops

While the boiler does not answer, the readings show as unavailable, "Boiler Link OK" turns off, and
Boiler Status reads "Link lost". After 3 minutes down, the ESP restarts itself, and "Link Watchdog
Restarts" counts how often that happened.

## Files

| Path | What it is |
|---|---|
| `packages/boiler.yaml` | The package: Modbus setup and every entity. Read-only. |
| `packages/usb-ft232.yaml` | Transport: ESP32-S3 as USB host for the boiler's FT232R. |
| `packages/uart.yaml` | Transport: the ESP32's own UART pins (untested). |
| `lochinvar.yaml` | A complete device config built from the packages. |
| `tests/` | Extra configs that the automatic checks build. |
