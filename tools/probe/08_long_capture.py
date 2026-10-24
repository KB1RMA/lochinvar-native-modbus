"""Long-running, change-only register logger for the boiler's built-in Modbus port.

READ-ONLY: the only request this script can build is function 03 (read holding registers).

Reads 0-255 every second and the rest of the readable map (256-955, 1006-1405) every minute,
in blocks of 50. Writes one row per register that changed: time,addr,value. Each daily file
starts with a full snapshot, so every file stands on its own.

While it runs, type a line and press Enter to log what the boiler display shows
(e.g. "prepurge", "DHW call", "SH 72%"). Notes are written as time,note,<text>.

Opens the port once. Reopens it if the adapter disappears, or after --reopen-after polls with no reply
(the link goes silent at ignition, see docs/reverse-engineering-log.md). Link events are logged as
time,link,silent|reopen|ok|port_error.

    caffeinate -is python3 -I tools/probe/08_long_capture.py
"""
import argparse, glob, os, select, struct, sys, termios, time

SLAVE = 1
FN_READ_HOLDING = 3          # the only function code this script ever sends
FAST = [(0, 256)]
SLOW = [(256, 956), (1006, 1406)]   # 956-1005 and 1406+ return exception 2
BLOCK = 50


def crc(d):
    c = 0xFFFF
    for x in d:
        c ^= x
        for _ in range(8): c = (c >> 1) ^ 0xA001 if c & 1 else c >> 1
    return struct.pack("<H", c)


def blocks(ranges):
    for lo, hi in ranges:
        for st in range(lo, hi, BLOCK):
            yield st, min(BLOCK, hi - st)


class Port:
    def __init__(self, path):
        self.path, self.fd = path, None

    def open(self):
        dev = self.path or (glob.glob("/dev/cu.usbserial*") + [None])[0]
        if dev is None: raise OSError("no /dev/cu.usbserial* device")
        fd = os.open(dev, os.O_RDWR | os.O_NOCTTY | os.O_NONBLOCK)
        try:
            a = termios.tcgetattr(fd); a[0] = a[1] = a[3] = 0
            a[2] = termios.CS8 | termios.CREAD | termios.CLOCAL; a[4] = a[5] = termios.B19200
            a[6][termios.VMIN] = 0; a[6][termios.VTIME] = 0
            termios.tcsetattr(fd, termios.TCSANOW, a)
        except termios.error as e:
            # The FTDI wedge (EINVAL from tcsetattr). termios.error is not an OSError, so convert it
            # for the retry loop in main().
            os.close(fd); raise OSError(*e.args) from e
        except Exception:
            os.close(fd); raise
        self.fd = fd
        return dev

    def close(self):
        if self.fd is not None:
            try: os.close(self.fd)
            except OSError: pass
        self.fd = None

    def read(self, addr, n):
        """Function 03 read. Returns a list, "exc" for a Modbus exception, or None on timeout/CRC error."""
        if self.fd is None: raise OSError("port not open")   # a reopen failed; main() retries
        req = bytes([SLAVE, FN_READ_HOLDING]) + struct.pack(">HH", addr, n); req += crc(req)
        want = 5 + 2 * n
        for _ in range(3):
            try: termios.tcflush(self.fd, termios.TCIOFLUSH)
            except termios.error as e: raise OSError(*e.args) from e
            os.write(self.fd, req)
            buf = b""; end = time.time() + 0.6
            while time.time() < end and len(buf) < want and not (len(buf) >= 5 and buf[1] == 0x83):
                if select.select([self.fd], [], [], 0.03)[0]: buf += os.read(self.fd, 512)
            if len(buf) == want and buf[:2] == req[:2] and buf[-2:] == crc(buf[:-2]):
                return list(struct.unpack(">%dH" % n, buf[3:3 + 2 * n]))
            if len(buf) == 5 and buf[1] == 0x83 and buf[3:5] == crc(buf[:3]):
                return "exc"
        return None


def stamp(t):
    return time.strftime("%Y-%m-%dT%H:%M:%S", time.localtime(t)) + ".%03d" % (int(t * 1000) % 1000)


class Log:
    def __init__(self, outdir):
        self.outdir, self.day, self.f = outdir, None, None
        os.makedirs(outdir, exist_ok=True)

    def rotate(self, t, state):
        day = time.strftime("%Y-%m-%d", time.localtime(t))
        if day == self.day: return
        if self.f: self.f.close()
        path = os.path.join(self.outdir, day + ".csv")
        new = not os.path.exists(path)
        self.f = open(path, "a"); self.day = day
        if new: self.f.write("time,addr,value\n")
        self.row(t, "snapshot", len(state))
        for k in sorted(state): self.row(t, k, state[k])
        print("logging to", path, flush=True)

    def row(self, t, addr, value):
        self.f.write("%s,%s,%s\n" % (stamp(t), addr, value))

    def note(self, t, text):
        self.row(t, "note", '"%s"' % text.replace('"', "'"))


