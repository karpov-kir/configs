package ecocheck_test

// The fixture builders and the assertions the case files beside it are written against. This is the
// only suite over these scans, so a case removed here is coverage gone, not coverage moved.
//
// Fixtures are built with os.MkdirAll and os.WriteFile. A forked process costs about 100ms on the
// machine these were written on, and a file write is too cheap to measure.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ecocheck "configs/ai/tools/eco-check"
	"configs/ai/tools/repo"
	"configs/ai/tools/repo/repotest"
	"configs/ai/tools/shell"
)

// The finding substrings the cases match on. Each head is bound to the constant the emit site and
// report.go's rank table both lead with, so a reworded kind stops compiling here rather than leaving a
// case matching text nothing prints. `uncounted`, `unfamiliedSkill` and `unclaimedRouter` are tails
// rather than heads, and stay written out. `basenames` carries no other finding's text whole —
// direction.go's scan comment says why.
const (
	cites           = ecocheck.SharedLayerCitesLane
	names           = ecocheck.SharedLayerNamesLane
	basenames       = ecocheck.SharedLayerReachesLaneByBasename
	unchecked       = ecocheck.BasenameNotChecked
	neverRan        = ecocheck.DirectionScanReadNoFiles
	refused         = ecocheck.ImportRefused
	uncounted       = "uncounted import"
	undelimited     = ecocheck.UndelimitedSectionCitation
	missingTest     = ecocheck.ScriptNamesMissingTest
	welded          = ecocheck.ScriptNamesAmbiguousTest
	noPosition      = ecocheck.ScriptDeclaresNoTestPosition
	notRegular      = ecocheck.CitationTargetNotRegular
	bareRule        = ecocheck.BareRuleIDCitation
	dangling        = ecocheck.DanglingSectionRef
	unresolved      = ecocheck.UnresolvableCitationPath
	uncheckable     = ecocheck.UncheckableCitation
	crossFamily     = ecocheck.AnyRepoNamesWorkflowFamily
	unfamiliedSkill = "is in neither declared family"
	unclaimedRouter = "claims no exception"
)

// What a file under `standards/` opens with when its case is not about the layering. Every such file
// is judged for a `**Layer:**` line, so one written without it raises a finding of a kind the case
// never asked about — which the counting cases next door then count.
const layerLine = "**Layer:** base\n\n"

// The lane fixture the citation and basename cases share cites its script by this path. It is a
// constant because those two case files have to state the same path. Written inline in a file the
// checker scans, a cited path that does not resolve in the real checkout becomes a finding against the
// checkout itself — but no scan reads a `.go` file, so nothing here is exposed that way.
const laneScriptRef = "~/.kk-flavor/skills/kk-humanize/scripts/voice-check.sh"

// Aliased, never written out: a restated copy goes on compiling after the bound moves, and every
// oversize fixture built one byte past it then measures nothing.
const maxReadBytes = shell.MaxFileBytes

// One case's tree. `home` empty means the case runs under the ambient HOME: a fixture root
// legitimately raises mount findings of its own, and several cases are written against a run that has
// them.
type fixture struct {
	t    *testing.T
	base string
	root string
	home string
	// The repository --gate puts its one question to. Every fixture carries one, so a case reaching for
	// the flag builds no repository of its own. A bare run leaves it untouched.
	git *repotest.Fake

	// The bash the parse scan reads its answers off. Every fixture carries one, so a script the case
	// wrote for the tree scan to walk costs it no fork.
	bash *ecocheck.FakeBash
}

// The least tree ecoroot.New accepts — `kk-flavor/` with `skills/` inside it, and nothing else.
// Bare rather than without-flavor: skills/ moved inside kk-flavor/, so every root the checker will
// even look at now has that directory, and a fixture genuinely lacking it takes Run to exit 2, which
// checkWith refuses. What `newRoot` adds on top is the standards tree and the router.
//
// `base` is the scratch directory a case may write outside the root into; `root` is the tree under
// review.
func newBareRoot(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t, newBase(t))
	f.mkdirAll(f.root + "/kk-flavor/skills")
	return f
}

// report.go cuts EVERY finding line at lineWidthCap, the width const, before printing it, and most of
// these cases assert a finding that quotes a fixture path. Such a case reads the root's length as much
// as the code's. What the root spends, the case's own content cannot.

// `t.TempDir()` makes that length ambient: about 140 bytes on a macOS runner
// (`/var/folders/<two>/<28 random>/T/<the test's own name>/001`) against around 60 on Linux. A case
// then passes on one machine and fails on another with no other change. Three did on the first macOS
// leg this repository ran, and three more went the same way under a temp path longer still.

