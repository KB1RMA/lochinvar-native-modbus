package modbus_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"slices"
	"testing"

	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/modbus"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/modbustest"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCRC16KnownFrame(t *testing.T) {
	t.Parallel()
	// 01 03 00 00 00 0A -> CRC C5 CD, the classic Modbus example.
	got := modbus.CRC16(mustHex(t, "01030000000A"))
	if !bytes.Equal(got[:], mustHex(t, "C5CD")) {
		t.Fatalf("CRC16 = %X, want C5CD", got)
	}
}

func TestReadRequest(t *testing.T) {
	t.Parallel()
	got, err := modbus.ReadRequest(1, 62, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := mustHex(t, "0103003E0002")
	crc := modbus.CRC16(want)
	want = append(want, crc[:]...)
	if !bytes.Equal(got, want) {
		t.Fatalf("ReadRequest = %X, want %X", got, want)
	}
	for _, count := range []uint16{0, modbus.MaxBlock + 1} {
		if _, err := modbus.ReadRequest(1, 0, count); err == nil {
			t.Errorf("ReadRequest(count=%d) succeeded, want error", count)
		}
	}
}

func openLink(t *testing.T) (*modbus.Link, *modbustest.Boiler) {
	t.Helper()
	boiler := modbustest.NewBoiler(modbustest.Running())
	link := boiler.Link()
	if err := link.Open(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(link.Close)
	return link, boiler
}

func TestReadValues(t *testing.T) {
	t.Parallel()
	link, boiler := openLink(t)
	got, err := link.Read(62, 2)
	if err != nil {
		t.Fatal(err)
	}
	if want := []uint16{boiler.Get(62), boiler.Get(63)}; !slices.Equal(got, want) {
		t.Fatalf("Read = %v, want %v", got, want)
	}
}

func TestExceptionReply(t *testing.T) {
	t.Parallel()
	link, boiler := openLink(t)
	boiler.SetException(2)
	_, err := link.Read(956, 10)
	var exc *modbus.ExceptionError
	if !errors.As(err, &exc) || exc.Code != 2 {
		t.Fatalf("Read error = %v, want exception 2", err)
	}
}

func TestSilenceRetriesThenNoReply(t *testing.T) {
	t.Parallel()
	link, boiler := openLink(t)
	boiler.SetSilent(true)
	if _, err := link.Read(62, 1); !errors.Is(err, modbus.ErrNoReply) {
		t.Fatalf("Read error = %v, want ErrNoReply", err)
	}
	if n := len(boiler.Written()); n != 3 {
		t.Fatalf("%d frames written, want 3 (retries)", n)
	}
}

func TestBadCRCIsNoReply(t *testing.T) {
	t.Parallel()
	link, boiler := openLink(t)
	boiler.SetCorrupt(true)
	if _, err := link.Read(62, 1); !errors.Is(err, modbus.ErrNoReply) {
		t.Fatalf("Read error = %v, want ErrNoReply", err)
	}
}

func TestPortErrors(t *testing.T) {
	t.Parallel()
	link, boiler := openLink(t)
	boiler.SetFail(true)
	var pe *modbus.PortError
	if _, err := link.Read(62, 1); !errors.As(err, &pe) || !errors.Is(err, modbustest.ErrIO) {
		t.Fatalf("Read error = %v, want PortError wrapping ErrIO", err)
	}
	link.Close()
	if err := link.Open(); !errors.As(err, &pe) {
		t.Fatalf("Open error = %v, want PortError", err)
	}
	if link.IsOpen() {
		t.Fatal("link open after a failed Open")
	}
	if _, err := modbus.NewLink("/dev/x", 1).Read(62, 1); !errors.As(err, &pe) {
		t.Fatalf("Read on a closed link: %v, want PortError", err)
	}
}

func FuzzReadRequest(f *testing.F) {
	f.Add(uint8(1), uint16(62), uint16(2))
	f.Fuzz(func(t *testing.T, slave uint8, address, count uint16) {
		req, err := modbus.ReadRequest(slave, address, count)
		if err != nil {
			if count >= 1 && count <= modbus.MaxBlock {
				t.Fatalf("valid count %d rejected: %v", count, err)
			}
			return
		}
		if len(req) != 8 || req[1] != modbus.FnReadHolding {
			t.Fatalf("bad frame %X", req)
		}
		if crc := modbus.CRC16(req[:6]); !bytes.Equal(req[6:], crc[:]) {
			t.Fatalf("bad CRC in %X", req)
		}
	})
}
