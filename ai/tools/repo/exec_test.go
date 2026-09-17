// The infrastructure level: the one suite in this module that runs a real git.
//
// Everything else drives repotest.Fake, so this file is the only thing holding the port's answers to
// git's. It builds ONE repository and asks every question of it, because the cost here is the spawn
// and not the assertion: a repository per case would multiply that by the number of questions.
// testing.md rule 6 — a real binary reaches an infrastructure-level test, one per adapter.
package repo_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kk-flavor/tools/repo"
)

// One repository, one linked worktree, one commit and a dirty tree over it: the shapes every method
// below needs, built once.
type fixture struct {
	t        *testing.T
	root     string
	linked   string
	git      repo.Exec
	head     string
	previous string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH, so the adapter cannot be held to it here")
	}
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	f := fixture{t: t, root: root, linked: filepath.Join(base, "linked")}
	f.git = repo.Exec{Env: repo.WithoutGitLocation(os.Environ())}

	mustRun(t, base, "git", "init", "-q", "-b", "main", root)
	mustRun(t, root, "git", "config", "user.email", "suite@example.invalid")
	mustRun(t, root, "git", "config", "user.name", "Suite")
	mustRun(t, root, "git", "config", "commit.gpgsign", "false")

	write(t, filepath.Join(root, "kept.txt"), "one\n")
	write(t, filepath.Join(root, "pkg", "moved.txt"), "before\n")
	write(t, filepath.Join(root, "gone.txt"), "removed later\n")
	write(t, filepath.Join(root, ".gitignore"), "ignored/\n")
	mustRun(t, root, "git", "add", "-A")
	mustRun(t, root, "git", "commit", "-qm", "first")
	f.previous = strings.TrimSpace(capture(t, root, "git", "rev-parse", "HEAD"))

	write(t, filepath.Join(root, "pkg", "moved.txt"), "after\n")
	write(t, filepath.Join(root, "added.txt"), "new\n")
	mustRun(t, root, "git", "rm", "-q", "gone.txt")
	mustRun(t, root, "git", "add", "-A")
	mustRun(t, root, "git", "commit", "-qm", "second")
	f.head = strings.TrimSpace(capture(t, root, "git", "rev-parse", "HEAD"))

	// Left in the working tree and never committed, so the untracked and status answers have something
	// to find. A name with a non-ASCII byte, because that is the one git C-quotes without `-z`.
	write(t, filepath.Join(root, "sundæ.txt"), "loose\n")
	write(t, filepath.Join(root, "ignored", "build.out"), "generated\n")

	mustRun(t, root, "git", "worktree", "add", "-q", f.linked, "-b", "other")
	return f
}

func TestTheAdapterAnswersGit(t *testing.T) {
	f := newFixture(t)
	t.Run("git's own paths", f.paths)
	t.Run("resolving revisions", f.revisions)
	t.Run("what the repository holds", f.listings)
	t.Run("what changed", f.changes_)
	t.Run("ignores and status", f.ignores)
	t.Run("worktrees", f.worktreeList)
	t.Run("a refusal carries what git said", f.refusal)
	// Last, because it writes to the index every case above reads.
	t.Run("staging", f.staging)
}

func (f fixture) paths(t *testing.T) {
	if got := f.str(f.git.TopLevel(filepath.Join(f.root, "pkg"))); !sameFile(t, got, f.root) {
		t.Errorf("TopLevel from a subdirectory = %q, wanted %s", got, f.root)
	}
	if got := f.str(f.git.Prefix(filepath.Join(f.root, "pkg"))); got != "pkg/" {
		t.Errorf("Prefix from a subdirectory = %q, wanted %q — git slash-terminates it", got, "pkg/")
	}
	if got := f.str(f.git.Prefix(f.root)); got != "" {
		t.Errorf("Prefix at the root = %q, wanted empty", got)
	}

	// The property every worktree-aware tool here rests on: a linked worktree has a git dir of its own
	// and the clone's common dir. Absolute both times — `--git-common-dir` answers a bare `.git` in an
	// ordinary repository, and a relative answer would resolve against the next caller's directory.
	common := f.str(f.git.CommonDir(f.linked))
	if !filepath.IsAbs(common) {
		t.Errorf("CommonDir answered the relative path %q", common)
	}
	if !sameFile(t, common, filepath.Join(f.root, ".git")) {
		t.Errorf("CommonDir from the linked worktree = %q, wanted the clone's own %s", common, filepath.Join(f.root, ".git"))
	}
	own := f.str(f.git.GitDir(f.linked))
	if sameFile(t, own, common) {
		t.Errorf("GitDir and CommonDir both answered %q from a linked worktree, so nothing here tells "+
			"a worktree's own store from the one its clone shares", own)
	}
	// A linked worktree's HEAD is not under the common dir, which is why this is asked rather than
	// joined.
	if got := f.str(f.git.GitPath(f.linked, "HEAD")); !strings.HasPrefix(got, own) {
		t.Errorf("GitPath(HEAD) from a linked worktree = %q, wanted it under that worktree's git dir %s", got, own)
	}
}