// A short base under `/tmp` leaves the whole of every bound for the case's own content to spend.
// TMPDIR is exactly what is too long here. A case that wants a root long enough to spend a bound grows
// one itself. TestTheGateRefusalStillNamesGitsReasonUnderALongRoot pads until the root outruns the cap
// whatever this machine's temp path costs.

// TestARefusalCarriesNoControlBytesFromTheRootItEchoes goes the other way and runs from inside the
// parent so the name it echoes fits.

// The scratch directory every fixture below is built under, 14 to 16 bytes wherever it runs: `/tmp/e`
// plus the eight-to-ten digit run os.MkdirTemp appends. This suite fixes that length, and the machine
// does not choose it. The directory is removed on the way out, like t.TempDir's own.
func newBase(t *testing.T) string {
	t.Helper()
	base, err := os.MkdirTemp("/tmp", "e")
	if err != nil {
		t.Fatalf("building a fixture root: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	return base
}

// What a fixture root may spend of a bounded finding before the case writes a byte. newBase, the test
// helper, builds a base of 14 to 16 bytes, and every fixture puts `/r` on the end of it. That leaves
// room to rename the prefix and none to go back to a path the machine picked. `t.TempDir()` costs
// upwards of 35 bytes on the shortest Linux runner and about 160 on a macOS one.
const maxFixtureRootBytes = 24

// A comment on each affected fixture only helps the author who reads it, so the property is held as a
// case here. A helper reaching back for `t.TempDir()` shows up as a handful of unrelated cases going
// red on one runner and green on another. That is how this class of defect was found, and it cost a CI
// leg.

// The bound is checked under a LONG TMPDIR as well as the ambient one, and that second leg is the
// point. A root read once says little about whether the machine chose its length, and this machine's
// temp path is short enough to hide the defect.

// The two legs are compared on length and never on sameness. os.MkdirTemp appends a run of eight to
// ten digits, so a root wobbles by two bytes inside a 24-byte budget, and no case here reads that
// wobble.

// newBase, the test helper, exists for this property.
func TestAFixtureRootIsTheSuitesToSpendAndNotTheMachines(t *testing.T) {
	// t.TempDir creates ONE directory per test and numbers the rest inside it. The helper newBase,
	// reaching for it, would answer out of a tree already pinned to the ambient TMPDIR, and the moved
	// TMPDIR would go unread. That second leg is what this case is for, hence os.MkdirTemp here.
	long, err := os.MkdirTemp("/tmp", strings.Repeat("d", 120))
	if err != nil {
		t.Fatalf("building the long temp path this case moves TMPDIR to: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(long) })

	for _, leg := range []struct{ what, tmpdir string }{
		{"under a TMPDIR as long as a macOS runner's", long},
		{"under a short TMPDIR", "/tmp"},
	} {
		t.Setenv("TMPDIR", leg.tmpdir)
		if root := newRoot(t).root; len(root) > maxFixtureRootBytes {
			t.Errorf("a fixture root %s is %d bytes, past the %d this suite allows itself — that much of "+
				"every bounded finding is spent before the case writes anything: %s",
				leg.what, len(root), maxFixtureRootBytes, root)
		}
	}
}

func newFixture(t *testing.T, base string) *fixture {
	t.Helper()
	root := base + "/r"
	return &fixture{t: t, base: base, root: root, git: repotest.New(root), bash: newFakeBash(t)}
}

// Two binaries, under names only this case uses. A script is parsed once per binary, and a single name
// leaves the per-binary loop unexercised. scripts.go keys its memo on the binary and holds it for the
// process, so a name two cases share lets one case's parses answer the other's.
func newFakeBash(t *testing.T) *ecocheck.FakeBash {
	t.Helper()
	return ecocheck.NewFakeBash(t.Name()+"/bash-5", t.Name()+"/bash-3.2")
}

// What a case hands a run it expects to refuse before any scan. The refusal path asks a repository or
// a bash for no answers. A run that reached for one would panic here, since the case arranged none.
var (
	noRepository repo.Git
	noBash       ecocheck.Bash
)

func newRoot(t *testing.T) *fixture {
	t.Helper()
	f := newBareRoot(t)
	f.mkdirAll(f.root + "/kk-flavor/standards")
	f.write(f.root+"/kk-flavor/inject.md", "# Flavor\n")
	return f
}

// The checker refuses to walk a symlinked `kk-flavor`, so a case writes into `$root/real-flavor` and
// never `$root/kk-flavor`.
//
// It builds its own root rather than starting from newBareRoot: skills/ lives inside
// kk-flavor/ now, so that helper has to create kk-flavor as a real directory, and the symlink this
// case is about can no longer be made over it. The whole tree therefore goes behind real-flavor,
// which is what the symlink then points at.
func newRootWithSymlinkedFlavor(t *testing.T) *fixture {
	t.Helper()
	base := newBase(t)
	f := newFixture(t, base)
	f.mkdirAll(f.root + "/real-flavor/standards")
	f.mkdirAll(f.root + "/real-flavor/skills")
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
	f.mkdirAll(f.root + "/kk-flavor/skills/" + name)
	f.write(f.root+"/kk-flavor/skills/"+name+"/SKILL.md", "# "+name+"\n")
}

// The parent is created, so a case can place a script under `<skill>/scripts/` — the real layout —
// without the write failing and leaving the case asserting against a tree that has no script in it.
func (f *fixture) newScript(name, body string) {
	f.t.Helper()
	path := f.root + "/kk-flavor/skills/" + name
	f.mkdirAll(filepath.Dir(path))
	f.write(path, body+"\n")
	if err := os.Chmod(path, 0o755); err != nil {
		f.t.Fatalf("chmod %s: %v", path, err)
	}
}

// A script that does not parse, and the lines `bash -n` refuses one with — each led by the script's own
// path, and each becoming a finding of its own. The table stands in for a fork here, since the parse
// scan is what forks and TestTheParseScanRunsARealBash is the only case that lets it.
func (f *fixture) newUnparsableScript(name, body string, complaints ...string) {
	f.t.Helper()
	f.newScript(name, body)
	f.bash.Refuse(body+"\n", complaints...)
}

func (f *fixture) newLaneWithScript() {
	f.t.Helper()
	f.newMountedSkill("kk-humanize")
	f.newScript("kk-humanize/scripts/voice-check.sh", "true")
}

// Two mounted lanes, one of them holding a script the other does not. Both halves of the basename
// scan's uniqueness gate are then live on one tree: `SKILL.md` is a name two lanes carry, and
// `voice-check.sh` is a name one lane carries. A tree with only the first has no unique lane
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
	flood.WriteString(layerLine)
	for i := 1; i <= count; i++ {
		fmt.Fprintf(&flood, format+"\n", i)
	}
	f.write(target, flood.String())
}

// The rank-5 flood a floor case buries its needles under. Zero-padded, so byte order over these
// lines is the numeric one. They dangle because nothing under a fixture's `$HOME` answers to them.
func (f *fixture) floodWithHomeRefs(target string, count int) {
	f.t.Helper()
	var flood strings.Builder
	flood.WriteString(layerLine)
	for i := 1; i <= count; i++ {
		fmt.Fprintf(&flood, "- ~/.kk-flavor/nope%03d.md\n", i)
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

// Sparse, because nothing reads it: every case built on this one asks about the size in its stat, so
// a fixture full of real bytes buys nothing and a flood of them writes the bound over and over.
func (f *fixture) writeOversize(path string) {
	f.t.Helper()
	f.write(path, "")
	if err := os.Truncate(path, maxReadBytes+1); err != nil {
		f.t.Fatalf("truncate %s: %v", path, err)
	}
}

func (f *fixture) floodWithOversize(dir string, count int) {
	f.t.Helper()
	for i := 1; i <= count; i++ {
		f.writeOversize(fmt.Sprintf("%s/huge%02d.md", dir, i))
	}
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

// Decline a case whose fixture is a path this process has to be refused, on a machine where mode 000
// refuses nobody. `what` finishes the sentence, so the skip still names what could not be built here
// rather than only the machine it was declined on.
func skipUnlessModeDeniesRead(t *testing.T, what string) {
	t.Helper()
	if modeDeniesRead(t) {
		return
	}
	t.Skip("this process reads a mode-000 path regardless of the mode (root, or CAP_DAC_OVERRIDE), so " + what)
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

// One run of the checker over this fixture, stdout and stderr merged into one buffer — the ordering
// cases read line positions out of that stream.
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
// can run alongside the rest. Both halves are settled once per case rather than at each t.Run: split
// across every case, the two sets would drift — and a case running twice may call neither t.Setenv
// nor t.Parallel a second time.
//
// Called after the fixture is built, so a case pays only its scan in the parallel phase. A fixture
// that later grows a mount panics in t.Setenv rather than quietly racing one.
func (f *fixture) isolate() {
	f.t.Helper()
	f.mountHome()
	if f.home == "" {
		f.t.Parallel()
	}
}

// The mount half of `isolate` on its own, because the root-spelling helpers below need it without the
// parallel one. TestARootSpellingCaseStillGetsItsOwnHomeMount fails if one of them stops calling it,
// and says what that costs.
func (f *fixture) mountHome() {
	f.t.Helper()
	if f.home != "" {
		f.t.Setenv("HOME", f.home)
	}
}

func (f *fixture) check() string {
	f.t.Helper()
	return f.checkWith(f.root)
}

// The same run under whatever arguments a case needs — the gate cases pass --gate beside the root, the
// spelling cases pass the root named some other way, and every other caller passes the root alone.
// Which spelling a caller used is not meant to change a finding, which is what those cases assert.
func (f *fixture) checkWith(args ...string) string {
	f.t.Helper()
	return runChecker(f.t, f.git, f.bash, append([]string{"--agent=claude"}, args...)...)
}

func runChecker(t *testing.T, git repo.Git, bash ecocheck.Bash, args ...string) string {
	t.Helper()
	var output bytes.Buffer
	if status := ecocheck.Run(args, git, bash, &output, &output); status == 2 {
		t.Fatalf("Run %v exited 2 — nothing was checked, so this case cannot be trusted\n%s", args, indent(output.String()))
	}
	return output.String()
}

// Both asserts take the whole output and fixed substrings, never a pattern: a match that a regexp
// would have swallowed turns a doesNotReport case into a silent pass.
//
// Both are variadic, because a fixture runs once: `isolate` calls t.Parallel or t.Setenv, neither of
// which a case may call twice. A property that needs a second finding checked against the same tree
// would otherwise have to rebuild the whole fixture in a case of its own, and the two copies then
// drift.
func (f *fixture) reports(needles ...string) {
	f.t.Helper()
	f.found(f.run(), needles...)
}

func (f *fixture) doesNotReport(needles ...string) {
	f.t.Helper()
	f.absent(f.run(), needles...)
}

// The same two assertions over a run that names the root `root` from working directory `dir`. t.Chdir
// is process-global and refuses a parallel case, so these run alone whatever the fixture is. That is
// the half of `isolate` they replace; `mountHome` is the other.
func (f *fixture) reportsWithRootNamed(dir, root, needle string) {
	f.t.Helper()
	f.mountHome()
	f.t.Chdir(dir)
	f.found(f.checkWith(root), needle)
}

func (f *fixture) doesNotReportWithRootNamed(dir, root string, needles ...string) {
	f.t.Helper()
	f.mountHome()
	f.t.Chdir(dir)
	f.absent(f.checkWith(root), needles...)
}

// One run a case expects to REFUSE, and its output. `run` and everything built on it fail a case that
// exits 2, because there it means the tree went unchecked and the case cannot be trusted; a case about
// a refusal wants exactly that exit, so it drives Run itself. Returned rather than asserted on here,
// because a refusal case also has to show the scan did not ALSO report what it skipped.
func (f *fixture) refuses(needles ...string) string {
	f.t.Helper()
	f.isolate()
	var output bytes.Buffer
	if status := ecocheck.Run([]string{"--agent=claude", f.root}, f.git, f.bash, &output, &output); status != 2 {
		f.t.Fatalf("expected exit 2, got %d\n%s", status, indent(output.String()))
	}
	f.found(output.String(), needles...)
	return output.String()
}

func (f *fixture) found(output string, needles ...string) {
	f.t.Helper()
	for _, needle := range needles {
		if !strings.Contains(output, needle) {
			f.t.Errorf("expected a finding containing %q\n%s", needle, indent(output))
		}
	}
}

func (f *fixture) absent(output string, needles ...string) {
	f.t.Helper()
	for _, needle := range needles {
		if strings.Contains(output, needle) {
			f.t.Errorf("expected no finding containing %q\n%s", needle, indent(output))
		}
	}
}

// What each of two checks of the same tree asked its bash for. The first run is the control, since a
// tree it never parsed leaves the second run's skipping unmeasured.
func (f *fixture) parseCounts() (first, second int) {
	f.t.Helper()
	f.isolate()
	f.check()
	first = f.bash.Parses()
	f.check()
	return first, f.bash.Parses() - first
}

// The same two assertions against a second check of the same tree — what the run before it left
// behind must not change what this one says.
func (f *fixture) reportsOnASecondRun(needles ...string) {
	f.t.Helper()
	f.found(f.runTwice(), needles...)
}

func (f *fixture) doesNotReportOnASecondRun(needles ...string) {
	f.t.Helper()
	f.absent(f.runTwice(), needles...)
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

// Ordering, never presence: the per-rank cap on its own puts a real finding on screen alongside a
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

// A finding built from text this checker did not choose, asserted over ONE run of one fixture: that
// the finding appears at all, and that no ESC reaches the output through it. The first half is not
// optional — without it the second passes on a run that raised no finding. Both come off the same
// output, so the two halves cannot end up describing two different runs, and the fixture is built
// once.
func assertNoControlByteEscapes(t *testing.T, what, finding string, build func(*testing.T) *fixture) {
	t.Run("reports "+what+", and no control byte from it reaches the output", func(t *testing.T) {
		f := build(t)
		output := f.run()
		f.found(output, finding)
		f.absent(output, "\x1b")
	})
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

func lineWith(output, needle string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

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
