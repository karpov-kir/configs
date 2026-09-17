package ecoreport_test

// The fixture builders and the assertions the cases are written against. This is the only suite over
// these gates, so a case removed here is coverage gone rather than coverage moved.
//
// Every input this suite gives the tool it writes itself — the template, the tree. Nothing is read
// from outside the module, and that is a cache property before it is a style one: `go test` keys its
// cache on the module, so a suite reading the shipped skill answers `ok (cached)` over a template that
// has changed under it.
//
// The repository is a table, not a checkout. `ai/tools/repo` names the questions this tool asks of one
// and `repotest.Fake` answers them, so no case forks git to have something to drive. What a repository
// still IS on disk is a directory holding `.git`, because layout.go reads the layout itself.
//
// Nothing here runs against this checkout. Every case gets its own tree under t.TempDir(), and every
// destructive path is aimed by the resolved idsd location rather than by a search: this suite reaches
// `discard`, which removes .idsd/ and deletes intent files. newWorkingTree says what stands where git's
// own "this directory is its own root" check used to.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	ecoreport "kk-flavor/tools/eco-report"
	"kk-flavor/tools/repo"
	"kk-flavor/tools/repo/repotest"
)

// The entries `promote` writes and `check-ignore` verifies, mirroring ignoreSurface() so a case and
// the tool cannot drift about the pattern. The report's entry is the one most cases assert on, and
// reportEntry names it rather than an index into this.
func ignoreEntries() []string {
	return []string{
		".idsd/intents/*/for-agents/decisions.md",
		".idsd/intents/*/for-agents/language.md",
		".idsd/intents/*/for-agents/playbook.md",
		".idsd/intents/*/for-agents/qualify-report.md",
	}
}

func reportEntry() string { return ".idsd/intents/*/for-agents/qualify-report.md" }

// Those entries as a gitignore file's worth of lines.
func ignoreBlock() string { return strings.Join(ignoreEntries(), "\n") + "\n" }

// One case's tree.
type fixture struct {
	// How this fixture's tree is fingerprinted. newTreeFingerprint by default; countFingerprints wraps
	// it to count, and one case swaps in a failing one.
	fingerprint func(root string) (string, error)
	// What the tool's repository questions are answered from. Never nil: the scope fixtures put this
	// table over a real seed repository, since applicability.go still runs one git command itself, but
	// every question still comes through here.
	fake *repotest.Fake

	t    *testing.T
	base string // scratch the case may write outside the repo into
	repo string // the fixture repository, and the directory every run acts from
	// A per-case copy of the skill directory: scripts beside templates, the layout the tool derives
	// its template path from. So a case can break it without touching this checkout's own, and one
	// copy per case means no mutation carries. It holds no report.sh — nothing reads that path, only
	// the directory above it.
	skill string
	// The HOME the tool runs against, built by newFlavorHome. Never the machine's own: see there.
	home string
	// Where the tool looks for this machine's override. Empty means there is none, which is what every
	// case wants but the few that write one; never the developer's real $XDG_CONFIG_HOME.
	configHome string
	// One table per linked worktree, keyed by the worktree's canonical path. newLinkedWorktree fills it.
	worktrees map[string]*repotest.Fake
	// Every FILE the tool's staging reached, beside the pathspecs it named — what `git diff --cached`
	// would have listed, since git expands a directory pathspec and `staged` is read as that listing.
	stagedFiles []string
	// Held across every repository answer, for the one case that submits two results at once: two
	// invocations then share one table, and the fake records what it was asked in a slice of its own.
	asking *sync.Mutex

	out    string // the last run's stdout and stderr, merged, with trailing newlines stripped
	status int
}

func newRepo(t *testing.T) *fixture {
	t.Helper()
	return newRepoNamed(t, "r")
}

