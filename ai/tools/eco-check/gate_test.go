package ecocheck_test

// The --gate flag: whether two checkouts of one commit can be made to answer differently.
//
// Nothing here forks git. What this package does is the FILTER — it drops a path from the walk, drops
// it from citation resolution, reports the count, and refuses where the question went unanswered — and
// a fake stating what git said serves all four.
//
// Git's own reading of a tree used to be argued here, because the call site was in this package. It is
// the ADAPTER's now, and `repo/exec_test.go` holds it against a real repository: a tracked file
// matching an ignore rule is not ignored, a path holding a newline survives the process boundary, and
// matching nothing is check-ignore's exit 1 rather than a failure.
//
// A case says what git answered by naming paths relative to the root under review, which is how
// `newIgnoredPaths` asks and how git answers.

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	ecocheck "configs/ai/tools/eco-check"
	"configs/ai/tools/repo/repotest"
)

const (
	gateLine      = "gitignored path(s) skipped"
	notChecked    = "check.sh: exit 2 — nothing was checked"
	gateUsage     = "usage: check.sh --agent=claude|codex [--gate] [<root>]"
	unanswerable  = "git check-ignore could not answer"
	citedSection  = "**A Real Section**"
	missingRegion = "**No Such Section**"
)

// The tree every gate case starts from, with the port left answering that nothing is ignored.
func newGatedRoot(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	// The file every citation here points at, so a citation's target resolves and only what a case
	// varies decides the finding. Committable, so the gate keeps it.
	f.write(f.root+"/kk-flavor/one.md", "# One\n\n## A Real Section\n")
	return f
}

// What git answered, as paths relative to the root under review. A directory is named without a
// trailing slash and everything under it is ignored with it, which is what check-ignore reports when
// the walk hands it a directory and the files inside it.
//
// Stated whole rather than appended to, so a case says the entire set it means and nothing a builder
// left behind decides its result.
func (f *fixture) ignores(paths ...string) {
	f.t.Helper()
	f.git.IgnoredPaths = paths
}

// What `repo.Exec` hands back when git refuses the question: git's own words behind the command that
// asked. The refusal cases below are about what this package does with that, so the shape is stated
// once here — `repo/exec.go`'s gitError is where it is made.
var gitRefused = errors.New("git check-ignore --stdin: fatal: not a git repository (or any of the parent directories): .git")

func newRefusingGit(root string) *repotest.Fake {
	git := repotest.New(root)
	git.Fail["Ignored"] = gitRefused
	return git
}

// A markdown file under skills/ citing a section that does not exist: the shape a scratch record takes,
// and a finding a commit cannot carry.
func (f *fixture) newScratchRecord(path string) {
	f.t.Helper()
	f.write(f.root+"/kk-flavor/skills/"+path, "# Scratch\n\nSee one.md → "+missingRegion+".\n")
}

func (f *fixture) runGated() string {
	f.t.Helper()
	f.isolate()
	return f.checkWith("--gate", f.root)
}

// The same tree judged twice in one case — once as the commit gate asks and once as a bare run does.
// A case needs both halves or it observes nothing: that the flag drops a finding is only a fact beside
// the run that still raises it. One call, because isolate may be reached only once per case.
func (f *fixture) bothWays() (gated, bare string) {
	f.t.Helper()
	f.isolate()
	return f.checkWith("--gate", f.root), f.checkWith(f.root)
}

func (f *fixture) assertHolds(what, output, needle string) {
	f.t.Helper()
	if !strings.Contains(output, needle) {
		f.t.Errorf("%s: expected %q\n%s", what, needle, indent(output))
	}
}

func (f *fixture) assertLacks(what, output, needle string) {
	f.t.Helper()
	if strings.Contains(output, needle) {
		f.t.Errorf("%s: expected nothing containing %q\n%s", what, needle, indent(output))
	}
}

// The defect the flag exists for: a file no commit can carry decided the verdict.
func TestAGitignoredFileIsJudgedWithoutTheFlagAndNotWithIt(t *testing.T) {
	f := newGatedRoot(t)
	f.newMountedSkill("kk-scratchy")
	f.newScratchRecord("kk-scratchy/findings.md")
	f.ignores("kk-flavor/skills/kk-scratchy/findings.md")

	gated, bare := f.bothWays()
	f.assertHolds("a bare run judges the checkout it is in", bare, dangling)
	f.assertLacks("--gate judges only what a commit carries", gated, dangling)
}

