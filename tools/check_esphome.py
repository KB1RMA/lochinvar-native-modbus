#!/usr/bin/env python3
"""Check esphome/lochinvar.yaml against registers/controller.yaml and enforce read-only."""
import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parent.parent
WRITE_COMPONENTS = {"switch", "number", "select", "output", "button", "light", "fan", "cover", "climate", "lock", "valve"}


class Loader(yaml.SafeLoader):
    pass


Loader.add_constructor("!secret", lambda loader, node: f"<secret {node.value}>")


def known_addresses(regmap):
    addrs = {}
    for section in ("live", "status", "counters", "settings", "info"):
        for entry in regmap[section]:
            base = entry["address"] if isinstance(entry["address"], list) else [entry["address"]]
            for a in base + entry.get("mirrors", []):
                addrs[a] = entry
    return addrs


def main() -> int:
    regmap = yaml.safe_load((ROOT / "registers" / "controller.yaml").read_text())
    cfg = yaml.load((ROOT / "esphome" / "lochinvar.yaml").read_text(), Loader=Loader)
    known = known_addresses(regmap)
    errors = []

    for comp in cfg:
        if comp in WRITE_COMPONENTS:
            errors.append(f"write-capable component present: {comp}")
    for platform_list in (cfg.get("sensor", []), cfg.get("binary_sensor", []), cfg.get("text_sensor", [])):
        for ent in platform_list:
            if ent.get("platform") != "modbus_controller":
                continue
            if ent.get("register_type") != "holding":
                errors.append(f"{ent['name']}: register_type must be holding (function 04 is rejected)")
            addr = ent["address"]
            if addr not in known:
                errors.append(f"{ent['name']}: address {addr} not in registers/controller.yaml")
            elif known[addr]["confidence"] == "unknown" and not ent.get("disabled_by_default"):
                errors.append(f"{ent['name']}: unknown register {addr} must be disabled_by_default")

    for e in errors:
        print("ERROR:", e)
    n = sum(1 for e in cfg.get("sensor", []) if e.get("platform") == "modbus_controller")
    print(f"{n} modbus sensors checked, {len(errors)} errors")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
