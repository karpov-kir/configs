package flavorconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "thing.conf")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A tracked default sits in the installed mount, never in the tree under review. A home that is not
// absolute is no mount, which the caller reads the same way as having no config at all.
func TestPathResolvesInsideTheMountAndIsEmptyWithoutOne(t *testing.T) {
	if got := Path("/home/someone", "idsd.conf"); got != "/home/someone/.kk-flavor/configs/idsd.conf" {
		t.Fatalf("got %q", got)
	}
	if got := Path("home", "idsd.conf"); got != "" {
		t.Fatalf("a relative home resolved to %q, which is a path inside whatever directory the caller stood in", got)
	}
}

// Absent is the one quiet outcome, and it is quiet for both shapes of nothing.
func TestAnAbsentConfigIsQuiet(t *testing.T) {
	for _, path := range []string{"", filepath.Join(t.TempDir(), "nothing.conf")} {
		settings, err := Read(path, []string{"key"})
		if err != nil || settings != nil {
			t.Fatalf("got %v %v for %q, want no settings and no error", settings, err, path)
		}
	}
}

func TestCommentsAndBlankLinesAreSkippedAndTheRestParses(t *testing.T) {
	settings, err := Read(write(t, "# why this number\n\n  alpha 1\nbeta /some/path\n"), []string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if settings["alpha"] != "1" || settings["beta"] != "/some/path" || len(settings) != 2 {
		t.Fatalf("got %v, want alpha=1 and beta=/some/path and nothing else", settings)
	}
}

// Present but unusable refuses, every way it can be unusable. A default quietly restored is
// indistinguishable from the config working.
func TestAnUnusableConfigRefusesRatherThanReadingAsAbsent(t *testing.T) {
	for _, c := range []struct{ name, content, says string }{
		{"a key this caller does not understand", "gamma 3\n", "does not understand"},
		{"a value carrying a space", "alpha one two\n", "does not understand"},
		{"the same key twice", "alpha 1\nalpha 2\n", "more than once"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := Read(write(t, c.content), []string{"alpha", "beta"})
			if err == nil {
				t.Fatal("an unusable config was read as usable")
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Fatalf("the refusal does not say %q: %v", c.says, err)
			}
		})
	}
}

// The refusal names the lines this caller would have accepted, so the human fixing it does not have
// to find the allowed set in the source.
func TestTheRefusalNamesTheSupportedLines(t *testing.T) {
	_, err := Read(write(t, "gamma 3\n"), []string{"alpha", "beta"})
	if err == nil || !strings.Contains(err.Error(), "`alpha <value>`, `beta <value>`") {
		t.Fatalf("the refusal does not name the supported lines: %v", err)
	}
}

func TestADirectoryWhereAFileBelongsRefuses(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "thing.conf")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(dir, []string{"alpha"}); err == nil {
		t.Fatal("a directory was read as a config file")
	}
}

// A link to a PERFECTLY GOOD file is refused too, and this is the case that needs saying: IsRegularFile
// follows the link, so without an explicit test the link passes and whoever can repoint it chooses what
// this reads on the next run. Only the final component is tested — the `~/.kk-flavor` mount is itself a
// symlink, and refusing that would refuse every installed machine.
func TestALinkToAGoodFileIsRefusedRatherThanFollowed(t *testing.T) {
	real := write(t, "alpha 1\n")
	link := filepath.Join(t.TempDir(), "thing.conf")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	settings, err := Read(link, []string{"alpha"})
	if err == nil {
		t.Fatalf("a symlinked config was followed and read as %v", settings)
	}
	if !strings.Contains(err.Error(), "symlink") || !strings.Contains(err.Error(), real) {
		t.Fatalf("the refusal does not name the link and its target: %v", err)
	}
}

// A dangling link is not absent: an existence test alone cannot see one, so without the symlink check
// a config whose target was moved away would restore the default in silence.
func TestADanglingLinkRefusesInsteadOfReadingAsAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thing.conf")
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), path); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path, []string{"alpha"}); err == nil {
		t.Fatal("a dangling link was read as no config at all")
	}
}

// The refusal echoes the offending line, and these files are hand-written: a line holding no newline
// is as long as the file, so an unbounded echo buries the refusal it belongs to in its own evidence.
func TestTheEchoedLineIsBounded(t *testing.T) {
	_, err := Read(write(t, strings.Repeat("x", 5000)+"\n"), []string{"alpha"})
	if err == nil {
		t.Fatal("a line this does not understand was accepted")
	}
	if len(err.Error()) > 400 {
		t.Fatalf("the refusal is %d bytes — the offending line was echoed whole", len(err.Error()))
	}
	if !strings.Contains(err.Error(), "does not understand") {
		t.Fatalf("the refusal lost its own words: %v", err)
	}
}
