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

	"configs/ai/tools/repo"
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
	// Committed EMPTY, and committed under a name holding a newline: the two shapes a batch read has to
	// tell from "this revision does not hold that path".
	write(t, filepath.Join(root, "empty.txt"), "")
	write(t, filepath.Join(root, "odd\nname.txt"), "newline in the name\n")
	write(t, filepath.Join(root, "pkg", "moved.txt"), "before\n")
	write(t, filepath.Join(root, "gone.txt"), "removed later\n")
	write(t, filepath.Join(root, ".gitignore"), "ignored/\n*.out\n")
	write(t, filepath.Join(root, "tracked-but-matched.out"), "committed anyway\n")
	mustRun(t, root, "git", "add", "-A", "-f")
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
	write(t, filepath.Join(root, "odd\nname.out"), "newline in the name\n")
	// One untracked file inside pkg/, so a listing asked from that subdirectory has something to name
	// and the cases below can tell "confined to pkg/" from "the whole tree".
	write(t, filepath.Join(root, "pkg", "loose.txt"), "loose in pkg\n")

	mustRun(t, root, "git", "worktree", "add", "-q", f.linked, "-b", "other")
	return f
}

func TestTheAdapterAnswersGit(t *testing.T) {
	f := newFixture(t)
	t.Run("git's own paths", f.paths)
	t.Run("resolving revisions", f.revisions)
	t.Run("what the repository holds", f.listings)
	t.Run("what changed", f.changes_)
	t.Run("many files at one revision", f.batchContent)
	t.Run("ignores and status", f.ignores)
	t.Run("worktrees", f.worktreeList)
	t.Run("a config value", f.configValue)
	t.Run("a worktree git means to prune", f.prunableWorktree)
	t.Run("a refusal carries what git said", f.refusal)
	t.Run("what the reader's own config cannot move", f.readerConfig)
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
	// Root-relative from a subdirectory, and CONFINED to it. `ls-files` prints paths relative to where
	// git ran unless it is told otherwise, and a caller joining those against the root would open files
	// that do not exist; it also lists only what sits under that directory, which is why a caller
	// wanting the whole tree asks at the root.
	fromSub := f.list(f.git.Tracked(filepath.Join(f.root, "pkg")))
	if !slices.Equal(fromSub, []string{"pkg/moved.txt"}) {
		t.Errorf("Tracked from pkg/ = %v, wanted only the root-relative pkg/moved.txt", fromSub)
	}
	if narrowed := f.list(f.git.Tracked(f.root, "pkg")); !slices.Equal(narrowed, []string{"pkg/moved.txt"}) {
		t.Errorf("Tracked under the pathspec pkg = %v, wanted only pkg/moved.txt", narrowed)
	}
	// A pathspec is relative to the directory git ran in, never to the root: `moved.txt` asked from
	// pkg/ names pkg/moved.txt, and `pkg` asked from pkg/ names pkg/pkg and matches nothing. A stand-in
	// reading the same spec from the root answers the opposite for both, so a case about running from a
	// subdirectory would pass in the suite and fail in production.
	if named := f.list(f.git.Tracked(filepath.Join(f.root, "pkg"), "moved.txt")); !slices.Equal(named, []string{"pkg/moved.txt"}) {
		t.Errorf("Tracked from pkg/ under the pathspec moved.txt = %v, wanted pkg/moved.txt", named)
	}
	if fromSubSpec := f.list(f.git.Tracked(filepath.Join(f.root, "pkg"), "pkg")); len(fromSubSpec) != 0 {
		t.Errorf("Tracked from pkg/ under the pathspec pkg = %v, wanted nothing — that spec names pkg/pkg", fromSubSpec)
	}

	untracked := f.list(f.git.Untracked(f.root))
	// The non-ASCII name is the case: without `-z` and core.quotePath=false git prints it C-quoted and
	// the caller reads a name no file has.
	if !slices.Contains(untracked, "sundæ.txt") {
		t.Errorf("Untracked did not name sundæ.txt as it is spelt on disk: %v", untracked)
	}
	// `--exclude-standard` is why: an untracked path git ignores is not in this listing at all, so a
	// caller need not filter one out and a stand-in offering one hands it work git never gives it.
	if slices.Contains(untracked, "ignored/build.out") {
		t.Errorf("Untracked named an ignored file: %v", untracked)
	}
	if loose := f.list(f.git.Untracked(filepath.Join(f.root, "pkg"))); !slices.Equal(loose, []string{"pkg/loose.txt"}) {
		t.Errorf("Untracked from pkg/ = %v, wanted only the root-relative pkg/loose.txt", loose)
	}

	// Asked of a whole commit whatever directory git runs in, and named from the root. `ls-tree` on its
	// own answers the subtree of the directory it ran in and names it from there, which is a different
	// question and one no caller here asks.
	held := f.list(f.git.NamesAt(f.root, f.previous))
	if !slices.Contains(held, "gone.txt") {
		t.Errorf("NamesAt(previous) did not name gone.txt, which that commit held: %v", held)
	}
	if heldFromSub := f.list(f.git.NamesAt(filepath.Join(f.root, "pkg"), f.previous)); !slices.Equal(heldFromSub, held) {
		t.Errorf("NamesAt from pkg/ = %v, wanted the same whole commit the root answered, %v", heldFromSub, held)
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

	// A diff's PATHSPEC is relative to the directory git ran in, where the comparison itself is not: the
	// case above named added.txt from pkg/, and `moved.txt` here names pkg/moved.txt while `pkg` names
	// pkg/pkg and matches nothing.
	if named := f.list(f.git.Changed(filepath.Join(f.root, "pkg"), []string{f.previous, f.head}, []string{"moved.txt"})); !slices.Equal(named, []string{"pkg/moved.txt"}) {
		t.Errorf("Changed from pkg/ under the pathspec moved.txt = %v, wanted pkg/moved.txt", named)
	}
	if spec := f.list(f.git.Changed(filepath.Join(f.root, "pkg"), []string{f.previous, f.head}, []string{"pkg"})); len(spec) != 0 {
		t.Errorf("Changed from pkg/ under the pathspec pkg = %v, wanted nothing — that spec names pkg/pkg", spec)
	}
	if !slices.Contains(fromSub, "added.txt") {
		t.Errorf("Changed from pkg/ with no pathspec = %v, wanted the whole comparison including the "+
			"root's added.txt — only a pathspec narrows to where git ran", fromSub)
	}

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
	// An absent side is git's all-zero mode and all-zero object id, and a caller reading either as a
	// value asks for an object that is not there or records a mode no file had.
	if got := byPath["added.txt"]; got.OldMode != "" || got.OldBlob != "" {
		t.Errorf("an added file came back with the base-side mode %q and blob %q, and it has no base "+
			"side: git spells that all-zero, which reads as a value", got.OldMode, got.OldBlob)
	}
	if got := byPath["gone.txt"]; got.NewMode != "" || got.Blob != "" {
		t.Errorf("a deleted file came back with the new-side mode %q and blob %q", got.NewMode, got.Blob)
	}
	// Both modes, because a file that was executable at the base and is a regular file now holds the
	// same bytes on both sides — no content check recovers that it changed.
	mustRun(t, f.root, "git", "update-index", "--chmod=+x", "kept.txt")
	mode := f.changes(f.git.ChangedWithStatus(f.root, []string{"--cached"}, []string{"kept.txt"}))
	if len(mode) != 1 || mode[0].OldMode == mode[0].NewMode {
		t.Errorf("flipping the executable bit came back as %+v, and the two modes have to differ or a "+
			"caller cannot see a mode-only change at all", mode)
	}
	// Both blobs, in the other direction: reading the base content through Show is no substitute,
	// because Show applies `--textconv` and the reader's own git config can turn one on.
	if got := byPath["pkg/moved.txt"]; got.OldBlob == "" || got.OldBlob == got.Blob {
		t.Errorf("a modified file came back with base blob %q and new blob %q, so the base content is "+
			"unreachable", got.OldBlob, got.Blob)
	}
	mustRun(t, f.root, "git", "update-index", "--chmod=-x", "kept.txt")
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

	// The patch, and the four anchors a parser reads it by. Each is something the reader's own git
	// config can move, so each is asserted rather than trusted to the flag being present in the source.
	patch := string(f.body(f.git.Patch(f.root, []string{f.previous, f.head}, nil)))
	for _, anchor := range []string{"diff --git ", "+++ b/pkg/moved.txt", "@@ ", "+after"} {
		if !strings.Contains(patch, anchor) {
			t.Errorf("the patch carries no %q, which every parser of it keys off:\n%s", anchor, patch)
		}
	}
	if strings.Contains(patch, "\x1b[") {
		t.Errorf("the patch carries colour escapes, so a parser reading it sees no line it expects:\n%s", patch)
	}

	// `--text` is the load-bearing one: without it a NUL byte collapses the body to "Binary files …
	// differ" and a scan reading this exits 0 over a real hit.
	write(t, filepath.Join(f.root, "binary.dat"), "one\x00two\n")
	mustRun(t, f.root, "git", "add", "binary.dat")
	binary := string(f.body(f.git.Patch(f.root, []string{"--cached"}, []string{"binary.dat"})))
	if strings.Contains(binary, "Binary files") {
		t.Errorf("a file holding a NUL byte came back as %q rather than as its lines, so a scan over it "+
			"reports nothing and exits clean", binary)
	}
	mustRun(t, f.root, "git", "rm", "-q", "--cached", "binary.dat")
}

// The whole list from one process, which is what keeps a tool reading a few hundred files off a few
// hundred spawns. Every property here is one a caller reads the answer by.
func (f fixture) batchContent(t *testing.T) {
	asked := []string{"kept.txt", "empty.txt", "no/such/file.txt", "no\nsuch.txt", "odd\nname.txt", "pkg/moved.txt"}
	body := map[string]string{}
	var visited []string
	if err := f.git.ContentsAt(f.root, f.previous, asked, 1<<20, func(path string, content []byte) {
		visited = append(visited, path)
		body[path] = string(content)
	}); err != nil {
		t.Fatalf("ContentsAt over the previous commit: %v", err)
	}

	// In the order asked, absent paths passed over. A caller indexes the answers by its own list, so an
	// answer arriving out of order attaches one file's content to another file's name. The absent path
	// holding a NEWLINE is the case that breaks a reader taking one line as one answer: git echoes an
	// unresolvable name back verbatim, so every object after it would be read from the wrong offset.
	want := []string{"kept.txt", "empty.txt", "odd\nname.txt", "pkg/moved.txt"}
	if !slices.Equal(visited, want) {
		t.Fatalf("ContentsAt visited %q, wanted %q", visited, want)
	}
	// Held empty, and it arrives as an answer rather than as silence — the one thing that tells it from
	// a path the revision does not hold, which a caller counting files has to distinguish.
	if got, seen := body["empty.txt"]; !seen || got != "" {
		t.Errorf("a file the commit holds empty came back as (%q, %v), wanted (\"\", true)", got, seen)
	}
	if got := body["pkg/moved.txt"]; got != "before\n" {
		t.Errorf("ContentsAt at the previous commit gave %q for pkg/moved.txt, wanted the content before the change", got)
	}
	if got := body["odd\nname.txt"]; got != "newline in the name\n" {
		t.Errorf("the path holding a newline came back as %q", got)
	}

	// The cap, and the oversized object is asked for FIRST: git writes its bytes whether or not the
	// caller wants them, so a reader that does not step over exactly them reads the next file's content
	// as this one's.
	var capped []string
	if err := f.git.ContentsAt(f.root, f.previous, []string{"pkg/moved.txt", "kept.txt"}, int64(len("one\n")), func(path string, _ []byte) {
		capped = append(capped, path)
	}); err != nil {
		t.Fatalf("ContentsAt under a byte cap: %v", err)
	}
	if !slices.Equal(capped, []string{"kept.txt"}) {
		t.Errorf("under a %d-byte cap ContentsAt visited %q, wanted only the file at or under it", len("one\n"), capped)
	}

	// Nothing asked is not a refusal, and it spawns nothing: a caller with an empty set has nothing
	// wrong with it.
	if err := f.git.ContentsAt(f.root, f.previous, nil, 1<<20, func(string, []byte) {
		t.Error("ContentsAt over no paths visited something")
	}); err != nil {
		t.Errorf("ContentsAt over no paths = %v, wanted nil", err)
	}
}

func (f fixture) ignores(t *testing.T) {
	ignored := f.set(f.git.Ignored(f.root, []string{"ignored/build.out", "kept.txt", "tracked-but-matched.out", "odd\nname.out"}))
	if !ignored["ignored/build.out"] {
		t.Errorf("Ignored did not call ignored/build.out ignored: %v", ignored)
	}
	if ignored["kept.txt"] {
		t.Errorf("Ignored called the tracked kept.txt ignored: %v", ignored)
	}
	// git's own reading of a tree, which is the whole reason these tools ask rather than matching
	// patterns themselves. A TRACKED file matching an ignore rule is not ignored — a list written in Go
	// would say it is, and every caller filtering on that would drop a committed file from its scan.
	if ignored["tracked-but-matched.out"] {
		t.Errorf("Ignored called the tracked tracked-but-matched.out ignored, and git does not: a "+
			"caller filtering on this drops a file every commit carries: %v", ignored)
	}
	// The same rule through the other question, because a caller reading a source reads it as "ignored,
	// and by this file" and would act on one git never gave.
	if source := f.str(f.git.IgnoreSource(f.root, "tracked-but-matched.out")); source != "" {
		t.Errorf("IgnoreSource over the tracked tracked-but-matched.out = %q, wanted empty — git does "+
			"not call a tracked file ignored whatever a rule says", source)
	}
	// A path holding a newline, which is what `-z` on both sides of the pipe is for. Without it the
	// answer arrives as two paths and neither names a file.
	if !ignored["odd\nname.out"] {
		t.Errorf("Ignored lost the path holding a newline, so a caller reads two names no file has: %v", ignored)
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

// Prunable is git's own verdict on an entry, not a guess from the filesystem: an installer writing
// into a worktree git is about to drop would be acting on metadata rather than on a checkout. The
// directory is removed here and `worktree prune` deliberately not run, which is exactly the window a
// post-checkout sync runs in.
func (f fixture) prunableWorktree(t *testing.T) {
	gone := filepath.Join(filepath.Dir(f.root), "throwaway")
	mustRun(t, f.root, "git", "worktree", "add", "-q", gone, "-b", "throwaway")
	if err := os.RemoveAll(gone); err != nil {
		t.Fatalf("removing the worktree directory: %v", err)
	}
	var found bool
	for _, one := range f.worktrees(f.git.Worktrees(f.root)) {
		if !sameName(one.Path, gone) {
			if one.Prunable {
				t.Errorf("%s came back prunable, and it is a checkout git still has", one.Path)
			}
			continue
		}
		found = true
		if !one.Prunable {
			t.Errorf("%s came back with Prunable false, and its directory is gone", one.Path)
		}
	}
	if !found {
		t.Errorf("Worktrees stopped naming %s once its directory went, so nothing could be flagged", gone)
	}
	mustRun(t, f.root, "git", "worktree", "prune")
}

// Unset and set-to-empty are different answers, and they mean opposite things to the one caller that
// asks: core.hooksPath set empty sends git looking for hooks in the worktree root, so a hook written
// where an installer puts one never runs. git spells both as a non-zero exit with nothing on stdout,
// so only `--get`'s exit code separates them.
func (f fixture) configValue(t *testing.T) {
	if value, isSet := f.git.ConfigValue(f.root, "core.hooksPath"); isSet || value != "" {
		t.Errorf("ConfigValue over an unset key = (%q, %v), wanted (%q, false)", value, isSet, "")
	}
	mustRun(t, f.root, "git", "config", "core.hooksPath", "")
	if value, isSet := f.git.ConfigValue(f.root, "core.hooksPath"); !isSet || value != "" {
		t.Errorf("ConfigValue over a key set to the empty string = (%q, %v), wanted (%q, true)", value, isSet, "")
	}
	mustRun(t, f.root, "git", "config", "core.hooksPath", ".githooks")
	if value, isSet := f.git.ConfigValue(f.root, "core.hooksPath"); !isSet || value != ".githooks" {
		t.Errorf("ConfigValue = (%q, %v), wanted (%q, true)", value, isSet, ".githooks")
	}
	mustRun(t, f.root, "git", "config", "--unset", "core.hooksPath")
}

// Two spellings of one directory, which is what /var being a symlink to /private/var on macOS makes of
// every path here. Compared by identity where both exist; a worktree whose directory is gone has no
// identity left, so its name is compared after the one resolution that can still be made.
func sameName(left, right string) bool {
	if left == right {
		return true
	}
	leftReal, leftErr := filepath.EvalSymlinks(filepath.Dir(left))
	rightReal, rightErr := filepath.EvalSymlinks(filepath.Dir(right))
	if leftErr != nil || rightErr != nil {
		return false
	}
	return filepath.Join(leftReal, filepath.Base(left)) == filepath.Join(rightReal, filepath.Base(right))
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
// The two properties a caller's own repository configuration would otherwise decide. Both are held
// here because both are the adapter's to keep: they were being held only by one tool's cases, where a
// second tool reaching git through this file would not have been covered by them at all.
func (f fixture) readerConfig(t *testing.T) {
	// A textconv filter renders a file rather than reading it, and the attribute that turns one on is
	// written by whoever wrote the branch. A caller measuring content would then be measuring the
	// filter's output. `cat` is the filter, so a conversion that happened at all is visible as the
	// marker never reaching the caller.
	write(t, filepath.Join(f.root, ".gitattributes"), "converted.txt diff=rot\n")
	write(t, filepath.Join(f.root, "converted.txt"), "raw bytes\n")
	mustRun(t, f.root, "git", "add", ".gitattributes", "converted.txt")
	mustRun(t, f.root, "git", "commit", "-qm", "a file with a diff attribute")
	mustRun(t, f.root, "git", "config", "diff.rot.textconv", "sed s/raw/CONVERTED/")
	if got := f.str(string(f.body(f.git.Show(f.root, "HEAD", "converted.txt"))), nil); got != "raw bytes\n" {
		t.Errorf("Show returned %q, which is the reader's filter rendering the file rather than the file", got)
	}

	// A file called HEAD makes `git diff HEAD` ambiguous, and git refuses rather than guessing. The
	// branch under review can commit that file, so a patch that did not end in `--` would be a scan any
	// branch could switch off for everyone reading it.
	write(t, filepath.Join(f.root, "HEAD"), "a file, not the revision\n")
	mustRun(t, f.root, "git", "add", "HEAD")
	mustRun(t, f.root, "git", "commit", "-qm", "a file called HEAD")
	write(t, filepath.Join(f.root, "converted.txt"), "raw bytes, edited\n")
	if _, err := f.git.Patch(f.root, []string{"HEAD"}, nil); err != nil {
		t.Errorf("Patch over a repository holding a file called HEAD refused: %v", err)
	}
}

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