// The other half of the same guard, and what makes the flag safe to put in the gate: the filter drops
// only what git named. A skill somebody has just written and not yet staged is untracked and no answer
// names it, so the check that proves a new skill is mounted goes on seeing it — where a blanket
// untracked-skip would hide the whole directory from that check.
//
// That a tracked file matching an ignore rule is not among the names git returns is git's own reading
// of a tree, and `repo/exec_test.go` holds it there against a real repository.
func TestAFileTheAnswerDoesNotNameIsStillJudgedUnderTheFlag(t *testing.T) {
	f := newGatedRoot(t)
	f.newMountedSkill("kk-scratchy")
	f.newScratchRecord("kk-scratchy/notes.md")
	f.ignores()

	f.assertHolds("an unstaged file is content a commit can carry", f.runGated(), dangling)
}

// A committed filename may hold a newline, and the filter round-trips every name git returns: joined
// back onto the root as the caller spelled it, cleaned, and compared against the walk's own path. A
// name that survives git and then fails to match here leaves an ignored file judged, which is the
// whole defect.
//
// That the name survives the process boundary at all is `-z` on both ends of it, which is
// `repo/exec_test.go`'s against a real git.
func TestAGitignoredFileWhoseNameHoldsANewlineIsStillFilteredOut(t *testing.T) {
	f := newGatedRoot(t)
	f.newMountedSkill("kk-scratchy")
	f.newFileWithNewlineName(f.root+"/kk-flavor/skills/kk-scratchy/two\nlines.md",
		"# Scratch\n\nSee one.md → "+missingRegion+".", "the newline-name gate case")
	f.ignores("kk-flavor/skills/kk-scratchy/two\nlines.md")

	gated, bare := f.bothWays()
	f.assertHolds("a bare run reads it", bare, dangling)
	f.assertLacks("--gate drops it whatever its name holds", gated, dangling)
}

// The resolve half. A citation is answered from a path spelled out in prose as well as from the walk,
// so a gitignored file left reachable there would still make a reference resolve — the gate reading
// green over a tree a fresh clone reports on.
func TestACitationResolvingOnlyThroughAGitignoredFileDangles(t *testing.T) {
	f := newGatedRoot(t)
	f.newMountedSkill("kk-scratchy")
	f.write(f.root+"/kk-flavor/skills/kk-scratchy/SKILL.md",
		"---\nname: kk-scratchy\ndescription: cites a file only this checkout holds\n---\n\nSee notes.md → "+citedSection+".\n")
	f.write(f.root+"/kk-flavor/skills/kk-scratchy/notes.md", "# Notes\n\n## A Real Section\n")
	f.ignores("kk-flavor/skills/kk-scratchy/notes.md")

	gated, bare := f.bothWays()
	f.assertLacks("the citation resolves against the checkout's own file", bare, unresolved)
	f.assertHolds("--gate answers as a fresh clone does", gated, unresolved)
}

// skills/ is listed off the directory rather than out of the walk, and the mount scan reads that list.
// A gitignored skill directory is `skill not mounted` in the install and nothing at all in a worktree
// of the same commit, which is the split this flag closes.
func TestAGitignoredSkillDirectoryIsNotCounted(t *testing.T) {
	f := newGatedRoot(t)
	// Installed, because the mount scan is the half only this list decides: everything else about a
	// skill directory is reachable through the walk, so a clone's fixture would leave the guard
	// unobserved and this case passing whether or not it is there.
	f.newHome()
	f.mkdirAll(f.home + "/.claude/skills")
	f.newMountedSkill("kk-real")
	f.symlink(f.root+"/kk-flavor/skills/kk-real", f.home+"/.claude/skills/kk-real")
	f.newMountedSkill("kk-local")
	f.ignores("kk-flavor/skills/kk-local")

	gated, bare := f.bothWays()
	f.assertHolds("a bare run wants the local directory mounted too", bare, ecocheck.SkillNotMounted)
	f.assertLacks("--gate asks only about the skills a commit carries", gated, ecocheck.SkillNotMounted)
	f.assertHolds("a bare run counts the directory sitting there", bare, "across 2 of 2 skills")
	f.assertHolds("--gate counts the skills a commit carries", gated, "across 1 of 1 skills")
	// A wholly ignored directory is one line, not one per file under it: the count reads as how much
	// of the tree went unjudged, and the SKILL.md inside is not a second thing to fix.
	f.assertHolds("the directory is named once, not every file inside it", gated, "1 "+gateLine)
}

