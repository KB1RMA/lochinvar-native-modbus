package modbus

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"go.bug.st/serial"
)

// Port is the part of a serial port the link uses. A read returns 0 bytes and no error on timeout.
type Port interface {
	io.ReadWriteCloser
	ResetInputBuffer() error
}

// Opener opens the serial device at path.
type Opener func(path string) (Port, error)

// OpenSerial opens the boiler's port with its link settings. Open it once per run where possible:
// on macOS the FTDI driver has wedged (tcsetattr EINVAL) after many open/close cycles.
func OpenSerial(path string) (Port, error) {
	p, err := serial.Open(path, &serial.Mode{
		BaudRate: 19200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if err := p.SetReadTimeout(50 * time.Millisecond); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("set read timeout on %s: %w", path, err)
	}
	return p, nil
}

// Link is the serial port plus function 03 reads. It is not safe for concurrent use.
type Link struct {
	path    string
	slave   byte
	open    Opener
	timeout time.Duration
	retries int
	port    Port
}

// Option changes a Link's defaults.
type Option func(*Link)

// WithOpener replaces the serial port opener (for tests).
func WithOpener(o Opener) Option { return func(l *Link) { l.open = o } }

// WithTimeout sets how long to wait for each reply.
func WithTimeout(d time.Duration) Option { return func(l *Link) { l.timeout = d } }

// NewLink returns a closed link to the device at path.
func NewLink(path string, slave byte, opts ...Option) *Link {
	l := &Link{path: path, slave: slave, open: OpenSerial, timeout: 600 * time.Millisecond, retries: 3}
	for _, o := range opts {
		o(l)
	}
	return l
}

// Path is the serial device.
func (l *Link) Path() string { return l.path }

// IsOpen reports whether the port is open.
func (l *Link) IsOpen() bool { return l.port != nil }

// Open (re)opens the port. Failures are *PortError.
func (l *Link) Open() error {
	l.Close()
	p, err := l.open(l.path)
	if err != nil {
		return &PortError{Err: err}
	}
	l.port = p
	return nil
}

// Close closes the port; closing a closed link does nothing.
func (l *Link) Close() {
	if l.port != nil {
		_ = l.port.Close()
		l.port = nil
	}
}

// Read reads count holding registers from address with function 03. It returns ErrNoReply when the
// boiler did not answer after every retry, *ExceptionError on an exception reply, and *PortError when
// the port fails.
func (l *Link) Read(address, count uint16) ([]uint16, error) {
	if l.port == nil {
		return nil, &PortError{Err: fmt.Errorf("%s not open", l.path)}
	}
	req, err := ReadRequest(l.slave, address, count)
	if err != nil {
		return nil, err
	}
	for range l.retries {
		values, err := l.transact(req, count)
		if !errors.Is(err, ErrNoReply) {
			return values, err
		}
		slog.Debug("no valid reply", "address", address, "count", count)
	}
	return nil, ErrNoReply
}

func (l *Link) transact(req []byte, count uint16) ([]uint16, error) {
	if err := l.port.ResetInputBuffer(); err != nil {
		return nil, &PortError{Err: err}
	}
	if _, err := l.port.Write(req); err != nil {
		return nil, &PortError{Err: err}
	}
	want := 5 + 2*int(count)
	exception := FnReadHolding | 0x80
	buf := make([]byte, 0, want)
	chunk := make([]byte, want)
	deadline := time.Now().Add(l.timeout)
	for time.Now().Before(deadline) && len(buf) < want {
		if len(buf) >= 5 && buf[1] == exception {
			break
		}
		n, err := l.port.Read(chunk[:want-len(buf)])
		if err != nil {
			return nil, &PortError{Err: err}
		}
		buf = append(buf, chunk[:n]...)
	}

	if len(buf) >= 5 && buf[0] == l.slave && buf[1] == exception {
		if crc := CRC16(buf[:3]); bytes.Equal(buf[3:5], crc[:]) {
			return nil, &ExceptionError{Code: buf[2]}
		}
		return nil, ErrNoReply
	}
	if len(buf) != want || buf[0] != l.slave || buf[1] != FnReadHolding || int(buf[2]) != 2*int(count) {
		return nil, ErrNoReply
	}
	if crc := CRC16(buf[:want-2]); !bytes.Equal(buf[want-2:], crc[:]) {
		return nil, ErrNoReply
	}
	values := make([]uint16, count)
	for i := range values {
		values[i] = binary.BigEndian.Uint16(buf[3+2*i:])
	}
	return values, nil
}
