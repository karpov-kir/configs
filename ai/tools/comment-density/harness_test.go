package density

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	gitrepo "kk-flavor/tools/repo"
	"kk-flavor/tools/repo/repotest"
)

// A repository the tool can be driven against without forking git. repotest.Fake answers every question
// the port asks; the files also sit in a temp directory, because the working-tree half of the bar and
// the untracked half of the scan open them with os.Lstat rather than through the port.
type repo struct {
	t    *testing.T
	dir  string
	fake *repotest.Fake
	// git is what Run is handed — the fake, unless a case wraps it to record which directory a question
	// went to.
	git gitrepo.Git
	// history is the commits, newest first. Every commit re-points HEAD, HEAD~1, … so a case names
	// revisions the way it would against git.
	history []map[string]string
	// onDisk is what each file holds, so a commit can snapshot the tree without reading it back.
	onDisk map[string]string
	stdout strings.Builder
	stderr strings.Builder
	code   int
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	r := newUnbornRepo(t)
	// One commit before any case starts, as the seed repository used to give: a repository with none is
	// a refusal of its own, and TestBarInARepositoryWithNoCommitNamesThat is where that belongs.
	r.write("seed.txt", "seed\n")
	r.commit("base")
	return r
}

// A repository with no commit at all, which is what a `git init` leaves behind.
func newUnbornRepo(t *testing.T) *repo {
	t.Helper()
	dir := t.TempDir()
	fake := repotest.New(dir)
	return &repo{t: t, dir: dir, fake: fake, git: fake, onDisk: map[string]string{}}
}