// The names ride the exit-0 path, where a tree that authored them prints under `wiring: clean`. The
// count stays exact and only the naming is trimmed, so both halves are asserted here.
func TestTheSkippedNamesAreBoundedAndTheCountIsNot(t *testing.T) {
	f := newGatedRoot(t)
	f.newMountedSkill("kk-scratchy")
	var ignored []string
	for _, name := range []string{"one", "two", "three", "four", "five", "six", "seven"} {
		f.write(f.root+"/kk-flavor/skills/kk-scratchy/local-"+name+".md", "# "+name+"\n")
		ignored = append(ignored, "kk-flavor/skills/kk-scratchy/local-"+name+".md")
	}
	f.ignores(ignored...)

	gated := f.runGated()
	f.assertHolds("the count is every one of them", gated, "7 "+gateLine)
	f.assertHolds("and the naming stops at the bound", gated, "and 2 more")
}

// The line is the flag's own report, and its absence is what says a run judged the whole directory.
func TestTheFlagSaysWhatItSkipped(t *testing.T) {
	f := newGatedRoot(t)
	f.newMountedSkill("kk-scratchy")
	f.newScratchRecord("kk-scratchy/findings.md")
	f.ignores("kk-flavor/skills/kk-scratchy/findings.md")

	gated, bare := f.bothWays()
	// The skip line itself, not the whole report: a filter that dropped nothing leaves the dangling
	// citation in, and that finding names this same path — so a case reading the whole output would go
	// on passing over a gate that filtered nothing.
	f.assertHolds("the flag names the path it dropped", lineWith(gated, gateLine), "skills/kk-scratchy/findings.md")
	f.assertHolds("and says what the count is about", gated, gateLine)
	f.assertLacks("a bare run claims no filtering", bare, gateLine)
}

// An empty answer is a clean checkout, so the run scans it and says it filtered nothing. Read as a
// failure instead, the flag would refuse every clean checkout it exists to serve — and the empty
// answer is what git's exit 1 arrives as, which `repo/exec_test.go` holds against a real repository.
func TestTheFlagRunsOnATreeWithNothingIgnored(t *testing.T) {
	f := newGatedRoot(t)
	f.newMountedSkill("kk-real")
	f.ignores()

	gated := f.runGated()
	f.assertHolds("the run reports its budget", gated, "always-loaded:")
	f.assertHolds("and says it filtered nothing", gated, "0 "+gateLine)
}

func TestTheFlagRefusesWhereGitCannotAnswer(t *testing.T) {
	f := newRoot(t)
	f.git = newRefusingGit(f.root)
	f.isolate()

	var output bytes.Buffer
	if status := ecocheck.Run([]string{"--agent=claude", "--gate", f.root}, f.git, f.bash, &output, &output); status != 2 {
		f.t.Fatalf("expected exit 2 where git could not answer, got %d\n%s", status, indent(output.String()))
	}
	f.assertHolds("the refusal names what could not answer", output.String(), unanswerable)
	f.assertHolds("and stops it being read as clean", output.String(), notChecked)
	f.assertLacks("no scan ran", output.String(), "always-loaded:")
}

// An argument that is neither the flag nor a root is refused rather than taken for a root: read as one
// it reports that the *tree* is not a checkout, which sends its reader to the wrong file.
func TestAnUnknownArgumentIsRefused(t *testing.T) {
	f := newRoot(t)
	f.isolate()

	for _, args := range [][]string{{"--tracked-only", f.root}, {f.root, f.root}} {
		var output bytes.Buffer
		if status := ecocheck.Run(append([]string{"--agent=claude"}, args...), f.git, f.bash, &output, &output); status != 2 {
			f.t.Errorf("expected exit 2 for %q, got %d\n%s", args, status, indent(output.String()))
		}
		f.assertHolds("the refusal names the usage", output.String(), gateUsage)
	}
}

