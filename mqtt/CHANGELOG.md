# Changelog

## [0.1.1](https://github.com/KB1RMA/lochinvar-native-modbus/compare/mqtt-v0.1.0...mqtt-v0.1.1) (2026-10-25)


### Features

* publish the target temperature, SH1 reset target and dual-sensor differences ([#9](https://github.com/KB1RMA/lochinvar-native-modbus/issues/9)) ([25b4ebf](https://github.com/KB1RMA/lochinvar-native-modbus/commit/25b4ebffd6cc36b5e56fa79596c693a20cb10ec2))

## 0.1.0 (2026-10-25)


### Features

* **esphome:** release the package on its own, pulled from github://KB1RMA/lochinvar-native-modbus/esphome/packages/boiler.yaml@esphome-vX.Y.Z ([e155fad](https://github.com/KB1RMA/lochinvar-native-modbus/commit/e155fadc69eb882d8d818d8e22c8b2cc4c02b949))
* **mqtt:** add a read-only Modbus RTU to MQTT bridge with Home Assistant discovery, published as ghcr.io/kb1rma/lochinvar-native-modbus for amd64, arm64 and arm/v7 ([e155fad](https://github.com/KB1RMA/lochinvar-native-modbus/commit/e155fadc69eb882d8d818d8e22c8b2cc4c02b949))
* **mqtt:** read settings from flags, environment variables, _FILE variables (Docker secrets) or a KEY=VALUE config file, with TLS and a private CA through MQTT_CA_FILE ([e155fad](https://github.com/KB1RMA/lochinvar-native-modbus/commit/e155fadc69eb882d8d818d8e22c8b2cc4c02b949))
* **mqtt:** reopen the serial port after 3 minutes of silence, backing off to 30 minutes, counted by Link Reconnects ([e155fad](https://github.com/KB1RMA/lochinvar-native-modbus/commit/e155fadc69eb882d8d818d8e22c8b2cc4c02b949))


### Bug Fixes

* **esphome:** keep Failed Ignitions from counting reads of the ignition counters taken during prepurge or ignition ([e155fad](https://github.com/KB1RMA/lochinvar-native-modbus/commit/e155fadc69eb882d8d818d8e22c8b2cc4c02b949))
* **esphome:** restart the ESP after 3 minutes of a dead boiler link, counted by a new Link Watchdog Restarts sensor ([e155fad](https://github.com/KB1RMA/lochinvar-native-modbus/commit/e155fadc69eb882d8d818d8e22c8b2cc4c02b949))
