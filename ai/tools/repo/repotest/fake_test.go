// The cases this fake earns, and no more. Almost everything it answers is a table a case filled in.
// A test asserting that a table hands back what was put in it tests the case while leaving the code
// untested. `testing.md` §3 asks for a fake's own test only where a bug in it could pass silently.
//
// Three answers the fake DERIVES, one case each. A bug in one is invisible from the suites driving
// it: the fixture goes quiet in a way that reads as a verdict about the tool.
//
// Each case says what git would answer, which is how every case driving this fake is written. That
// real git answers so is `repo/exec_test.go`'s, against a real repository. No case here forks one.
package repotest_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"configs/ai/tools/repo/repotest"
)

// git answers about a commit by its object id as readily as by the name that reached it. A tool that
// resolves a base-ref to an id and then diffs against THAT asks by the id. A fake holding the commit
// under its name alone answers "no such revision" to the tool's second question. The suite then
// reads a hole in the fixture as the tool refusing.
func TestACommitAnswersToItsObjectIdAsWellAsItsName(t *testing.T) {
	git := repotest.New("/repo")
	git.Commit("base", map[string]string{"tracked.txt": "base\n"})
	git.Write("tracked.txt", "changed\n")

	id := git.Refs["base"]
	if id == "" {
		t.Fatal("the commit resolved to no object id, so this case cannot ask by one")
	}

	names, err := git.NamesAt("/repo", id)
	if err != nil || !slices.Equal(names, []string{"tracked.txt"}) {
		t.Errorf("NamesAt(%s) = (%v, %v), wanted the commit's own files", id, names, err)
	}
	body, err := git.Show("/repo", id, "tracked.txt")
	if err != nil || string(body) != "base\n" {
		t.Errorf("Show(%s, tracked.txt) = (%q, %v), wanted what the commit holds", id, body, err)
	}
	if resolved, err := git.Resolve("/repo", id); err != nil || resolved != id {
		t.Errorf("Resolve(%s) = (%q, %v) — `rev-parse` answers an object id with itself", id, resolved, err)
	}
	changed, err := git.Changed("/repo", []string{id}, nil)
	if err != nil || !slices.Equal(changed, []string{"tracked.txt"}) {
		t.Errorf("Changed against %s = (%v, %v), wanted the file the working tree moved", id, changed, err)
	}
}

// A linked worktree is one clone seen from another directory. Its git dir is its own — that is where
// its identity is minted — and its common dir, history and ignore rules are the clone's. A sibling
// with its own history is a second clone wearing a worktree's shape, and it hides the whole point of
// sharing a clone. A case driving two trees of one repository through it tests neither.
func TestALinkedWorktreeSharesTheCloneAndAnswersItsOwnGitDir(t *testing.T) {
	clone := repotest.New("/clone")
	linked := clone.LinkedWorktreeAt("/linked", "/clone/.git/worktrees/wt")

	if dir, err := linked.GitDir("/linked"); err != nil || dir != "/clone/.git/worktrees/wt" {
		t.Errorf("GitDir = (%q, %v), wanted the worktree's own git dir", dir, err)
	}
	if dir, err := linked.CommonDir("/linked"); err != nil || dir != "/clone/.git" {
		t.Errorf("CommonDir = (%q, %v), wanted the store the clone shares", dir, err)
	}
	if root, err := linked.TopLevel("/linked"); err != nil || root != "/linked" {
		t.Errorf("TopLevel = (%q, %v), wanted the linked checkout", root, err)
	}

	// The commit lands AFTER the worktree was added. What the linked worktree reads here is the
	// clone's own history as it stands now, and never a copy of it taken at that moment.
	clone.Commit("HEAD", map[string]string{"tracked.txt": "base\n"})
	names, err := linked.NamesAt("/linked", "HEAD")
	if err != nil || !slices.Equal(names, []string{"tracked.txt"}) {
		t.Errorf("NamesAt(HEAD) from the linked worktree = (%v, %v), wanted the clone's commit", names, err)
	}

	// The refusal switch is the clone's too. A case turning one question off means it off wherever
	// that repository is asked. A fake answering happily from the sibling sends the case's refusal
	// down a path it never meant to drive.
	clone.Fail["Tracked"] = errors.New("fatal: unable to read the index")
	if _, err := linked.Tracked("/linked"); err == nil {
		t.Error("the linked worktree answered a question the clone was told to refuse")
	}
}

