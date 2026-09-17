package ecostats_test

// The fixture builders and the assertions the cases in stats_test.go are written against. This is the
// only suite over these measurements, so a case removed here is coverage gone rather than moved.
//
// Fixtures are built with os.MkdirAll and os.WriteFile rather than by shelling out: on the machine
// these are written on, a process costs about 100ms and a file write costs nothing.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	ecocheck "kk-flavor/tools/eco-check"
	ecostats "kk-flavor/tools/eco-stats"
	"kk-flavor/tools/repo"
)

// The figures a case reads back out of the report, and the two forms of the always-loaded line the
// agreement cases compare. Fixed patterns, anchored to the head of a line, because a report line is
// the contract each of these cases is written against.
var (
	statsRouterWords = regexp.MustCompile(`(?m)^always-loaded:.*= ([0-9]*) router`)
	checkRouterWords = regexp.MustCompile(`(?m)^always-loaded: [0-9]* lines, ([0-9]*) words across`)
)

// One case's tree. `home` empty means the case runs under the ambient HOME.
type fixture struct {
	t    *testing.T
	base string
	root string
	home string
	// HOME is process-global, so a case that needs its own mount runs alone and every other case runs
	// alongside the rest. The decision is taken once, at the first run, because several cases run the
	// tool more than once and neither t.Setenv nor t.Parallel may be called twice.
	prepared bool
}

// `base` is the scratch directory a case may write outside the root into; `root` is the tree under
// measurement.
func newRoot(t *testing.T) *fixture {
	t.Helper()
	base := newBase(t)
	f := &fixture{t: t, base: base, root: base + "/r"}
	f.mkdirAll(f.root + "/kk-flavor/standards")
	f.mkdirAll(f.root + "/kk-flavor/skills")
	f.write(f.root+"/kk-flavor/inject.md", "# Flavor\n")
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

// The path Run is handed as its own name. The row goes to ../stats.md relative to the program, so a
// case that appends calls installStats first and one that only measures never creates the directory
// at all.
func (f *fixture) self() string {
	return f.root + "/kk-flavor/skills/kk-reduce/scripts/stats.sh"
}

func (f *fixture) installStats() {
	f.t.Helper()
	f.mkdirAll(f.root + "/kk-flavor/skills/kk-reduce/scripts")
}

// The ledger a case starts from, holding whatever that case needs the file to already say.
func (f *fixture) newLedger(content string) string {
	f.t.Helper()
	f.installStats()
	path := f.root + "/kk-flavor/skills/kk-reduce/stats.md"
	f.write(path, content)
	return path
}

func (f *fixture) prepare() {
	f.t.Helper()
	if f.prepared {
		return
	}
	f.prepared = true
	if f.home != "" {
		f.t.Setenv("HOME", f.home)
	} else {
		f.t.Parallel()
	}
}

// One run over this fixture, with the two streams kept apart: every case here knows which of them it
// is asking about. `run` picks claude; a case that turns on the agent names its own through runAs.
func (f *fixture) run(args ...string) (stdout, stderr string, status int) {
	f.t.Helper()
	return f.runAs("claude", args...)
}

func (f *fixture) runAs(agent string, args ...string) (stdout, stderr string, status int) {
	f.t.Helper()
	f.prepare()
	var out, errOut bytes.Buffer
	status = ecostats.Run(f.self(), append([]string{"--agent=" + agent}, args...), &out, &errOut)
	return out.String(), errOut.String(), status
}

// A figure off the report by the name it is printed under, and empty when the line is not there at all.
func (f *fixture) figure(name string) string {
	f.t.Helper()
	stdout, _, _ := f.run(f.root)
	return firstSubmatch(regexp.MustCompile(`(?m)^`+regexp.QuoteMeta(name)+`: *([0-9]*) words`), stdout)
}

// The invariant the agreement cases hold: for one tree, both tools report the same router figure.
func (f *fixture) assertScriptsAgree() {
	f.t.Helper()
	fromCheck := f.routerWordsFromCheck()
	fromStats := f.routerWordsFromStats()
	if fromCheck == "" || fromCheck != fromStats {
		f.t.Errorf("check.sh router words: %q\nstats.sh router words: %q", fromCheck, fromStats)
	}
}

func (f *fixture) routerWordsFromStats() string {
	f.t.Helper()
	stdout, _, _ := f.run(f.root)
	return firstSubmatch(statsRouterWords, stdout)
}

// Neither run below passes `--gate`, which is the only thing in check.sh that puts a question to a
// repository — so there is nothing here for one to answer.
var noRepository repo.Git

func (f *fixture) routerWordsFromCheck() string {
	f.t.Helper()
	f.prepare()
	var out bytes.Buffer
	ecocheck.Run([]string{"--agent=claude", f.root}, noRepository, ecocheck.InstalledBash{}, &out, io.Discard)
	return firstSubmatch(checkRouterWords, out.String())
}

// The other tool's own report over this tree, both streams merged.
func (f *fixture) checkOutput() string {
	f.t.Helper()
	f.prepare()
	var out bytes.Buffer
	ecocheck.Run([]string{"--agent=claude", f.root}, noRepository, ecocheck.InstalledBash{}, &out, &out)
	return out.String()
}

func lineWith(output, needle string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

// The rows a ledger holds, header and rule included.
func rowsIn(t *testing.T, path string) int {
	t.Helper()
	rows := 0
	for _, line := range strings.Split(readFile(t, path), "\n") {
		if strings.HasPrefix(line, "|") {
			rows++
		}
	}
	return rows
}

// Everything a ledger says before its column header — the half the seed block writes and the live
// file is compared against.
func ledgerProse(text string) string {
	var prose strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "| date |") {
			break
		}
		prose.WriteString(line + "\n")
	}
	return prose.String()
}

