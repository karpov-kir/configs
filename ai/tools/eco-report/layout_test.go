package ecoreport

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The layout resolver replaced three git subprocesses with filesystem reads, so what has to be held is
// not that it follows the rules in layout.go — that would be the same claim twice, in two places, and
// a rule restated cannot catch a rule that is wrong. What is held is that it gives GIT'S OWN ANSWER,
// asked side by side, for every shape this package's fixtures build.
//
// A rule-shaped test would have passed on the first draft of `layoutRoot`, which returned the logical
// path where git returns the physical one — right by the documented rule and wrong on every fixture
// under macOS's /var.
func TestTheLayoutResolverAgreesWithGit(t *testing.T) {
	base := t.TempDir()
	main := filepath.Join(base, "main")
	mustGit(t, "", "init", "-q", main)
	for _, args := range [][]string{
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
	} {
		mustGit(t, main, args...)
	}
	if err := os.WriteFile(filepath.Join(main, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	mustGit(t, main, "add", "seed.txt")
	mustGit(t, main, "commit", "-qm", "base")

	deep := filepath.Join(main, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	shapes := []struct{ name, dir string }{
		{"an ordinary repository, from its root", main},
		{"an ordinary repository, from a subdirectory", deep},
	}

	// A linked worktree is the shape the whole common-dir distinction exists for, and the one a
	// filesystem read is most likely to get wrong: its `.git` is a FILE, and its git dir is not its
	// common dir. Skipped rather than failed where git cannot build one, since that is the machine's
	// limitation and not this resolver's.
	linked := filepath.Join(base, "linked")
	if err := exec.Command("git", "-C", main, "worktree", "add", "-q", linked, "-b", "other").Run(); err == nil {
		shapes = append(shapes, struct{ name, dir string }{"a linked worktree", linked})
		sub := filepath.Join(linked, "c")
		if err := os.MkdirAll(sub, 0o755); err == nil {
			shapes = append(shapes, struct{ name, dir string }{"a linked worktree, from a subdirectory", sub})
		}
	} else {
		t.Log("git worktree add is unavailable here, so the linked-worktree shapes were not compared")
	}
	// The control on the instrument: two shapes is the ordinary pair, and a run that compared only
	// those has not touched the case the resolver is most likely to fail.
	if len(shapes) < 3 {
		t.Log("only the ordinary shapes were compared — the linked-worktree arms did not run")
	}

	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			gotRoot, ok := layoutRoot(shape.dir)
			if !ok {
				t.Fatalf("the resolver declined %s, so nothing was compared", shape.dir)
			}
			wantRoot := askGit(t, shape.dir, "rev-parse", "--show-toplevel")
			if gotRoot != wantRoot {
				t.Errorf("root: resolver says %q, git says %q", gotRoot, wantRoot)
			}

			gotGitDir, ok := layoutGitDir(gotRoot)
			if !ok {
				t.Fatalf("the resolver found no git dir for %s", gotRoot)
			}
			wantGitDir := absolute(shape.dir, askGit(t, shape.dir, "rev-parse", "--git-dir"))
			if !sameDir(t, gotGitDir, wantGitDir) {
				t.Errorf("git dir: resolver says %q, git says %q", gotGitDir, wantGitDir)
			}

			gotCommon, ok := layoutCommonDir(gotGitDir)
			if !ok {
				t.Fatalf("the resolver found no common dir for %s", gotGitDir)
			}
			wantCommon := absolute(shape.dir, askGit(t, shape.dir, "rev-parse", "--git-common-dir"))
			if !sameDir(t, gotCommon, wantCommon) {
				t.Errorf("common dir: resolver says %q, git says %q", gotCommon, wantCommon)
			}

			// The name form the tool actually uses, and the one a caller writes a marker through.
			gotPath := filepath.Join(gotGitDir, "idsd-worktree-id")
			wantPath := absolute(shape.dir, askGit(t, shape.dir, "rev-parse", "--git-path", "idsd-worktree-id"))
			if !sameDir(t, gotPath, wantPath) {
				t.Errorf("git path: resolver says %q, git says %q", gotPath, wantPath)
			}
		})
	}
}

// An environment override moves the answer in a way the on-disk layout does not show, so the resolver
// must decline and let the caller ask git. Declining is the whole guard: a resolver that answered from
// the layout here would be confidently wrong rather than slow.
func TestTheResolverDeclinesWhenTheEnvironmentOverridesTheLayout(t *testing.T) {
	base := t.TempDir()
	mustGit(t, "", "init", "-q", base)
	if _, ok := layoutRoot(base); !ok {
		t.Fatalf("the resolver declined a plain repository, so the case below proves nothing")
	}
	for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR"} {
		t.Run(name+" set", func(t *testing.T) {
			t.Setenv(name, base+"/elsewhere")
			if _, ok := layoutRoot(base); ok {
				t.Errorf("the resolver answered with %s set, where only git knows the answer", name)
			}
			if _, ok := layoutGitDir(base); ok {
				t.Errorf("layoutGitDir answered with %s set", name)
			}
			if _, ok := layoutCommonDir(base + "/.git"); ok {
				t.Errorf("layoutCommonDir answered with %s set", name)
			}
		})
	}
}

// A `.git` file whose contents are not the one shape this knows is git's question, not ours.
func TestAnUnknownGitFileShapeIsDeclined(t *testing.T) {
	base := t.TempDir()
	for _, body := range []string{"", "not a pointer\n", "gitdir:\n", "gitdir:   \n"} {
		root := filepath.Join(base, "r"+strings.ReplaceAll(strings.TrimSpace(body), " ", "_"))
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, ".git"), []byte(body), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if _, ok := layoutGitDir(root); ok {
			t.Errorf("a .git file holding %q was read as a pointer", body)
		}
	}
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func askGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s in %s: %v", strings.Join(args, " "), dir, err)
	}
	return strings.TrimRight(string(out), "\n")
}

// git answers `--git-dir` and friends relative to the CALLER'S DIRECTORY in an ordinary repo, and
// absolute in a linked worktree, so a comparison has to absolutize against that directory before it
// means anything. Against the repo ROOT instead, a call from two levels down resolves two levels too
// high — which is what the first draft of this file did, and what its own failure caught.
func absolute(root, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(root, path))
}

// Compared physically, because one side may reach the same directory through a symlink — /var against
// /private/var on macOS — and a literal comparison would fail on every fixture.
func sameDir(t *testing.T, a, b string) bool {
	t.Helper()
	resolve := func(path string) string {
		if got, err := filepath.EvalSymlinks(path); err == nil {
			return got
		}
		// A path that does not exist yet — a marker not written — still compares by its parent.
		parent, err := filepath.EvalSymlinks(filepath.Dir(path))
		if err != nil {
			return filepath.Clean(path)
		}
		return filepath.Join(parent, filepath.Base(path))
	}
	return resolve(a) == resolve(b)
}
