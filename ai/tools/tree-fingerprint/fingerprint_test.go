// Cases for the tree fingerprint. Three must not be weakened, and each one's own comment says what it
// rests on.
//
// "an untracked file's content never reaches the repository's object store" is the security property
// the throwaway object store exists for. Weakened, fingerprinting a tree leaves the caller's working
// files, a live credential among them, recoverable from `.git/objects` for good.
//
// "a tracked file matching an ignore rule still changes the fingerprint" is the seed. Weakened, it
// lets a stale ledger pass as a valid resume point, which is the failure this whole recipe exists to
// prevent.
//
// "commits in a nested repository do not move the fingerprint" is determinism. Weakened, the hash
// moves while nothing here changes, and a stamp can no longer tell a tree that was rewritten under a
// stage from one that was not — so a whole round's staleness reading says nothing.
package treefingerprint

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// The developer's own git config must not reach these fixtures. Both variables, because NOSYSTEM
	// blocks /etc/gitconfig alone: a global core.excludesFile is what actually reaches in here, and
	// one holding *.txt turns every case below red on correct code.
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	os.Exit(m.Run())
}

type repo struct {
	t   *testing.T
	dir string
}

// newRepo builds a repository with one commit. Built rather than copied: this package IS the thing
// that reads a repository's layout, so a fixture assembled by hand would be asserting against a shape
// git did not make.
func newRepo(t *testing.T) *repo {
	t.Helper()
	dir := t.TempDir()
	r := &repo{t: t, dir: dir}
	r.git("init", "-q")
	r.git("config", "user.email", "t@t")
	r.git("config", "user.name", "t")
	r.git("config", "commit.gpgsign", "false")
	r.write("tracked.txt", "base\n")
	r.git("add", "tracked.txt")
	r.git("commit", "-qm", "base")
	return r
}

// newBareDir is a directory inside no repository. Asserted to be one, because every refusal case rests
// on it: built inside a checkout, they would pass for the wrong reason.
func newBareDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output(); err == nil {
		t.Fatalf("%s resolves to a repository, so the refusal cases below would prove nothing", dir)
	}
	return dir
}

func (r *repo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimRight(string(out), "\n")
}

func (r *repo) write(name, body string) {
	r.t.Helper()
	full := filepath.Join(r.dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		r.t.Fatalf("write %s: %v", name, err)
	}
}

func (r *repo) fingerprint() string {
	r.t.Helper()
	tree, err := Fingerprint(r.dir)
	if err != nil {
		r.t.Fatalf("fingerprinting %s: %v", r.dir, err)
	}
	return tree
}

// --- the hash itself ------------------------------------------------------------------------------

func TestItPrintsATreeHash(t *testing.T) {
	r := newRepo(t)
	tree := r.fingerprint()
	if len(tree) != 40 {
		t.Errorf("tree hash is %d characters (%q), wanted 40", len(tree), tree)
	}
	for _, char := range tree {
		if !strings.ContainsRune("0123456789abcdef", char) {
			t.Fatalf("tree hash is not hex: %q", tree)
		}
	}
}

func TestAnUnchangedTreeFingerprintsTheSameTwice(t *testing.T) {
	r := newRepo(t)
	if first, second := r.fingerprint(), r.fingerprint(); first != second {
		t.Errorf("two readings of one tree differ: %q then %q", first, second)
	}
}

func TestWhatMovesTheFingerprint(t *testing.T) {
	cases := []struct {
		name string
		act  func(r *repo)
	}{
		{"an untracked file", func(r *repo) { r.write("fresh.txt", "new\n") }},
		{"an unstaged edit to a tracked file", func(r *repo) { r.write("tracked.txt", "edited\n") }},
		{"a new file in a new directory", func(r *repo) { r.write("sub/deep.txt", "deep\n") }},
	}
	for _, tc := range cases {
		t.Run(tc.name+" changes the fingerprint", func(t *testing.T) {
			r := newRepo(t)
			before := r.fingerprint()
			tc.act(r)
			if after := r.fingerprint(); after == before {
				t.Errorf("the fingerprint did not move: still %q", before)
			}
		})
	}
}

