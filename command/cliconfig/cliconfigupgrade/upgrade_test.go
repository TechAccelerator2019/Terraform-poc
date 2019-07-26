package cliconfigupgrade

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestUpgrade(t *testing.T) {
	dirs, err := ioutil.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}

	for _, info := range dirs {
		name := info.Name()
		dir := filepath.Join("testdata", name)
		beforeFilename := filepath.Join(dir, "input.tfrc")
		afterFilename := filepath.Join(dir, "want.tfrc")
		if info, err := os.Stat(beforeFilename); err != nil || info.IsDir() {
			continue
		}
		t.Run(name, func(t *testing.T) {
			input, err := ioutil.ReadFile(beforeFilename)
			if err != nil {
				t.Fatalf("failed to open %s: %s", beforeFilename, err)
			}
			want, err := ioutil.ReadFile(afterFilename)
			if err != nil {
				t.Fatalf("failed to open %s: %s", afterFilename, err)
			}

			got := UpgradeOldHCLConfig(input)
			if !bytes.Equal(got, want) {
				t.Errorf("wrong result\ngot:\n%s\n\nwant:\n%s", got, want)
			}
		})
	}
}
