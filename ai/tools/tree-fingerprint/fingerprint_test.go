// Cases for the tree fingerprint. Three of them must hold, and each one's own comment says what it
// rests on.

// "an untracked file's content never reaches the repository's object store" is the security property
// the throwaway object store exists for. Weaken it and fingerprinting a tree leaves the caller's
// working files, a live credential among them, recoverable from `.git/objects` for good.

// "a tracked file matching an ignore rule still changes the fingerprint" is the seed. Weaken it and
// a stale ledger passes as a valid resume point, which is the failure this whole recipe exists to
// prevent.

// "commits in a nested repository do not move the fingerprint" is determinism. Weaken it and the
// hash moves while no file here changes. A stamp can then no longer tell a tree rewritten under a
// stage from one left alone, and a whole round's staleness reading is worthless.

// These cases run real git and are meant to. This package's whole body of work is a recipe in git's
// own commands: a scratch index seeded from HEAD, a throwaway object store, `add -A` with an
// exclusion, `write-tree`.

// A fake index would assert against a walk git never did, which is a suite agreeing with itself.
// `ai/kk-flavor/standards/testing.md` asks for the real thing driven, and for test code held to the
// same bar as production code.

// What they must not do is build a repository per case. One is built here holding every shape they
// need at once. Each case takes a single fingerprint of it and compares that against the reading the
// case before it left.

// A fingerprint is five git processes, and a process costs about 100ms on the machine these are
// written on. A `before` of its own per case would double the suite for no gain.
package treefingerprint

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The untracked content the object-store case looks for afterwards. The case that asserts an
// untracked file moves the fingerprint writes it into the tree. That case needs an untracked file,
// and this one needs a fingerprint to have been taken over it.
const untrackedSecret = "a-credential-no-ref-would-ever-point-at\n"

func TestMain(m *testing.M) {
	// The developer's own git config must not reach these fixtures. Both variables, because NOSYSTEM
	// blocks /etc/gitconfig alone. A global core.excludesFile, the config key, is what actually reaches in
	// here, and one holding *.txt turns every case in this file red on correct code.
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	// The identity the commits in this file need, set in the environment instead of in each
	// repository's config. Six repositories are built across this suite, and `git config` three times
	// in each is eighteen processes for something git reads from here just as well. The two blanked
	// config files leave commit.gpgsign unset, and no setting is left that could turn it on.
	os.Setenv("GIT_AUTHOR_NAME", "t")
	os.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	os.Setenv("GIT_COMMITTER_NAME", "t")
	os.Setenv("GIT_COMMITTER_EMAIL", "t@t")
	os.Exit(m.Run())
}

// One git call against a fixture, which must work. These build the tree the cases read. A failure
// here is the suite losing its subject, and carries no verdict on the tool.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimRight(string(out), "\n")
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// A repository with one commit at path inside dir. It is what an agent session's worktree is to the
// checkout it was opened inside: another repository, sharing only the directory it sits in.
func newNested(t *testing.T, dir, path string) string {
	t.Helper()
	nested := filepath.Join(dir, path)
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	git(t, nested, "init", "-q")
	write(t, filepath.Join(nested, "inside.txt"), "one\n")
	git(t, nested, "add", "inside.txt")
	git(t, nested, "commit", "-qm", "inside")
	return nested
}

// fixture is the single repository the cases read, and the last fingerprint taken of it.
type fixture struct {
	dir    string
	linked string
	// bare is a directory inside no repository. The refusal cases need one, and it cannot be a corner
	// of the repository the dir field names. That is the only fixture here a second one is genuinely
	// needed beside.
	bare string
	last string
}

// git builds it, because this package IS the thing that reads a repository's layout. A fixture
// assembled by hand would be asserting against a shape git did not make.

// The untracked nested repository is named `wt*` for the case about a nested repository whose name
// globs over its siblings. Every other case that needs a nested repository needs no more of it, and a
// second one would be four more processes to prove the same exclusion twice. wtKEEP, a directory in
// the fixture, is the sibling that name globs over.

