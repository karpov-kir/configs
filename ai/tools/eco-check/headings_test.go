package ecocheck

// Asserted on map identity rather than on elapsed time: a second parse produces an equal map, so
// equality cannot tell a memo from a re-parse, and a clock makes the case a coin flip on a loaded
// machine.

import (
	"os"
	"reflect"
	"testing"
)

func newCheckerOverTree(t *testing.T, body string) (*checker, string) {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{root + "/kk-flavor/standards", root + "/skills"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	path := root + "/kk-flavor/standards/target.md"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	c, ok := newChecker(root)
	if !ok {
		t.Fatal("the fixture is not a checkout")
	}
	return c, path
}

func TestAMarkdownFileIsParsedOncePerRun(t *testing.T) {
	c, path := newCheckerOverTree(t, "# T\n\n## Alpha\n\nan **Emphatic run** here\n")

	t.Run("headings", func(t *testing.T) {
		first := c.markdownHeadings(path)
		if _, ok := first["alpha"]; !ok {
			t.Fatalf("the fixture reached no heading, so this case observes nothing: %v", first)
		}
		if second := c.markdownHeadings(path); !sameMap(first, second) {
			t.Error("the target was parsed a second time")
		}
	})

	t.Run("bolded runs", func(t *testing.T) {
		first := c.boldedRuns(path)
		if _, ok := first["emphatic run"]; !ok {
			t.Fatalf("the fixture reached no bolded run, so this case observes nothing: %v", first)
		}
		if second := c.boldedRuns(path); !sameMap(first, second) {
			t.Error("the target was parsed a second time")
		}
	})

	t.Run("and a second file gets its own", func(t *testing.T) {
		other := t.TempDir() + "/other.md"
		if err := os.WriteFile(other, []byte("# O\n\n## Beta\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, ok := c.markdownHeadings(other)["beta"]; !ok {
			t.Error("a second path was answered from the first path's memo")
		}
	})
}

func sameMap(a, b map[string]string) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}
