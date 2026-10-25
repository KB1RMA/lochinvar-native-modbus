// Package modbus talks Modbus RTU to the boiler controller's built-in serial port (FTDI FT232R,
// 19200 8N1, slave 1).
//
// READ-ONLY: the only request this package can build is function 03 (read holding registers). There
// is no Modbus library underneath, so there is no write function to reach by mistake. The bridge's
// tests check that this package is the only code that writes to the port.
package modbus

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// FnReadHolding is the only function code this program ever sends.
const FnReadHolding byte = 0x03

// MaxBlock is the largest read used. Larger reads were not tested, and blocks touching unmapped
// addresses (956-1005, 1406+) fail with exception 2.
const MaxBlock = 50

// ErrNoReply means the boiler did not answer (timeout or bad CRC) after every retry.
var ErrNoReply = errors.New("modbus: no reply")

// ExceptionError is a Modbus exception reply (2 = illegal address).
type ExceptionError struct {
	Code byte
}

func (e *ExceptionError) Error() string {
	return fmt.Sprintf("modbus: exception %d", e.Code)
}

// PortError wraps a failure of the serial port itself (adapter unplugged or wedged).
type PortError struct {
	Err error
}

func (e *PortError) Error() string {
	return "serial port: " + e.Err.Error()
}

func (e *PortError) Unwrap() error {
	return e.Err
}

// CRC16 is the Modbus RTU CRC, sent low byte first.
func CRC16(data []byte) [2]byte {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for range 8 {
			if crc&1 != 0 {
				crc = crc>>1 ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	var out [2]byte
	binary.LittleEndian.PutUint16(out[:], crc)
	return out
}

// ReadRequest builds a function 03 frame.
func ReadRequest(slave byte, address, count uint16) ([]byte, error) {
	if count < 1 || count > MaxBlock {
		return nil, fmt.Errorf("modbus: count %d out of range 1-%d", count, MaxBlock)
	}
	req := make([]byte, 6, 8)
	req[0], req[1] = slave, FnReadHolding
	binary.BigEndian.PutUint16(req[2:], address)
	binary.BigEndian.PutUint16(req[4:], count)
	crc := CRC16(req)
	return append(req, crc[:]...), nil
}