// An ignored path is not part of the tree a ledger names, and an un-ignored one is. Both halves,
// because a recipe that ignored everything would satisfy the first alone.
func TestIgnoredPathsStayOutAndOthersDoNot(t *testing.T) {
	r := newRepo(t)
	r.write(".gitignore", "ignored.txt\n")
	r.git("add", ".gitignore")
	r.git("commit", "-qm", "ignore rule")
	before := r.fingerprint()

	r.write("ignored.txt", "invisible\n")
	if after := r.fingerprint(); after != before {
		t.Errorf("an ignored path moved the fingerprint: %q became %q", before, after)
	}
	r.write("seen.txt", "visible\n")
	if after := r.fingerprint(); after == before {
		t.Errorf("an un-ignored path did not move the fingerprint: still %q", before)
	}
}

// The seed's whole reason. Git applies ignore rules only to paths the index does not already hold, so
// an index built from nothing drops a TRACKED file matching an ignore rule out of the walk — and it
// could then be rewritten between two runs with the fingerprint unmoved.
func TestATrackedFileMatchingAnIgnoreRuleStillMoves(t *testing.T) {
	r := newRepo(t)
	r.write("kept.txt", "one\n")
	r.git("add", "kept.txt")
	r.write(".gitignore", "kept.txt\n")
	r.git("add", ".gitignore")
	r.git("commit", "-qm", "tracked and ignored")

	before := r.fingerprint()
	r.write("kept.txt", "two\n")
	if after := r.fingerprint(); after == before {
		t.Errorf("a tracked file matching an ignore rule did not move the fingerprint: still %q — "+
			"the index was not seeded from HEAD, and a rewrite of that file is now invisible to every ledger", before)
	}
}

// nestedRepo builds a repository with one commit inside r, at path. It is what an agent session's
// worktree is to the checkout it was opened inside — another repository, sharing nothing with this
// tree but the directory it sits in.
func (r *repo) nestedRepo(path string) *repo {
	r.t.Helper()
	full := filepath.Join(r.dir, path)
	if err := os.MkdirAll(full, 0o755); err != nil {
		r.t.Fatalf("mkdir %s: %v", path, err)
	}
	nested := &repo{t: r.t, dir: full}
	nested.git("init", "-q")
	nested.git("config", "user.email", "t@t")
	nested.git("config", "user.name", "t")
	nested.git("config", "commit.gpgsign", "false")
	nested.write("inside.txt", "one\n")
	nested.git("add", "inside.txt")
	nested.git("commit", "-qm", "inside")
	return nested
}

// The determinism this whole recipe is stamped for. `add -A` records a nested repository as a gitlink
// holding that repository's HEAD, so every commit made in there moved this fingerprint while no file
// here changed — and a ledger stamped with it could not tell that apart from the tree it named having
// actually been rewritten.
func TestCommitsInANestedRepositoryDoNotMoveTheFingerprint(t *testing.T) {
	r := newRepo(t)
	nested := r.nestedRepo("session-worktree")
	before := r.fingerprint()

	nested.git("commit", "-q", "--allow-empty", "-m", "a session next door commits")
	if after := r.fingerprint(); after != before {
		t.Errorf("a commit in a nested repository moved the fingerprint: %q became %q — the hash names "+
			"that repository's HEAD, so a ledger stamped with it invalidates over work that is not this tree's", before, after)
	}
	nested.git("commit", "-q", "--allow-empty", "-m", "and again")
	if after := r.fingerprint(); after != before {
		t.Errorf("a second commit in a nested repository moved the fingerprint again: %q became %q", before, after)
	}
}

// A nested repository's own files are no more part of this tree than its HEAD is. Asserted separately
// from the case above, which a recipe recording those files instead of the gitlink would otherwise
// satisfy — and that recipe hands every path in a sibling session's worktree to this ledger.
func TestANestedRepositorysFilesAreNotThisTree(t *testing.T) {
	r := newRepo(t)
	nested := r.nestedRepo("session-worktree")
	before := r.fingerprint()

	nested.write("inside.txt", "rewritten\n")
	if after := r.fingerprint(); after != before {
		t.Errorf("an edit inside a nested repository moved the fingerprint: %q became %q", before, after)
	}
	nested.write("fresh.txt", "new\n")
	if after := r.fingerprint(); after != before {
		t.Errorf("a new file inside a nested repository moved the fingerprint: %q became %q", before, after)
	}
}

