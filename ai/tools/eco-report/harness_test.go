package ecoreport_test

// The fixture builders and the assertions the cases are written against. They were ported one for
// one from a shell suite that no longer exists — it was deleted once the skills switched to this
// binary, and git history is where the pairing can still be read. This is now the only suite over
// these gates, so a case removed here is coverage gone rather than coverage moved. It is also the
// only coverage of `todo-gate.sh`'s caller side: newSkillCopy copies that script in for real, and one
// case stubs it to exit 3.
//
// Fixtures are built with os.MkdirAll and os.WriteFile rather than by shelling out — the forks were
// the cost of the shell suite — but the repository itself is made by git, because what several cases
// pin is what git answers about it.
//
// Nothing here runs against this checkout. Every case gets its own repository under t.TempDir(), and
// newRepo refuses to hand one back until git agrees that directory is its own root: this suite
// reaches `discard`, which removes .idsd/ and deletes intent files, and a fixture that is not its own
// repository would run every destructive case against whatever repository encloses the temp dir.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ecoreport "kk-flavor/tools/eco-report"
)

// The installed skill this suite copies its template and todo-gate.sh from, relative to this package.
// The shell version read them out of its own directory; the same two files, reached the way the
// checkout lays them out.
const skillSource = "../../skills/idsd-qualify"

// One case's tree.
type fixture struct {
	t    *testing.T
	base string // scratch the case may write outside the repo into
	repo string // the fixture repository, and the directory every run acts from
	// A per-case copy of the skill directory: scripts beside templates, the layout the tool derives
	// its template and todo-gate paths from. So a case can break either without touching this
	// checkout's own, and one copy per case means no mutation carries. It holds no report.sh —
	// nothing reads that path, only the directory above it.
	skill string
	home  string

	out    string // the last run's stdout and stderr, merged, with trailing newlines stripped
	status int
}

func newRepo(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	f := &fixture{t: t, base: base, repo: base + "/r", home: os.Getenv("HOME")}
	f.mkdirAll(f.repo)
	f.newSkillCopy()
	f.mustGit("init", "-q")
	// Compared physically: on macOS the temp dir sits under /var, a symlink to /private/var, and git
	// answers with the resolved path. A literal comparison would fail on every fixture, making this
	// guard look like the very thing it exists to catch.
	resolved, _ := f.git("rev-parse", "--show-toplevel")
	physical, err := filepath.EvalSymlinks(f.repo)
	if err != nil || resolved != physical {
		t.Fatalf("%s resolves to %q, not itself — stopping before any destructive case runs", f.repo, resolved)
	}
	// Checked by its effect, not just by exit status: `git add -A` needs a HEAD to compare against, so
	// a fixture whose commit never landed sends every case below down the unfingerprintable-tree path
	// instead of the one it meant to test.
	f.write(f.repo+"/tracked.txt", "base\n")
	f.mustGit("add", "tracked.txt")
	f.commit("base")
	if _, status := f.git("diff", "--quiet", "HEAD", "--", "tracked.txt"); status != 0 {
		t.Fatalf("could not commit the fixture in %s — stopping before any destructive case runs", f.repo)
	}
	return f
}

// The other repo mode: .idsd/ tracked through a durable charter, with qualify-reports/ gitignored the
// way a shared idsd setup does it. Every plain newRepo fixture is a throwaway. qualify-reports/ is
// left absent, since `init` is what creates it.
func newCommittedRepo(t *testing.T) *fixture {
	t.Helper()
	f := newRepo(t)
	f.newDurableCharter()
	f.write(f.repo+"/.gitignore", ".idsd/qualify-reports/\n")
	f.mustGit("add", ".gitignore", ".idsd/charter.md")
	f.commit("committed idsd")
	f.assertFixtureIsCommitted()
	return f
}