// What a repository answers that no table can hold: what is on disk and outside the index, what
// `add` swept up, and what a `.gitignore` written MID-RUN says. A tool that writes such a rule and
// then asks whether it took effect is answered by an arrangement made before the run. That
// arrangement matches whatever the tool wrote, and it can never fail.
func TestOnDiskModeAnswersAboutTheTreeRatherThanATable(t *testing.T) {
	git := repotest.New(filepath.Join(t.TempDir(), "repo"))
	if err := git.OnDisk(); err != nil {
		t.Fatalf("building the repository on disk: %v — nothing was tested", err)
	}
	git.Commit("HEAD", map[string]string{"tracked.txt": "base\n"}).Write("tracked.txt", "base\n")
	write(t, git.Root, "tracked.txt", "base\n")
	write(t, git.Root, "records/one.md", "# one\n")
	write(t, git.Root, "records/build.out", "noise\n")
	write(t, git.Root, ".gitignore", "*.out\n")

	untracked, err := git.Untracked(git.Root)
	if err != nil || !slices.Equal(untracked, []string{".gitignore", "records/one.md"}) {
		t.Errorf("Untracked = (%v, %v) — `ls-files --others --exclude-standard` answers what is on "+
			"disk, outside the index and unignored", untracked, err)
	}

	source, err := git.IgnoreSource(git.Root, "records/build.out")
	if err != nil || !strings.Contains(source, ".gitignore") {
		t.Errorf("IgnoreSource over an ignored path = (%q, %v), wanted the .gitignore that decided", source, err)
	}
	if source, err := git.IgnoreSource(git.Root, "records/one.md"); err != nil || source != "" {
		t.Errorf("IgnoreSource over a path no rule covers = (%q, %v), wanted empty", source, err)
	}

	// `git add <directory>` stages what it sweeps up and passes over what a rule covers, in silence.
	if err := git.Add(git.Root, []string{"records"}); err != nil {
		t.Fatalf("adding a directory whose files are partly ignored: %v — git exits 0 there", err)
	}
	if !slices.Equal(git.Staged, []string{"records/one.md"}) {
		t.Errorf("the add staged %v, wanted the one file no rule covers — a fake recording only the "+
			"pathspec answers that nothing was staged, and every promotion then refuses", git.Staged)
	}
	if body := git.Revs[repotest.WorkTree]["records/one.md"]; body != "# one\n" {
		t.Errorf("the index holds %q for the staged file, wanted what was on disk", body)
	}
	if _, staged := git.Revs[repotest.WorkTree]["records/build.out"]; staged {
		t.Error("a directory pathspec staged a file a rule covers")
	}

	// The same file NAMED is refused outright. A caller naming its files one by one is counting on
	// that refusal, and a fake that swallowed it would report a file as staged that git never took.
	if err := git.Add(git.Root, []string{"records/build.out"}); err == nil {
		t.Error("`add` over a named ignored path exited 0, and git refuses it")
	}
}

func write(t *testing.T, root, name, body string) {
	t.Helper()
	full := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("building the fixture directory for %s: %v", name, err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("writing the fixture %s: %v", name, err)
	}
}

// A pathspec that climbs out of the repository. Git names it and refuses, and a fake that walked it
// would send WalkDir up into whatever sits above the fixture.
func TestAPathspecThatClimbsOutOfTheTreeIsRefused(t *testing.T) {
	git := repotest.New(filepath.Join(t.TempDir(), "repo"))
	if err := git.OnDisk(); err != nil {
		t.Fatalf("could not build the fixture repo: %v — nothing was measured", err)
	}
	outside := filepath.Join(filepath.Dir(git.Root), "outside.txt")
	if err := os.WriteFile(outside, []byte("not this tree's\n"), 0o644); err != nil {
		t.Fatalf("writing the file above the fixture: %v — nothing was measured", err)
	}

	err := git.Add(git.Root, []string{"../outside.txt"})
	if err == nil {
		t.Fatal("a pathspec above the repository was accepted, so the walk ran outside the fixture")
	}
	if !strings.Contains(err.Error(), "outside repository") {
		t.Errorf("the refusal says %q, which does not name the cause git names", err)
	}
	for _, held := range git.Staged {
		if strings.Contains(held, "outside.txt") {
			t.Errorf("the file above the fixture was staged as %q", held)
		}
	}
}
