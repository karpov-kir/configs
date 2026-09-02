package ecoreport_test

// What a careless or hostile input can make this tool do on disk. These cases are grouped by the
// question they answer rather than by the file they exercise, the way mutants.go groups the same set.
// The question is the one the move out of the tree opened: the scratch directory is now chosen by a
// machine-local config file, lives outside the repository, holds the only copy of the intents, and is
// the directory `discard` removes.
//
// Every case here has a negative control in it, because each guards a silence: the wrong answer is not
// an error, it is a write that lands somewhere nobody was told about.

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestANonAbsoluteConfigHomeIsNotAnOverride(t *testing.T) {
	// Not parallel: t.Chdir is process-global, and the process's own directory is the input under test.
	f := newRepo(t)
	// The decoy a checkout would ship. `XDG_CONFIG_HOME=cfg` read as given resolves this file against
	// whatever directory the caller stands in — so the tree under review chooses where its own reports
	// are written and which directory `discard` removes. The XDG spec says to ignore a relative value,
	// and that is the whole of the fix.
	f.mkdirAll(f.repo + "/cfg/kk-flavor")
	f.write(f.repo+"/cfg/kk-flavor/idsd.conf", "root "+f.base+"/hijacked\n")
	t.Chdir(f.repo)
	f.configHome = "cfg"

	root := f.runReportStdout("root")
	f.record("a non-absolute config home is not read as an override",
		root == f.sharedIdsd(), "resolved: "+root+"\nwanted: "+f.sharedIdsd())

	// The negative control, and the one that would catch a fix that merely stopped printing the note:
	// the write has to land at the default location and nowhere else.
	f.runReport("check-ignore")
	f.runReport("init", "001-relative-config")
	f.record("and init wrote the report at the default location",
		f.status == 0 && f.isFile(f.sharedIdsd()+"/qualify-reports/001-relative-config-qualify-report.md"),
		f.evidence())
	f.record("and nothing was written where the relative file pointed",
		!f.exists(f.base+"/hijacked"), "")
	f.record("and the override was never announced, since there was none",
		!strings.Contains(f.out, "overridden by"), f.out)
}

func TestTheScratchDirectoryIsReadableByItsOwnerAlone(t *testing.T) {
	// Deliberately NOT parallel: the umask pinned on the next line is process-global, so running this
	// beside other cases would change the mode of every directory they create.
	//
	// Pinned because the control is otherwise umask-dependent: under umask 077 a mutated 0o777 yields
	// 0700 anyway, the mutant survives, and the case passes while observing nothing. 022 is the ordinary
	// default and makes 0o777 land as 0755, which the assertion below can tell apart.
	defer syscall.Umask(syscall.Umask(0o022))
	// What the mode buys is at init.go's MkdirAll, which is the one place in the tool that creates this
	// tree. Both assertions rest on this fixture reaching `init` before anything else does, and that is
	// not the order every skill takes — so the case pins what this tool does, not what the directory is
	// guaranteed to be.
	f := newRepo(t)
	f.runReport("check-ignore")
	f.runReport("init", "001-modes")
	f.record("fixture: the report was written, so init created the tree it sits in",
		f.status == 0 && f.isFile(f.reportPath("001-modes")), f.evidence())

	// Both levels, because MkdirAll builds every missing parent with the same mode — a missing override
	// root included, which is the one a human never sees created.
	for _, dir := range []string{f.sharedIdsd(), f.sharedIdsd() + "/qualify-reports"} {
		info, err := os.Stat(dir)
		mode := "unreadable"
		if err == nil {
			mode = info.Mode().Perm().String()
		}
		f.record(dir+" is reachable by its owner alone",
			err == nil && info.Mode().Perm() == 0o700, "mode: "+mode+", wanted -rwx------")
	}
}

