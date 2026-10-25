package bot

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/happyx-app/HappyX.Bot/internal/i18n"
)

// Every literal key the code looks up must be in en.json, or users would see the raw key.
func TestUsedKeysExist(t *testing.T) {
	call := regexp.MustCompile(`\.T\("([a-z_.0-9]+)"[,)]`)
	files, _ := filepath.Glob("*.go")
	more, _ := filepath.Glob("../report/*.go")
	en := i18n.Get(i18n.Default)
	for _, f := range append(files, more...) {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range call.FindAllStringSubmatch(string(src), -1) {
			if !en.Has(m[1]) {
				t.Errorf("%s: %q is not in en.json", f, m[1])
			}
		}
	}
}