// write puts a file in the working tree. A file the index does not hold yet is untracked, which is what
// git would say of one written and not committed.
func (r *repo) write(name, body string) {
	r.t.Helper()
	full := filepath.Join(r.dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatalf("could not create the parent for %s: %v", name, err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		r.t.Fatalf("could not write the fixture %s: %v", name, err)
	}
	r.onDisk[name] = body
	if _, tracked := r.fake.Revs[repotest.WorkTree][name]; tracked {
		r.fake.Write(name, body)
		return
	}
	if !slices.Contains(r.fake.UntrackedPaths, name) {
		r.fake.AddUntracked(name)
	}
}

// symlink plants a link in the working tree and lists it untracked, as git would.
func (r *repo) symlink(name, target string) {
	r.t.Helper()
	if err := os.Symlink(target, filepath.Join(r.dir, name)); err != nil {
		r.t.Fatalf("could not plant the symlink %s: %v", name, err)
	}
	r.fake.AddUntracked(name)
}

// remove deletes a file from the tree and the index, which is what `git rm` leaves behind.
func (r *repo) remove(name string) {
	r.t.Helper()
	if err := os.Remove(filepath.Join(r.dir, name)); err != nil {
		r.t.Fatalf("could not delete the fixture %s: %v", name, err)
	}
	delete(r.onDisk, name)
	delete(r.fake.Revs[repotest.WorkTree], name)
	r.fake.UntrackedPaths = slices.DeleteFunc(r.fake.UntrackedPaths, func(held string) bool { return held == name })
}

// commit stages everything and snapshots it. message is carried so a case still says at its call site
// what the commit is for; the fake names commits by their distance from HEAD and never by their message.
func (r *repo) commit(message string) {
	r.t.Helper()
	for _, name := range r.fake.UntrackedPaths {
		r.fake.Write(name, r.onDisk[name])
	}
	r.fake.UntrackedPaths = nil
	held := map[string]string{}
	for name, body := range r.fake.Revs[repotest.WorkTree] {
		held[name] = body
	}
	r.history = append([]map[string]string{held}, r.history...)
	r.nameHistory()
}

// The fake models no revision grammar, so every name a case passes has to be one it holds: HEAD and
// HEAD~n after each commit, plus the merge base of every pair, which on one line of history is the older
// of the two. Each commit answers under its object id as well as its name, because a merge base comes
// back as an id and the listing taken at it has to find the same tree.
func (r *repo) nameHistory() {
	names := make([]string, len(r.history))
	for i, held := range r.history {
		names[i] = "HEAD"
		if i > 0 {
			names[i] = fmt.Sprintf("HEAD~%d", i)
		}
		r.fake.Commit(names[i], held)
		r.fake.Commit(r.fake.Refs[names[i]], held)
	}
	for i, newer := range names {
		for _, older := range names[i:] {
			r.fake.Bases[newer+"\x00"+older] = r.fake.Refs[older]
		}
	}
}

// diffs states the patch git would print for this change. Stated rather than derived from the fixture:
// which lines a change added is git's own answer, and a suite deriving it would be agreeing with itself
// — repotest.Fake answers patch text verbatim for exactly that reason. Unstated, the patch is empty,
// which is what git prints for a change that only added untracked files.
func (r *repo) diffs(patches ...string) {
	r.fake.PatchText = strings.Join(patches, "")
}

// patchOf is the diff of a file every line of which the change added — one it created, or rewrote whole.
func patchOf(file, body string) string {
	return patchAdding(file, addedLines(body)...)
}

func patchAdding(file string, added ...string) string {
	return patchHeaded(file, "b/"+file, added...)
}

// patchHeaded states the `+++ ` field apart from the name, so a case can drive the C-quoted form git
// prints for a path holding a control character.
func patchHeaded(file, header string, added ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n--- a/%s\n+++ %s\n@@ -1 +1,%d @@\n", file, file, file, header, len(added))
	for _, line := range added {
		fmt.Fprintf(&b, "+%s\n", line)
	}
	return b.String()
}

func addedLines(body string) []string {
	return strings.Split(strings.TrimSuffix(body, "\n"), "\n")
}

// rewrote writes a file and states the diff that landed it: every line added, which is what git prints
// for a file the change created or rewrote whole.
func (r *repo) rewrote(name, body string) {
	r.t.Helper()
	r.write(name, body)
	r.fake.PatchText += patchOf(name, body)
}

func baseConfig() Config {
	return Config{MaxRatio: defaultMaxRatio, MinLines: defaultMinLines, MaxFileBytes: defaultMaxFileBytes}
}

func (r *repo) run(args ...string) {
	r.runWith(baseConfig(), args...)
}

func (r *repo) runWith(cfg Config, args ...string) {
	r.runIn(r.dir, cfg, args...)
}

func (r *repo) runIn(cwd string, cfg Config, args ...string) {
	r.stdout.Reset()
	r.stderr.Reset()
	r.code = Run("comment-density.sh", args, cwd, r.git, cfg, &r.stdout, &r.stderr)
}

func (r *repo) expectCode(want int) {
	r.t.Helper()
	if r.code != want {
		r.t.Errorf("exit %d, wanted %d\nstdout: %s\nstderr: %s", r.code, want, r.stdout.String(), r.stderr.String())
	}
}

func (r *repo) expectStdoutHas(want string) {
	r.t.Helper()
	if !strings.Contains(r.stdout.String(), want) {
		r.t.Errorf("wanted %q on stdout, got: %s", want, r.stdout.String())
	}
}

func (r *repo) expectStdoutLacks(unwanted string) {
	r.t.Helper()
	if strings.Contains(r.stdout.String(), unwanted) {
		r.t.Errorf("%q appears on stdout: %s", unwanted, r.stdout.String())
	}
}

// A refused run must leave nothing on stdout: anything there is what a caller capturing the report
// reads as a finding.
func (r *repo) expectNoStdout() {
	r.t.Helper()
	if r.stdout.Len() != 0 {
		r.t.Errorf("expected nothing on stdout, got: %s", r.stdout.String())
	}
}

func (r *repo) expectStderrHas(want string) {
	r.t.Helper()
	if !strings.Contains(r.stderr.String(), want) {
		r.t.Errorf("wanted %q on stderr, got: %s", want, r.stderr.String())
	}
}

func (r *repo) expectStderrLacks(unwanted string) {
	r.t.Helper()
	if strings.Contains(r.stderr.String(), unwanted) {
		r.t.Errorf("%q appears on stderr: %s", unwanted, r.stderr.String())
	}
}

func heavy(comments, code int) string {
	var b strings.Builder
	for i := 0; i < comments; i++ {
		fmt.Fprintf(&b, "// comment %d\n", i)
	}
	for i := 0; i < code; i++ {
		fmt.Fprintf(&b, "x := %d\n", i)
	}
	return b.String()
}