func TestPromoteRefusesASymlinkedScratchRatherThanCommittingTheLink(t *testing.T) {
	t.Parallel()
	// `promote` is the one subcommand that stages, and a rename does not follow a link at its source, so
	// a symlinked scratch dir lands in the tree as a link and reaches every clone that pulls. The full
	// mechanism is at assertScratchDirsAreReal; what is pinned here is that the refusal comes first, so
	// none of it happens.
	f := newRepo(t)
	f.runReport("check-ignore")
	outside := f.base + "/outside-promote"
	f.mkdirAll(outside + "/qualify-reports")
	f.write(outside+"/qualify-reports/001-linked-qualify-report.md", "---\nintent: 001-linked\n---\n")
	// Something durable, so promote has a reason to get as far as the move: without it the refusal below
	// could be the nothing-to-promote guard instead, and the case would pass observing nothing.
	f.write(outside+"/intents-placeholder.md", "# intent\n")
	f.symlink(outside, f.sharedIdsd())

	f.runReport("promote")
	f.assertRefused("promote refuses a symlinked scratch directory")
	f.assertReports("is a symlink", "and names the link rather than the git answer downstream of it")
	f.record("and committed no link into the tree", !f.exists(f.treeIdsd()), "")
	staged, _ := f.git("diff", "--cached", "--name-only")
	f.record("and staged nothing", !strings.Contains(staged, ".idsd"), "staged:\n"+staged)
	f.record("and left the report where it was, outside the tree",
		f.isFile(outside+"/qualify-reports/001-linked-qualify-report.md"), "")
	f.remove(f.sharedIdsd())
}

func TestAReportStemCannotNameTheGitDirItself(t *testing.T) {
	t.Parallel()
	// A stem of `..` reaches `gitPath("idsd-stage-returns/" + stem)` bare, where it IS the git dir, and
	// three subcommands os.RemoveAll that path. reportNameFor's leading-dot case is what stands there,
	// and holds why Go's RemoveAll does not stop it.
	f := newShip(t, "001-real")
	head := f.mustGit("rev-parse", "HEAD")
	// What makes the stem reachable at all, and the fixture without which this case cannot fail: every
	// subcommand needs the report to be there, so a stem of `..` only gets past requireReport when a
	// file of that name exists. This tool will not create one — reportNameFor refuses the leading dot —
	// but a committed one arrives through someone else's branch, and the argument is then all it takes.
	f.write(f.scratch()+"/qualify-reports/..-qualify-report.md",
		"---\nintent: 001-real\nreviewed-tree: pending\nreviewed-worktree: pending\nreviewed-stages: pending\n---\n")
	f.record("fixture: a report named for the stem under test is in place",
		f.isFile(f.scratch()+"/qualify-reports/..-qualify-report.md"), "")
	for _, subcommand := range []string{"invalidate", "close", "discard", "state", "gate"} {
		f.runReport(subcommand, "..")
		f.assertRefused(subcommand + " refuses '..' as an intent name")
	}
	// The harm, asserted where it would land. The exit alone would not tell a refusal from a run that
	// deleted the git dir and then failed for want of it.
	f.record("the git dir is intact",
		f.isFile(f.repo+"/.git/HEAD") && f.isFile(f.repo+"/.git/index"),
		joinLines(f.entries(f.repo+"/.git")))
	f.record("and git still answers for the repository",
		f.mustGit("rev-parse", "HEAD") == head, head)
	f.record("and the real ship is untouched", f.isFile(f.reportPath("001-real")), "")
}

func TestAWorktreePathCarriesNoControlByteIntoTheReport(t *testing.T) {
	t.Parallel()
	// The stamp records `<token> <worktree path>`, and the path half is not a value this tool chose:
	// `git rev-parse --show-toplevel` hands back a control byte in a checkout's path verbatim. A newline
	// there opens a second frontmatter line that every reader reaches ahead of the real one, which is a
	// forged stage record — currentWorktreeRecord in worktree.go has the ordering that makes it one.
	f, ok := newRepoAtAControlBytePath(t)
	if !ok {
		t.Skip("this filesystem refused a carriage return in a directory name, so this case cannot run here")
	}
	f.runReport("check-ignore")
	f.runReport("init", "099-cr-path")
	f.record("fixture: a repository whose own path holds a carriage return",
		f.status == 0 && strings.ContainsRune(f.repo, '\r') && f.isFile(f.reportPath("099-cr-path")),
		f.evidence())
	f.stampFullPass("099-cr-path")

	report := f.read(f.reportPath("099-cr-path"))
	f.record("the stamp landed", containsLine(report, "reviewed-stages: code-review,security-review,tighten,refactor"), report)
	f.record("and no control byte from the path reached the frontmatter",
		!strings.ContainsRune(report, '\r'), report)
	// One of each line, which is the property a newline in that value would have broken by opening a
	// second one the readers reach first.
	f.record("and the frontmatter still carries one reviewed-stages line",
		countLinesWithPrefix(report, "reviewed-stages:") == 1 &&
			countLinesWithPrefix(report, "reviewed-worktree:") == 1, report)
	f.runReport("gate", "099-cr-path")
	f.record("and the gate reads it as this worktree's own review", f.status == 0, f.evidence())
}

