#!/usr/bin/env python3
"""Check the ESPHome configs against registers/controller.yaml, enforce read-only, and check versions.

Every YAML file under esphome/ (except secrets) is checked on its own, so the packages are checked
whether or not a device config includes them.
"""
import json
import re
import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parent.parent
ESPHOME = ROOT / "esphome"
PACKAGE = ESPHOME / "packages" / "boiler.yaml"
WRITE_COMPONENTS = {"switch", "number", "select", "output", "button", "light", "fan", "cover", "climate", "lock", "valve"}
# Any of these keys on a modbus entity would let it send something other than a function 03 read.
WRITE_KEYS = {"custom_command", "write_lambda", "use_write_multiple", "write_multiply"}


class Loader(yaml.SafeLoader):
    pass


Loader.add_constructor("!secret", lambda loader, node: f"<secret {node.value}>")
Loader.add_constructor("!include", lambda loader, node: f"<include {node.value}>")


def known_addresses(regmap):
    addrs = {}
    for section in ("live", "status", "counters", "settings", "info"):
        for entry in regmap[section]:
            base = entry["address"] if isinstance(entry["address"], list) else [entry["address"]]
            for a in base + entry.get("mirrors", []):
                addrs[a] = entry
    return addrs


def check_config(path, cfg, known):
    errors = []
    for comp in cfg:
        if comp in WRITE_COMPONENTS:
            errors.append(f"write-capable component present: {comp}")
    for platform_list in (cfg.get("sensor", []), cfg.get("binary_sensor", []), cfg.get("text_sensor", [])):
        for ent in platform_list:
            if ent.get("platform") != "modbus_controller":
                continue
            for key in WRITE_KEYS & ent.keys():
                errors.append(f"{ent['name']}: {key} is not allowed (read-only)")
            if ent.get("register_type") != "holding":
                errors.append(f"{ent['name']}: register_type must be holding (function 04 is rejected)")
            addr = ent["address"]
            if addr not in known:
                errors.append(f"{ent['name']}: address {addr} not in registers/controller.yaml")
            elif known[addr]["confidence"] == "unknown" and not ent.get("disabled_by_default"):
                errors.append(f"{ent['name']}: unknown register {addr} must be disabled_by_default")
    return [f"{path.relative_to(ROOT)}: {e}" for e in errors]


def check_versions():
    """release-please bumps these together; a mismatch means one was edited by hand. The ESPHome package
    is the "esphome" component (tags esphome-vX.Y.Z); the MQTT bridge in mqtt/ is checked by its tests."""
    expected = json.loads((ROOT / ".release-please-manifest.json").read_text())["esphome"]
    errors = []
    project = yaml.load(PACKAGE.read_text(), Loader=Loader)["esphome"]["project"]["version"]
    if project != expected:
        errors.append(f"{PACKAGE.relative_to(ROOT)}: project version {project} != manifest {expected}")
    readme = ESPHOME / "README.md"
    refs = re.findall(r"@esphome-v([^\s#]+)\s*# x-release-please-version", readme.read_text())
    if not refs:
        errors.append(f"{readme.relative_to(ROOT)}: no package refs marked x-release-please-version")
    for ref in refs:
        if ref != expected:
            errors.append(f"{readme.relative_to(ROOT)}: package ref esphome-v{ref} != manifest {expected}")
    return errors


def main() -> int:
    regmap = yaml.safe_load((ROOT / "registers" / "controller.yaml").read_text())
    known = known_addresses(regmap)
    errors, n = [], 0

    for path in sorted(ESPHOME.rglob("*.yaml")):
        if path.name.startswith("secrets") or ".esphome" in path.parts:
            continue
        cfg = yaml.load(path.read_text(), Loader=Loader) or {}
        errors += check_config(path, cfg, known)
        n += sum(1 for e in cfg.get("sensor", []) if e.get("platform") == "modbus_controller")
    errors += check_versions()

    for e in errors:
        print("ERROR:", e)
    print(f"{n} modbus sensors checked, {len(errors)} errors")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
