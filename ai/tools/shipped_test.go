package tools_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// `go build -o <dir>/ ./...` names each binary after the directory its main package sits in, so two
// mains in directories of the same name write to one path and the last one wins — a green build
// carrying fewer tools than the module holds. That is how `<tool>/cmd/` laid out three tools as one
// binary called `cmd`. Nothing else notices: resolve.sh names every `-o` explicitly, so it stayed
// correct while the plain command went two tools short.
func TestNoTwoMainPackagesBuildToTheSameName(t *testing.T) {
	dirs := mainPackageDirs(t)
	if len(dirs) == 0 {
		t.Fatal("found no main packages, so this case would pass against any layout at all")
	}

	byName := map[string][]string{}
	for _, dir := range dirs {
		name := filepath.Base(dir)
		byName[name] = append(byName[name], dir)
	}
	for name, sharing := range byName {
		if len(sharing) > 1 {
			t.Errorf("%s all build to a binary called %q, so `go build -o <dir>/ ./...` writes %d of them "+
				"to one path and keeps the last — rename them apart, one directory per tool",
				strings.Join(sharing, ", "), name, len(sharing))
		}
	}
}
