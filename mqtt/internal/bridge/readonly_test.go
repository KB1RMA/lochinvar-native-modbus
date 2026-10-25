package bridge_test

import (
	"bytes"
	"encoding/binary"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/bridge"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/modbus"
)

// READ-ONLY: the bridge must never send anything but function 03 to the boiler.

func TestEveryFrameIsAFunction03Read(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.bridge.Step(at(0))
	f.boiler.SetSilent(true)
	for s := 5.0; s < 400; s += 5 { // through a link loss and a reopen
		f.bridge.Step(at(s))
	}
	f.boiler.SetSilent(false)
	f.bridge.Step(at(400))

	allowed := map[bridge.Block]bool{}
	for _, b := range append(append([]bridge.Block{}, bridge.FastBlocks...), bridge.SlowBlocks...) {
		allowed[b] = true
	}
	frames := f.boiler.Written()
	if len(frames) == 0 {
		t.Fatal("nothing written")
	}
	for _, fr := range frames {
		if len(fr) != 8 || fr[0] != 1 || fr[1] != modbus.FnReadHolding {
			t.Fatalf("frame %X is not a function 03 read from slave 1", fr)
		}
		if crc := modbus.CRC16(fr[:6]); !bytes.Equal(fr[6:], crc[:]) {
			t.Fatalf("frame %X has a bad CRC", fr)
		}
		blk := bridge.Block{Start: binary.BigEndian.Uint16(fr[2:]), Count: binary.BigEndian.Uint16(fr[4:])}
		if !allowed[blk] {
			t.Fatalf("frame %X reads %v, not one of the known blocks", fr, blk)
		}
	}
}

// sources returns every non-test Go file of the module.
func sources(t *testing.T) map[string]*ast.File {
	t.Helper()
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		files[filepath.ToSlash(rel)] = f
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestOnlyTheModbusLinkWritesToThePort(t *testing.T) {
	t.Parallel()
	writes := map[string]int{}
	for path, f := range sources(t) {
		if strings.HasPrefix(path, "internal/modbustest/") {
			continue // the fake boiler implements Write
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Write" {
					writes[path]++
				}
			}
			return true
		})
	}
	if len(writes) != 1 || writes["internal/modbus/link.go"] != 1 {
		t.Fatalf("Write calls %v, want exactly one, in internal/modbus/link.go", writes)
	}
}

// The Write check above only sees calls named Write. Writing to the port by other means needs the port
// itself, so only internal/modbus may open it, and that package may not use the helpers that write to
// an io.Writer.
func TestOnlyTheModbusPackageReachesThePort(t *testing.T) {
	t.Parallel()
	writers := map[string]bool{
		"fmt.Fprint": true, "fmt.Fprintf": true, "fmt.Fprintln": true,
		"io.WriteString": true, "io.Copy": true, "io.CopyN": true, "io.CopyBuffer": true,
	}
	for path, f := range sources(t) {
		inModbus := strings.HasPrefix(path, "internal/modbus/")
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if p == "go.bug.st/serial" && !inModbus {
				t.Errorf("%s imports %s; only internal/modbus may open the port", path, p)
			}
			if p == "bufio" && inModbus {
				t.Errorf("%s imports bufio; write to the port only in link.go", path)
			}
		}
		if !inModbus {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if pkg, ok := sel.X.(*ast.Ident); ok && writers[pkg.Name+"."+sel.Sel.Name] {
						t.Errorf("%s calls %s.%s; write to the port only in link.go", path, pkg.Name, sel.Sel.Name)
					}
				}
			}
			return true
		})
	}
}

func TestOnlyFunction03Defined(t *testing.T) {
	t.Parallel()
	for path, f := range sources(t) {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				for _, name := range spec.(*ast.ValueSpec).Names { //nolint:forcetypeassert // const specs are ValueSpecs
					if strings.HasPrefix(name.Name, "Fn") && name.Name != "FnReadHolding" {
						t.Errorf("%s defines function code %s", path, name.Name)
					}
				}
			}
		}
	}
	if modbus.FnReadHolding != 0x03 {
		t.Fatalf("FnReadHolding = %#x", modbus.FnReadHolding)
	}
}

func TestNoModbusLibrary(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if m := regexp.MustCompile(`(?i)\S*modbus\S*`).FindAll(data, -1); len(m) != 1 { // just the module path
		t.Fatalf("go.mod mentions modbus %d times (%q): no Modbus library allowed", len(m), m)
	}
}
