// Package bridge is the poll loop: read the boiler, decode, publish. It behaves like
// esphome/packages/boiler.yaml on the live ESP: fast readings every 5 s, settings and counters every
// 5 min, readings cleared and Boiler Status "Link lost" while the boiler does not answer.
//
// Link recovery differs, because a container cannot reboot itself the way the ESP does: after the
// link has been down 3 minutes the serial port is closed and reopened, and each reopen that doesn't
// help doubles the wait (up to 30 min), to limit open/close cycles (the macOS FTDI driver has wedged
// after many). Like the ESP's watchdog, this only arms once the boiler has answered since start, so a
// boiler that is off does not cause a reopen loop. Port errors (adapter unplugged) retry every 10 s.
// A Modbus exception reply is not a link loss: the boiler answered, so the link stays up and the port
// is never reopened for it; that poll's readings are cleared and Boiler Status reads "Unavailable". The
// slow blocks are read one by one, so an exception on one clears only the settings or counters in it.
package bridge

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/decode"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/discovery"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/entities"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/modbus"
)

// Block is one function 03 read.
type Block struct {
	Start, Count uint16
}

// The function 03 blocks tools/probe/08_long_capture.py reads, plus the installer settings 1161-1183.
var (
	FastBlocks = []Block{{50, 50}, {100, 50}} // r62-r140
	// Installer settings 1161-1183, settings 1286-1293, counters 1302-1315.
	SlowBlocks = []Block{{1161, 23}, {1256, 50}, {1306, 50}}
)

// Covers reports whether address is in the block.
func (blk Block) Covers(address uint16) bool {
	return address >= blk.Start && address-blk.Start < blk.Count
}

// Reader is the boiler link (*modbus.Link).
type Reader interface {
	Path() string
	IsOpen() bool
	Open() error
	Close()
	Read(address, count uint16) ([]uint16, error)
}

// Publisher sends one MQTT message. It must not block for long.
type Publisher interface {
	Publish(topic, payload string, retain bool)
}

// Options are the timings; the zero value of a field means its default.
type Options struct {
	FastInterval time.Duration // 5 s
	SlowInterval time.Duration // 5 min
	OfflineSkip  int           // fast polls skipped while the link is down: 6, like the ESP
	ReopenAfter  time.Duration // 3 min
	ReopenMax    time.Duration // 30 min
	PortRetry    time.Duration // 10 s
}

func (o Options) withDefaults() Options {
	def := func(d *time.Duration, v time.Duration) {
		if *d <= 0 {
			*d = v
		}
	}
	def(&o.FastInterval, 5*time.Second)
	def(&o.SlowInterval, 5*time.Minute)
	def(&o.ReopenAfter, 3*time.Minute)
	def(&o.ReopenMax, 30*time.Minute)
	def(&o.PortRetry, 10*time.Second)
	if o.OfflineSkip <= 0 {
		o.OfflineSkip = 6
	}
	return o
}

// offlineWait is the poll interval while the link is down or answering with exceptions.
func (o Options) offlineWait() time.Duration { return o.FastInterval * time.Duration(o.OfflineSkip) }

type linkState uint8

const (
	linkUnknown linkState = iota
	linkUp
	linkDown
)

// Bridge holds the poll state. Step, RepublishAll and Shutdown must be called from one goroutine.
type Bridge struct {
	link       Reader
	pub        Publisher
	topics     discovery.Topics
	deviceName string
	opt        Options

	payloads      map[string]string // key -> payload last published
	order         []string          // keys in first-published order, for RepublishAll
	state         linkState
	linkSeenOK    bool
	linkDownSince time.Time
	reopenWait    time.Duration
	reconnects    int
	status        decode.Value // latest Status Code; None while the link is down
	guard         decode.IgnitionGuard
	nextFast      time.Time
	nextSlow      time.Time
	nextPort      time.Time
}

// New returns a bridge; nothing is read or published until Step.
func New(link Reader, pub Publisher, topics discovery.Topics, deviceName string, opt Options) *Bridge {
	opt = opt.withDefaults()
	b := &Bridge{
		link: link, pub: pub, topics: topics, deviceName: deviceName, opt: opt,
		payloads: map[string]string{}, reopenWait: opt.ReopenAfter,
	}
	b.set(entities.LinkReconnects, decode.Num(0))
	return b
}

// Format renders a value the way Home Assistant expects for the entity. No reading is "None",
// which Home Assistant shows as unknown.
func Format(e entities.Entity, v decode.Value) string {
	if !v.Valid() {
		return "None"
	}
	switch e.Component {
	case entities.BinarySensor:
		if v.On() {
			return "ON"
		}
		return "OFF"
	case entities.TextSensor:
		return v.String()
	case entities.Sensor:
	}
	f, ok := v.Float()
	if !ok {
		return v.String()
	}
	if e.Precision == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strconv.FormatFloat(f, 'f', *e.Precision, 64)
}

