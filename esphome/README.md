# ESPHome package

Run the bridge on a small ESP32-S3 board plugged into the boiler's **PC Connection** port. The board
sends the readings to Home Assistant over Wi-Fi, using the ESPHome API or MQTT. This is the easiest
way if you run Home Assistant OS.

For what you get and which boilers work, see the [main README](../README.md). If you'd rather use a
computer you already have by the boiler, see the [Docker container](../mqtt/README.md).

```
Boiler's PC Connection port  --USB cable-->  ESP32-S3  --Wi-Fi-->  Home Assistant
```

## What you need

- A Lochinvar boiler with the SMART SYSTEM display and a USB **PC Connection** port. See
  [Will it work on my boiler?](../README.md#will-it-work-on-my-boiler)
- An **ESP32-S3** board whose native USB port can act as a USB host and supply 5 V. We used an
  ESP32-S3-DevKitC-1 clone. Its two USB ports are labeled **USB** and **COM**.
- A USB cable from the board's USB port to the boiler. The boiler end is USB-B (the square printer
  kind). The board end is usually USB-C or micro-USB.
- A second USB cable and power supply for the board.
- A soldering iron and a little solder.
- A recent ESPHome. Our automatic checks build with ESPHome 2026.10.0. Device Builder (the ESPHome
  add-on or dashboard) is the easiest way to use it.

## Wiring it up

1. Bridge the **USB-OTG** solder pad on the board. Put a small blob of solder across its two halves.
2. Plug the boiler's USB cable into the board's **USB** port (sometimes labeled "OTG").
3. Power the board through its other port (labeled **COM** or **UART**).

> [!IMPORTANT]
> Don't skip the solder pad. The boiler's USB chip gets its power from the ESP32's USB port, and out of
> the box that port sends no power. Powering the board through COM does not fix this. Without the pad,
> the boiler never shows up and the log says "Channel not initialised".

You don't need to set the baud rate or device ID. The package does that (19200 baud, 8N1, ID 1).

The boiler's display keeps working as normal. You can't use Lochinvar's WinPro software on the port at
the same time, because the ESP is using it.

## Setting it up

The bridge is an [ESPHome package](https://esphome.io/components/packages.html). Your config holds
your board, Wi-Fi and Home Assistant settings. Two lines then pull in the boiler part from GitHub. You
never copy the sensors into your own file.

### Quick start with ESPHome Device Builder

1. Wire the board as above.
2. In Device Builder, click **New device**. Name it something like `boiler` and pick **ESP32-S3**.
   Device Builder fills in the board, Wi-Fi, API and OTA parts for you.
3. Click **Edit** and add these lines at the end of the file:

   ```yaml
   packages:
     transport: github://KB1RMA/lochinvar-native-modbus/esphome/packages/usb-ft232.yaml@esphome-v0.1.1 # x-release-please-version
     boiler: github://KB1RMA/lochinvar-native-modbus/esphome/packages/boiler.yaml@esphome-v0.1.1 # x-release-please-version
   ```

4. Make sure the `esp32:` block uses the ESP-IDF framework (`type: esp-idf`). The USB host needs it.
5. Click **Install**. The first time, plug the board's **COM** port into your computer and choose
   "Plug into this computer". After that, updates go over Wi-Fi.
6. In Home Assistant, go to **Settings > Devices & services**. The new device shows up there.
   Click **Configure** to add it. If it asks for a key, copy the `key:` line from the device's
   config.

Within a few seconds you should see Boiler Link OK turn on and the temperatures fill in. Check them
against the boiler's display.

### Full example (Home Assistant API)

This is a complete config. Everything below `packages:` comes from this project.

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
  transport: github://KB1RMA/lochinvar-native-modbus/esphome/packages/usb-ft232.yaml@esphome-v0.1.1 # x-release-please-version
  boiler: github://KB1RMA/lochinvar-native-modbus/esphome/packages/boiler.yaml@esphome-v0.1.1 # x-release-please-version
```

Entities are named after your `friendly_name`, like `sensor.boiler_outlet_temperature` and
`binary_sensor.boiler_heat_demand`. The [main README](../README.md#what-you-get) lists them all.

### Using MQTT instead of the API

Swap the `api:` block for `mqtt:`. Home Assistant then finds the sensors by itself, using MQTT
discovery (on by default).

```yaml
mqtt:
  broker: !secret mqtt_broker
  username: !secret mqtt_username
  password: !secret mqtt_password
```

### Pinning and updating

The `@esphome-v...` at the end of each line locks you to one release, so your setup won't change by
surprise. To update, change the version in **both** lines and click **Install**. See
[CHANGELOG.md](CHANGELOG.md) for what changed.

### Changing the package

You can override anything in the package from your own config, the usual ESPHome way. For example,
add filters to a sensor by its `id` (like `outlet_temp`), or change `update_interval`. Look in
[`packages/boiler.yaml`](packages/boiler.yaml) for the ids. Please don't add anything that writes to
the boiler.

### Choosing a transport

The `transport` line picks how the ESP32 reaches the boiler:

- `usb-ft232.yaml`: the ESP32-S3 talks to the boiler's USB port. **This is the tested way.**
- `uart.yaml`: uses the ESP32's own serial pins. This has not been tested. It only helps if you reach
  the boiler's serial line some other way. To pick the pins, set `uart_tx_pin` and `uart_rx_pin`
  under `substitutions:`. They default to GPIO17 and GPIO16.

### Building from a copy of this repo

Use [`lochinvar.yaml`](lochinvar.yaml). It loads the packages from your copy instead of GitHub. Copy
`secrets.example.yaml` to `secrets.yaml` and fill in your Wi-Fi and MQTT details.

## Troubleshooting

| What you see | What to check |
|---|---|
| Log says "Channel not initialised" | The USB-OTG pad isn't bridged, or the cable is in the COM port |
| Boiler Link OK stays off | Is the cable in **PC Connection**, not USB Memory Drive? Try another cable. |
| Boiler Link OK turns off later | The link dropped. See below. |
| Temperatures look a few degrees off | The display lags about 10 s. Compare while the burner is steady. |
| Boiler Status reads "Unknown (n)" | A status we haven't seen yet, maybe a lockout. Please open an issue with the number and what the display says. |

## When the link drops

Sometimes the boiler stops answering. When it does:

- the readings show as unavailable,
- Boiler Link OK turns off,
- Boiler Status reads "Link lost".

If the link stays down for 3 minutes, the ESP restarts itself. "Link Watchdog Restarts" counts how
often that happened. If restarts don't bring it back, unplug the boiler's USB cable and plug it back
in.

## Files

| Path | What it is |
|---|---|
| `packages/boiler.yaml` | The package: Modbus setup and every entity. Read-only. |
| `packages/usb-ft232.yaml` | Transport: the ESP32-S3 as USB host for the boiler's FT232R chip |
| `packages/uart.yaml` | Transport: the ESP32's own UART pins (untested) |
| `lochinvar.yaml` | A complete device config built from the packages |
| `tests/` | Extra configs that the automatic checks build |
