#!/usr/bin/env python3
"""Sanity-check registers/controller.yaml and print a summary."""
import sys
from pathlib import Path

import yaml

SECTIONS = ("live", "status", "counters", "settings", "info")
CONFIDENCE = {"confirmed", "likely", "guess", "unknown"}


def main() -> int:
    path = Path(__file__).resolve().parent.parent / "registers" / "controller.yaml"
    data = yaml.safe_load(path.read_text())
    encodings = set(data["encodings"])
    errors, names, addrs = [], set(), {}
    counts = {}

    for section in SECTIONS:
        for entry in data[section]:
            name = entry["name"]
            if name in names:
                errors.append(f"duplicate name {name}")
            names.add(name)
            if entry["encoding"] not in encodings:
                errors.append(f"{name}: unknown encoding {entry['encoding']}")
            if entry["confidence"] not in CONFIDENCE:
                errors.append(f"{name}: bad confidence {entry['confidence']}")
            counts[entry["confidence"]] = counts.get(entry["confidence"], 0) + 1
            base = entry["address"] if isinstance(entry["address"], list) else [entry["address"]]
            for a in base + entry.get("mirrors", []):
                width = entry.get("words", 1)
                for w in range(width):
                    owner = addrs.setdefault(a + w, name)
                    if owner != name:
                        errors.append(f"address {a + w} used by {owner} and {name}")

    for e in errors:
        print("ERROR:", e)
    print(f"{len(names)} entries, {len(addrs)} registers; by confidence: {counts}")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
