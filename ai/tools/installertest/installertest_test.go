package installertest_test

// The two pieces of this package a suite can be wrong about without any case of its own going red.

// Every other method here is a writer or an Expect, and a bug in one fails the suite that drove it.
// A bound taken from the wrong directory, and a query that answers the same whatever is on disk, both
// stay quiet: the writes land, the assertions pass, and the case reports on a tree nobody checked.

// The containment refusal itself has no case here. It calls t.Fatalf on the case's own *testing.T, so
// a case driving it would be failing itself. Its negative control is hand-run, and
// ~/.kk-flavor/standards/core-principles.md → 5 bounds it to that one run: pointing a fixture write
// outside the base made every case using it fail, naming the guard rather than a result.

import (
	"os"
	"path/filepath"
	"testing"

	"configs/ai/tools/installertest"
)

// A suite whose fixture root exists before the tree does hands that root in. The bound has to become
// that directory, since the whole point is writing into a tree the caller laid out. A New here would
// bound the writes to a temp directory of its own, and every write the caller meant would be refused.
func TestATreeTakesTheDirectoryItWasGivenAsItsBound(t *testing.T) {
	t.Parallel()
	chosen := filepath.Join(t.TempDir(), "laid-out-first")
	if err := os.MkdirAll(chosen, 0o755); err != nil {
		t.Fatalf("building the fixture root: %v", err)
	}

	tree := installertest.NewIn(t, chosen)

	// Physically, the way every bound in this package is resolved: /var is a symlink to /private/var on
	// macOS, and a bound left unresolved refuses every write made through a resolved path.
	if want := installertest.Physical(t, chosen); tree.Base() != want {
		t.Errorf("the tree bound itself to %s, wanted the directory it was given, %s", tree.Base(), want)
	}
	// And the bound is live, not just reported: a write inside the chosen directory goes through.
	tree.Write(filepath.Join(chosen, "inside/file.txt"), "body")
	tree.ExpectFileBody(filepath.Join(chosen, "inside/file.txt"), "body")
}

// The queries answer rather than failing the case, so the table drives every shape at once: a case
// that fataled on one row would never reach the next. The rows are the full set these three separate,
// and each pair of them disagrees somewhere — an empty file from a missing one, a directory from a
// file, a link that resolves from one that does not.
func TestTheQueriesAnswerForEveryShapeOnDisk(t *testing.T) {
	t.Parallel()
	tree := installertest.New(t)
	base := tree.Base()
	tree.Write(base+"/file.txt", "body")
	tree.Write(base+"/empty.txt", "")
	tree.MkdirAll(base + "/directory")
	tree.Symlink(base+"/file.txt", base+"/link-to-file")
	tree.Symlink(base+"/nothing-is-here", base+"/dangling-link")

	for _, c := range []struct {
		name     string
		path     string
		body     string
		wasThere bool
		exists   bool
		isFile   bool
	}{
		{name: "a file", path: "/file.txt", body: "body", wasThere: true, exists: true, isFile: true},
		// The row the second return value exists for. "" alone cannot say which of these two it is.
		{name: "an empty file", path: "/empty.txt", body: "", wasThere: true, exists: true, isFile: true},
		{name: "nothing at all", path: "/missing.txt", body: "", wasThere: false},
		{name: "a directory", path: "/directory", exists: true},
		// Stat follows it, so what the name holds is a file however it was reached.
		{name: "a link to a file", path: "/link-to-file", body: "body", wasThere: true, exists: true, isFile: true},
		// Lstat does not, so the entry is there while nothing readable is.
		{name: "a link resolving nowhere", path: "/dangling-link", exists: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			path := base + c.path
			body, wasThere := tree.Body(path)
			if body != c.body || wasThere != c.wasThere {
				t.Errorf("Body(%s) = %q, %v, wanted %q, %v", c.name, body, wasThere, c.body, c.wasThere)
			}
			if got := tree.Exists(path); got != c.exists {
				t.Errorf("Exists(%s) = %v, wanted %v", c.name, got, c.exists)
			}
			if got := tree.IsFile(path); got != c.isFile {
				t.Errorf("IsFile(%s) = %v, wanted %v", c.name, got, c.isFile)
			}
		})
	}
}