func TestAReviewedWorktreeValueCarriesNoControlByteToTheTerminal(t *testing.T) {
	t.Parallel()
	// The echo half. The gate quotes that field back when it blocks, and the value is a line out of a
	// hand-editable report, so an ESC in it rewrites the lines printed above — a crafted one can put
	// `gate clean` on screen where a BLOCK was printed. The exit code still blocks, which is what keeps
	// this to the terminal; what reads this output is an agent as often as a person.
	f := newShip(t, "099-echoed")
	f.newIntentFile("099-echoed")
	f.stampFullPass("099-echoed")
	f.runReport("gate", "099-echoed")
	f.record("fixture: it gates clean before the value is tampered with", f.status == 0, f.evidence())

	f.replaceLine(f.reportPath("099-echoed"), "reviewed-worktree:",
		"reviewed-worktree: 0123456789abcdef /somewhere\x1b[2Kgate clean: tree fresh, untrimmed qualify, no open TODOs")
	f.runReport("gate", "099-echoed")
	f.record("the gate still blocks for a review recorded against another worktree",
		f.status != 0, f.evidence())
	f.assertReports("reviewed in another worktree", "and says why it blocked")
	f.record("and no control byte from that value reached the output",
		!strings.ContainsRune(f.out, 0x1b), f.out)

	// The other branch that quotes the same field, and it is the one where the value is arbitrary by
	// definition: a first field that is not a well-formed token cannot be compared, so the report is
	// blocked for recording no usable worktree — and whatever text failed that check is what gets
	// echoed. Asserted separately because the two branches are two call sites.
	f.replaceLine(f.reportPath("099-echoed"), "reviewed-worktree:",
		"reviewed-worktree: not-a-token\x1b[2Kgate clean: tree fresh, untrimmed qualify, no open TODOs")
	f.runReport("gate", "099-echoed")
	f.record("a value that is not a usable worktree token also blocks", f.status != 0, f.evidence())
	f.assertReports("no usable reviewing worktree", "and says that is why")
	f.record("and no control byte reached the output from that branch either",
		!strings.ContainsRune(f.out, 0x1b), f.out)
}

func TestAReportRewriteIsStagedBesideTheReport(t *testing.T) {
	t.Parallel()
	// Where the temp file lives decides whether the rewrite can be atomic at all; rewriteReport says why.
	//
	// Pinned through which failure fires when the reports directory cannot be written: staged beside the
	// report, the temp file itself cannot be created, and that is a different sentence from a write that
	// got as far as the move.
	f := newShip(t, "099-staged")
	f.chmod(f.scratch()+"/qualify-reports", 0o500)
	f.runReport("invalidate", "099-staged")
	status := f.status
	output := f.out
	f.chmod(f.scratch()+"/qualify-reports", 0o755)
	if status == 0 {
		t.Skip("this process writes into a mode-0500 directory regardless of the mode (root, or CAP_DAC_OVERRIDE), so this case cannot be built here")
	}
	f.record("the rewrite fails at creating its temp file, which is beside the report",
		strings.Contains(output, "mktemp failed"),
		"exit "+strconv.Itoa(status)+"\n"+output)
	f.record("and the report is left exactly as it was",
		containsLine(f.read(f.reportPath("099-staged")), "reviewed-tree: <hash>"),
		f.read(f.reportPath("099-staged")))
}

// A fixture repository whose own path holds a carriage return — the ordinary builder, under a name
// that is the input under test. False means this filesystem refused the name, which is probed first:
// the builder fails the case on anything that goes wrong, and "this machine cannot hold such a name"
// is the one outcome that is not a failure.
func newRepoAtAControlBytePath(t *testing.T) (*fixture, bool) {
	t.Helper()
	const name = "cr\rrepo"
	if err := os.MkdirAll(t.TempDir()+"/"+name, 0o755); err != nil {
		return nil, false
	}
	return newRepoNamed(t, name), true
}