func (b *Bridge) set(key string, v decode.Value) {
	e, ok := entities.ByKey(key)
	if !ok {
		panic(fmt.Sprintf("bridge: unknown entity %q", key))
	}
	payload := Format(e, v)
	old, seen := b.payloads[key]
	if seen && old == payload {
		return
	}
	if !seen {
		b.order = append(b.order, key)
	}
	b.payloads[key] = payload
	b.pub.Publish(b.topics.State(e), payload, true)
}

// RepublishAll sends availability, discovery and every known state. Call it on every (re)connect to
// the broker and whenever Home Assistant comes online.
func (b *Bridge) RepublishAll() error {
	msgs, err := discovery.Messages(b.topics, b.deviceName)
	if err != nil {
		return err
	}
	b.pub.Publish(b.topics.Status(), discovery.Online, true)
	for _, m := range msgs {
		b.pub.Publish(m.Topic, m.Payload, true)
	}
	switch b.state {
	case linkUp:
		b.pub.Publish(b.topics.Link(), discovery.Online, true)
	case linkDown:
		b.pub.Publish(b.topics.Link(), discovery.Offline, true)
	case linkUnknown:
	}
	for _, key := range b.order {
		e, _ := entities.ByKey(key)
		b.pub.Publish(b.topics.State(e), b.payloads[key], true)
	}
	return nil
}

// Shutdown marks the bridge offline and closes the port.
func (b *Bridge) Shutdown() {
	b.pub.Publish(b.topics.Status(), discovery.Offline, true)
	b.link.Close()
}

// ReopenWait is how long the link must stay down before the next reopen.
func (b *Bridge) ReopenWait() time.Duration { return b.reopenWait }

func (b *Bridge) linkUp() {
	if b.state != linkUp {
		slog.Info("boiler answering")
		b.state = linkUp
		b.pub.Publish(b.topics.Link(), discovery.Online, true)
	}
	b.linkSeenOK = true
	b.linkDownSince = time.Time{}
	b.reopenWait = b.opt.ReopenAfter
}

func (b *Bridge) linkDown(now time.Time) {
	if b.state != linkDown {
		slog.Warn("boiler link lost")
		b.state = linkDown
		b.pub.Publish(b.topics.Link(), discovery.Offline, true)
		b.clearFast("Link lost")
	}
	b.status = decode.None
	if b.linkSeenOK && b.linkDownSince.IsZero() {
		b.linkDownSince = now
	}
}

// clearFast clears the live readings and sets Boiler Status.
func (b *Bridge) clearFast(boilerStatus string) {
	for _, g := range []entities.Group{entities.Fast, entities.Derived} {
		for _, e := range entities.InGroup(g) {
			b.set(e.Key, decode.None)
		}
	}
	b.set(entities.BoilerStatus, decode.Text(boilerStatus))
}

func (b *Bridge) portError(now time.Time, err error) {
	slog.Warn("serial port error, retrying", "err", err, "in", b.opt.PortRetry)
	b.link.Close()
	b.linkDown(now)
	b.nextPort = now.Add(b.opt.PortRetry)
}

func (b *Bridge) maybeReopen(now time.Time) {
	if b.linkDownSince.IsZero() || now.Sub(b.linkDownSince) < b.reopenWait {
		return
	}
	slog.Warn("boiler silent; reopening the serial port", "down", now.Sub(b.linkDownSince))
	b.link.Close()
	b.reconnects++
	b.set(entities.LinkReconnects, decode.Num(float64(b.reconnects)))
	b.linkDownSince = now
	b.reopenWait = min(2*b.reopenWait, b.opt.ReopenMax)
	b.nextPort = now.Add(2 * time.Second)
}

type readResult uint8

const (
	readOK        readResult = iota
	readNoReply              // the boiler did not answer
	readException            // the boiler answered with a Modbus exception
)

// readBlock adds the block to regs and returns how the read went. Port errors are returned.
func (b *Bridge) readBlock(blk Block, regs map[uint16]uint16) (readResult, error) {
	values, err := b.link.Read(blk.Start, blk.Count)
	var exc *modbus.ExceptionError
	switch {
	case err == nil:
	case errors.As(err, &exc):
		slog.Warn("read failed", "start", blk.Start, "count", blk.Count, "err", err)
		return readException, nil
	case errors.Is(err, modbus.ErrNoReply):
		return readNoReply, nil
	default:
		return readNoReply, err
	}
	for i, v := range values {
		regs[blk.Start+uint16(i)] = v
	}
	return readOK, nil
}

// readBlocks returns address -> raw and how the read went, stopping at the first block that fails.
// Port errors are returned.
func (b *Bridge) readBlocks(blocks []Block) (map[uint16]uint16, readResult, error) {
	regs := make(map[uint16]uint16)
	for _, blk := range blocks {
		if res, err := b.readBlock(blk, regs); res != readOK || err != nil {
			return nil, res, err
		}
	}
	return regs, readOK, nil
}

