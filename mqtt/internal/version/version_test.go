package version_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/version"
)

// release-please bumps these together (the "mqtt" component); a mismatch means one was edited by hand.
func TestVersionsMatchManifest(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", ".release-please-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]string
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	want := manifest["mqtt"]
	if version.Version != want {
		t.Errorf("version.Version = %s, manifest mqtt = %s", version.Version, want)
	}
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	refs := regexp.MustCompile(`lochinvar-native-modbus:(\S+)\s*# x-release-please-version`).FindAllSubmatch(readme, -1)
	if len(refs) == 0 {
		t.Fatal("no image refs marked x-release-please-version in mqtt/README.md")
	}
	for _, r := range refs {
		if string(r[1]) != want {
			t.Errorf("mqtt/README.md image tag %s, manifest mqtt = %s", r[1], want)
		}
	}
}