// A path a scan names for itself rather than taking from the walk — the root CLAUDE.md, a skill's own
// SKILL.md, a doc the router lists. Each was a second existence oracle behind the flag's back, and a
// gitignored SKILL.md was the one that read green over a tree every fresh clone reports on.
func TestAGitignoredSkillFileLeavesItsDirectoryWithoutOne(t *testing.T) {
	f := newGatedRoot(t)
	f.newMountedSkill("kk-scratchy")
	f.ignores("kk-flavor/skills/kk-scratchy/SKILL.md")

	gated, bare := f.bothWays()
	f.assertLacks("the checkout has the file sitting right there", bare, ecocheck.SkillDirWithoutSkillFile)
	f.assertHolds("--gate answers as a fresh clone does", gated, ecocheck.SkillDirWithoutSkillFile)
	f.assertHolds("and counts no skill it cannot read a description from", gated, "across 0 of 0 skills")
}

// The router's own file. Gitignored, its lines are not in the commit, so neither the budget figure
// they feed nor the direction scan that reads them may be taken from this checkout.
func TestAGitignoredClaudeMdIsNeitherCountedNorScanned(t *testing.T) {
	f := newGatedRoot(t)
	f.newMountedSkill("kk-real")
	f.write(f.root+"/CLAUDE.md", "# Root\n\nRun kk-real for it.\n")
	f.ignores("CLAUDE.md")

	gated, bare := f.bothWays()
	f.assertHolds("a bare run counts both router files", bare, "across 2 files")
	f.assertHolds("--gate counts the one a commit carries", gated, "across 1 files")
	f.assertHolds("a bare run reads its prose as the shared layer's", bare, names)
	f.assertLacks("--gate reads no prose the commit does not carry", gated, names)
}

// A doc the router lists that only this checkout holds. Reported absent, in the wording a clone's run
// uses — never as a file nothing could answer for, which is what an Lstat over the present file says.
func TestAGitignoredReadAlwaysTargetIsReportedAbsent(t *testing.T) {
	f := newGatedRoot(t)
	f.newMountedSkill("kk-real")
	f.write(f.root+"/kk-flavor/inject.md", "# Flavor\n\n## Read always\n\n- [standards/local.md](standards/local.md)\n")
	f.write(f.root+"/kk-flavor/standards/local.md", "# Local\n")
	f.ignores("kk-flavor/standards/local.md")

	gated, bare := f.bothWays()
	f.assertHolds("a bare run counts it into the budget", bare, "across 2 files")
	f.assertHolds("--gate counts only what a commit carries", gated, "across 1 files")
	f.assertHolds("and says the router lists a file the tree does not hold", gated, "does not exist")
	f.assertLacks("never that nothing could answer for it", gated, ecocheck.BudgetFileRefused)
}

// A citation is a path *prose* wrote, so it arrives spelled however its author spelled it. Compared
// as written, `./notes.md` misses an ignored set keyed on `notes.md` and names the same file: the run
// printed that file on its own skipped list and then resolved a reference through it, reporting
// `wiring: clean` where a fresh clone reports an unresolvable citation.
func TestACitationSpelledNonCanonicallyStillHitsTheGate(t *testing.T) {
	f := newGatedRoot(t)
	f.newMountedSkill("kk-demo")
	f.write(f.root+"/kk-flavor/skills/kk-demo/SKILL.md",
		"---\nname: kk-demo\ndescription: cites a file only this checkout holds\n---\n\nSee ./notes.md → "+citedSection+".\n")
	f.write(f.root+"/kk-flavor/skills/kk-demo/notes.md", "# Notes\n\n## A Real Section\n")
	f.ignores("kk-flavor/skills/kk-demo/notes.md")

	gated, bare := f.bothWays()
	f.assertLacks("the citation resolves against the checkout's own file", bare, unresolved)
	f.assertHolds("--gate answers as a fresh clone does, however the path was spelled", gated, unresolved)
}

// One root under every spelling, gated against bare. The flag decides which files a run judges; it
// may never decide how a path is *keyed*, and the two runs must agree for each spelling with only the
// skip line between them.
//
// What it catches is a field going missing from the filtered tree. keepCommittable rebuilds the walk
// through tree.add, so whatever the walk records about itself has to be carried across; carried
// wrong, the gated index keys differently from the bare one, the citation below stops resolving under
// --gate alone, and the extra finding shows here. No bare run can show it, and no case that fixes the
// root's spelling can either.
//
// Its own fixture rather than the shared one: a relative spelling needs t.Chdir, t.Chdir bars
// t.Parallel, and isolate() decides parallelism once per case. Still newBase rather than t.TempDir,
// because this case and the one below compare two runs line for line — and a root long enough for the
// printer's bound to cut both sides alike makes that comparison hold over nothing at all.
func TestAGatedRunAndABareRunAgreeHoweverTheRootIsNamed(t *testing.T) {
	base := newBase(t)
	git := newSpellingTree(t, base)

	t.Chdir(base)
	for _, spelling := range []string{"r", "r/", "./r", "./r/", base + "/r", base + "/r/"} {
		assertGatedMatchesBare(t, git, spelling)
	}

	// The root named from inside itself, the spelling with no leading component at all.
	t.Run("and from inside the root", func(t *testing.T) {
		t.Chdir(base + "/r")
		assertGatedMatchesBare(t, git, ".")
	})
}

