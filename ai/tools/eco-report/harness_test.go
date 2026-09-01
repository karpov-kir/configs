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
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	ecoreport "kk-flavor/tools/eco-report"
)

// The installed skill this suite copies its template and todo-gate.sh from, relative to this package.
// The shell version read them out of its own directory; the same two files, reached the way the
// checkout lays them out.
const skillSource = "../../skills/idsd-qualify"

// This checkout's kk-flavor, reached the same way, and the source of the one script the tool execs
// out of HOME. Never the developer's own `~/.kk-flavor`: that copy is what install.sh puts there, it
// is not part of this repository, and a suite reading it passes on a machine that has installed the
// skills and fails on every machine that has not — release-tools.yml's ubuntu-latest among them.
const flavorSource = "../../kk-flavor"

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
	// The HOME the tool runs against, built by newFlavorHome. Never the machine's own: see there.
	home string
	// Where the tool looks for this machine's override. Empty means there is none, which is what every
	// case wants but the few that write one; never the developer's real $XDG_CONFIG_HOME.
	configHome string

	out    string // the last run's stdout and stderr, merged, with trailing newlines stripped
	status int
}

func newRepo(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	f := &fixture{t: t, base: base, repo: base + "/r"}
	// Pinned at a fixture directory holding no override, never left empty: empty falls back to the
	// process's own $XDG_CONFIG_HOME, and this suite would then read the developer's real idsd.conf —
	// passing on a machine that has one and failing on every machine that does not. The same rule
	// newFlavorHome states for HOME.
	f.configHome = base + "/config"
	f.mkdirAll(f.configHome)
	f.mkdirAll(f.repo)
	f.newFlavorHome()
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

// A repository with the scratch excluded and one report scaffolded: the two commands every pass runs
// before it has a report to act on. A case that varies either of them runs them itself.
func newShip(t *testing.T, intent string) *fixture {
	t.Helper()
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", intent)
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
	f.runReportIn(f.repo, args...)
}

// One invocation, and the one place this suite says which skill directory and which HOME the tool runs
// against — the two fields a case never varies and would otherwise restate at each call.
func (f *fixture) invoke(dir string, out, errOut io.Writer, args []string) int {
	f.t.Helper()
	return ecoreport.Invocation{
		Args: args,
		Dir:  dir,
		Self: f.skill + "/scripts/report.sh",
		Home: f.home,
		// Empty for almost every case, which means "no override file", since the fixture HOME holds no
		// .config. A case that wants one sets f.configHome and never touches the developer's own.
		ConfigHome: f.configHome,
		Out:        out,
		Err:        errOut,
	}.Exec()
}

// The same run from another directory, for the linked-worktree case: a worktree is its own root, and
// its info/exclude is an absolute path where an ordinary repo's is relative.
func (f *fixture) runReportIn(dir string, args ...string) {
	f.t.Helper()
	var output bytes.Buffer
	f.status = f.invoke(dir, &output, &output, args)
	f.out = strings.TrimRight(output.String(), "\n")
}

// The stdout-only form from another directory. Needed wherever a value is read back while an override
// is active: runReportIn merges the streams, so the override's stderr note would arrive as part of the
// answer.
func (f *fixture) runReportStdoutIn(dir string, args ...string) string {
	f.t.Helper()
	var out, errOut bytes.Buffer
	f.invoke(dir, &out, &errOut, args)
	return strings.TrimRight(out.String(), "\n")
}

// The stdout-only form, for the one case that pins which stream a note goes to.
func (f *fixture) runReportStdout(args ...string) string {
	f.t.Helper()
	var out, errOut bytes.Buffer
	f.invoke(f.repo, &out, &errOut, args)
	return strings.TrimRight(out.String(), "\n")
}

// A standalone `review: …` has no slug and shares the one `review` stem, which is what most fixtures
// below use.
func (f *fixture) reportPath(name string) string {
	if name == "" {
		name = "review"
	}
	return f.scratch() + "/qualify-reports/" + name + "-qualify-report.md"
}

// Where this fixture's scratch directory is, by the same rule the tool applies: in the tree while
// .idsd/ is tracked, under the shared git dir otherwise. Asked of git rather than of the tool, because
// every assertion helper below calls this and running the tool here would overwrite the f.out and
// f.status the case is about to read.
func (f *fixture) scratch() string {
	if tracked, _ := f.git("ls-files", ".idsd"); tracked != "" {
		return f.treeIdsd()
	}
	return f.sharedIdsd()
}

// The default throwaway location, as a literal. A case asserting WHERE the scratch landed wants this
// rather than f.scratch(), which derives from the same rule the tool does and would agree with it by
// construction.
//
// Physically resolved, for the reason newRepo states: on macOS the temp dir sits under /var, a symlink
// to /private/var, and git answers with the resolved path. Compared literally, every location
// assertion would fail and look like the defect it exists to catch.
func (f *fixture) sharedIdsd() string {
	if real, err := filepath.EvalSymlinks(f.repo); err == nil {
		return real + "/.git/idsd"
	}
	return f.repo + "/.git/idsd"
}

// The in-tree location — what committed mode uses and what `promote` moves into. Physically resolved
// for the reason sharedIdsd is: the tool reports paths built from git's answer, which is resolved.
func (f *fixture) treeIdsd() string {
	if real, err := filepath.EvalSymlinks(f.repo); err == nil {
		return real + "/.idsd"
	}
	return f.repo + "/.idsd"
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

// The exit and the output together — what a case hands `record` when the assertion is about both.
func (f *fixture) evidence() string {
	return "exit " + strconv.Itoa(f.status) + "\n" + f.out
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
	entries, _ := os.ReadDir(f.scratch() + "/qualify-reports")
	f.record(name, len(entries) == 0, "")
}

// Discard succeeded and took the whole scratch dir with it. Both locations are asserted: the shared
// one because that is what discard removes, and the in-tree one because a throwaway run that left a
// directory there has broken the mode's contract just as thoroughly.
func (f *fixture) assertIdsdRemoved(name string) {
	f.t.Helper()
	shared, tree := f.sharedIdsd(), f.treeIdsd()
	removed := f.status == 0 && !f.exists(shared) && !f.exists(tree)
	f.record(name, removed, fmt.Sprintf("exit %d; left: %s %s\n%s",
		f.status, strings.Join(f.find(shared), " "), strings.Join(f.find(tree), " "), f.out))
}

// This machine's override, as a case builds one. Written under the fixture's own configHome, so no
// case can reach the developer's real ~/.config/kk-flavor/idsd.conf.
func (f *fixture) writeOverride(content string) {
	f.t.Helper()
	f.mkdirAll(f.configHome + "/kk-flavor")
	f.write(f.configHome+"/kk-flavor/idsd.conf", content)
}

// The invariant that replaced the local exclusion: throwaway scratch sits where `git add -A` cannot
// reach it, so there is nothing to hide and no exclusion to keep in step across worktrees. Stronger
// than what it replaced — an exclude entry can be edited away, a path outside the tree cannot — and
// asked of `git status` rather than of the layout, so it fails if git can see the scratch by any route.
func (f *fixture) treeIsFreeOfScratch() bool {
	if f.exists(f.treeIdsd()) {
		return false
	}
	dirty, status := f.git("status", "--porcelain")
	return status == 0 && !strings.Contains(dirty, ".idsd")
}

// Everything a stamp demands short of the stamp itself: this pass invalidated, and every stage
// marked returned and then empty. Invalidate comes first because a marker means nothing until it is
// known which pass made it.
func (f *fixture) armFullPass(ship string) {
	f.t.Helper()
	f.runReport("invalidate", ship)
	for _, stage := range []string{"code-review", "security-review", "tighten", "refactor"} {
		f.runReport("stage-returned", stage, ship)
		f.runReport("no-items", stage, ship)
	}
}

// Drive a ship to a stamped, tree-fresh state. Unstamped, the state token answers `resume` without
// reading the tree at all, so a case that pins anything past the freshness checks has to come
// through here.
func (f *fixture) stampFullPass(ship string) {
	f.t.Helper()
	f.armFullPass(ship)
	f.runReport("stamp", "code-review,security-review,tighten,refactor", ship)
}

// An intent file for one slug. Its body is a fixed constant because no case asserts on it: what they
// care about is the file's presence, and whether `discard` takes it or leaves it.
func (f *fixture) newIntentFile(slug string) {
	f.t.Helper()
	f.mkdirAll(f.scratch() + "/intents")
	f.write(f.scratch()+"/intents/"+slug+".md", "# intent\n")
}

// The human's own durable file, in the SCRATCH dir rather than the tree: what keeps the scratch
// directory standing when a ship's own files go. The in-tree variant below exists to build committed
// mode, which is a different job.
func (f *fixture) newDurableCharterInScratch() {
	f.t.Helper()
	f.mkdirAll(f.scratch())
	f.write(f.scratch()+"/charter.md", "# durable\n")
}

// The human's own durable file: what keeps .idsd/ standing when a ship's scratch goes, and what
// `promote` needs something of.
func (f *fixture) newDurableCharter() {
	f.t.Helper()
	// Always in-tree, never f.scratch(): this helper exists to CREATE committed mode, and at the moment
	// it runs the repo is still throwaway, so scratch() would answer the shared dir and the `git add`
	// that follows would have nothing to stage.
	f.mkdirAll(f.treeIdsd())
	f.write(f.treeIdsd()+"/charter.md", "# durable\n")
}

// A copy of the skill dir the tool resolves its two neighbours from.
// The HOME every case runs against: a fixture directory holding the one script the tool execs out of
// HOME, copied in from this checkout. A copy rather than the installed one, for the reason above
// flavorSource, and a copy rather than a symlink so a case may chmod it without reaching this
// checkout's own.
func (f *fixture) newFlavorHome() {
	f.t.Helper()
	f.home = f.base + "/home"
	f.mkdirAll(f.home + "/.kk-flavor/scripts")
	f.copyIn(flavorSource+"/scripts/tree-fingerprint.sh", f.fingerprintScriptIn(f.home), 0o755)
}

// Where a HOME holds the fingerprint script. One expression of the layout eco-report.go derives
// r.fingerprintBin from, so the fixture and the tool cannot drift apart about it.
func (f *fixture) fingerprintScriptIn(home string) string {
	return home + "/.kk-flavor/scripts/tree-fingerprint.sh"
}

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
	handle, err := os.Open(path)
	if err != nil {
		return true
	}
	handle.Close()
	f.t.Logf("skip  chmod does not restrict this user (root?) — %s cannot run", what)
	return false
}

// The same fixture bound to a subtest's own *testing.T, so a case that declines is reported as a skip
// against its own name instead of a log line inside its parent's pass. The fixture is a flat value, so
// the copy shares the tree and nothing else.
func (f *fixture) inSubtest(t *testing.T) *fixture {
	bound := *f
	bound.t = t
	return &bound
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
//
// It wraps the script in the HOME the fixture already had rather than naming a source of its own, so
// there is one answer in this suite to where that script comes from.
func (f *fixture) newCountingFingerprintHome() string {
	f.t.Helper()
	wrapped := f.fingerprintScriptIn(f.home)
	home := f.base + "/counting-home"
	log := home + "/fingerprints.log"
	f.mkdirAll(home + "/.kk-flavor/scripts")
	shim := f.fingerprintScriptIn(home)
	f.write(shim, "#!/bin/sh\nprintf '%s\\n' \"$*\" >>\""+log+"\"\nexec \""+wrapped+"\" \"$@\"\n")
	f.chmod(shim, 0o755)
	f.home = home
	return log
}

func (f *fixture) nonEmptyLinesIn(path string) int {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return countNonEmptyLines(string(content))
}