def poll(port, ranges, state, log, errs):
    """Returns (time, silent). silent means the first block got no reply, and the rest were skipped."""
    t = time.time()
    for i, (st, n) in enumerate(blocks(ranges)):
        r = port.read(st, n)
        if not isinstance(r, list):
            errs[st] = errs.get(st, 0) + 1
            if i == 0 and r is None: return t, True
            continue
        for j, v in enumerate(r):
            k = st + j
            if state.get(k) != v:
                if k in state and log.f: log.row(t, k, v)
                state[k] = v
    return t, False


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--port", help="serial device (default: first /dev/cu.usbserial*)")
    ap.add_argument("--out", default="captures/long", help="output directory (default: %(default)s)")
    ap.add_argument("--fast", type=float, default=1.0, help="seconds between 0-255 polls")
    ap.add_argument("--slow", type=float, default=60.0, help="seconds between full-map polls")
    ap.add_argument("--hours", type=float, default=0, help="stop after this many hours (0 = run until Ctrl-C)")
    ap.add_argument("--reopen-after", type=int, default=10, help="silent polls before reopening the port")
    args = ap.parse_args()

    port, log, state, errs = Port(args.port), Log(args.out), {}, {}
    while True:
        try: print("opened", port.open(), flush=True); break
        except OSError as e: print(stamp(time.time()), "open failed:", e, "- replug the adapter; retrying in 10 s", flush=True); time.sleep(10)
    # Initial full read; changes found during it are not logged, the snapshot covers them.
    poll(port, FAST + SLOW, state, log, errs)   # a silent start just leaves registers out of the snapshot
    t0 = last_slow = time.time()
    log.rotate(t0, state); log.f.flush()
    if errs: print("initial read failed for blocks:", sorted(errs), flush=True)
    stdin_ok = sys.stdin.isatty()
    if stdin_ok: print("type a note and press Enter to log what the display shows; Ctrl-C to stop", flush=True)
    # Silent polls before the port is closed and reopened. Doubles after each reopen that doesn't help
    # (up to 8x), to limit open/close cycles on the macOS FTDI driver; resets once replies come back.
    last_status, silent, reopen_after = t0, 0, args.reopen_after
    try:
        while not args.hours or time.time() - t0 < args.hours * 3600:
            start = time.time()
            try:
                ranges = FAST + (SLOW if start - last_slow >= args.slow else [])
                if len(ranges) > 1: last_slow = start
                t, quiet = poll(port, ranges, state, log, errs)
                if quiet:
                    silent += 1
                    if silent == 1: log.row(t, "link", "silent")
                    if silent >= reopen_after:
                        print(stamp(t), "no answer for %d polls - reopening port" % silent, flush=True)
                        log.row(t, "link", "reopen"); log.f.flush()
                        port.close(); time.sleep(2)
                        print("reopened", port.open(), flush=True)
                        silent, reopen_after = 0, min(reopen_after * 2, args.reopen_after * 8)
                else:
                    if silent or reopen_after > args.reopen_after:
                        log.row(t, "link", "ok"); print(stamp(t), "boiler answering again", flush=True)
                    silent, reopen_after = 0, args.reopen_after
                log.rotate(t, state)
            except OSError as e:
                # Adapter unplugged or wedged: reopen slowly rather than hammering open/close.
                print(stamp(time.time()), "port error:", e, "- retrying in 10 s", flush=True)
                if log.f: log.row(time.time(), "link", "port_error"); log.f.flush()
                port.close(); time.sleep(10)
                try: print("reopened", port.open(), flush=True)
                except OSError as e2: print("reopen failed:", e2, flush=True)
                continue
            while True:
                left = args.fast - (time.time() - start)
                if left <= 0: break
                if stdin_ok and select.select([sys.stdin], [], [], left)[0]:
                    line = sys.stdin.readline()
                    if not line: stdin_ok = False; continue
                    if line.strip():
                        log.note(time.time(), line.strip()); log.f.flush()
                        print("  noted", flush=True)
                elif not stdin_ok:
                    time.sleep(left)
            log.f.flush()
            if time.time() - last_status >= 600:
                last_status = time.time()
                print(stamp(last_status), "running; %d registers tracked; failed reads so far: %d"
                      % (len(state), sum(errs.values())), flush=True)
    except KeyboardInterrupt:
        pass
    finally:
        if log.f: log.f.close()
        port.close()
        print("stopped; failed reads by block start:", dict(sorted(errs.items())), flush=True)


if __name__ == "__main__":
    main()