// Checked at the one place the state is built. A fixture whose commit did not land is a throwaway,
// and the committed-mode branches its cases test (discard's refusal, check-ignore's warning, init's
// acceptance) answer the same way in both modes. So every case above such a fixture passes while
// testing nothing, all at once.
//
// The shell version named this case by fixture directory, since four identical pass lines could not
// be told apart in one stream; subtests are already told apart by their parent.
func (f *fixture) assertFixtureIsCommitted() {
	f.t.Helper()
	f.runReport("repo-mode")
	tracked, _ := f.git("ls-files", ".idsd")
	f.record("fixture rN is a committed repo", f.out == "committed", "git ls-files .idsd printed: '"+tracked+"'")
}

// Runs the tool the way a skill does: from inside the repo, so the root resolves to the fixture
// rather than to this checkout, and with stdout and stderr merged the way the shell version's `2>&1`
// merged them.
func (f *fixture) runReport(args ...string) {
	f.t.Helper()
	var output bytes.Buffer
	f.status = ecoreport.Invocation{
		Args: args,
		Dir:  f.repo,
		Self: f.skill + "/scripts/report.sh",
		Home: f.home,
		Out:  &output,
		Err:  &output,
	}.Exec()
	f.out = strings.TrimRight(output.String(), "\n")
}

// The same run from another directory, for the linked-worktree case: a worktree is its own root, and
// its info/exclude is an absolute path where an ordinary repo's is relative.
func (f *fixture) runReportIn(dir string, args ...string) {
	f.t.Helper()
	var output bytes.Buffer
	f.status = ecoreport.Invocation{Args: args, Dir: dir, Self: f.skill + "/scripts/report.sh", Home: f.home, Out: &output, Err: &output}.Exec()
	f.out = strings.TrimRight(output.String(), "\n")
}

// The stdout-only form, for the one case that pins which stream a note goes to.
func (f *fixture) runReportStdout(args ...string) string {
	f.t.Helper()
	var out, errOut bytes.Buffer
	ecoreport.Invocation{Args: args, Dir: f.repo, Self: f.skill + "/scripts/report.sh", Home: f.home, Out: &out, Err: &errOut}.Exec()
	return strings.TrimRight(out.String(), "\n")
}

// A standalone `review: …` has no slug and shares the one `review` stem, which is what most fixtures
// below use.
func (f *fixture) reportPath(name string) string {
	if name == "" {
		name = "review"
	}
	return f.repo + "/.idsd/qualify-reports/" + name + "-qualify-report.md"
}

// One shell case, one subtest, with the evidence a FAIL line would have carried under it.
func (f *fixture) record(name string, passed bool, evidence string) {
	f.t.Helper()
	f.t.Run(name, func(t *testing.T) {
		if passed {
			return
		}
		if evidence == "" {
			evidence = f.out
		}
		t.Errorf("failed\n%s", indent(evidence))
	})
}

func (f *fixture) assertReports(needle, name string) {
	f.t.Helper()
	f.record(name, strings.Contains(f.out, needle), "")
}

func (f *fixture) assertRefused(name string) {
	f.t.Helper()
	f.record(name, f.status == 2, fmt.Sprintf("exit %d, wanted 2\n%s", f.status, f.out))
}

// A refusal wrote no report. Asserted on the directory, so a report under any name counts.
func (f *fixture) assertNoReportWritten(name string) {
	f.t.Helper()
	entries, _ := os.ReadDir(f.repo + "/.idsd/qualify-reports")
	f.record(name, len(entries) == 0, "")
}

// Discard succeeded and took the whole scratch dir with it.
func (f *fixture) assertIdsdRemoved(name string) {
	f.t.Helper()
	removed := f.status == 0 && !f.exists(f.repo+"/.idsd")
	f.record(name, removed, fmt.Sprintf("exit %d; left: %s\n%s", f.status, strings.Join(f.find(f.repo+"/.idsd"), " "), f.out))
}

// Whether .idsd/ is still hidden from the human's `git add -A`. That exclusion is the whole mechanism
// keeping a throwaway report out of their commits.
func (f *fixture) hasLocalExclusion() bool {
	content, err := os.ReadFile(f.repo + "/.git/info/exclude")
	if err != nil {
		return false
	}
	return containsLine(string(content), ".idsd/")
}

