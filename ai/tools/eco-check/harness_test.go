package ecocheck_test

// The fixture builders and the four assertions the cases in check_test.go are written against, ported
// one for one from `~/.claude/skills/kk-ecosystem/scripts/check-test.sh`. Each builder keeps the name
// and the meaning its shell counterpart had, because that suite is still the cross-check: a case here
// and the case of the same name there have to be the same case.
//
// Fixtures are built with os.MkdirAll and os.WriteFile rather than by shelling out — the forks were
// the whole cost of the shell suite, and the mutation harness above this one multiplies that cost by
// the length of its mutation list.

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	ecocheck "kk-flavor/tools/eco-check"
)

// The finding substrings the cases match on. `basenames` carries no other finding's text whole —
// direction.go's scan comment says why.
const (
	cites       = "shared layer cites into a lane"
	names       = "shared layer names a lane"
	basenames   = "shared layer reaches into a lane by basename"
	unchecked   = "basename not checked"
	neverRan    = "direction scan read no files"
	refused     = "import refused"
	uncounted   = "uncounted import"
	undelimited = "undelimited section citation"
	missingTest = "script names a missing test"
	noPosition  = "script declares no test position"
	notRegular  = "citation target is not a regular file"
)

// The lane fixture the citation and basename cases share cites its script by this path. It is a
// constant for the reason the shell version made it a variable: written inline in a file the checker
// scans, a cited path that does not resolve in the real checkout becomes a finding against the
// checkout itself. No scan reads a `.go` file, so nothing here is exposed that way — the constant
// stays because the two suites have to state the same path.
const laneScriptRef = "~/.claude/skills/kk-humanize/scripts/comment-density.sh"

// One case's tree. `home` empty means the case runs under the ambient HOME, exactly as the shell
// version's empty `check_home` did: a fixture root legitimately raises mount findings of its own, and
// several cases are written against a run that has them.
type fixture struct {
	t    *testing.T
	base string
	root string
	home string
}

// The `$base/r$N` of the shell version: `base` is the scratch directory a case may write outside the
// root into, `root` the tree under review.
func newRootWithoutFlavor(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	f := &fixture{t: t, base: base, root: base + "/r"}
	f.mkdirAll(f.root + "/skills")
	return f
}

func newRoot(t *testing.T) *fixture {
	t.Helper()
	f := newRootWithoutFlavor(t)
	f.mkdirAll(f.root + "/kk-flavor/standards")
	f.write(f.root+"/kk-flavor/inject.md", "# Flavor\n")
	return f
}

// The checker refuses to walk a symlinked `kk-flavor`, so a case writes into `$root/real-flavor` and
// never `$root/kk-flavor`.
func newRootWithSymlinkedFlavor(t *testing.T) *fixture {
	t.Helper()
	f := newRootWithoutFlavor(t)
	f.mkdirAll(f.root + "/real-flavor/standards")
	f.write(f.root+"/real-flavor/inject.md", "# Flavor\n")
	f.symlink(f.root+"/real-flavor", f.root+"/kk-flavor")
	return f
}

func (f *fixture) newHomeWithoutFlavorMount() {
	f.t.Helper()
	f.home = f.root + "/home"
	f.mkdirAll(f.home + "/.claude")
}

func (f *fixture) newHome() {
	f.t.Helper()
	f.newHomeWithoutFlavorMount()
	f.symlink(f.root+"/kk-flavor", f.home+"/.kk-flavor")
}

// A skill the bare-name half of the direction scan can resolve: it counts a name only when a skill
// answers to it.
func (f *fixture) newMountedSkill(name string) {
	f.t.Helper()
	f.mkdirAll(f.root + "/skills/" + name)
	f.write(f.root+"/skills/"+name+"/SKILL.md", "# "+name+"\n")
}

// The parent is created, so a case can place a script under `<skill>/scripts/` — the real layout —
// without the write failing and leaving the case asserting against a tree that has no script in it.
func (f *fixture) newScript(name, body string) {
	f.t.Helper()
	path := f.root + "/skills/" + name
	f.mkdirAll(dirOf(path))
	f.write(path, body+"\n")
	if err := os.Chmod(path, 0o755); err != nil {
		f.t.Fatalf("chmod %s: %v", path, err)
	}
}

// The lane fixture the citation and basename cases share: one mounted skill holding one script.
func (f *fixture) newLaneWithScript() {
	f.t.Helper()
	f.newMountedSkill("kk-humanize")
	f.newScript("kk-humanize/scripts/comment-density.sh", "true")
}

// A committed filename holding a newline — the shape every forgery case here turns on. Writing a
// plain name instead would satisfy the assertion while testing nothing, so a filesystem that refuses
// one stops the case rather than leaving it to pass on a fixture it never got.
func (f *fixture) newFileWithNewlineName(path, content, what string) {
	f.t.Helper()
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		f.t.Fatalf("this filesystem refused a newline in a filename — %s cannot run here: %v", what, err)
	}
}