func (b *Bridge) pollFast(now time.Time) (bool, error) {
	regs, res, err := b.readBlocks(FastBlocks)
	if err != nil {
		return false, err
	}
	switch res {
	case readOK:
	case readNoReply:
		b.linkDown(now)
		b.nextFast = now.Add(b.opt.offlineWait()) // back off like the ESP
		b.maybeReopen(now)
		return false, nil
	case readException:
		// The boiler answered, so the link is up and reopening the port would not help. Only this
		// poll's readings are unusable.
		b.linkUp()
		b.clearFast("Unavailable")
		b.status = decode.None
		b.nextFast = now.Add(b.opt.offlineWait())
		return false, nil
	}
	b.linkUp()
	b.nextFast = now.Add(b.opt.FastInterval)

	v := map[string]decode.Value{}
	for _, e := range entities.InGroup(entities.Fast) {
		v[e.Key] = e.Decode(regs[e.Address])
	}
	b.status = v[entities.StatusCode]
	b.guard.NoteStatus(b.status, now)
	for _, e := range entities.InGroup(entities.Fast) {
		if e.Combine != "" {
			v[e.Key] = e.Combine.Apply(v[e.With], v[e.Key])
		}
	}
	v[entities.DeltaT] = decode.DeltaT(v[entities.OutletTemp], v[entities.InletTemp])
	v[entities.FlamePresent] = decode.FlamePresent(v[entities.FlameSignal])
	for _, g := range []entities.Group{entities.Fast, entities.Derived} {
		for _, e := range entities.InGroup(g) {
			b.set(e.Key, v[e.Key])
		}
	}
	b.set(entities.BoilerStatus, decode.Text(decode.BoilerStatus(true, b.status, v[entities.AntiCycle],
		v[entities.HeatDemand], v[entities.BurnerRequest], v[entities.BoilerPumpOutput])))
	return true, nil
}

// pollSlow reads each slow block on its own, so a block that answers with an exception only clears its
// own entities. No reply stops the poll and keeps every value, like the ESP's slow group.
func (b *Bridge) pollSlow(now time.Time) error {
	regs := make(map[uint16]uint16)
	var failed []Block
	for _, blk := range SlowBlocks {
		res, err := b.readBlock(blk, regs)
		if err != nil {
			return err
		}
		switch res {
		case readOK:
		case readNoReply:
			b.nextSlow = now.Add(b.opt.offlineWait())
			return nil
		case readException:
			failed = append(failed, blk)
		}
	}
	b.nextSlow = now.Add(b.opt.SlowInterval)
	if len(failed) == len(SlowBlocks) {
		b.nextSlow = now.Add(b.opt.offlineWait())
	}
	v := map[string]decode.Value{}
	for _, e := range entities.InGroup(entities.Slow) {
		switch {
		case !e.IsModbus():
			continue
		case inAny(failed, e):
			v[e.Key] = decode.None
		case e.Words == 2:
			v[e.Key] = decode.U32(regs[e.Address], regs[e.Address+1])
		default:
			v[e.Key] = e.Decode(regs[e.Address])
		}
		b.set(e.Key, v[e.Key])
	}
	if b.guard.CountersClean(b.status, now) {
		b.set(entities.FailedIgnitions, decode.FailedIgnitions(v[entities.IgnitionAttempts], v[entities.Ignitions]))
	} else {
		// Read again once the ignition has settled rather than a whole slow interval later.
		slog.Info("counters read during an ignition; Failed Ignitions held")
		b.nextSlow = now.Add(min(b.opt.SlowInterval, decode.IgnitionSettle))
	}
	return nil
}

// inAny reports whether any of the entity's registers is in one of the blocks.
func inAny(blocks []Block, e entities.Entity) bool {
	for _, blk := range blocks {
		for w := range e.Words {
			if blk.Covers(e.Address + uint16(w)) {
				return true
			}
		}
	}
	return false
}

// Step does whatever is due at now and returns how long to wait before the next Step.
func (b *Bridge) Step(now time.Time) time.Duration {
	if !b.link.IsOpen() {
		if now.Before(b.nextPort) {
			return b.nextPort.Sub(now)
		}
		if err := b.link.Open(); err != nil {
			b.portError(now, err)
			return b.opt.PortRetry
		}
		slog.Info("opened serial port", "path", b.link.Path())
		// Time the silence from now: the port was unusable until this open, so a reopen now would not help.
		if !b.linkDownSince.IsZero() {
			b.linkDownSince = now
		}
		b.nextFast = now
	}
	if !now.Before(b.nextFast) {
		ok, err := b.pollFast(now)
		if err == nil && ok && !now.Before(b.nextSlow) {
			err = b.pollSlow(now)
		}
		if err != nil {
			b.portError(now, err)
			return b.opt.PortRetry
		}
	}
	if !b.link.IsOpen() {
		return max(0, b.nextPort.Sub(now))
	}
	return max(0, b.nextFast.Sub(now))
}
