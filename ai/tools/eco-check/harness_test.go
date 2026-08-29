package ecocheck_test

// The fixture builders and the assertions the case files beside it are written against. This is the
// only suite over these scans, so a case removed here is coverage gone, not coverage moved.
//
// Fixtures are built with os.MkdirAll and os.WriteFile, never by shelling out: a mutation harness
// multiplies every fork by the length of its mutation list. `ai/tools/go-mutate` is what shows a case
// here can fail.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
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
	welded      = "script names an ambiguous test"
	noPosition  = "script declares no test position"
	notRegular  = "citation target is not a regular file"
	bareRule    = "bare rule-ID citation"
	dangling    = "dangling section ref"
	unresolved  = "unresolvable citation path"
	uncheckable = "uncheckable citation"
)

// The lane fixture the citation and basename cases share cites its script by this path. Written
// inline in a file the checker scans, a cited path that does not resolve in the real checkout becomes
// a finding against the checkout itself. No scan reads a `.go` file, so nothing here is exposed that
// way — the constant stays because the two suites have to state the same path.
const laneScriptRef = "~/.claude/skills/kk-humanize/scripts/comment-density.sh"

// The checker's own per-file read bound, restated here because the package under test does not export
// it. A fixture one byte past it is what every oversize case is built on.
const maxReadBytes = 8 << 20

// One case's tree. `home` empty means the case runs under the ambient HOME: a fixture root
// legitimately raises mount findings of its own, and several cases are written against a run that has
// them.
type fixture struct {
	t    *testing.T
	base string
	root string
	home string
}

// `base` is the scratch directory a case may write outside the root into; `root` is the tree under
// review.
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
	f.mkdirAll(filepath.Dir(path))
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

// Two mounted lanes, one of them holding a script the other does not. Both halves of the basename
// scan's uniqueness gate are then live on one tree: `SKILL.md` is a name two lanes carry, and
// `comment-density.sh` is a name one lane carries. A tree with only the first has no unique lane
// basename at all, and the scan the case is written against never runs.
func newTwoLaneTree(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newLaneWithScript()
	f.newMountedSkill("kk-drive")
	return f
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

// One markdown link per line, numbered — the flood a ranking case buries its needle under. The format
// takes the line's number, so a case can write a target that forges a finding of its own.
func (f *fixture) floodWithLinks(target string, count int, format string) {
	f.t.Helper()
	var flood strings.Builder
	for i := 1; i <= count; i++ {
		fmt.Fprintf(&flood, format+"\n", i)
	}
	f.write(target, flood.String())
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

// A file one byte past the read bound, made of newlines because that is the cheap shape: a branch
// commits almost nothing and the checker allocates a slice header per line.
func (f *fixture) writeOversize(path string) {
	f.t.Helper()
	body := make([]byte, maxReadBytes+1)
	for i := range body {
		body[i] = '\n'
	}
	f.write(path, string(body))
}

func (f *fixture) mkdirAll(dir string) {
	f.t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatalf("mkdir %s: %v", dir, err)
	}
}

// No parent is created here, deliberately: a builder that quietly creates one hides a fixture written
// against a tree it does not have.
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

// True when a mode of 000 actually stops this process reading. Probed rather than compared against
// uid 0: root is the common case, but CAP_DAC_OVERRIDE without root and a filesystem that does not
// carry the bit behave the same way, and all three make a mode-000 fixture a file the tool reads
// happily. ecostats' suite carries the same probe for the same reason; neither package can import the
// other's test helpers.
func modeDeniesRead(t *testing.T) bool {
	t.Helper()
	probe := t.TempDir() + "/probe"
	if err := os.WriteFile(probe, []byte("alpha\n"), 0o644); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	if err := os.Chmod(probe, 0o000); err != nil {
		t.Fatalf("chmod probe: %v", err)
	}
	file, err := os.Open(probe)
	if err != nil {
		return true
	}
	file.Close()
	return false
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
func (f *fixture) run() string {
	f.t.Helper()
	f.isolate()
	return f.check()
}

// The same tree checked twice in one process, and the second check's output. Nothing the checker
// carries between runs is meant to change what it reports, and the `bash -n` memo is the one piece
// held across them. A case about that memo cannot be written against a single run: within one run the
// parse workers reach both copies of a script before either has stored anything.
func (f *fixture) runTwice() string {
	f.t.Helper()
	f.isolate()
	f.check()
	return f.check()
}

// HOME is process-global, so a case that needs its own mount has to run alone and every other case
// can run alongside the rest. The decision is made here, once per case, never at each t.Run. Split
// across every case, the two sets would drift — and a case running twice may call neither t.Setenv
// nor t.Parallel a second time.
//
// Called after the fixture is built, so a case pays only its scan in the parallel phase. A fixture
// that later grows a mount panics in t.Setenv rather than quietly racing one.
func (f *fixture) isolate() {
	f.t.Helper()
	if f.home != "" {
		f.t.Setenv("HOME", f.home)
		return
	}
	f.t.Parallel()
}

func (f *fixture) check() string {
	f.t.Helper()
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

// The same two assertions against a second check of the same tree — what the run before it left
// behind must not change what this one says.
func (f *fixture) reportsOnASecondRun(needle string) {
	f.t.Helper()
	output := f.runTwice()
	if !strings.Contains(output, needle) {
		f.t.Errorf("expected a second run to still report %q\n%s", needle, indent(output))
	}
}

func (f *fixture) doesNotReportOnASecondRun(needle string) {
	f.t.Helper()
	output := f.runTwice()
	if strings.Contains(output, needle) {
		f.t.Errorf("expected a second run to still report nothing containing %q\n%s", needle, indent(output))
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

func indent(text string) string {
	var out strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		fmt.Fprintf(&out, "          %s\n", line)
	}
	return out.String()
}