func (f fixture) revisions(t *testing.T) {
	if got := f.str(f.git.Resolve(f.root, "HEAD")); got != f.head {
		t.Errorf("Resolve(HEAD) = %q, wanted %s", got, f.head)
	}
	// The answer the whole port rests on: a revision naming nothing is the empty string and NOT an
	// error. Every caller reads that as "no commit yet", and an error there would turn a fresh
	// repository into a refusal.
	got, err := f.git.Resolve(f.root, "no-such-ref")
	if err != nil || got != "" {
		t.Errorf("Resolve over a name that resolves to nothing = (%q, %v), wanted (\"\", nil)", got, err)
	}
	if base := f.str(f.git.MergeBase(f.root, f.head, f.previous)); base != f.previous {
		t.Errorf("MergeBase(head, previous) = %q, wanted the previous commit %s", base, f.previous)
	}
}

func (f fixture) listings(t *testing.T) {
	tracked := f.list(f.git.Tracked(f.root))
	for _, want := range []string{"kept.txt", "pkg/moved.txt", "added.txt"} {
		if !slices.Contains(tracked, want) {
			t.Errorf("Tracked did not name %s: %v", want, tracked)
		}
	}
	if slices.Contains(tracked, "gone.txt") {
		t.Errorf("Tracked named gone.txt, which the second commit removed: %v", tracked)
	}
	// Root-relative from a subdirectory. `ls-files` prints paths relative to where git ran unless it is
	// told otherwise, and a caller joining those against the root would open files that do not exist.
	fromSub := f.list(f.git.Tracked(filepath.Join(f.root, "pkg")))
	if !slices.Contains(fromSub, "pkg/moved.txt") {
		t.Errorf("Tracked from pkg/ = %v, wanted the root-relative pkg/moved.txt", fromSub)
	}
	if narrowed := f.list(f.git.Tracked(f.root, "pkg")); !slices.Equal(narrowed, []string{"pkg/moved.txt"}) {
		t.Errorf("Tracked under the pathspec pkg = %v, wanted only pkg/moved.txt", narrowed)
	}

	untracked := f.list(f.git.Untracked(f.root))
	// The non-ASCII name is the case: without `-z` and core.quotePath=false git prints it C-quoted and
	// the caller reads a name no file has.
	if !slices.Contains(untracked, "sundæ.txt") {
		t.Errorf("Untracked did not name sundæ.txt as it is spelt on disk: %v", untracked)
	}
	if slices.Contains(untracked, "ignored/build.out") {
		t.Errorf("Untracked named an ignored file: %v", untracked)
	}

	held := f.list(f.git.NamesAt(f.root, f.previous))
	if !slices.Contains(held, "gone.txt") {
		t.Errorf("NamesAt(previous) did not name gone.txt, which that commit held: %v", held)
	}
}