// The other half: a gitlink HEAD already holds is a declared submodule, and its pointer is part of
// what this repository tracks. A recipe that dropped every gitlink would pass the two cases above and
// go blind on a submodule bump. The entry is written into the index directly rather than through
// `git submodule add`, which wants a URL it can clone and a protocol allowance to clone it over.
func TestADeclaredSubmodulesPointerStillMoves(t *testing.T) {
	r := newRepo(t)
	nested := r.nestedRepo("declared")
	r.git("update-index", "--add", "--cacheinfo", "160000,"+nested.git("rev-parse", "HEAD")+",declared")
	r.git("commit", "-qm", "declare the submodule")
	if !strings.Contains(r.git("ls-tree", "HEAD", "--", "declared"), "commit") {
		t.Fatal("the fixture committed no gitlink, so this case would prove nothing")
	}

	before := r.fingerprint()
	nested.git("commit", "-q", "--allow-empty", "-m", "the submodule moves on")
	if after := r.fingerprint(); after == before {
		t.Errorf("a declared submodule's pointer moved and the fingerprint did not: still %q — a bump "+
			"is a change to this repository, and every ledger stamped with this hash now reads as fresh over it", before)
	}
}

// A nested repository with no commit yet aborts `add -A` outright ("does not have a commit checked
// out"). While a sibling session was setting its worktree up, no fingerprint could be taken here at
// all, and every lane that needed one refused.
func TestANestedRepositoryWithNoCommitStillFingerprints(t *testing.T) {
	r := newRepo(t)
	empty := filepath.Join(r.dir, "half-built-worktree")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if out, err := exec.Command("git", "-C", empty, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	r.write("half-built-worktree/staged.txt", "content\n")

	tree, err := Fingerprint(r.dir)
	if err != nil {
		t.Fatalf("a nested repository with no commit made the tree unfingerprintable: %v", err)
	}
	if len(tree) != 40 {
		t.Errorf("fingerprinting past an uncommitted nested repository produced %q, not a tree hash", tree)
	}
}

// A nested repository is held out by its name and not by a pattern read out of it. The `*` below is a
// wildcard to git unless the exclusion says otherwise, and `wt*` then covers the sibling directory
// too — untracked content dropped out of the hash by a name that only looked like its own.
func TestANestedRepositoryNamedWithAGlobHoldsOutOnlyItself(t *testing.T) {
	r := newRepo(t)
	nested := r.nestedRepo("wt*")
	r.write("wtKEEP/sibling.txt", "one\n")
	before := r.fingerprint()

	nested.git("commit", "-q", "--allow-empty", "-m", "commit next door")
	if after := r.fingerprint(); after != before {
		t.Errorf("a nested repository named with a wildcard moved the fingerprint: %q became %q", before, after)
	}
	r.write("wtKEEP/sibling.txt", "two\n")
	if after := r.fingerprint(); after == before {
		t.Errorf("a sibling directory the nested repository's name globs over went missing from the "+
			"fingerprint: still %q, so an edit under it is invisible to every ledger", before)
	}
}

// A root inside the repository still names the whole repository, and a nested repository elsewhere in
// it is still held out. The scope is what the exclusion above is written against: pathspecs read from
// the caller's own directory rather than the repository root would hold out a path that is not there,
// and the nested repository's HEAD is back in the hash with nothing saying so.
func TestASubdirectoryRootStillNamesTheWholeRepository(t *testing.T) {
	r := newRepo(t)
	nested := r.nestedRepo("session-worktree")
	r.write("sub/deep.txt", "deep\n")
	sub := &repo{t: t, dir: filepath.Join(r.dir, "sub")}

	if fromSub, fromRoot := sub.fingerprint(), r.fingerprint(); fromSub != fromRoot {
		t.Errorf("fingerprinting from a subdirectory named a different tree: %q against %q from the root", fromSub, fromRoot)
	}
	before := sub.fingerprint()
	nested.git("commit", "-q", "--allow-empty", "-m", "commit in the sibling directory")
	if after := sub.fingerprint(); after != before {
		t.Errorf("read from a subdirectory, a commit in a nested repository elsewhere moved the fingerprint: %q became %q", before, after)
	}
}

func TestARepositoryWithNoCommitStillFingerprints(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	r := &repo{t: t, dir: dir}
	before := r.fingerprint()
	r.write("only.txt", "untracked\n")
	if after := r.fingerprint(); after == before {
		t.Errorf("an unborn HEAD swallowed the untracked file: still %q", before)
	}
}

// --- what it must not touch -------------------------------------------------------------------------

// The security property. Without the throwaway object store, `add -A` writes every untracked file's
// content into the repository's own store, where no ref points at it and nothing collects it.
func TestAnUntrackedFilesContentNeverReachesTheObjectStore(t *testing.T) {
	r := newRepo(t)
	secret := "a-credential-no-ref-would-ever-point-at\n"
	r.write("secret.txt", secret)

	// The blob id git WOULD give it, computed without writing: the thing to look for afterwards.
	cmd := exec.Command("git", "-C", r.dir, "hash-object", "--stdin")
	cmd.Stdin = strings.NewReader(secret)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hash-object: %v", err)
	}
	blob := strings.TrimSpace(string(out))

	r.fingerprint()

	// `cat-file -e` is the question asked of the REAL store, which is the one that would keep it.
	if err := exec.Command("git", "-C", r.dir, "cat-file", "-e", blob).Run(); err == nil {
		t.Errorf("the untracked file's content is in the repository's object store as %s — "+
			"it is recoverable for good, and nothing will ever collect it", blob)
	}
}