// The same fixture under a chosen directory name, for the one case whose input IS the repository's own
// path. It comes through here rather than being built beside it so that the HOME, the skill directory
// and the config home every case depends on are settled in one place: a second builder would carry
// none of them, and the case it serves would run against a fixture nothing arranged.
func newRepoNamed(t *testing.T, name string) *fixture {
	t.Helper()
	base := t.TempDir()
	f := &fixture{t: t, base: base, repo: base + "/" + name, asking: &sync.Mutex{}}
	// Pinned at a fixture directory holding no override, never left empty: empty falls back to the
	// process's own $XDG_CONFIG_HOME, and this suite would then read the developer's real idsd.conf —
	// passing on a machine that has one and failing on every machine that does not. The same rule
	// newFlavorHome states for HOME.
	f.configHome = base + "/config"
	f.mkdirAll(f.configHome)
	f.mkdirAll(f.repo)
	f.newFlavorHome()
	f.newSkillCopy()
	f.newWorkingTree()
	f.fingerprint = f.newTreeFingerprint()
	return f
}

// What a repository is to this tool: a directory holding `.git`, plus a table of answers. layout.go
// reads the layout off the disk itself and the port answers everything that is a decision rather than a
// location, so nothing has to run git to build one.
//
// This is also where the old fixture's destructive-case guard lived, and it is replaced by something
// stronger rather than dropped. That guard asked git whether the fixture directory was its own root,
// because a fixture inside an enclosing repository would have sent `discard` at that repository's
// .idsd/. Here there is no discovery to go wrong: the tool resolves its root from the `.git` this
// builder just made, and the fake answers about that root alone — an enclosing checkout is not
// reachable from either, whatever stands above t.TempDir(). What the old guard could only check
// afterwards is now arranged, which is the same move newNeutral made in `ai/tools/cadence`.
func (f *fixture) newWorkingTree() {
	f.t.Helper()
	// `git init`'s own skeleton, as far as anything here reads it: HEAD, and the three directories a
	// case or the tool writes into — `info/` holds the exclude file migrate.go cleans up.
	for _, dir := range []string{"/.git/info", "/.git/objects", "/.git/refs"} {
		f.mkdirAll(f.repo + dir)
	}
	f.write(f.repo+"/.git/HEAD", "ref: refs/heads/main\n")
	f.write(f.repo+"/tracked.txt", "base\n")
	// Rooted at the canonical path, because that is what the tool resolves: on macOS a temp dir sits
	// under /var, a symlink to /private/var, and layoutRoot answers physically for the reason it states.
	f.fake = repotest.New(f.canonicalRepo())
	f.worktrees = map[string]*repotest.Fake{}
	f.track("tracked.txt")
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

// A ship in the other repo mode, scaffolded through the two commands every pass runs plus the intent
// file. Unlike newShip it writes that file itself: every case built on this one archives the ship, and
// the archived intent.md is half of what the move has to stage.
func newCommittedShip(t *testing.T, intent string) *fixture {
	t.Helper()
	f := newCommittedRepo(t)
	f.runReport("init", intent)
	f.newIntentFile(intent)
	return f
}

// The other repo mode: .idsd/ tracked through a durable charter, with each ship's scratch gitignored
// the way a committed idsd setup does it. Every plain newRepo fixture is external. No ship folder is
// made here, since `init` is what creates one.
func newCommittedRepo(t *testing.T) *fixture {
	t.Helper()
	f := newRepo(t)
	f.newDurableCharter()
	f.write(f.repo+"/.gitignore", ignoreBlock())
	f.track(".gitignore", ".idsd/charter.md")
	f.assertFixtureIsCommitted()
	return f
}

// Checked at the one place the state is built. A fixture whose .idsd/ is not tracked is external,
// and the committed-mode branches its cases test (discard's refusal, check-ignore's warning, init's
// acceptance) answer the same way in both modes. So every case above such a fixture passes while
// testing nothing, all at once.
func (f *fixture) assertFixtureIsCommitted() {
	f.t.Helper()
	f.runReport("repo-mode")
	f.record("fixture rN is a committed repo", f.out == "committed", "repo-mode printed: '"+f.out+"'")
}

// Runs the tool the way a skill does: from inside the repo, so the root resolves to the fixture rather
// than to this checkout, and with stdout and stderr merged.
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
		Git:  f.repoGit(dir),
		Self: f.skill + "/scripts/report.sh",
		Home: f.home,
		// Empty for almost every case, which means "no override file", since the fixture HOME holds no
		// .config. A case that wants one sets f.configHome and never touches the developer's own.
		ConfigHome:  f.configHome,
		Out:         out,
		Err:         errOut,
		Fingerprint: f.fingerprint,
	}.Exec()
}