// The repository every case reads, holding every shape they need at once. It carries a tracked file,
// a tracked file that matches an ignore rule, and a subdirectory. It also carries a repository
// nested inside this one, a nested one with no commit, a gitlink this repository declares, and a
// linked worktree.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{dir: t.TempDir(), linked: filepath.Join(t.TempDir(), "linked"), bare: t.TempDir()}

	// Asserted here, because a bare directory sitting inside a checkout would let every refusal case
	// in this file pass for the wrong reason.
	if _, err := exec.Command("git", "-C", f.bare, "rev-parse", "--show-toplevel").Output(); err == nil {
		t.Fatalf("%s resolves to a repository, so the refusal cases below would prove nothing", f.bare)
	}

	git(t, f.dir, "init", "-q")
	write(t, filepath.Join(f.dir, "tracked.txt"), "base\n")
	write(t, filepath.Join(f.dir, "sub", "deep.txt"), "deep\n")
	// kept.txt is tracked AND matches an ignore rule, which is what the seed case is about. `-f` is
	// what lets one `add` stage it alongside the rule that covers it.
	write(t, filepath.Join(f.dir, "kept.txt"), "one\n")
	write(t, filepath.Join(f.dir, ".gitignore"), "kept.txt\nignored.txt\n")
	git(t, f.dir, "add", "-f", "tracked.txt", "sub/deep.txt", "kept.txt", ".gitignore")

	// The gitlink this repository declares. It goes into the index directly, because `git submodule
	// add` wants a URL it can clone and a protocol allowance to clone it over.
	declared := newNested(t, f.dir, "declared")
	git(t, f.dir, "update-index", "--add", "--cacheinfo", "160000,"+git(t, declared, "rev-parse", "HEAD")+",declared")
	git(t, f.dir, "commit", "-qm", "base")
	if !strings.Contains(git(t, f.dir, "ls-tree", "HEAD", "--", "declared"), "commit") {
		t.Fatal("the fixture committed no gitlink, so the submodule case would prove nothing")
	}

	newNested(t, f.dir, "wt*")
	write(t, filepath.Join(f.dir, "wtKEEP", "sibling.txt"), "one\n")

	// A nested repository with no commit yet aborts `add -A` outright. git says "does not have a
	// commit checked out". That is the state a sibling worktree is in while it is being set up. It
	// sits in the fixture from the start, so every reading in this file is also a reading taken past
	// one.
	halfBuilt := filepath.Join(f.dir, "half-built-worktree")
	if err := os.MkdirAll(halfBuilt, 0o755); err != nil {
		t.Fatalf("mkdir half-built-worktree: %v", err)
	}
	git(t, halfBuilt, "init", "-q")

	git(t, f.dir, "worktree", "add", "-q", f.linked, "-b", "other")

	// The reading the first case compares against. Taken here, because every case in this file then
	// still has a baseline when `-run` selects it alone.
	f.reading(t)
	return f
}

// Takes one fingerprint of the tree, and adopts it as the reading the next case compares against.
func (f *fixture) reading(t *testing.T) (before, now string) {
	t.Helper()
	before = f.last
	tree, err := Fingerprint(f.dir)
	if err != nil {
		t.Fatalf("fingerprinting %s: %v", f.dir, err)
	}
	f.last = tree
	return before, tree
}

func (f *fixture) write(t *testing.T, name, body string) {
	t.Helper()
	write(t, filepath.Join(f.dir, name), body)
}

