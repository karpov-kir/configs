package ecoreport

import (
	"os"
	"path/filepath"
	"strings"
)

// Where the repository is, read from the layout rather than asked of git.
//
// git is still asked everywhere it decides POLICY, and this file must never grow either of those:
// `ls-files` reads the index and `check-ignore` applies ignore rules, so their answers have to be the
// ones git will give the human's next command. What is answered here is the LAYOUT, which is a
// documented on-disk fact rather than a decision: the working tree's root is the directory holding
// `.git`; a linked worktree's `.git` is a file naming its git dir; and that git dir's `commondir`
// names the shared one.
//
// Read rather than spawned because each was a child process per invocation — across this package's
// suite, `rev-parse --show-toplevel` 937 times, `--git-common-dir` 891, `--git-path` 731. On a machine
// whose security agent inspects every exec, that count IS the runtime: the suite spent its time in
// fork, not in git.
//
// Two things keep this honest. Every answer is proven equal to git's own, for each shape the suite
// builds, by TestTheLayoutResolverAgreesWithGit — a differential case, not a restatement of the rules
// below. And where the ENVIRONMENT overrides the layout, this refuses and the caller asks git instead:
// GIT_DIR, GIT_WORK_TREE and GIT_COMMON_DIR move the answer in ways the on-disk layout does not show,
// so guessing past them would be confidently wrong rather than slow.
func layoutOverridden() bool {
	for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR"} {
		if os.Getenv(name) != "" {
			return true
		}
	}
	return false
}

// The working tree's root: the nearest ancestor of dir holding `.git`, physically resolved.
//
// Physical, because that is what `rev-parse --show-toplevel` answers — on macOS a temp dir sits under
// `/var`, a symlink to `/private/var`, and a logical answer would differ from git's on every fixture.
func layoutRoot(dir string) (string, bool) {
	if layoutOverridden() {
		return "", false
	}
	probe, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", false
	}
	for {
		if _, err := os.Lstat(filepath.Join(probe, ".git")); err == nil {
			return probe, true
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return "", false
		}
		probe = parent
	}
}

// This worktree's own git dir — `<root>/.git` in an ordinary repo, and whatever the `.git` FILE names
// in a linked worktree. Absolute, because a relative `gitdir:` is relative to the worktree holding it.
func layoutGitDir(root string) (string, bool) {
	if layoutOverridden() {
		return "", false
	}
	candidate := filepath.Join(root, ".git")
	info, err := os.Lstat(candidate)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return candidate, true
	}
	// A `.git` file, which is how a linked worktree points at its git dir. One line, `gitdir: <path>`.
	// Anything else is a shape this does not know, and an unknown shape is git's question, not ours.
	body, err := os.ReadFile(candidate)
	if err != nil {
		return "", false
	}
	pointer, ok := strings.CutPrefix(strings.TrimSpace(string(body)), "gitdir:")
	if !ok {
		return "", false
	}
	pointer = strings.TrimSpace(pointer)
	if pointer == "" {
		return "", false
	}
	if !filepath.IsAbs(pointer) {
		pointer = filepath.Join(root, pointer)
	}
	return filepath.Clean(pointer), true
}

// The git dir shared by every worktree of one repository. A linked worktree's git dir holds a
// `commondir` file naming it; an ordinary repo's git dir IS the common one.
//
// This is the difference the scratch location rests on: a date or a report keyed on the per-worktree
// dir is invisible from every other worktree of the same repository.
func layoutCommonDir(gitDir string) (string, bool) {
	if layoutOverridden() {
		return "", false
	}
	body, err := os.ReadFile(filepath.Join(gitDir, "commondir"))
	if err != nil {
		if os.IsNotExist(err) {
			return gitDir, true
		}
		return "", false
	}
	pointer := strings.TrimSpace(string(body))
	if pointer == "" {
		return "", false
	}
	if !filepath.IsAbs(pointer) {
		pointer = filepath.Join(gitDir, pointer)
	}
	return filepath.Clean(pointer), true
}