// Drive a ship to a stamped, tree-fresh state. Unstamped, the state token answers `resume` without
// reading the tree at all, so a case that pins anything past the freshness checks has to come
// through here.
func (f *fixture) stampFullPass(ship string) {
	f.t.Helper()
	f.runReport("invalidate", ship)
	for _, stage := range []string{"code-review", "security-review", "tighten", "refactor", "retro"} {
		f.runReport("stage-returned", stage, ship)
		f.runReport("no-items", stage, ship)
	}
	f.runReport("stamp", "code-review,security-review,tighten,refactor,retro", ship)
}

// An intent file for one slug. Its body is a fixed constant because no case asserts on it: what they
// care about is the file's presence, and whether `discard` takes it or leaves it.
func (f *fixture) newIntentFile(slug string) {
	f.t.Helper()
	f.mkdirAll(f.repo + "/.idsd/intents")
	f.write(f.repo+"/.idsd/intents/"+slug+".md", "# intent\n")
}

// The human's own durable file: what keeps .idsd/ standing when a ship's scratch goes, and what
// `promote` needs something of.
func (f *fixture) newDurableCharter() {
	f.t.Helper()
	f.mkdirAll(f.repo + "/.idsd")
	f.write(f.repo+"/.idsd/charter.md", "# durable\n")
}

// A copy of the skill dir the tool resolves its two neighbours from.
func (f *fixture) newSkillCopy() {
	f.t.Helper()
	f.skill = f.base + "/skill"
	f.mkdirAll(f.skill + "/scripts")
	f.mkdirAll(f.skill + "/templates")
	f.copyIn(skillSource+"/scripts/todo-gate.sh", f.skill+"/scripts/todo-gate.sh", 0o755)
	f.copyIn(skillSource+"/templates/qualify-report-template.md", f.templatePath(), 0o644)
}

func (f *fixture) templatePath() string {
	return f.skill + "/templates/qualify-report-template.md"
}

func (f *fixture) todoGatePath() string {
	return f.skill + "/scripts/todo-gate.sh"
}

// Take read permission off a fixture file for the cases that need one. False means chmod did not
// restrict this user (root reads anything), so the case is skipped by name rather than failed.
// Restore the file afterwards, so the fixture teardown can remove it.
func (f *fixture) madeUnreadable(path, what string) bool {
	f.t.Helper()
	f.chmod(path, 0)
	if handle, err := os.Open(path); err != nil {
		return true
	} else {
		handle.Close()
	}
	f.t.Logf("skip  chmod does not restrict this user (root?) — %s cannot run", what)
	return false
}

// The staged/unstaged split this fixture is keeping, in the form a human would lose it in.
func (f *fixture) indexState() string {
	f.t.Helper()
	staged, _ := f.git("diff", "--name-only", "--cached")
	unstaged, _ := f.git("diff", "--name-only")
	return "staged:" + sortedWords(staged) + "\nunstaged:" + sortedWords(unstaged)
}

// A HOME whose tree-fingerprint.sh logs every invocation and then execs the real one. The shell
// version counted `write-tree` calls through a `git` shim on PATH; PATH is process-global, and a
// suite that runs its cases in parallel cannot have one. The seam moves out one script, and what it
// counts is the same thing: how many times this tool walked the tree for one `list`.
func (f *fixture) newCountingFingerprintHome() string {
	f.t.Helper()
	home := f.base + "/counting-home"
	log := home + "/fingerprints.log"
	f.mkdirAll(home + "/.kk-flavor/scripts")
	shim := home + "/.kk-flavor/scripts/tree-fingerprint.sh"
	f.write(shim, "#!/bin/sh\nprintf '%s\\n' \"$*\" >>\""+log+"\"\nexec \""+
		os.Getenv("HOME")+"/.kk-flavor/scripts/tree-fingerprint.sh\" \"$@\"\n")
	f.chmod(shim, 0o755)
	f.home = home
	return log
}

func (f *fixture) countLines(path string) int {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return countNonEmptyLines(string(content))
}