func firstSubmatch(pattern *regexp.Regexp, text string) string {
	if found := pattern.FindStringSubmatch(text); found != nil {
		return found[1]
	}
	return ""
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

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func indent(text string) string {
	var out strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		fmt.Fprintf(&out, "          %s\n", line)
	}
	return out.String()
}

// The scratch directory every fixture above is built under, and short by a length this suite fixes
// rather than one the machine hands it.
//
// Several of these cases assert where a BOUNDED message was cut, and the messages that quote a path
// are bounded at 80 or 160 bytes — so what the root spends, the case's own content cannot.
// `t.TempDir()` makes that length ambient: about 140 bytes on a macOS runner (`/var/folders/<two>/<28
// random>/T/<the test's own name>/001`) against around 60 on Linux, so the answer is a property of the
// machine before it is a property of the code. Measured: two cases here passed on one macOS temp path
// and failed on another with nothing else changed.
//
// This is 14 to 16 bytes wherever it runs — `/tmp/e` and the eight-to-ten digit run os.MkdirTemp
// appends — which leaves the whole of every bound to be spent by what the case itself writes. A case
// that wants a root long enough to spend a bound grows one itself rather than hoping TMPDIR does.
//
// `/tmp` rather than TMPDIR, because TMPDIR is exactly what is too long. Removed on the way out, like
// t.TempDir's own.
func newBase(t *testing.T) string {
	t.Helper()
	base, err := os.MkdirTemp("/tmp", "e")
	if err != nil {
		t.Fatalf("building a fixture root: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	return base
}

// What a fixture root may spend of a bounded message before the case writes a byte. newBase builds a
// base of 14 to 16 bytes and every fixture puts `/r` on the end of it, so this leaves room to rename the
// prefix and none at all to go back to a path the machine picked: `t.TempDir()` costs upwards of 35
// bytes on the shortest Linux runner and about 160 on a macOS one.
const maxFixtureRootBytes = 24

// The property newBase exists for. Held as a case because a comment on each affected fixture only
// works while the next author reads it: without this, a helper reaching back for `t.TempDir()` shows
// up as a handful of unrelated cases going red on one runner and green on another, which is how this
// class was found in the first place and cost a CI leg to find.
//
// The bound is checked under a LONG TMPDIR as well as the ambient one, and that second leg is the
// whole point: a root read once tells you nothing about whether the machine chose its length, and the
// machine this runs on is exactly the one whose temp path is short enough to hide the defect. Length
// rather than sameness between the two legs, because os.MkdirTemp appends a run of eight to ten
// digits and a root that wobbles by two bytes inside a 24-byte budget is not what any case here reads.
func TestAFixtureRootIsTheSuitesToSpendAndNotTheMachines(t *testing.T) {
	// Built without t.TempDir, which would defeat the leg it is for: that call creates ONE directory
	// per test and numbers the rest inside it, so a newBase reaching for it would answer out of a tree
	// already pinned to the ambient TMPDIR and the moved one would never be read.
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
				"every bounded message is spent before the case writes anything: %s",
				leg.what, len(root), maxFixtureRootBytes, root)
		}
	}
}
