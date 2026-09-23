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

func TestPathResolvesInsideTheMountAndIsEmptyWithoutOne(t *testing.T) {
	if got := Path("/home/someone", "idsd.conf"); got != "/home/someone/.kk-flavor/configs/idsd.conf" {
		t.Fatalf("got %q", got)
	}
	if got := Path("home", "idsd.conf"); got != "" {
		t.Fatalf("a relative home resolved to %q, which is a path inside whatever directory the caller stood in", got)
	}
}

// A binary in a checkout's `ai/tools/bin/` reads that checkout's configs, whatever the mount holds.
// That is the whole of why a worktree can change a config and gate it before it lands.
func TestABinaryInACheckoutReadsThatCheckoutsConfigs(t *testing.T) {
	root := t.TempDir()
	configs := filepath.Join(root, "ai", "kk-flavor", "configs")
	bin := filepath.Join(root, "ai", "tools", "bin")
	for _, dir := range []string{configs, bin} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	exe := filepath.Join(bin, "reader-judge")
	if err := os.WriteFile(exe, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	// The temp root may itself sit behind a symlink, as macOS's /var does, and the binary's path is
	// resolved before its checkout is read off it.
	real, err := filepath.EvalSymlinks(configs)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirFor(exe, "/home/someone"); got != real {
		t.Fatalf("got %q, want the checkout's own %q", got, real)
	}
}

// Anywhere else — a test build, a checkout that ships no configs — the mount under home answers.
func TestABinaryOutsideACheckoutFallsBackToTheMount(t *testing.T) {
	stray := filepath.Join(t.TempDir(), "tools", "bin")
	if err := os.MkdirAll(stray, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, exe := range []string{"", "/tmp/go-build1/b001/x.test", filepath.Join(stray, "x")} {
		if got := dirFor(exe, "/home/someone"); got != "/home/someone/.kk-flavor/configs" {
			t.Errorf("%q answered %q, want the mount", exe, got)
		}
	}
}

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

func TestADanglingLinkRefusesInsteadOfReadingAsAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thing.conf")
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), path); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path, []string{"alpha"}); err == nil {
		t.Fatal("a dangling link was read as no config at all")
	}
}

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