// One line repeated past a scan's own bound — what every budget case here needs and nothing else does.
func (f *fixture) floodWithLine(target string, remaining int, line string) {
	f.t.Helper()
	var flood strings.Builder
	for range remaining {
		flood.WriteString(line + "\n")
	}
	f.appendTo(target, flood.String())
}

func (f *fixture) mkdirAll(dir string) {
	f.t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatalf("mkdir %s: %v", dir, err)
	}
}

// No parent is created here, deliberately: the shell version's `>` failed on a missing directory, and
// a builder that quietly creates one hides a fixture written against a tree it does not have.
func (f *fixture) write(path, content string) {
	f.t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatalf("write %s: %v", path, err)
	}
}

func (f *fixture) appendTo(path, content string) {
	f.t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		f.t.Fatalf("append %s: %v", path, err)
	}
	defer file.Close()
	if _, err := file.WriteString(content); err != nil {
		f.t.Fatalf("append %s: %v", path, err)
	}
}

func (f *fixture) symlink(target, link string) {
	f.t.Helper()
	if err := os.Symlink(target, link); err != nil {
		f.t.Fatalf("symlink %s -> %s: %v", link, target, err)
	}
}

func (f *fixture) chmod(path string, mode os.FileMode) {
	f.t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		f.t.Fatalf("chmod %s: %v", path, err)
	}
}

// One run of the checker over this fixture, stdout and stderr merged into one buffer the way the
// shell version's `2>&1` merged them — the ordering cases read line positions out of that stream.
//
// No case asserts on the exit code: a fixture root legitimately has findings of its own, so "clean"
// would pass every case for the wrong reason. Exit 2 is the exception, because then nothing was
// checked at all.
// A case is parallelised here rather than at each t.Run, because the one thing that decides it is the
// one thing this function knows: HOME is process-global, so a case that needs its own mount has to run
// alone, and every other case can run alongside the rest. Splitting the decision across every t.Run
// is what would let the two sets drift — and t.Parallel is called *after* the fixture is built, so a
// case pays only its scan in the parallel phase. A fixture that later grows a mount panics in
// t.Setenv instead of quietly racing one.
func (f *fixture) run() string {
	f.t.Helper()
	if f.home != "" {
		f.t.Setenv("HOME", f.home)
	} else {
		f.t.Parallel()
	}
	var output bytes.Buffer
	if status := ecocheck.Run(f.root, &output, &output); status == 2 {
		f.t.Fatalf("Run exited 2 — nothing was checked, so this case cannot be trusted\n%s", indent(output.String()))
	}
	return output.String()
}

// Both asserts take the whole output and a fixed substring, never a pattern: a match that a regexp
// would have swallowed turns a doesNotReport case into a silent pass.
func (f *fixture) reports(needle string) {
	f.t.Helper()
	output := f.run()
	if !strings.Contains(output, needle) {
		f.t.Errorf("expected a finding containing %q\n%s", needle, indent(output))
	}
}

func (f *fixture) doesNotReport(needle string) {
	f.t.Helper()
	output := f.run()
	if strings.Contains(output, needle) {
		f.t.Errorf("expected no finding containing %q\n%s", needle, indent(output))
	}
}

// The checker prints its budget lines before any finding, so a leak from a scan loop lands ahead of
// them.
func (f *fixture) reportedViaFindings(needle string) {
	f.t.Helper()
	output := f.run()
	budgetLine := firstLineWith(output, "always-loaded:")
	findingLine := firstLineWith(output, needle)
	if budgetLine < 0 || findingLine < 0 || findingLine <= budgetLine {
		f.t.Errorf("expected %q on a line after the always-loaded: budget (budget at %d, finding at %d)\n%s",
			needle, budgetLine, findingLine, indent(output))
	}
}

// Ordering, never presence: the per-class cap on its own puts a real finding on screen alongside a
// flood, so asserting the real one is merely *present* passes and observes nothing. What this decides
// is which of the two lands first.
func (f *fixture) ranksAbove(above, below string) {
	f.t.Helper()
	output := f.run()
	high := firstLineWith(output, above)
	low := firstLineWith(output, below)
	if high < 0 || low < 0 || high >= low {
		f.t.Errorf("expected %q above %q (at %d and %d)\n%s", above, below, high, low, indent(output))
	}
}

// The number of output lines that start with the given prefix — the `grep -c '^…'` one case counts a
// forged finding line with.
func (f *fixture) countLinesStartingWith(prefix string) (int, string) {
	f.t.Helper()
	output := f.run()
	count := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, prefix) {
			count++
		}
	}
	return count, output
}

// The index of the first line holding the substring, or -1 — `grep -nF -m1 | cut -d: -f1`.
func firstLineWith(output, needle string) int {
	for i, line := range strings.Split(output, "\n") {
		if strings.Contains(line, needle) {
			return i
		}
	}
	return -1
}

func dirOf(path string) string {
	if i := strings.LastIndexByte(path, '/'); i > 0 {
		return path[:i]
	}
	return "."
}

func indent(text string) string {
	var out strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		fmt.Fprintf(&out, "          %s\n", line)
	}
	return out.String()
}
