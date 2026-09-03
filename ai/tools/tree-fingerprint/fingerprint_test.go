// Cases for the tree fingerprint. Two must not be weakened.
//
// "an untracked file's content never reaches the repository's object store" is the security property
// the throwaway object store exists for: `add -A` hashes every un-ignored file, no ref points at those
// blobs, and nothing ever collects them — so without the throwaway store, fingerprinting a tree leaves
// the caller's working files, a live credential among them, recoverable from `.git/objects` for good.
//
// "a tracked file matching an ignore rule still changes the fingerprint" is the seed. Git applies
// ignore rules only to paths the index does not already hold, so an index built from nothing drops
// such a file from the walk entirely — and it could then be rewritten between two runs with the
// fingerprint unmoved. That is a stale ledger passing as a valid resume point, which is the failure
// this whole recipe exists to prevent.
//
// Two cases from the shell suite are gone rather than ported: both drove `CDPATH` corrupting the
// script's own `cd`-based root resolution, and there is no `cd` here to corrupt.
package treefingerprint

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// The developer's own git config must not reach these fixtures.
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
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
