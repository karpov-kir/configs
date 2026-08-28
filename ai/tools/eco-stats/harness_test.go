package ecostats_test

// The fixture builders and the assertions the cases in stats_test.go are written against. They were
// ported one for one from a shell suite that no longer exists — it was deleted once the skills
// switched to this binary, and git history is where the pairing can still be read. This is now the
// only suite over these measurements, so a case removed here is coverage gone rather than moved.
//
// Fixtures are built with os.MkdirAll and os.WriteFile rather than by shelling out: the forks were
// the whole cost of the shell suite, and a mutation harness multiplies that cost by the length of its
// mutation list. `ai/tools/go-mutate` is what shows a case here can fail.

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
)

// The figures a case reads back out of the report, and the two forms of the always-loaded line the
// agreement cases compare. Fixed patterns, anchored to the head of a line, because a report line is
// the contract each of these cases is written against.
var (
	statsRouterWords = regexp.MustCompile(`(?m)^always-loaded:.*= ([0-9]*) router`)
	checkRouterWords = regexp.MustCompile(`(?m)^always-loaded: [0-9]* lines, ([0-9]*) words across`)
)

// One case's tree. `home` empty means the case runs under the ambient HOME, exactly as the shell
// version's empty `check_home` did.
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

// The `$base/r$N` of the shell version: `base` is the scratch directory a case may write outside the
// root into, `root` the tree under measurement.
func newRoot(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	f := &fixture{t: t, base: base, root: base + "/r"}
	f.mkdirAll(f.root + "/kk-flavor/standards")
	f.mkdirAll(f.root + "/skills")
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

// The path Run is handed as its own name. It is where the shell version's `cp "$stats" …` put the
// script, because the row goes to ../stats.md relative to the program — so a case that appends calls
// installStats first, exactly as every --append case there did, and one that only measures never
// creates the directory at all.
func (f *fixture) self() string {
	return f.root + "/skills/kk-reduce/scripts/stats.sh"
}

func (f *fixture) installStats() {
	f.t.Helper()
	f.mkdirAll(f.root + "/skills/kk-reduce/scripts")
}

// The ledger a case starts from, holding whatever that case needs the file to already say.
func (f *fixture) newLedger(content string) string {
	f.t.Helper()
	f.installStats()
	path := f.root + "/skills/kk-reduce/stats.md"
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

// One run over this fixture. The two streams are kept apart — the shell suite merged them with 2>&1
// only where it greps, and every case here knows which stream it is asking about.
func (f *fixture) run(args ...string) (stdout, stderr string, status int) {
	f.t.Helper()
	f.prepare()
	var out, errOut bytes.Buffer
	status = ecostats.Run(f.self(), args, &out, &errOut)
	return out.String(), errOut.String(), status
}

// A figure off the report by the name it is printed under — the shell version's
// `sed -n "s/^$1: *\([0-9]*\) words.*/\1/p"`, and empty when the line is not there at all.
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

func (f *fixture) routerWordsFromCheck() string {
	f.t.Helper()
	f.prepare()
	var out bytes.Buffer
	ecocheck.Run(f.root, &out, io.Discard)
	return firstSubmatch(checkRouterWords, out.String())
}

// The other tool's own report over this tree, both streams merged the way the shell case read it.
func (f *fixture) checkOutput() string {
	f.t.Helper()
	f.prepare()
	var out bytes.Buffer
	ecocheck.Run(f.root, &out, &out)
	return out.String()
}

// `grep -c '^|'` — the rows a ledger holds, header and rule included.
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

// No parent is created here, deliberately: the shell version's `>` failed on a missing directory, and
// a builder that quietly creates one hides a fixture written against a tree it does not have.
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
