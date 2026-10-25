package bridge_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/bridge"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/entities"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/modbus"
)

// Every register the bridge decodes is in registers/controller.yaml, and the blocks it reads cover them.

type mapEntry struct {
	Name       string    `yaml:"name"`
	Address    yaml.Node `yaml:"address"` // an int or a list of ints
	Mirrors    []uint16  `yaml:"mirrors"`
	Words      int       `yaml:"words"`
	Confidence string    `yaml:"confidence"`
}

type registerMap struct {
	Link struct {
		Baud         int `yaml:"baud"`
		SlaveID      int `yaml:"slave_id"`
		ReadFunction int `yaml:"read_function"`
	} `yaml:"link"`
	Live     []mapEntry `yaml:"live"`
	Status   []mapEntry `yaml:"status"`
	Counters []mapEntry `yaml:"counters"`
	Settings []mapEntry `yaml:"settings"`
	Info     []mapEntry `yaml:"info"`
}

func loadMap(t *testing.T) (registerMap, map[uint16]mapEntry) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "registers", "controller.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var m registerMap
	if err := yaml.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	known := map[uint16]mapEntry{}
	for _, section := range [][]mapEntry{m.Live, m.Status, m.Counters, m.Settings, m.Info} {
		for _, e := range section {
			var addrs []uint16
			if e.Address.Kind == yaml.SequenceNode {
				if err := e.Address.Decode(&addrs); err != nil {
					t.Fatalf("%s: %v", e.Name, err)
				}
			} else {
				var a uint16
				if err := e.Address.Decode(&a); err != nil {
					t.Fatalf("%s: %v", e.Name, err)
				}
				addrs = []uint16{a}
			}
			if e.Words == 0 {
				e.Words = 1
			}
			for _, a := range append(addrs, e.Mirrors...) {
				known[a] = e
			}
		}
	}
	return m, known
}

func TestAddressesInMap(t *testing.T) {
	t.Parallel()
	_, known := loadMap(t)
	for _, e := range entities.All {
		if !e.IsModbus() {
			continue
		}
		entry, ok := known[e.Address]
		if !ok {
			t.Errorf("%s: address %d not in registers/controller.yaml", e.Name, e.Address)
			continue
		}
		if entry.Words != e.Words {
			t.Errorf("%s: %d words, map says %d", e.Name, e.Words, entry.Words)
		}
		if entry.Confidence == "unknown" && !e.DisabledByDefault {
			t.Errorf("%s: unknown register %d must be disabled by default", e.Name, e.Address)
		}
	}
}

func covered(blocks []bridge.Block) map[uint16]bool {
	out := map[uint16]bool{}
	for _, b := range blocks {
		for a := b.Start; a < b.Start+b.Count; a++ {
			out[a] = true
		}
	}
	return out
}

func TestBlocksCoverEntitiesAndAvoidUnmappedRanges(t *testing.T) {
	t.Parallel()
	fast, slow := covered(bridge.FastBlocks), covered(bridge.SlowBlocks)
	for _, e := range entities.InGroup(entities.Fast) {
		if !fast[e.Address] {
			t.Errorf("%s: address %d not in a fast block", e.Name, e.Address)
		}
	}
	for _, e := range entities.InGroup(entities.Slow) {
		for w := range e.Words {
			if a := e.Address + uint16(w); e.IsModbus() && !slow[a] {
				t.Errorf("%s: address %d not in a slow block", e.Name, a)
			}
		}
	}
	for _, b := range append(append([]bridge.Block{}, bridge.FastBlocks...), bridge.SlowBlocks...) {
		if b.Count < 1 || b.Count > modbus.MaxBlock {
			t.Errorf("block %v: count out of range", b)
		}
		for a := range covered([]bridge.Block{b}) {
			if (a >= 956 && a <= 1005) || a >= 1406 { // exception 2
				t.Errorf("block %v reads unmapped address %d", b, a)
			}
		}
	}
}

func TestLinkSettingsMatchMap(t *testing.T) {
	t.Parallel()
	m, _ := loadMap(t)
	if m.Link.Baud != 19200 || m.Link.SlaveID != 1 || m.Link.ReadFunction != int(modbus.FnReadHolding) {
		t.Errorf("map link settings %+v, bridge uses 19200 baud, slave 1, function 3", m.Link)
	}
}