// The blob id git would give this content. The content itself stays unwritten, and this id is what
// the object-store cases look for afterwards.
func (f *fixture) blobIDOf(t *testing.T, body string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", f.dir, "hash-object", "--stdin")
	cmd.Stdin = strings.NewReader(body)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hash-object: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// Whether the REAL store holds that object, which is the store that would keep it.
func (f *fixture) storeHolds(blob string) bool {
	return exec.Command("git", "-C", f.dir, "cat-file", "-e", blob).Run() == nil
}

// The order matters twice over, so moving a case is not a refactor. Each case compares against the
// reading the case before it left. The cases are also grouped by what they disturb: reads first,
// then writes into the repositories nested inside this one, then writes into this tree itself. A
// case that mutates out of turn changes what every later case is comparing against.

// One case selected with `-run` still stands up, because the fixture takes the baseline reading
// itself. A case run alone then compares against a tree it has seen.

// t.Chdir in the last case bars this test from running in parallel. No case in this package may take
// t.Parallel while it stands.

// Reads one repository, in order.
func TestTheFingerprintOfOneTree(t *testing.T) {
	f := newFixture(t)

	t.Run("it prints a tree hash", f.printsATreeHash)
	t.Run("an unchanged tree fingerprints the same twice", f.sameTwice)

	t.Run("commits in a nested repository do not move the fingerprint", f.nestedCommits)
	t.Run("a nested repository's files are not this tree", f.nestedFiles)
	t.Run("a declared submodule's pointer still moves", f.declaredSubmodule)
	t.Run("a subdirectory root still names the whole repository", f.subdirectoryRoot)
	t.Run("a nested repository with no commit still fingerprints", f.halfBuiltNested)

	t.Run("what moves the fingerprint", f.whatMoves)
	t.Run("an untracked file's content never reaches the object store", f.untrackedStaysOutOfTheStore)
	t.Run("ignored paths stay out and others do not", f.ignoredPaths)
	t.Run("a tracked file matching an ignore rule still moves", f.trackedAndIgnored)
	t.Run("a nested repository named with a glob holds out only itself", f.globNamedNested)
	t.Run("a linked worktree leaves the main store alone", f.linkedWorktree)
	t.Run("the caller's index is untouched", f.callersIndex)

	t.Run("the command's argument table", f.argumentTable)
	t.Run("with no path it answers for the working directory", f.noPath)
}

// --- the hash itself ------------------------------------------------------------------------------

func (f *fixture) printsATreeHash(t *testing.T) {
	tree := f.last
	if len(tree) != 40 {
		t.Errorf("tree hash is %d characters (%q), wanted 40", len(tree), tree)
	}
	for _, char := range tree {
		if !strings.ContainsRune("0123456789abcdef", char) {
			t.Fatalf("tree hash is not hex: %q", tree)
		}
	}
}

func (f *fixture) sameTwice(t *testing.T) {
	if first, second := f.reading(t); first != second {
		t.Errorf("two readings of one tree differ: %q then %q", first, second)
	}
}

// --- what the repositories nested inside this one may not do ----------------------------------------

// `add -A` records a nested repository as a gitlink holding that repository's HEAD. Every commit made
// in there moved this fingerprint while no file here changed. A ledger stamped with it could not tell
// that apart from the tree it named having actually been rewritten.

// Two commits before one reading. A hash cannot move and move back to the value it had, so a reading
// taken after both catches either of them. What it gives up is saying which, and that is the trade
// for paying five processes once instead of twice.

// The determinism this whole recipe is stamped for.
func (f *fixture) nestedCommits(t *testing.T) {
	nested := filepath.Join(f.dir, "wt*")
	git(t, nested, "commit", "-q", "--allow-empty", "-m", "a session next door commits")
	git(t, nested, "commit", "-q", "--allow-empty", "-m", "and again")
	if before, after := f.reading(t); after != before {
		t.Errorf("a commit in a nested repository moved the fingerprint: %q became %q — the hash names "+
			"that repository's HEAD, so a ledger stamped with it invalidates over work that is not this tree's", before, after)
	}
}

// A nested repository's own files are no more part of this tree than its HEAD is. This is asserted
// apart from nestedCommits, which a recipe recording those files in place of the gitlink would
// otherwise satisfy. That recipe hands every path in a sibling session's worktree to this ledger.
func (f *fixture) nestedFiles(t *testing.T) {
	nested := filepath.Join(f.dir, "wt*")
	write(t, filepath.Join(nested, "inside.txt"), "rewritten\n")
	write(t, filepath.Join(nested, "fresh.txt"), "new\n")
	if before, after := f.reading(t); after != before {
		t.Errorf("an edit inside a nested repository moved the fingerprint: %q became %q", before, after)
	}
}

// The other half: a gitlink HEAD already holds is a declared submodule, and its pointer is part of
// what this repository tracks. A recipe that dropped every gitlink would pass the cases nestedCommits,
// and nestedFiles, and go blind on a submodule bump.
func (f *fixture) declaredSubmodule(t *testing.T) {
	git(t, filepath.Join(f.dir, "declared"), "commit", "-q", "--allow-empty", "-m", "the submodule moves on")
	if before, after := f.reading(t); after == before {
		t.Errorf("a declared submodule's pointer moved and the fingerprint did not: still %q — a bump "+
			"is a change to this repository, and every ledger stamped with this hash now reads as fresh over it", before)
	}
}

// The scope is what the exclusion is written against, and its pathspecs are read from the repository
// root. Pathspecs read from the caller's own directory instead would hold out a path that is absent
// there, and the nested repository's HEAD would be back in the hash in silence.

// A root inside the repository still names the whole repository, and a nested repository elsewhere
// in it is still held out.
func (f *fixture) subdirectoryRoot(t *testing.T) {
	fromSub := func() string {
		t.Helper()
		tree, err := Fingerprint(filepath.Join(f.dir, "sub"))
		if err != nil {
			t.Fatalf("fingerprinting from a subdirectory: %v", err)
		}
		return tree
	}
	if got := fromSub(); got != f.last {
		t.Errorf("fingerprinting from a subdirectory named a different tree: %q against %q from the root", got, f.last)
	}
	git(t, filepath.Join(f.dir, "wt*"), "commit", "-q", "--allow-empty", "-m", "commit in the sibling directory")
	if got := fromSub(); got != f.last {
		t.Errorf("read from a subdirectory, a commit in a nested repository elsewhere moved the fingerprint: %q became %q", f.last, got)
	}
}

// A nested repository with no commit yet aborts `add -A` outright. While a sibling session was setting
// its worktree up, no fingerprint could be taken here at all, and every lane that needed one refused.
// The content appearing under it is what makes the walk reach in.
func (f *fixture) halfBuiltNested(t *testing.T) {
	f.write(t, "half-built-worktree/staged.txt", "content\n")
	_, tree := f.reading(t)
	if len(tree) != 40 {
		t.Errorf("fingerprinting past an uncommitted nested repository produced %q, not a tree hash", tree)
	}
}

// --- what moves this tree's own hash ----------------------------------------------------------------

func (f *fixture) whatMoves(t *testing.T) {
	for _, c := range []struct {
		what string
		act  func(t *testing.T)
	}{
		// This row doubles as the untracked content the object-store case looks for afterwards.
		{"an untracked file", func(t *testing.T) { f.write(t, "secret.txt", untrackedSecret) }},
		{"an unstaged edit to a tracked file", func(t *testing.T) { f.write(t, "tracked.txt", "edited\n") }},
		{"a new file in a new directory", func(t *testing.T) { f.write(t, "fresh/deep.txt", "deep\n") }},
	} {
		t.Run(c.what+" changes the fingerprint", func(t *testing.T) {
			c.act(t)
			if before, after := f.reading(t); after == before {
				t.Errorf("the fingerprint did not move: still %q", before)
			}
		})
	}
}

// The security property. `add -A` writes every untracked file's content into whatever object store
// it is pointed at. The throwaway store is what keeps that content out of the repository's own.
// Content written there has no ref pointing at it, and gc leaves it in place for good.
func (f *fixture) untrackedStaysOutOfTheStore(t *testing.T) {
	// whatMoves, a case in this file, took the fingerprint that would have written it. With the file gone
	// this case asks about a blob the store was never offered, and its absence would prove only that.
	if _, err := os.Stat(filepath.Join(f.dir, "secret.txt")); err != nil {
		t.Fatalf("the untracked credential is not in the tree, so this case would prove nothing: %v", err)
	}
	if blob := f.blobIDOf(t, untrackedSecret); f.storeHolds(blob) {
		t.Errorf("the untracked file's content is in the repository's object store as %s — "+
			"it is recoverable for good, and nothing will ever collect it", blob)
	}
}

// An ignored path is not part of the tree a ledger names, and an un-ignored one is. Both halves,
// because a recipe that ignored everything would satisfy the first alone.
func (f *fixture) ignoredPaths(t *testing.T) {
	f.write(t, "ignored.txt", "invisible\n")
	if before, after := f.reading(t); after != before {
		t.Errorf("an ignored path moved the fingerprint: %q became %q", before, after)
	}
	f.write(t, "seen.txt", "visible\n")
	if before, after := f.reading(t); after == before {
		t.Errorf("an un-ignored path did not move the fingerprint: still %q", before)
	}
}

// The seed's whole reason. Git applies ignore rules only to paths the index does not already hold.
// An index built from an empty tree therefore drops a tracked file matching an ignore rule out of
// the walk. That file can then be rewritten between two runs with the fingerprint unmoved.
func (f *fixture) trackedAndIgnored(t *testing.T) {
	f.write(t, "kept.txt", "two\n")
	if before, after := f.reading(t); after == before {
		t.Errorf("a tracked file matching an ignore rule did not move the fingerprint: still %q — "+
			"the index was not seeded from HEAD, and a rewrite of that file is now invisible to every ledger", before)
	}
}

// A nested repository is held out by its literal name, and a pattern read out of that name would be
// something else. The `*` in the fixture's nested repository is a wildcard to git unless the
// exclusion says otherwise, and `wt*` then covers the sibling directory too. Untracked content would
// drop out of the hash under a name that only looked like its own.

// Every case before this one asserts that the nested repository itself stays held out, since `wt*`
// is the name they all use. This is the other half: the sibling its name globs over must still be in.
func (f *fixture) globNamedNested(t *testing.T) {
	f.write(t, "wtKEEP/sibling.txt", "two\n")
	if before, after := f.reading(t); after == before {
		t.Errorf("a sibling directory the nested repository's name globs over went missing from the "+
			"fingerprint: still %q, so an edit under it is invisible to every ledger", before)
	}
}

// A linked worktree shares the repository, so its fingerprint must not leave objects in the main
// repo's store either.
func (f *fixture) linkedWorktree(t *testing.T) {
	const secret = "worktree-only-content\n"
	write(t, filepath.Join(f.linked, "wt.txt"), secret)
	blob := f.blobIDOf(t, secret)
	if f.storeHolds(blob) {
		t.Fatalf("the shared store already holds %s before the linked worktree was read, so this case "+
			"would pass on nothing", blob)
	}

	tree, err := Fingerprint(f.linked)
	if err != nil {
		t.Fatalf("fingerprinting the linked worktree: %v", err)
	}
	if len(tree) != 40 {
		t.Errorf("the linked worktree produced %q, not a tree hash", tree)
	}
	if f.storeHolds(blob) {
		t.Errorf("a linked worktree's untracked content reached the shared object store as %s", blob)
	}
}

// The caller's own index must come back exactly as it was: this reads a tree, it does not stage one.
// What gets staged here is content the working tree already holds, so the reading taken between the
// two checks is of the same tree as the reading before it.
func (f *fixture) callersIndex(t *testing.T) {
	git(t, f.dir, "add", "tracked.txt")
	before := git(t, f.dir, "diff", "--name-only", "--cached")
	if before != "tracked.txt" {
		t.Fatalf("the fixture staged %q, not tracked.txt — the case below would prove nothing", before)
	}
	f.reading(t)
	if after := git(t, f.dir, "diff", "--name-only", "--cached"); after != before {
		t.Errorf("the caller's index moved: staged %q before, %q after", before, after)
	}
}

// --- the command ----------------------------------------------------------------------------------

// Only the first row reaches git, and every refusal is settled before a fingerprint is attempted.

// The grammar the stub's header documents, driven through Run. `stub_usage_test.go` holds the two
// against each other, and this says what each shape does once the line is what it should be.
func (f *fixture) argumentTable(t *testing.T) {
	for _, c := range []struct {
		what   string
		args   []string
		status int
		want   string
	}{
		{"a path alone prints the hash", []string{f.dir}, 0, f.last},
		// A second root is a caller who does not know which tree they are asking about. The hash of the
		// first one reads exactly like an answer about the pair.
		{"two paths are refused with the grammar", []string{f.dir, f.dir}, 2, usage},
		{"a flag after the path is refused with the grammar", []string{f.dir, "--nope"}, 2, usage},
		// A directory may legitimately be named `-rf`, so a lone dash-leading argument is a path and its
		// refusal is about the tree, not about the grammar.
		{"a dash-leading argument is a path, not a flag", []string{"-rf"}, 2, ""},
		{"a directory in no repository refuses without the grammar", []string{f.bare}, 2, ""},
	} {
		var out, errOut bytes.Buffer
		if status := Run(c.args, &out, &errOut); status != c.status {
			t.Errorf("%s: exited %d, want %d\nstdout: %s\nstderr: %s", c.what, status, c.status, out.String(), errOut.String())
			continue
		}
		if c.status == 0 {
			got := strings.TrimRight(out.String(), "\n")
			if got != c.want {
				t.Errorf("%s: printed %q, want %q", c.what, got, c.want)
			}
			f.last = got
			continue
		}
		if out.Len() != 0 {
			t.Errorf("%s: refused and still wrote %q to stdout, which a caller reads as a hash", c.what, out.String())
		}
		if errOut.Len() == 0 {
			t.Errorf("%s: refused in silence, and a caller with no reason cannot tell a refusal from an empty tree", c.what)
		}
		if c.want != "" && !strings.Contains(errOut.String(), c.want) {
			t.Errorf("%s: refused with %q, which does not carry %q", c.what, errOut.String(), c.want)
		}
		if c.want == "" && strings.Contains(errOut.String(), usage) {
			t.Errorf("%s: answered a tree it could not read with the grammar, which sends the caller to fix a sound invocation", c.what)
		}
	}
}

// The invocation with no path at all: the default every caller of the stub takes. It is the single
// branch argumentTable, a case in this file, cannot reach, since every row of it supplies a path.
func (f *fixture) noPath(t *testing.T) {
	t.Chdir(f.dir)
	var out, errOut bytes.Buffer
	if status := Run(nil, &out, &errOut); status != 0 {
		t.Fatalf("exited %d for the working directory\nstderr: %s", status, errOut.String())
	}
	if got := strings.TrimRight(out.String(), "\n"); got != f.last {
		t.Errorf("printed %q for the working directory, want %q", got, f.last)
	}
}

// --- the refusals, and the trees the shared fixture cannot be ----------------------------------------

func TestADirectoryOutsideAnyRepositoryRefuses(t *testing.T) {
	// A directory inside no repository. This asserts that, because a directory sitting inside a
	// checkout would let both cases here pass for the wrong reason.
	dir := t.TempDir()
	if _, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output(); err == nil {
		t.Fatalf("%s resolves to a repository, so these cases would prove nothing", dir)
	}

	tree, err := Fingerprint(dir)
	if err == nil {
		t.Fatalf("a directory in no repository produced %q instead of refusing", tree)
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("the refusal does not say what was wrong: %v", err)
	}
	// A refusal must carry no hash. A caller reading both would write a ledger head over a tree it
	// never read.
	if tree != "" {
		t.Errorf("a refusal came back with %q as well", tree)
	}

	file := filepath.Join(dir, "afile")
	write(t, file, "x\n")
	if tree, err := Fingerprint(file); err == nil {
		t.Errorf("a plain file produced %q instead of refusing", tree)
	}
}

// A repository with no commit at all, which the shared fixture cannot also be: that fixture is built
// around a commit every case in TestTheFingerprintOfOneTree reads through.
func TestARepositoryWithNoCommitStillFingerprints(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")

	before, err := Fingerprint(dir)
	if err != nil {
		t.Fatalf("fingerprinting a repository with no commit: %v", err)
	}
	write(t, filepath.Join(dir, "only.txt"), "untracked\n")
	after, err := Fingerprint(dir)
	if err != nil {
		t.Fatalf("fingerprinting a repository with no commit: %v", err)
	}
	if after == before {
		t.Errorf("an unborn HEAD swallowed the untracked file: still %q", before)
	}
}

// A HEAD that resolves and cannot be read. It gets a repository of its own, because building it
// destroys the fixture. One refusal serves both subtests: they are two readings of a single failure,
// and corrupting a second repository to ask the second question costs four processes for no gain.
func TestAnUnreadableHeadRefusesAndCarriesGitsOwnReason(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	write(t, filepath.Join(dir, "tracked.txt"), "base\n")
	git(t, dir, "add", "tracked.txt")
	git(t, dir, "commit", "-qm", "base")
	head := git(t, dir, "rev-parse", "HEAD")

	object := filepath.Join(dir, ".git", "objects", head[:2], head[2:])
	if _, err := os.Stat(object); err != nil {
		t.Skipf("this repository packs its objects, so the unreadable-HEAD case cannot be built: %v", err)
	}
	// Removed first, because git writes its loose objects read-only. An in-place overwrite is refused,
	// and the case would then fail on its own fixture, with no verdict left about the tool.
	if err := os.Remove(object); err != nil {
		t.Fatalf("removing HEAD's object: %v", err)
	}
	if err := os.WriteFile(object, []byte("corrupt"), 0o644); err != nil {
		t.Fatalf("corrupting HEAD's object: %v", err)
	}

	tree, err := Fingerprint(dir)

	// An index seeded from an empty tree makes the walk silently miss everything committed. The hash
	// it would return is of a smaller tree than the caller asked about.
	t.Run("an unreadable HEAD refuses", func(t *testing.T) {
		if err == nil {
			t.Fatalf("an unreadable HEAD produced %q instead of refusing", tree)
		}
	})

	// git's own account of a failure reaches the caller, and this package's summary of it does not
	// stand in. A refusal naming only "could not fingerprint" sends a reader looking in the wrong
	// place.
	t.Run("a failure carries git's own reason", func(t *testing.T) {
		if err == nil {
			t.Fatal("the corrupt object did not produce a refusal")
		}
		// Two halves: this package says which step failed, git says why.
		if !strings.Contains(err.Error(), dir) {
			t.Errorf("the refusal does not name the tree it was reading: %v", err)
		}
		if !strings.Contains(strings.ToLower(err.Error()), "error") && !strings.Contains(err.Error(), "object") {
			t.Errorf("the refusal carries none of git's own account: %v", err)
		}
	})
}
