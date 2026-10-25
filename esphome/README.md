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

The bridge is an [ESPHome](https://esphome.io) package. You keep a short config for your own board
(name, Wi-Fi, how it talks to Home Assistant) and add two lines that pull the boiler part from
GitHub. You never copy the sensors into your own file.

### Quick start with ESPHome Device Builder

1. Wire the board as described above.
2. In ESPHome Device Builder (the ESPHome add-on or dashboard), click **New device**, give it a
   name such as `boiler`, and pick **ESP32-S3**. Device Builder writes the board, Wi-Fi, API and OTA
   parts for you.
3. Click **Edit** and add these lines at the end of the file:

   ```yaml
   packages:
     transport: github://KB1RMA/lochinvar-native-modbus/esphome/packages/usb-ft232.yaml@esphome-v0.1.0 # x-release-please-version
     boiler: github://KB1RMA/lochinvar-native-modbus/esphome/packages/boiler.yaml@esphome-v0.1.0 # x-release-please-version
   ```

4. Make sure the `esp32:` block uses the ESP-IDF framework (`type: esp-idf`). The USB host needs it.
5. Click **Install**. The first time, plug the board's **COM** port into your computer and choose
   "Plug into this computer". After that, updates go over Wi-Fi.
6. Home Assistant finds the new device under **Settings > Devices & services**. Click **Configure**
   to add it. If it asks for an encryption key, copy the `key:` from the device's config.

### Full example (Home Assistant API)

This is the whole config. Everything below `packages:` comes from this project.

```yaml
esphome:
  name: boiler
  friendly_name: Boiler

esp32:
  variant: esp32s3
  framework:
    type: esp-idf          # required for the USB host

logger:

api:
  encryption:
    key: !secret api_key

ota:
  - platform: esphome

wifi:
  ssid: !secret wifi_ssid
  password: !secret wifi_password

packages:
  transport: github://KB1RMA/lochinvar-native-modbus/esphome/packages/usb-ft232.yaml@esphome-v0.1.0 # x-release-please-version
  boiler: github://KB1RMA/lochinvar-native-modbus/esphome/packages/boiler.yaml@esphome-v0.1.0 # x-release-please-version
```

Entities show up in Home Assistant as `sensor.boiler_outlet_temperature`,
`binary_sensor.boiler_heat_demand` and so on, named after your `friendly_name`.

### Using MQTT instead of the API

Swap the `api:` block for `mqtt:`. Home Assistant picks the entities up through MQTT discovery,
which is on by default.

```yaml
mqtt:
  broker: !secret mqtt_broker
  username: !secret mqtt_username
  password: !secret mqtt_password
```

### Pinning and updating

The `@esphome-v...` at the end of each line pins a release, so your boiler setup won't change by
surprise. To update, change the version in **both** lines to the new release and click **Install**.
Releases and their changes are listed in [CHANGELOG.md](CHANGELOG.md).

### Choosing a transport

The `transport` line picks how the ESP32 reaches the boiler:

- `usb-ft232.yaml`: the ESP32-S3 talks to the boiler's USB port. **This is the tested way.**
- `uart.yaml`: uses the ESP32's own serial pins. This has not been tested. It only helps if you
  reach the boiler's serial line some other way. Set the pins with `substitutions:`
  (`uart_tx_pin`, `uart_rx_pin`; the defaults are GPIO17 and GPIO16).

`boiler.yaml` sets up Modbus and all the sensors, and works with either transport.

### Building from a copy of this repo

Use [`lochinvar.yaml`](lochinvar.yaml), which includes the packages from the local folder. Copy
`secrets.example.yaml` to `secrets.yaml` and fill in your Wi-Fi and MQTT details.

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