// Where a run from this directory gets its repository answers.
//
// A linked worktree answers for itself. Its git dir is its OWN and its common dir is the clone's, and
// the whole scratch location rests on that difference — one table answering both would give every
// caller the main tree's git dir and pass a tool that had stopped asking.
func (f *fixture) repoGit(dir string) repo.Git {
	fake := f.fake
	if linked, found := f.worktrees[canonical(dir)]; found {
		fake = linked
	}
	return stagingRepo{Fake: fake, f: f}
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
	return f.runReportStdoutIn(f.repo, args...)
}

// A standalone `review: …` has no slug and shares the one `review` stem, which is what most fixtures
// below use.
func (f *fixture) reportPath(name string) string {
	return f.shipDir(name) + "/for-agents/qualify-report.md"
}

// An archived ship keeps its folder, so the record a build leaves travels as one directory.
func (f *fixture) archiveDir(name string) string {
	return f.scratch() + "/archive/" + name
}

// One ship's folder. Every file a ship owns lives in it — the intent, the three intent-local records,
// and the report — so a ship is torn down by removing one directory rather than by naming its files.
// A standalone review has no intent file, and shares the one `review` folder for the rest.
func (f *fixture) shipDir(name string) string {
	if name == "" {
		name = "review"
	}
	return f.scratch() + "/intents/" + name
}

// Where this fixture's scratch directory is, by the same rule the tool applies: in the tree while
// .idsd/ is tracked, under the shared git dir otherwise. Asked of the index rather than of the tool,
// because every assertion helper below calls this and running the tool here would overwrite the f.out
// and f.status the case is about to read.
func (f *fixture) scratch() string {
	if f.isTracked(".idsd") {
		return f.treeIdsd()
	}
	return f.sharedIdsd()
}

// This fixture's repo path as the tool records it, and the base every location below is built from.
//
// Physically resolved, because that is the root the tool resolves: layoutRoot answers physically, and
// so does git's own `--show-toplevel`, so on macOS a fixture under /var is reported under /private/var.
func (f *fixture) canonicalRepo() string {
	if real, err := filepath.EvalSymlinks(f.repo); err == nil {
		return real
	}
	return f.repo
}

// The default external location, as a literal. A case asserting WHERE the scratch landed wants this
// rather than f.scratch(), which derives from the same rule the tool does and would agree with it by
// construction.
func (f *fixture) sharedIdsd() string {
	return f.canonicalRepo() + "/.git/idsd"
}

// The in-tree location — what committed mode uses and what `promote` moves into.
func (f *fixture) treeIdsd() string {
	return f.canonicalRepo() + "/.idsd"
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
	entries, _ := os.ReadDir(f.scratch() + "/intents")
	f.record(name, len(entries) == 0, "")
}

// Discard succeeded and took the whole scratch dir with it. Both locations are asserted: the shared
// one because that is what discard removes, and the in-tree one because an external run that left a
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

// The invariant that replaced the local exclusion: an external idsd sits where `git add -A` cannot
// reach it, so there is nothing to hide and no exclusion to keep in step across worktrees. Stronger
// than what it replaced — an exclude entry can be edited away, a path outside the tree cannot.
//
// Asked of the working tree itself rather than of `git status`: what makes the property hold is that
// nothing named .idsd exists anywhere under the root, which is also what git would have to be reading
// to report one. Whether git's own status sees a path inside the tree is `repo/exec_test.go`'s.
func (f *fixture) treeIsFreeOfScratch() bool {
	for _, path := range f.find(f.repo) {
		if strings.HasPrefix(path, f.repo+"/.git") {
			continue
		}
		if strings.Contains(path, ".idsd") {
			return false
		}
	}
	return true
}

