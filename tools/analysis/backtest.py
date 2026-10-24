#!/usr/bin/env python3
"""Backtest the register map's candidate mappings against captured logs. Offline: reads CSVs only.

Replays two capture formats into per-poll register states:
  - full rows, one per poll (captures/flame_log.csv, session 1)
  - change-only rows from tools/probe/08_long_capture.py (captures/long/*.csv, session 2 onwards)

Each state is weighted by how long it held (until the next poll that changed something), so the
counts below are seconds of boiler time, not rows. A change-only log has no row for a poll where
nothing changed, so without the weighting busy periods would count for more than quiet ones.

Link drops: from a `link,silent|reopen|port_error` row the state is unknown and nothing is sampled.
Sampling resumes at the next `snapshot` (a full read) or `link,ok` (written after the first complete
poll). Early in session 2 the link also died without a `link` row (11:24:52, 11:31:44), so a gap of
more than MAX_GAP seconds between polls counts as a drop too: r62 and r72 change every few seconds,
and quiet gaps in the 2026-10-24 log never exceeded 5 s. The last poll before a drop is held 1 s.

    python3 -I tools/analysis/backtest.py
    python3 -I tools/analysis/backtest.py --long captures/long/2026-10-25.csv --notes captures/long/notes-2026-10-25.csv
"""
import argparse, collections, csv, statistics as st
from datetime import datetime
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent.parent
NO_DEMAND_32F = 43036       # temp_f raw for exactly 32.0 F
DOWN = {"silent", "reopen", "port_error"}
MAX_GAP = 8.0               # seconds between polls of a change-only log before it counts as a drop


def tf(v): return ((v & 0x7FFF) - 9468) / 25


def load_full(path):
    """Full-row log: one row per poll, columns time,r0..rN. Returns (time, dt, regs, drop_after) per poll."""
    polls = []
    for r in csv.DictReader(open(path)):
        regs = {int(k[1:]): int(v) for k, v in r.items() if k.startswith("r") and v != ""}
        polls.append([datetime.strptime(r["time"], "%H:%M:%S.%f"), regs])
    out = []
    for (t, regs), nxt in zip(polls, polls[1:] + [None]):
        gap = (nxt[0] - t).total_seconds() if nxt else None
        drop = gap is None or gap > MAX_GAP
        out.append((t, 1.0 if drop else gap, regs, drop))
    return out


def load_changes(path):
    """Change-only log from 08_long_capture.py. Returns (time, dt, regs, drop_after) per poll while the link is up."""
    state, polls, up, cur = {}, [], True, None   # polls: [time, regs, ends_in_drop]
    def flush():
        if cur is not None and up: polls.append([cur, dict(state), False])
    for r in csv.DictReader(open(path)):
        t = datetime.fromisoformat(r["time"])
        if t != cur:
            flush(); cur = t
        if r["addr"] == "link":
            if r["value"] in DOWN:
                if up and polls: polls[-1][2] = True
                up = False
            elif r["value"] == "ok":
                up = True
        elif r["addr"] == "snapshot":
            up = True
        elif r["addr"].isdigit():
            state[int(r["addr"])] = int(r["value"])
    flush()
    out = []
    for (t, regs, drop), nxt in zip(polls, polls[1:] + [None]):
        gap = (nxt[0] - t).total_seconds() if nxt else None
        drop = drop or gap is None or gap > MAX_GAP
        out.append((t, 1.0 if drop else gap, regs, drop))
    return out


def ct(rows, f):
    """Seconds spent in each value of f(regs)."""
    c = collections.Counter()
    for _, dt, x, _ in rows: c[f(x)] += dt
    return {k: round(v) for k, v in sorted(c.items(), key=str)}


def agree(rows, f):
    tot = sum(dt for _, dt, _, _ in rows)
    return 100 * sum(dt for _, dt, x, _ in rows if f(x)) / tot


def corr(rows, f, g):
    xs, ys = [f(x) for _, _, x, _ in rows], [g(x) for _, _, x, _ in rows]
    try: return round(st.correlation(xs, ys), 3)
    except st.StatisticsError: return None


def derived_status(x):
    """What a MODB-I-O board would call the state, built from native registers."""
    if x[84] == 9: return "running"
    if x[84] == 6: return "prepurge"
    if x[84] == 10: return "post-purge"
    if x[137] == 1: return "anti-cycle (MODB blocking 13)"
    if x[118] == 1 and x[140] == 112: return "demand, held off"
    if x[118] == 1: return "demand"
    if x[132] > 0: return "pump delay (MODB status 33)"
    return "idle (MODB status 34)"