// A tree holding the two things that case needs: a citation resolving through the walk's suffix index
// rather than through a path spelled out in prose, and a gitignored file with nothing to do with it.
// The ignored file is not what the assertion is about — it is there so the filter runs and the index
// is rebuilt, which is where a dropped field would bite.
//
// The port comes back with it, answering for that one file under every spelling of the root: the walk
// hands git paths with the root's own prefix cut off, so what git is asked about does not move.
func newSpellingTree(t *testing.T, base string) *repotest.Fake {
	t.Helper()
	root := base + "/r"
	for _, dir := range []string{root + "/kk-flavor/standards", root + "/kk-flavor/skills/kk-one"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	write(root+"/kk-flavor/inject.md", "# Flavor\n")
	// Cited by a bare name sitting in neither the citing file's directory nor the root, so the only
	// thing that can resolve it is the suffix index.
	write(root+"/kk-flavor/standards/deep.md", layerLine+"# Deep\n\n## A Real Section\n")
	write(root+"/kk-flavor/skills/kk-one/SKILL.md",
		"---\nname: kk-one\ndescription: cites a file only the index can reach\n---\n\nSee deep.md → "+citedSection+".\n")
	// A path ref naming the tree from its ROOT, which only the whole-path index can resolve. Spelled
	// `r/...`, it is the entire walked path when the root is named `r` with nothing before it, so `*/`
	// has no leading component to consume. Every other ref in this fixture resolves some other way.
	write(root+"/kk-flavor/standards/rooted.md",
		layerLine+"# Rooted\n\nThe router is `r/kk-flavor/inject.md`.\n")
	write(root+"/local.txt", "scratch\n")

	git := repotest.New(root)
	git.IgnoredPaths = []string{"local.txt"}
	return git
}

// Both runs over one spelling, compared as sets of lines with the skip line removed — the one line the
// flag is allowed to add.
// The findings are a fact about the tree, so every spelling of one root must produce the same set.
// They did not: a ref naming the tree from its root resolved through `./r` and dangled through `r`,
// because the suffix index held only the tails after each `/` and `*/` had nothing to consume; and a
// trailing slash doubled the separator in every walked path, so `r/` matched nothing that `r` did.
func TestTheFindingsAreTheSameHoweverTheRootIsNamed(t *testing.T) {
	base := newBase(t)
	git := newSpellingTree(t, base)
	t.Chdir(base)

	want := runLines(t, git, "./r")
	// A control on the instrument: a fixture reporting nothing would make every comparison below hold
	// no matter what the checker did with the spelling.
	if len(want) == 0 {
		t.Fatal("the fixture produced no output at all, so comparing spellings checks nothing")
	}
	for _, spelling := range []string{"r", "r/", "./r/", base + "/r", base + "/r/"} {
		got := runLines(t, git, spelling)
		if !slices.Equal(want, got) {
			t.Errorf("root spelled %q reports a different tree from \"./r\"\nwant:\n%s\ngot:\n%s",
				spelling, indent(strings.Join(want, "\n")), indent(strings.Join(got, "\n")))
		}
	}
}

func assertGatedMatchesBare(t *testing.T, git *repotest.Fake, root string) {
	t.Helper()
	bare := runLines(t, git, root)
	gated := runLines(t, git, "--gate", root)
	if !slices.Equal(bare, gated) {
		t.Errorf("root spelled %q: --gate reports a different tree from a bare run\nbare:\n%s\ngated:\n%s",
			root, indent(strings.Join(bare, "\n")), indent(strings.Join(gated, "\n")))
	}
}

func runLines(t *testing.T, git *repotest.Fake, args ...string) []string {
	t.Helper()
	output := runChecker(t, git, newFakeBash(t), append([]string{"--agent=claude"}, args...)...)
	var kept []string
	for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		if !strings.HasPrefix(line, "gate: ") {
			kept = append(kept, line)
		}
	}
	slices.Sort(kept)
	return kept
}

