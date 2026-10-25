// Package modbustest provides a fake boiler for tests: a serial port that answers function 03 reads
// from a register map and records every frame written to it.
package modbustest

import (
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/modbus"
)

// ErrIO is what the fake returns while Fail is set, like an unplugged adapter.
var ErrIO = errors.New("input/output error")

// Boiler is a fake serial port. Set its fields to change how it answers; it is safe for concurrent use.
type Boiler struct {
	mu        sync.Mutex
	registers map[uint16]uint16
	written   [][]byte
	pending   []byte
	opened    []string
	silent    bool
	exception byte
	fail      bool
	corrupt   bool
}

// NewBoiler answers from a copy of registers; unset addresses read 0.
func NewBoiler(registers map[uint16]uint16) *Boiler {
	regs := make(map[uint16]uint16, len(registers))
	for k, v := range registers {
		regs[k] = v
	}
	return &Boiler{registers: regs}
}

// Set changes one register.
func (b *Boiler) Set(address, value uint16) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.registers[address] = value
}

// Get reads one register.
func (b *Boiler) Get(address uint16) uint16 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.registers[address]
}

// SetSilent stops (or restarts) all replies, like the link at ignition.
func (b *Boiler) SetSilent(v bool) { b.mu.Lock(); b.silent = v; b.mu.Unlock() }

// SetException makes every reply a Modbus exception with this code; 0 turns it off.
func (b *Boiler) SetException(code byte) { b.mu.Lock(); b.exception = code; b.mu.Unlock() }

// SetFail makes opening, reading and writing fail, like an unplugged adapter.
func (b *Boiler) SetFail(v bool) { b.mu.Lock(); b.fail = v; b.mu.Unlock() }

// SetCorrupt flips bits in each reply's CRC.
func (b *Boiler) SetCorrupt(v bool) { b.mu.Lock(); b.corrupt = v; b.mu.Unlock() }

// Written returns a copy of every frame written so far.
func (b *Boiler) Written() [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([][]byte, len(b.written))
	copy(out, b.written)
	return out
}

// Opened returns how many times the port was opened.
func (b *Boiler) Opened() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.opened)
}

// Open is a modbus.Opener that hands out this boiler.
func (b *Boiler) Open(path string) (modbus.Port, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fail {
		return nil, ErrIO
	}
	b.opened = append(b.opened, path)
	return b, nil
}

// Link returns a link to this boiler with a short reply timeout.
func (b *Boiler) Link() *modbus.Link {
	return modbus.NewLink("/dev/fake", 1, modbus.WithOpener(b.Open), modbus.WithTimeout(time.Millisecond))
}

// ResetInputBuffer drops any unread reply.
func (b *Boiler) ResetInputBuffer() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pending = nil
	return nil
}

// Write records the frame and prepares the reply.
func (b *Boiler) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fail {
		return 0, ErrIO
	}
	b.written = append(b.written, append([]byte(nil), p...))
	if b.silent || len(p) < 6 {
		return len(p), nil
	}
	slave, fn := p[0], p[1]
	var reply []byte
	if b.exception != 0 {
		reply = []byte{slave, fn | 0x80, b.exception}
	} else {
		address, count := binary.BigEndian.Uint16(p[2:]), binary.BigEndian.Uint16(p[4:])
		reply = make([]byte, 3, 3+2*int(count)+2)
		reply[0], reply[1], reply[2] = slave, fn, byte(2*count) //nolint:gosec // count <= 50
		for i := range count {
			reply = binary.BigEndian.AppendUint16(reply, b.registers[address+i])
		}
	}
	crc := modbus.CRC16(reply)
	if b.corrupt {
		crc[1] ^= 0xFF
	}
	reply = append(reply, crc[:]...)
	b.pending = reply
	return len(p), nil
}

// Read returns pending reply bytes, or none (a timeout).
func (b *Boiler) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fail {
		return 0, ErrIO
	}
	n := copy(p, b.pending)
	b.pending = b.pending[n:]
	return n, nil
}

// Close is a no-op.
func (b *Boiler) Close() error { return nil }

// RawTemp is the register value for a temperature in °F (the inverse of decode.TempF).
func RawTemp(f float64) uint16 {
	return uint16(f*25 + 9468 + 0.5)
}

// Running is a boiler running at 40 %, in the shape registers/controller.yaml describes.
func Running() map[uint16]uint16 {
	return map[uint16]uint16{
		62: RawTemp(150.0), 73: RawTemp(130.0), 63: RawTemp(135.5), 75: RawTemp(41.3),
		64: RawTemp(150.6), 65: RawTemp(134.8), 142: 0x8000 | RawTemp(172.1), 146: 0x8000 | RawTemp(172.1),
		66: 2100, 110: 3600, 80: 4000, 132: 10000, 84: 9, 118: 1, 137: 0, 138: 4, 139: 16, 140: 96,
		1286: RawTemp(140), 1287: RawTemp(180), 1288: RawTemp(125), 1289: RawTemp(125),
		1291: RawTemp(200), 1292: RawTemp(210),
		1302: 0, 1303: 40000, 1304: 0, 1305: 5109, 1306: 0, 1307: 5000, 1308: 0, 1309: 3828, 1310: 0, 1311: 2055,
		1312: 0, 1313: 4500,
	}
}