var (
	allStages          = []string{"code-review", "security-review", "edit", "refactor"}
	allStagesStampedAs = strings.Join(allStages, ",")
)

func (f *fixture) armFullPass(ship string) {
	f.t.Helper()
	f.armFullPassIn(f.repo, ship)
}

// Drive a ship to a stamped, tree-fresh state. Unstamped, the state token answers `resume` without
// reading the tree at all, so a case that pins anything past the freshness checks has to come
// through here.
func (f *fixture) stampFullPass(ship string) {
	f.t.Helper()
	f.stampFullPassIn(f.repo, ship)
}

// The same two, driven from another directory — a linked worktree, which is where a case about WHICH
// worktree earned a stamp has to run them.
func (f *fixture) armFullPassIn(dir, ship string) {
	f.t.Helper()
	f.runReportIn(dir, "invalidate", ship)
	for _, stage := range allStages {
		f.recordCleanStageIn(cleanStageOptions{dir: dir, stage: stage, intent: ship})
	}
	f.runReportIn(dir, "decisions-reviewed", ship)
}

func (f *fixture) stampFullPassIn(dir, ship string) {
	f.t.Helper()
	f.armFullPassIn(dir, ship)
	f.runReportIn(dir, "stamp", allStagesStampedAs, ship)
}

// An intent file for one slug. The body is fixed because no case asserts on it — what they care about
// is the file's presence, and whether `discard` takes it or leaves it. The frontmatter is not: the
// merge gate refuses an intent that never reached `status: approved`, so a status-less fixture would
// make every gate case block for a reason it is not about. A case about that arm writes its own file.
func (f *fixture) newIntentFile(slug string) {
	f.t.Helper()
	f.mkdirAll(f.shipDir(slug))
	f.write(f.shipDir(slug)+"/intent.md", "---\nstatus: approved\n---\n\n# intent\n")
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
	// it runs the repo is still external, so scratch() would answer the shared dir and the `git add`
	// that follows would have nothing to stage.
	f.mkdirAll(f.treeIdsd())
	f.write(f.treeIdsd()+"/charter.md", "# durable\n")
}

// The HOME every case runs against: a fixture directory holding the one script the tool looks for
// before it fingerprints. Its CONTENT is never run — the recipe is called in process through
// Invocation.Fingerprint, and currentTree only asks whether the install is complete — so the body is a
// refusal, which is what a case would see if that ever stopped being true. Written rather than copied
// from this checkout, so the suite reads nothing `go test` cannot key its cache on, and so a case may
// chmod it without reaching the real install.
func (f *fixture) newFlavorHome() {
	f.t.Helper()
	f.home = f.base + "/home"
	f.mkdirAll(f.home + "/.kk-flavor/scripts")
	f.write(f.fingerprintScriptIn(f.home), "#!/bin/sh\necho 'the fingerprint recipe runs in process; this script is only looked for' >&2\nexit 2\n")
	f.chmod(f.fingerprintScriptIn(f.home), 0o755)
}

// Where a HOME holds the fingerprint script. One expression of the layout eco-report.go derives
// r.fingerprintBin from, so the fixture and the tool cannot drift apart about it.
func (f *fixture) fingerprintScriptIn(home string) string {
	return home + "/.kk-flavor/scripts/tree-fingerprint.sh"
}

// The skill dir the tool resolves its template from, written by the fixture: scripts beside templates,
// the layout the tool derives that path from. So a case can break the template without touching this
// checkout's own, and one per case means no mutation carries.
func (f *fixture) newSkillCopy() {
	f.t.Helper()
	f.skill = f.base + "/skill"
	f.mkdirAll(f.skill + "/scripts")
	f.mkdirAll(f.skill + "/templates")
	f.write(f.templatePath(), reportTemplate)
}