def tests(name, rows):
    tot = sum(dt for _, dt, _, _ in rows)
    print(f"===== {name}: {len(rows)} polls, {tot:.0f} s with the link up")
    print("r127 burner_running vs flame r66 > 500 nA:", ct(rows, lambda x: (x[127], x[66] > 500)))
    print("  r127 == (r84 == 9): %.1f%%" % agree(rows, lambda x: x[127] == (x[84] == 9)))
    print("  r130 == r127: %.1f%%" % agree(rows, lambda x: x[130] == x[127]))
    for t, _, x, _ in rows:
        if x[127] == 0 and x[66] > 500:
            print(f"  flame without r127 at {t:%H:%M:%S}: r66 {x[66]}, r84 {x[84]}")
    print("r146 == 32.0 F vs r118 demand:", ct(rows, lambda x: (x[146] == NO_DEMAND_32F, x[118])))
    print("  r147 == r146: %.1f%%" % agree(rows, lambda x: x[147] == x[146]))
    print("safety_inputs r120-r126 values:", {a: sorted({x[a] for _, _, x, _ in rows}) for a in range(120, 127)})
    print("  r124 by r84 state:", {s: sorted({x[124] for _, _, x, _ in rows if x[84] == s}) for s in (4, 6, 9, 10)})
    print("constant no-sensor/zero:", {a: sorted({x[a] for _, _, x, _ in rows}) for a in (68, 70, 77, 157, 159, 134)})
    for m, s in ((154, 62), (155, 73), (158, 75)):
        print("  mirror r%d == r%d: %.1f%%" % (m, s, agree(rows, lambda x: x[m] == x[s])))
    print("r67 vs pump on (r132 > 0):", ct(rows, lambda x: (x[67], x[132] > 0)))
    print("  r67 vs r135:", ct(rows, lambda x: (x[67], x[135])))
    t74 = [tf(x[74]) for _, _, x, _ in rows]
    print("r74 cabinet_temp %.1f-%.1f F; corr outdoor %s, inlet %s, exhaust %s, firing %s" % (
        min(t74), max(t74), corr(rows, lambda x: tf(x[74]), lambda x: tf(x[75])),
        corr(rows, lambda x: tf(x[74]), lambda x: tf(x[73])),
        corr(rows, lambda x: tf(x[74]), lambda x: tf(x[63])), corr(rows, lambda x: tf(x[74]), lambda x: x[80])))
    run = [p for p in rows if p[2][84] == 9]
    if run:
        print("r104 fan_drive while running: corr r80 %s, r110 %s, r66 %s" % (
            corr(run, lambda x: x[104], lambda x: x[80]), corr(run, lambda x: x[104], lambda x: x[110]),
            corr(run, lambda x: x[104], lambda x: x[66])))
        r = [x[104] / x[110] for _, _, x, _ in run if x[110]]
        print("  r104 / r110 median %.4f, sd %.4f" % (st.median(r), st.pstdev(r)))
    print("r111 fan_ramp_limit by r84 state:", ct(rows, lambda x: (x[111], x[84])))
    print("  r112 == -r111 always:", all((x[111] + x[112]) % 65536 == 0 for _, _, x, _ in rows))


def timeline(rows, notes):
    """Derived status changes interleaved with display notes and link drops."""
    events, prev = [], None
    for i, (t, _, x, drop) in enumerate(rows):
        s = derived_status(x)
        if s != prev: events.append((t, s)); prev = s
        if drop and i < len(rows) - 1: events.append((t, "-- no data (link down) --")); prev = None
    if notes and Path(notes).exists():
        for r in csv.DictReader(open(notes)):
            events.append((datetime.fromisoformat(r["time"]), "NOTE " + r["note"][:100]))
    print("===== derived status vs display notes")
    for t, s in sorted(events, key=lambda e: e[0]): print(f"{t:%H:%M:%S} {s}")


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--full", default=str(ROOT / "captures/flame_log.csv"))
    ap.add_argument("--long", default=str(ROOT / "captures/long/2026-10-24.csv"))
    ap.add_argument("--notes", default=str(ROOT / "captures/long/notes-2026-10-24.csv"))
    a = ap.parse_args()
    tests("full-row log " + Path(a.full).name, load_full(a.full))
    rows = load_changes(a.long)
    tests("change log " + Path(a.long).name, rows)
    timeline(rows, a.notes)


if __name__ == "__main__":
    main()