func TestAnUntrustworthyOverrideRootIsRefused(t *testing.T) {
	// Why an override root's mode, its link status and the directories above it all matter is in
	// assertOverrideRootIsTrustworthy. The refusals come first, then the ordinary roots that must not
	// trip them — and the two allowances the ancestor walk deliberately makes are among the latter,
	// because a check nobody can live with gets the config file deleted rather than the root moved.
	t.Run("a symlinked root is refused", func(t *testing.T) {
		f := newRepo(t)
		real := f.base + "/real-elsewhere"
		f.mkdirAll(real)
		f.symlink(real, f.base+"/linked-elsewhere")
		f.writeOverride("root " + f.base + "/linked-elsewhere\n")
		f.runReport("check-ignore")
		f.assertRefused("a root that is a symlink is refused")
		f.assertReports("is a symlink", "and says so")
		// `init`, because `check-ignore` writes nothing anywhere: an empty target after it alone is what
		// this fixture started with, and the assertion below would hold with the guard deleted. `init` is
		// the first command that would build the scratch tree through the link, so running it is what
		// makes the emptiness evidence that the refusal came first.
		f.runReport("init", "001-linked")
		f.record("and nothing was written through it", len(f.entries(real)) == 0, strings.Join(f.entries(real), " "))
	})

	t.Run("a group- or world-writable root is refused", func(t *testing.T) {
		f := newRepo(t)
		loose := f.base + "/loose-elsewhere"
		f.mkdirAll(loose)
		f.chmod(loose, 0o777)
		f.writeOverride("root " + loose + "\n")
		f.runReport("check-ignore")
		f.assertRefused("a group- or world-writable root is refused")
		f.assertReports("group- or world-writable", "and says which permission is the problem")
	})

	// A root whose own mode is impeccable and whose parent is not. Checking the final component alone
	// passed this, and the substitution it leaves open needs no privilege at all: any account with write
	// on the parent runs `mv root root.stolen && mkdir root`, and the next init writes every report into
	// the directory they left behind while `discard` removes it for them.
	t.Run("a root under a group- or world-writable parent is refused", func(t *testing.T) {
		f := newRepo(t)
		parent := f.base + "/loose-parent"
		f.mkdirAll(parent + "/root")
		f.chmod(parent+"/root", 0o700)
		f.chmod(parent, 0o777)
		defer f.chmod(parent, 0o755)
		f.writeOverride("root " + parent + "/root\n")
		f.runReport("check-ignore")
		f.assertRefused("a root under a writable parent is refused")
		f.assertReports("loose-parent", "and names the parent rather than the root, which is not the problem")
		f.runReport("init", "001-parent")
		f.record("and nothing was written under it",
			len(f.entries(parent+"/root")) == 0, strings.Join(f.entries(parent+"/root"), " "))
	})

	// The other half of the same hole, and the one that says the walk FOLLOWS a link instead of reading
	// the link's own attributes. The root is a real owner-only directory reached through a symlinked
	// ancestor; what stands behind that link is world-writable, so the refusal has to name what the link
	// points at. Checking the final component alone landed the reports in there and let `discard` follow
	// the link wherever it was next repointed.
	t.Run("and so is one reached through a symlinked ancestor to a writable directory", func(t *testing.T) {
		f := newRepo(t)
		behind := f.base + "/loose-behind-link"
		f.mkdirAll(behind + "/root")
		f.chmod(behind+"/root", 0o700)
		f.chmod(behind, 0o777)
		defer f.chmod(behind, 0o755)
		f.symlink(behind, f.base+"/via")
		f.writeOverride("root " + f.base + "/via/root\n")
		f.runReport("check-ignore")
		f.assertRefused("an ancestor symlink is followed, not trusted")
		f.assertReports("loose-behind-link", "and names what the link points at rather than the link")
	})

	// The ordinary cases, which must not be caught by any of those checks: a private directory, one under
	// a sticky world-writable ancestor, and one that does not exist yet because this is the first run.
	//
	// Each asserts that the override was HONOURED and not merely tolerated. Exit 0 alone is what a silent
	// fallback to the default also looks like, and writing this clone's reports into a directory the human
	// was never told about is the one failure an override must not have.
	t.Run("while an owner-only root is accepted", func(t *testing.T) {
		f := newRepo(t)
		tight := f.base + "/tight-elsewhere"
		f.mkdirAll(tight)
		f.chmod(tight, 0o700)
		f.writeOverride("root " + tight + "\n")
		f.runReport("check-ignore")
		f.record("an owner-only root is accepted", f.status == 0, f.evidence())
		f.record("and it is the location this run uses", strings.Contains(f.out, tight+"/"), f.evidence())
	})

	// The allowance /tmp rests on, here and on every Linux runner: sticky is exactly what stops another
	// account renaming an entry it does not own, so a sticky world-writable ancestor cannot host the
	// substitution the case above describes. Without the exemption this refuses, and the ancestor walk
	// outlaws the most ordinary place there is to keep scratch.
	t.Run("and so is one under a sticky world-writable ancestor", func(t *testing.T) {
		f := newRepo(t)
		sticky := f.base + "/sticky-parent"
		f.mkdirAll(sticky + "/root")
		f.chmod(sticky+"/root", 0o700)
		f.chmod(sticky, 0o777|os.ModeSticky)
		defer f.chmod(sticky, 0o755)
		f.writeOverride("root " + sticky + "/root\n")
		f.runReport("check-ignore")
		f.record("a sticky world-writable ancestor is accepted", f.status == 0, f.evidence())
		f.record("and it is the location this run uses",
			strings.Contains(f.out, sticky+"/root/"), f.evidence())
	})

	// The branch a mutation found unobserved: absent is fine and ordinary, but a root whose mode could
	// not be READ is a different fact wearing the same shape, and it must not pass as one this tool
	// checked. Without this case, treating every stat error as "absent" went unnoticed.
	t.Run("a root whose permissions cannot be read is refused, not treated as absent", func(t *testing.T) {
		f := newRepo(t)
		enclosing := f.base + "/sealed"
		f.mkdirAll(enclosing + "/root-inside")
		if !f.madeUnreadable(enclosing, "the unreadable-override-root case") {
			t.Skip("this process reads a mode-0 directory regardless of the mode (root, or CAP_DAC_OVERRIDE)")
		}
		defer f.chmod(enclosing, 0o755)
		f.writeOverride("root " + enclosing + "/root-inside\n")
		f.runReport("check-ignore")
		f.assertRefused("a root whose mode cannot be read is refused")
		f.assertReports("could not read", "and says the permissions could not be checked")
	})

	t.Run("and a root that does not exist yet is accepted", func(t *testing.T) {
		f := newRepo(t)
		f.writeOverride("root " + f.base + "/not-created-yet\n")
		f.runReport("check-ignore")
		f.record("an absent root is accepted, since the first run creates it", f.status == 0, f.evidence())
		f.record("and it is the location this run uses",
			strings.Contains(f.out, f.base+"/not-created-yet/"), f.evidence())
	})
}