func (f *fixture) templatePath() string {
	return f.skill + "/templates/qualify-report-template.md"
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

// A directory this process cannot LIST but can still traverse: 0300 drops read while keeping execute,
// so os.ReadDir fails and a path through it still resolves. A ship folder now holds both the report
// and the intent, so mode 0 would refuse a subcommand at "no such ship" long before it read a listing
// — and the case would pass for a reason that has nothing to do with the guard under test.
//
// False means the mode did not restrict this process (root, or CAP_DAC_OVERRIDE), which is a skip
// rather than a failure. Probed rather than inferred from the uid, for the reason testing.md → **4.
// Setup strategy** states.
func (f *fixture) madeUnlistable(path, what string) bool {
	f.t.Helper()
	f.chmod(path, 0o300)
	if _, err := os.ReadDir(path); err != nil {
		return true
	}
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

// Everything this tool has staged in the fixture so far, which is the whole of what it can do to a
// human's index: the one write in `ai/tools/repo`'s port is Add, and the fake records every path it is
// given.
func (f *fixture) indexState() string {
	f.t.Helper()
	return "staged:" + sortedWords(f.staged())
}

// A counter around this fixture's fingerprint, for the case whose subject is how MANY times one run
// walks the tree. Wraps whatever the fixture already had rather than naming a recipe of its own, so
// there is one answer in this suite to where a fingerprint comes from.
func (f *fixture) countFingerprints() *int {
	f.t.Helper()
	calls := 0
	walk := f.fingerprint
	f.fingerprint = func(root string) (string, error) {
		calls++
		return walk(root)
	}
	return &calls
}

func (f *fixture) nonEmptyLinesIn(path string) int {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return countNonEmptyLines(string(content))
}

// Say which file git would name as ignoring each ship's working files, for the cases whose subject is
// that the source decides. `.gitignore` travels with the repository and nothing else does, which is the
// difference the tool turns on.
//
// The entries are expanded here because `check-ignore` reads its argument as a literal pathname — an
// entry holding `*` can only be asked about through a path it covers, which is what the tool's own
// ignoreProbe does.
func (f *fixture) ignoreShipFiles(source string, slugs ...string) {
	f.t.Helper()
	var paths []string
	for _, entry := range ignoreEntries() {
		for _, slug := range append([]string{"__probe__"}, slugs...) {
			paths = append(paths, f.canonicalRepo()+"/"+strings.Replace(entry, "*", slug, 1))
		}
	}
	f.ignore(source, paths...)
}

// The fake, plus the one thing a table cannot hold on its own: after `git add`, the index HOLDS what
// was staged. `promote` reads exactly that back — through repoMode, and deliberately rather than
// through the add's exit, because `git add` on a directory whose every file is ignored stages nothing
// and still succeeds. A fake that only recorded the request would answer "still external" and every
// promotion would refuse.
//
// Only the files git would really have staged: an ignorable ship working file is not added, which is
// the state that refusal exists for.
type stagingRepo struct {
	*repotest.Fake
	f *fixture
}

// The tree's own .gitignore decides first, as it does for git; anything a case stated answers what it
// does not cover. gitignoreSourceFor holds why this is read per question rather than arranged once.
func (s stagingRepo) IgnoreSource(dir, full string) (string, error) {
	defer s.holdTheRepository()()
	if source, matched := s.f.gitignoreSourceFor(full); matched {
		return source, nil
	}
	return s.Fake.IgnoreSource(dir, full)
}

func (s stagingRepo) Add(dir string, paths []string) error {
	defer s.holdTheRepository()()
	if err := s.Fake.Add(dir, paths); err != nil {
		return err
	}
	// A path NAMED to `git add` and covered by an ignore rule is refused outright, where the same file
	// swept up by a directory pathspec is silently passed over. finalize names its records one by one
	// for exactly that reason, and this is the refusal it is counting on.
	for _, named := range paths {
		if source, matched := s.f.gitignoreSourceFor(named); matched && source != "" && s.f.isFile(named) {
			return fmt.Errorf("The following paths are ignored by one of your .gitignore files:\n%s\nhint: Use -f if you really want to add them.", named)
		}
	}
	for _, path := range paths {
		full := path
		if !filepath.IsAbs(full) {
			full = s.f.canonicalRepo() + "/" + path
		}
		for _, found := range s.f.find(full) {
			name := strings.TrimPrefix(found, s.f.canonicalRepo()+"/")
			if name == found || !s.f.isFile(found) || fingerprintSkips(name) {
				continue
			}
			s.Fake.Write(name, s.f.read(found))
			s.f.stagedFiles = append(s.f.stagedFiles, name)
		}
	}
	return nil
}

// Make one repository question fail, as an unreadable index or a concurrent `git add` makes git's own
// answer fail. `question` is the port method's name — "Tracked" is the index read the repo mode rests
// on, "Add" the one write this tool makes.
//
// Arranged rather than provoked through a permission bit. Root ignores mode bits, so the `chmod 000
// .git/index` form these cases used to take could not restrict a CI image running as one, and the case
// then skipped itself on exactly the machine that runs it most — testing.md → **4. Setup strategy**.
func (f *fixture) failsToAnswer(question, reason string) {
	f.t.Helper()
	f.fake.Fail[question] = errors.New(reason)
}

func (f *fixture) answersAgain(question string) {
	f.t.Helper()
	delete(f.fake.Fail, question)
}

// Every repository answer this tool asks for, serialised. One case submits two stage results at once,
// and both invocations then share one table — which records what it was asked in a slice of its own,
// so two unguarded callers lose entries or worse. The methods below are the ones this tool calls; the
// rest of the port reaches the fake unwrapped, and nothing here asks them.
func (s stagingRepo) TopLevel(dir string) (string, error) {
	defer s.holdTheRepository()()
	return s.Fake.TopLevel(dir)
}

func (s stagingRepo) CommonDir(dir string) (string, error) {
	defer s.holdTheRepository()()
	return s.Fake.CommonDir(dir)
}

func (s stagingRepo) GitPath(dir, name string) (string, error) {
	defer s.holdTheRepository()()
	return s.Fake.GitPath(dir, name)
}

func (s stagingRepo) Resolve(dir, rev string) (string, error) {
	defer s.holdTheRepository()()
	return s.Fake.Resolve(dir, rev)
}

func (s stagingRepo) Tracked(dir string, pathspec ...string) ([]string, error) {
	defer s.holdTheRepository()()
	return s.Fake.Tracked(dir, pathspec...)
}

// Read off the working tree rather than from a list a case kept in step with it: what `ls-files
// --others --exclude-standard` answers is "present on disk, not in the index, not ignored", and every
// case that writes a file into the fixture means exactly that. Declared over stated, so a case cannot
// write a file and forget to mention it — which would read as a scope with nothing in it.
func (s stagingRepo) Untracked(dir string, pathspec ...string) ([]string, error) {
	if _, err := s.Fake.Untracked(dir, pathspec...); err != nil {
		return nil, err
	}
	defer s.holdTheRepository()()
	var names []string
	for _, found := range s.f.find(s.f.canonicalRepo()) {
		name := strings.TrimPrefix(found, s.f.canonicalRepo()+"/")
		if name == found || strings.HasPrefix(name, ".git/") || !s.f.exists(found) {
			continue
		}
		if info, err := os.Lstat(found); err != nil || info.IsDir() {
			continue
		}
		if _, held := s.Fake.Revs[repotest.WorkTree][name]; held {
			continue
		}
		if source, matched := s.f.gitignoreSourceFor(found); matched && source != "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (s stagingRepo) Blob(dir, id string) ([]byte, int64, error) {
	defer s.holdTheRepository()()
	return s.Fake.Blob(dir, id)
}

func (s stagingRepo) holdTheRepository() func() {
	s.f.asking.Lock()
	return s.f.asking.Unlock
}