func (f fixture) changes_(t *testing.T) {
	changed := f.list(f.git.Changed(f.root, []string{f.previous, f.head}, nil))
	if !slices.Contains(changed, "pkg/moved.txt") || !slices.Contains(changed, "added.txt") {
		t.Errorf("Changed between the two commits = %v, wanted pkg/moved.txt and added.txt", changed)
	}
	// Deletions are dropped, because every caller reads the file afterwards and a deleted one has no
	// content to read.
	if slices.Contains(changed, "gone.txt") {
		t.Errorf("Changed named the deleted gone.txt: %v", changed)
	}

	// The same question from a subdirectory. Under `diff.relative=true` git names `moved.txt` here and
	// drops every changed file outside pkg/, so the adapter passing `--no-relative` is what this holds.
	mustRun(t, f.root, "git", "config", "diff.relative", "true")
	fromSub := f.list(f.git.Changed(filepath.Join(f.root, "pkg"), []string{f.previous, f.head}, nil))
	if !slices.Contains(fromSub, "pkg/moved.txt") {
		t.Errorf("Changed from pkg/ under diff.relative=true = %v, wanted the root-relative "+
			"pkg/moved.txt — the adapter has stopped passing --no-relative", fromSub)
	}
	mustRun(t, f.root, "git", "config", "--unset", "diff.relative")

	raw := f.changes(f.git.ChangedWithStatus(f.root, []string{f.previous, f.head}, nil))
	byPath := map[string]repo.Change{}
	for _, one := range raw {
		byPath[one.Path] = one
	}
	if got := byPath["added.txt"]; got.Status != "A" {
		t.Errorf("ChangedWithStatus calls added.txt %q, wanted A", got.Status)
	}
	if got := byPath["gone.txt"]; got.Status != "D" {
		t.Errorf("ChangedWithStatus calls gone.txt %q, wanted D", got.Status)
	}
	// The blob is what makes one call enough: a caller reads the new content without a second listing.
	body, size, err := f.git.Blob(f.root, byPath["added.txt"].Blob)
	if err != nil {
		t.Fatalf("reading the blob ChangedWithStatus named for added.txt: %v", err)
	}
	if string(body) != "new\n" || size != int64(len("new\n")) {
		t.Errorf("that blob = (%q, %d), wanted (%q, %d)", body, size, "new\n", len("new\n"))
	}

	if got := f.body(f.git.Show(f.root, f.previous, "pkg/moved.txt")); string(got) != "before\n" {
		t.Errorf("Show at the previous commit = %q, wanted the content before the change", got)
	}
}

func (f fixture) ignores(t *testing.T) {
	ignored := f.set(f.git.Ignored(f.root, []string{"ignored/build.out", "kept.txt"}))
	if !ignored["ignored/build.out"] {
		t.Errorf("Ignored did not call ignored/build.out ignored: %v", ignored)
	}
	if ignored["kept.txt"] {
		t.Errorf("Ignored called the tracked kept.txt ignored: %v", ignored)
	}
	// Matching nothing is check-ignore's exit 1, and an adapter reading that as a failure would turn
	// every clean tree into a refusal.
	none, err := f.git.Ignored(f.root, []string{"kept.txt"})
	if err != nil || len(none) != 0 {
		t.Errorf("Ignored over a path git ignores nothing about = (%v, %v), wanted an empty set and no error", none, err)
	}
	if source := f.str(f.git.IgnoreSource(f.root, "ignored/build.out")); !strings.Contains(source, ".gitignore") {
		t.Errorf("IgnoreSource = %q, wanted it to name the .gitignore that decided", source)
	}
	if source := f.str(f.git.IgnoreSource(f.root, "kept.txt")); source != "" {
		t.Errorf("IgnoreSource over a path nothing ignores = %q, wanted empty", source)
	}

	status := f.list(f.git.Status(f.root))
	if !slices.ContainsFunc(status, func(line string) bool { return strings.Contains(line, "sundæ.txt") }) {
		t.Errorf("Status did not name the untracked sundæ.txt: %v", status)
	}
}

func (f fixture) worktreeList(t *testing.T) {
	listed := f.worktrees(f.git.Worktrees(f.root))
	if len(listed) != 2 {
		t.Fatalf("Worktrees over a clone with one linked worktree named %d: %v", len(listed), listed)
	}
	var paths []string
	for _, one := range listed {
		paths = append(paths, one.Path)
		if one.Head == "" {
			t.Errorf("%s came back with no HEAD, and both worktrees here have a commit", one.Path)
		}
	}
	for _, want := range []string{f.root, f.linked} {
		if !slices.ContainsFunc(paths, func(got string) bool { return sameFile(t, got, want) }) {
			t.Errorf("Worktrees did not name %s: %v", want, paths)
		}
	}
}