func TestTheOverrideConfigIsJudgedLikeTheRootItNames(t *testing.T) {
	// A security review closed the root's own substitution routes and handed back the file that names
	// it: every guard below judges the value, and a value an attacker chose passes all of them, because
	// the root they name can be an ordinary private directory. The guard is only as strong as its input.
	t.Run("a world-writable config is refused", func(t *testing.T) {
		f := newRepo(t)
		// The root it names is impeccable, which is what makes the case one only the config's own mode
		// can refuse: every check on the root passes it. The needle below names the config's mode rather
		// than the phrase alone, because the root's refusal and the ancestor walk's both carry that phrase.
		elsewhere := f.base + "/named-elsewhere"
		f.mkdirAll(elsewhere)
		f.chmod(elsewhere, 0o700)
		f.writeOverride("root " + elsewhere + "\n")
		f.chmod(f.configHome+"/kk-flavor/idsd.conf", 0o666)
		f.runReport("check-ignore")
		f.assertRefused("a group- or world-writable config is refused")
		f.assertReports("idsd.conf is group- or world-writable (mode 0666)",
			"and names the config file and which permission is the problem")
	})

	t.Run("a symlinked config is refused rather than judged by its target", func(t *testing.T) {
		f := newRepo(t)
		elsewhere := f.base + "/link-target-root"
		f.mkdirAll(elsewhere)
		f.chmod(elsewhere, 0o700)
		real := f.base + "/real-idsd.conf"
		f.write(real, "root "+elsewhere+"\n")
		f.mkdirAll(f.configHome + "/kk-flavor")
		f.symlink(real, f.configHome+"/kk-flavor/idsd.conf")
		f.runReport("check-ignore")
		f.assertRefused("a symlinked config is refused")
		// The link's own path, and it has to fall on the left of the arrow: both paths reach this line, so
		// a bare "is a symlink" would hold just as well for a message that named the target as the problem.
		f.assertReports("/kk-flavor/idsd.conf is a symlink -> ", "and names the link rather than its target")
	})

	t.Run("a config under a world-writable directory is refused", func(t *testing.T) {
		f := newRepo(t)
		elsewhere := f.base + "/dir-case-root"
		f.mkdirAll(elsewhere)
		f.chmod(elsewhere, 0o700)
		f.writeOverride("root " + elsewhere + "\n")
		f.chmod(f.configHome+"/kk-flavor", 0o777)
		defer f.chmod(f.configHome+"/kk-flavor", 0o755)
		f.runReport("check-ignore")
		f.assertRefused("a config under a writable directory is refused")
		f.assertReports("sits under a group- or world-writable directory", "and says that is why")
		// The directory by its tail and its mode rather than by its full path: the walk reports it with
		// every symlink above it resolved, and on macOS the fixture sits under /var, which is one.
		f.assertReports("kk-flavor, mode 0777)", "and names the directory that is the problem, and its mode")
	})

	// The exemption, named rather than left to fire implicitly. On a Linux runner a fixture under sticky
	// `/tmp` exercises this by accident; on macOS it never does, so without this case the branch is
	// unguarded on the machine this is most often run on. Sticky is what stops another account renaming
	// an entry it does not own, which is the whole of the substitution the walk looks for.
	t.Run("and a sticky world-writable directory above the config is not substitutable", func(t *testing.T) {
		f := newRepo(t)
		elsewhere := f.base + "/sticky-case-root"
		f.mkdirAll(elsewhere)
		f.chmod(elsewhere, 0o700)
		f.writeOverride("root " + elsewhere + "\n")
		holder := f.configHome + "/kk-flavor"
		// os.Chmod takes the sticky bit as os.ModeSticky, NOT as octal 0o1000 — passing 0o1777 sets 0777
		// and no sticky bit at all, which made this case skip silently and observe nothing.
		f.chmod(holder, 0o777|os.ModeSticky)
		defer f.chmod(holder, 0o755)
		info, err := os.Stat(holder)
		if err != nil || info.Mode()&os.ModeSticky == 0 {
			t.Skip("this filesystem does not carry the sticky bit, so the exemption cannot be built here")
		}
		f.runReport("check-ignore")
		f.record("a sticky world-writable holder is accepted, not refused",
			f.status == 0, f.evidence())
		f.record("and the run still resolves to the configured root",
			strings.Contains(f.out, elsewhere+"/"), f.evidence())
	})

	// The ordinary case, which must not be caught: a private config naming a private root.
	t.Run("while an owner-only config naming an owner-only root is accepted", func(t *testing.T) {
		f := newRepo(t)
		elsewhere := f.base + "/ordinary-root"
		f.mkdirAll(elsewhere)
		f.chmod(elsewhere, 0o700)
		f.writeOverride("root " + elsewhere + "\n")
		f.chmod(f.configHome+"/kk-flavor/idsd.conf", 0o600)
		f.runReport("check-ignore")
		f.record("an ordinary override is accepted", f.status == 0, f.evidence())
		// Accepted AND used: exit 0 alone is also what a silent fallback to the default produces, which is
		// the one failure an override must not have.
		// The trailing slash is what makes this "uses" rather than "mentions": the clone's own directory
		// hangs off the configured root, so only a run that resolved to it prints the root with a `/` after.
		f.record("and the run announces and uses the configured root",
			strings.Contains(f.out, elsewhere+"/"), f.evidence())
	})
}