// The caller's own index must come back exactly as it was: this reads a tree, it does not stage one.
func TestTheCallersIndexIsUntouched(t *testing.T) {
	r := newRepo(t)
	r.write("staged.txt", "staged\n")
	r.git("add", "staged.txt")
	before := r.git("diff", "--name-only", "--cached")
	if before != "staged.txt" {
		t.Fatalf("the fixture staged %q, not staged.txt — the case below would prove nothing", before)
	}
	r.fingerprint()
	if after := r.git("diff", "--name-only", "--cached"); after != before {
		t.Errorf("the caller's index moved: staged %q before, %q after", before, after)
	}
}

// A linked worktree shares the repository, so its fingerprint must not leave objects in the main
// repo's store either.
func TestALinkedWorktreeLeavesTheMainStoreAlone(t *testing.T) {
	r := newRepo(t)
	linked := filepath.Join(t.TempDir(), "linked")
	if err := exec.Command("git", "-C", r.dir, "worktree", "add", "-q", linked, "-b", "other").Run(); err != nil {
		t.Skipf("git worktree add is unavailable here, so this case cannot run: %v", err)
	}
	secret := "worktree-only-content\n"
	if err := os.WriteFile(filepath.Join(linked, "wt.txt"), []byte(secret), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cmd := exec.Command("git", "-C", r.dir, "hash-object", "--stdin")
	cmd.Stdin = strings.NewReader(secret)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hash-object: %v", err)
	}
	blob := strings.TrimSpace(string(out))

	tree, err := Fingerprint(linked)
	if err != nil {
		t.Fatalf("fingerprinting the linked worktree: %v", err)
	}
	if len(tree) != 40 {
		t.Errorf("the linked worktree produced %q, not a tree hash", tree)
	}
	if err := exec.Command("git", "-C", r.dir, "cat-file", "-e", blob).Run(); err == nil {
		t.Errorf("a linked worktree's untracked content reached the shared object store as %s", blob)
	}
}

// --- the refusals -------------------------------------------------------------------------------------

func TestADirectoryOutsideAnyRepositoryRefuses(t *testing.T) {
	dir := newBareDir(t)
	tree, err := Fingerprint(dir)
	if err == nil {
		t.Fatalf("a directory in no repository produced %q instead of refusing", tree)
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("the refusal does not say what was wrong: %v", err)
	}
	// A refusal must carry no hash: a caller reading both would write a ledger head naming nothing.
	if tree != "" {
		t.Errorf("a refusal came back with %q as well", tree)
	}
}

