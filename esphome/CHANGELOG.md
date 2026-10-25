# Changelog

## 0.1.0 (2026-10-25)


### Features

* add Delta T sensor and drop zero temperature readings ([33313db](https://github.com/KB1RMA/lochinvar-native-modbus/commit/33313db257d121c9ad7c5543b2fbfb15a6759c68))
* ESP32-S3 USB host bridge as ESPHome packages ([9d1b504](https://github.com/KB1RMA/lochinvar-native-modbus/commit/9d1b5041456610b8726eba1e4be00b0d97c49997))
* **esphome:** release the package on its own, pulled from github://KB1RMA/lochinvar-native-modbus/esphome/packages/boiler.yaml@esphome-vX.Y.Z ([e155fad](https://github.com/KB1RMA/lochinvar-native-modbus/commit/e155fadc69eb882d8d818d8e22c8b2cc4c02b949))
* **mqtt:** add a read-only Modbus RTU to MQTT bridge with Home Assistant discovery, published as ghcr.io/kb1rma/lochinvar-native-modbus for amd64, arm64 and arm/v7 ([e155fad](https://github.com/KB1RMA/lochinvar-native-modbus/commit/e155fadc69eb882d8d818d8e22c8b2cc4c02b949))
* **mqtt:** read settings from flags, environment variables, _FILE variables (Docker secrets) or a KEY=VALUE config file, with TLS and a private CA through MQTT_CA_FILE ([e155fad](https://github.com/KB1RMA/lochinvar-native-modbus/commit/e155fadc69eb882d8d818d8e22c8b2cc4c02b949))
* **mqtt:** reopen the serial port after 3 minutes of silence, backing off to 30 minutes, counted by Link Reconnects ([e155fad](https://github.com/KB1RMA/lochinvar-native-modbus/commit/e155fadc69eb882d8d818d8e22c8b2cc4c02b949))
* show status code 7 as Ignition ([1afa557](https://github.com/KB1RMA/lochinvar-native-modbus/commit/1afa5578575e4671ec65ae4f5a974f1d833cd816))


### Bug Fixes

* clear readings when the boiler link drops ([c169d22](https://github.com/KB1RMA/lochinvar-native-modbus/commit/c169d223739361708c71e246d667535be99b9390))
* **esphome:** keep Failed Ignitions from counting reads of the ignition counters taken during prepurge or ignition ([e155fad](https://github.com/KB1RMA/lochinvar-native-modbus/commit/e155fadc69eb882d8d818d8e22c8b2cc4c02b949))
* **esphome:** restart the ESP after 3 minutes of a dead boiler link, counted by a new Link Watchdog Restarts sensor ([e155fad](https://github.com/KB1RMA/lochinvar-native-modbus/commit/e155fadc69eb882d8d818d8e22c8b2cc4c02b949))
* show Heat Demand and Flame Present as On/Off ([bdaf418](https://github.com/KB1RMA/lochinvar-native-modbus/commit/bdaf41842860d770962baa107c6b104329b6ab11))