func (f fixture) staging(t *testing.T) {
	if err := f.git.Add(f.root, []string{"sundæ.txt"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	staged := capture(t, f.root, "git", "-c", "core.quotePath=false", "diff", "--name-only", "--cached")
	if !strings.Contains(staged, "sundæ.txt") {
		t.Errorf("after Add, the index holds %q — the path was not staged", staged)
	}
	// Nothing to stage is not a refusal: `git add --` with no path exits non-zero, and a caller with an
	// empty set has nothing wrong with it.
	if err := f.git.Add(f.root, nil); err != nil {
		t.Errorf("Add over no paths = %v, wanted nil", err)
	}
}

// git's own words reach the caller. A refusal summarised here sends its reader looking for a cause
// this process already had in hand.
func (f fixture) refusal(t *testing.T) {
	_, err := f.git.Show(f.root, f.head, "no/such/file.txt")
	if err == nil {
		t.Fatal("Show over a path the commit does not hold returned no error")
	}
	if !strings.Contains(err.Error(), "no/such/file.txt") {
		t.Errorf("the refusal reads %q, and does not carry git's own account of it", err)
	}
}

// A tool run from a git hook is given GIT_DIR, and git reads that before the directory it was handed —
// so without this the tool answers about the hook's repository whatever it was asked about.
func TestAnInheritedGitDirDoesNotDecideTheRepository(t *testing.T) {
	f := newFixture(t)
	other := filepath.Join(t.TempDir(), "other")
	mustRun(t, t.TempDir(), "git", "init", "-q", other)

	stray := filepath.Join(other, ".git")
	stripped := repo.Exec{Env: repo.WithoutGitLocation(append(os.Environ(), "GIT_DIR="+stray))}
	if got := f.str(stripped.CommonDir(f.root)); !sameFile(t, got, filepath.Join(f.root, ".git")) {
		t.Errorf("CommonDir under an inherited GIT_DIR = %q, wanted the store of the directory it was "+
			"asked about, %s", got, filepath.Join(f.root, ".git"))
	}

	// The control. Without the stripping the same call answers about the other repository, so the
	// assertion above is measuring the stripping and not a git that ignores the variable.
	inheriting := repo.Exec{Env: append(os.Environ(), "GIT_DIR="+stray)}
	if got, err := inheriting.CommonDir(f.root); err == nil && sameFile(t, got, filepath.Join(f.root, ".git")) {
		t.Errorf("an inherited GIT_DIR changed nothing, so the case above holds no property of "+
			"WithoutGitLocation: CommonDir answered %q either way", got)
	}
}

// The adapter's answers, unwrapped. Methods rather than one generic function, because a call whose
// arguments are another call's results may carry nothing else — and a method's receiver is not an
// argument, while a *testing.T parameter would be. Go has no generic methods, so there is one per
// answer shape.
func (f fixture) str(value string, err error) string {
	f.t.Helper()
	if err != nil {
		f.t.Fatalf("the adapter refused: %v", err)
	}
	return value
}

func (f fixture) list(value []string, err error) []string {
	f.t.Helper()
	if err != nil {
		f.t.Fatalf("the adapter refused: %v", err)
	}
	return value
}

func (f fixture) changes(value []repo.Change, err error) []repo.Change {
	f.t.Helper()
	if err != nil {
		f.t.Fatalf("the adapter refused: %v", err)
	}
	return value
}

func (f fixture) worktrees(value []repo.Worktree, err error) []repo.Worktree {
	f.t.Helper()
	if err != nil {
		f.t.Fatalf("the adapter refused: %v", err)
	}
	return value
}

func (f fixture) body(value []byte, err error) []byte {
	f.t.Helper()
	if err != nil {
		f.t.Fatalf("the adapter refused: %v", err)
	}
	return value
}

func (f fixture) set(value map[string]bool, err error) map[string]bool {
	f.t.Helper()
	if err != nil {
		f.t.Fatalf("the adapter refused: %v", err)
	}
	return value
}

// Compared by identity, not by spelling: /var is a symlink to /private/var on macOS, so git's answer
// and the path the fixture built differ as strings over one directory.
func sameFile(t *testing.T, left, right string) bool {
	t.Helper()
	leftInfo, err := os.Stat(left)
	if err != nil {
		return false
	}
	rightInfo, err := os.Stat(right)
	if err != nil {
		return false
	}
	return os.SameFile(leftInfo, rightInfo)
}

func mustRun(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

func capture(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s %s: %v", name, strings.Join(args, " "), err)
	}
	return string(out)
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