func TestAPathThatIsNotADirectoryRefuses(t *testing.T) {
	dir := newBareDir(t)
	file := filepath.Join(dir, "afile")
	if err := os.WriteFile(file, []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if tree, err := Fingerprint(file); err == nil {
		t.Errorf("a plain file produced %q instead of refusing", tree)
	}
}

// A HEAD that resolves and cannot be read must refuse rather than fingerprint without it: seeded from
// nothing, the walk silently misses everything committed, and the hash it returns is of a smaller tree
// than the one the caller asked about.
func TestAHeadThatCannotBeReadRefuses(t *testing.T) {
	r := newRepo(t)
	head := r.git("rev-parse", "HEAD")
	object := filepath.Join(r.dir, ".git", "objects", head[:2], head[2:])
	if _, err := os.Stat(object); err != nil {
		t.Skipf("this repository packs its objects, so the unreadable-HEAD case cannot be built: %v", err)
	}
	// Removed first: git writes its loose objects read-only, so an in-place overwrite is refused and
	// the case would fail on its own fixture rather than on the tool.
	if err := os.Remove(object); err != nil {
		t.Fatalf("removing HEAD's object: %v", err)
	}
	if err := os.WriteFile(object, []byte("corrupt"), 0o644); err != nil {
		t.Fatalf("corrupting HEAD's object: %v", err)
	}
	if tree, err := Fingerprint(r.dir); err == nil {
		t.Errorf("an unreadable HEAD produced %q instead of refusing", tree)
	}
}

// git's own account of a failure reaches the caller, rather than being replaced by this package's
// summary of it. A refusal naming only "could not fingerprint" sends a reader looking in the wrong
// place.
func TestAFailureCarriesGitsOwnReason(t *testing.T) {
	r := newRepo(t)
	head := r.git("rev-parse", "HEAD")
	object := filepath.Join(r.dir, ".git", "objects", head[:2], head[2:])
	if _, err := os.Stat(object); err != nil {
		t.Skipf("this repository packs its objects: %v", err)
	}
	if err := os.Remove(object); err != nil {
		t.Fatalf("removing: %v", err)
	}
	if err := os.WriteFile(object, []byte("corrupt"), 0o644); err != nil {
		t.Fatalf("corrupting: %v", err)
	}
	_, err := Fingerprint(r.dir)
	if err == nil {
		t.Fatal("the corrupt object did not produce a refusal")
	}
	// Two halves: this package says which step failed, git says why.
	if !strings.Contains(err.Error(), r.dir) {
		t.Errorf("the refusal does not name the tree it was reading: %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "error") && !strings.Contains(err.Error(), "object") {
		t.Errorf("the refusal carries none of git's own account: %v", err)
	}
}

// --- the command ----------------------------------------------------------------------------------

// The grammar the stub's header documents, driven through Run. `stub_usage_test.go` holds the two
// against each other; this says what each shape does once the line is what it should be.
func TestTheCommandsArgumentTable(t *testing.T) {
	r := newRepo(t)
	tree := r.fingerprint()

	for _, c := range []struct {
		what   string
		args   []string
		status int
		want   string
	}{
		{"a path alone prints the hash", []string{r.dir}, 0, tree},
		// A second root is a caller who does not know which tree they are asking about, and the hash of
		// the first one reads exactly like an answer about the pair.
		{"two paths are refused with the grammar", []string{r.dir, r.dir}, 2, usage},
		{"a flag after the path is refused with the grammar", []string{r.dir, "--nope"}, 2, usage},
		// A directory may legitimately be named `-rf`, so a lone dash-leading argument is a path and its
		// refusal is about the tree, not about the grammar.
		{"a dash-leading argument is a path, not a flag", []string{"-rf"}, 2, ""},
		{"a directory in no repository refuses without the grammar", []string{newBareDir(t)}, 2, ""},
	} {
		var out, errOut bytes.Buffer
		if status := Run(c.args, &out, &errOut); status != c.status {
			t.Errorf("%s: exited %d, want %d\nstdout: %s\nstderr: %s", c.what, status, c.status, out.String(), errOut.String())
			continue
		}
		if c.status == 0 {
			if got := strings.TrimRight(out.String(), "\n"); got != c.want {
				t.Errorf("%s: printed %q, want %q", c.what, got, c.want)
			}
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

// The invocation with no path at all — the default every caller of the stub takes, and the one branch
// the table above cannot reach, since every row of it supplies a path. t.Chdir bars this case from
// running in parallel, so no case in this package may take t.Parallel while it stands.
func TestWithNoPathItAnswersForTheWorkingDirectory(t *testing.T) {
	r := newRepo(t)
	t.Chdir(r.dir)
	var out, errOut bytes.Buffer
	if status := Run(nil, &out, &errOut); status != 0 {
		t.Fatalf("exited %d for the working directory\nstderr: %s", status, errOut.String())
	}
	if got, want := strings.TrimRight(out.String(), "\n"), r.fingerprint(); got != want {
		t.Errorf("printed %q for the working directory, want %q", got, want)
	}
}