// The three reaches that ask about a skill's own SKILL.md by name rather than through the walk: the
// lane alternation, the whole-token re-test that alternation's matches are checked against, and the
// unknown-skill scan. A gitignored SKILL.md left in any of them makes the gated run call a skill known
// that a fresh clone calls unknown.
//
// Three shapes on one tree, because each reach is masked by a different sibling if a case names only
// its own. The alternation is reached through a *citation*, which nothing re-tests; the whole-token
// re-test needs a token that is not itself a lane name, so `kk-real-extra` rides in on `kk-real`; and
// the unknown-skill scan is reached by the bare name.
func TestAGitignoredSkillFileIsNotALaneUnderTheFlag(t *testing.T) {
	f := newGatedRoot(t)
	f.newMountedSkill("kk-real")
	f.newMountedSkill("kk-scratchy")
	f.newScript("kk-scratchy/scripts/thing.sh", "true")
	f.newMountedSkill("kk-real-extra")
	f.write(f.root+"/kk-flavor/standards/shared.md",
		"# Shared\n\nSee ~/.kk-flavor/skills/kk-scratchy/scripts/thing.sh for it.\n\nRun kk-real-extra for it.\n")
	f.ignores("kk-flavor/skills/kk-scratchy/SKILL.md", "kk-flavor/skills/kk-real-extra/SKILL.md")

	gated, bare := f.bothWays()
	// The alternation. Its names reach the citation scan, which has no SKILL.md test of its own, so a
	// lane the gate should not know about shows up there and nowhere else.
	f.assertHolds("a bare run treats the gitignored skill as a lane", bare, cites)
	f.assertLacks("--gate knows no lane a commit does not carry", gated, cites)
	// The whole-token re-test, reached by a token that starts with a real lane name and is not one.
	f.assertHolds("a bare run names the token as a lane", bare, names)
	f.assertLacks("--gate does not", gated, names)
	// The unknown-skill scan, which is what a fresh clone reports instead.
	f.assertHolds("--gate reports the name as no skill at all, as a clone does", gated, ecocheck.UnknownSkillReferenced)
	f.assertLacks("where a bare run finds the file sitting there", bare, ecocheck.UnknownSkillReferenced)
}

// A leftover checkout inside the tree — another session's scratch, which this repository really grew
// at ai/tools/eco-report/.git. Nothing under a `.git` can reach a commit, and the ignored-path filter
// cannot drop it: those paths are neither tracked nor ignored, so they survive the filter and were
// scanned as if a commit could carry them.
//
// Its control is the case below, which plants the same finding one directory higher and requires it
// to be reported. Both are needed: green here alone is also what a scan that reads nothing returns.
func TestAScratchCheckoutInsideTheTreeIsNotScanned(t *testing.T) {
	f := newGatedRoot(t)
	scratch := f.root + "/tools/eco-report/.git/idsd"
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		t.Fatalf("planting a nested checkout: %v", err)
	}
	f.write(scratch+"/qualify-report.md", "# Report\n\nSee one.md → "+missingRegion+".\n")

	out := f.runGated()
	if strings.Contains(out, "qualify-report.md") {
		t.Errorf("the gate reported a finding inside a nested .git, which no commit can carry — the "+
			"walk is treating another checkout's state as committable content: %s", out)
	}
}

// The control for the case above: the same file one directory higher is committable, so its dangling
// citation has to be reported. Without this, a walk that read nothing at all would pass that case.
func TestTheSameScratchRecordOutsideAGitDirectoryIsScanned(t *testing.T) {
	f := newGatedRoot(t)
	visible := f.root + "/tools/eco-report/idsd"
	if err := os.MkdirAll(visible, 0o755); err != nil {
		t.Fatalf("planting the control: %v", err)
	}
	f.write(visible+"/qualify-report.md", "# Report\n\nSee one.md → "+missingRegion+".\n")

	out := f.runGated()
	if !strings.Contains(out, "qualify-report.md") {
		t.Errorf("the gate did not report a dangling citation in committable content, so the case "+
			"beside this one would pass over a walk that reads nothing: %s", out)
	}
}
