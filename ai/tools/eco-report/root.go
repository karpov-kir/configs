package ecoreport

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"kk-flavor/tools/shell"
)

// Where the scratch directory lives, and the only place that decides it. Committed mode keeps it in
// the tree, where git tracks it and every worktree gets it for free. Throwaway mode holds it outside
// the tree entirely — the shared git dir by default, or wherever this machine's override points.
//
// One rule shapes all of it: the location must resolve the same from every worktree of one clone.
// `--git-common-dir` is what delivers that, and the two obvious alternatives silently do not.
// `--git-path` answers the per-worktree git dir; `--show-toplevel` answers the worktree's own
// directory. A location built from either gives each worktree its own scratch and reproduces the bug
// this file exists to fix, while every command still reports success.
//
// Nothing here is reached by hand from a skill: `report.sh root` prints what this resolved, and that
// is how the skills that used to hardcode `.idsd/` find it.

// The directory name under the shared git dir. Not the git dir root, so it cannot collide with the
// per-repository records other skills keep there — `cadence.sh`'s `idsd-audit-offer` is one.
const sharedDirName = "idsd"

// How much of the realpath digest goes into a repo key. Six hex is 16.7M values over the handful of
// clones one machine holds, and the key is only ever compared to itself.
const repoKeyDigestLength = 6

// The in-tree location. What `repoMode` asks git about and what `promote` writes into, whatever the
// resolved scratch root turns out to be.
func (r *run) treeIdsdDir() string { return r.root + "/.idsd" }

// Absolute path to <name> in the git dir SHARED by every worktree of this clone. The sibling of
// gitPath, and the difference between them is the whole point of this file: gitPath is per-worktree,
// which is right for a ship's own stage markers and wrong for everything else here.
//
// Absolutized against the root, because `--git-common-dir` answers relative to the caller in an
// ordinary repo — a bare `.git` from the root — so a relative answer would resolve against whatever
// directory the next caller happened to stand in.
func (r *run) gitCommonPath(name string) string {
	path, status := r.captureGit(r.errOut, "rev-parse", "--git-common-dir")
	if status != 0 || path == "" {
		r.refuse("error: could not resolve this repository's shared git dir (git rev-parse --git-common-dir) —",
			"  the idsd scratch location is unknown, so nothing was read and nothing was written.")
	}
	if !filepath.IsAbs(path) {
		path = r.root + "/" + path
	}
	path = filepath.Clean(path)
	if name == "" {
		return path
	}
	return path + "/" + name
}

// This machine's override file. Alongside the flavor's other machine-local override, so one directory
// holds every per-machine setting and none of them makes the checkout dirty.
//
// A config home that is not absolute is treated as unset, which is what the XDG spec itself says to do
// with one. Taken as given it resolves against whatever directory the process stands in — so a
// checkout shipping `kk-flavor/idsd.conf` would choose where this clone's reports are written and
// which directory `discard` removes. The tree under review does not get to decide that. Absent and
// non-absolute both fall to the default, since `filepath.IsAbs("")` is false.
func (r *run) overrideConfigPath() string {
	config := r.configHome
	if !filepath.IsAbs(config) {
		config = r.home + "/.config"
	}
	return config + "/kk-flavor/idsd.conf"
}

// The override's root, or empty when there is no override file.
//
// Strict: a file that is present but unusable refuses, and no path here falls back to the default. A
// silent fallback would write this clone's intents into a directory the human was never told about,
// which is the one failure an override must not have. Absent is different from broken, and only absent
// is quiet.
func (r *run) overrideRoot() string {
	path := r.overrideConfigPath()
	// `IsSymlink` as well, so a dangling link refuses instead of reading as absent — an existence test
	// alone cannot see one.
	if !shell.PathExists(path) && !shell.IsSymlink(path) {
		return ""
	}
	if !shell.IsRegularFile(path) || !isReadable(path) {
		r.refuse("error: "+path+" is not a readable file — the idsd scratch location is unknown.",
			"  Fix it or remove it. Falling back to the default would put this repo's scratch somewhere you were not told about.")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		r.refuse("error: could not read " + path + " (" + err.Error() + ") — the idsd scratch location is unknown, and nothing was read or written.")
	}
	root := ""
	for _, line := range shell.SplitLines(string(content)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := shell.SplitFields(trimmed)
		// Refused rather than skipped, for the reason the whole function is strict: a line the human
		// meant as a setting, silently ignored, is a scratch dir somewhere they are not looking. The
		// echo is one-lined, since this value reaches the terminal and the file is hand-written.
		if len(fields) != 2 || fields[0] != "root" {
			r.refuse("error: "+path+" has a line this does not understand: "+shell.Oneline(trimmed),
				"  the only supported line is `root <path>`. Nothing was read or written.")
		}
		if root != "" {
			r.refuse("error: " + path + " sets `root` more than once — which one wins is not this tool's guess to make. Nothing was read or written.")
		}
		root = fields[1]
	}
	if root == "" {
		r.refuse("error: "+path+" sets no `root` — the idsd scratch location is unknown.",
			"  Add a `root <path>` line, or remove the file to use the default (this repository's shared git dir).")
	}
	// `~` expanded here because this file is written by hand, where `~/…` is what a person types and
	// nothing else would expand it. Only the `~/` prefix: a bare `~user` form would need passwd lookup
	// to mean anything, and guessing it wrong builds a literal directory called `~user`.
	if root == "~" || strings.HasPrefix(root, "~/") {
		root = r.home + strings.TrimPrefix(root, "~")
	}
	if !filepath.IsAbs(root) {
		r.refuse("error: "+path+" sets a relative root ("+shell.Oneline(root)+") — the idsd scratch location is unknown.",
			"  A relative path would resolve against whatever directory the caller stood in, so it must be absolute (or start with `~/`).")
	}
	root = filepath.Clean(root)
	r.assertOverrideRootIsTrustworthy(path, root)
	return root
}

// An override root has to be somewhere only its owner can steer. `discard` calls RemoveAll on what this
// resolves, and every report beneath it carries a pass's security findings — so a directory another
// account can replace or rename is a directory another account can aim that deletion with.
//
// Both checks are on the root itself, and deliberately not on its ancestors. Refusing a symlinked
// ancestor would outlaw ordinary setups — a home directory that is itself a symlink is common, and on
// macOS `/tmp` is one. So this refuses what the human wrote in the config and stops there: that value
// is the one they chose, and the one this tool can fairly hold them to.
func (r *run) assertOverrideRootIsTrustworthy(configPath, root string) {
	if shell.IsSymlink(root) {
		r.refuse("error: "+configPath+" sets a root that is a symlink ("+shell.Oneline(root)+" -> "+shell.Oneline(readLink(root))+") — nothing was read or written.",
			"  `discard` removes this directory, so whoever can repoint the link chooses what that removes.",
			"  Point `root` at a real directory.")
	}
	info, err := os.Stat(root)
	if err != nil {
		// Absent is fine and ordinary — the first run creates it. Anything else is a directory whose mode
		// could not be read, and a permission this tool could not check must not pass as one it did.
		if errors.Is(err, fs.ErrNotExist) {
			return
		}
		r.refuse("error: could not read " + shell.Oneline(root) + " (" + err.Error() + ") — whether it is safe to write reports there is unknown, so nothing was read or written.")
	}
	if mode := info.Mode().Perm(); mode&0o022 != 0 {
		r.refuse("error: "+configPath+" sets a root that is group- or world-writable ("+shell.Oneline(root)+", mode "+fmt.Sprintf("%04o", mode)+") — nothing was read or written.",
			"  Every report written there carries a pass's findings, and `discard` removes the directory,",
			"  so another account could read the first and steer the second. `chmod go-w` it, then re-run.")
	}
}

// This clone's directory name under an override root: `<basename>-<digest>`, readable and unique.
//
// The basename comes from the shared git dir's PARENT, never from `--show-toplevel`. That answers the
// worktree's own directory name, so a key built from it hands every worktree a different scratch dir
// — the exact bug, wearing the shape of a fix.
//
// The digest is over the same realpath, which is the only sound identity available: two clones of one
// repository share a remote URL and often a basename, and never a git-dir realpath. Realpath rather
// than the path as given, so reaching the same clone through a symlinked parent resolves to one key.
func (r *run) repoKey() string {
	shared := r.gitCommonPath("")
	real := shell.CanonicalDir(shared)
	if real == "" {
		// Not a fallback to the unresolved path: that would key one clone two ways depending on how the
		// caller reached it, and the two scratch dirs would each hold half the ship.
		r.refuse("error: could not resolve "+shared+" to a real path — this clone cannot be named under the override root.",
			"  Nothing was read or written.")
	}
	digest := sha256.Sum256([]byte(real))
	name := shell.BaseName(shell.DirName(real))
	if name == "" || name == "/" || name == "." {
		name = "repo"
	}
	return name + "-" + hex.EncodeToString(digest[:])[:repoKeyDigestLength]
}

// Decide the scratch root for this invocation. Called once, before any subcommand runs, so every
// path built below it is built from one answer.
func (r *run) resolveIdsdDir() {
	// Asked before anything else, because the mode decides whether the location is even a question:
	// a tracked .idsd/ IS the durable record and git already puts it on every branch and worktree.
	if r.repoMode() == "committed" {
		r.idsdDir = r.treeIdsdDir()
		return
	}
	if override := r.overrideRoot(); override != "" {
		r.idsdDir = override + "/" + r.repoKey()
		r.overrideNote = "note: idsd scratch location overridden by " + r.overrideConfigPath() + " — using " + r.idsdDir
		return
	}
	r.idsdDir = r.gitCommonPath(sharedDirName)
}

// Said on stderr, and on every command the override affects, because a human acting on a listing has
// to know which directory it came from. stderr specifically: `state` prints exactly one token on
// stdout and callers route on it.
func (r *run) noteOverride() {
	if r.overrideNote != "" {
		r.errLines(r.overrideNote)
	}
}

// The property throwaway mode now rests on: no write lands anywhere `git add -A` can reach, so no
// ignore entry is needed and no report sits inside the tree it fingerprints.
//
// Asserted, never assumed. The default root satisfies it by sitting under the git dir — which is
// lexically INSIDE the checkout, so "outside the root directory" is the wrong test and would refuse
// the default. An override, though, can point anywhere, including into the working tree, and that
// puts back every failure this change removed while every command still reports success.
func (r *run) assertScratchIsUnreachableByGit() {
	if r.idsdDir == r.treeIdsdDir() {
		return
	}
	scratch, root, gitDir := resolveExisting(r.idsdDir), shell.CanonicalDir(r.root), shell.CanonicalDir(r.gitCommonPath(""))
	if root == "" {
		r.refuse("error: could not resolve " + r.root + " to a real path — whether the idsd scratch is inside the working tree is unknown, so nothing was read or written.")
	}
	// Under the git dir is safe: git tracks nothing there and no commit or `git add -A` reaches it.
	if gitDir != "" && (scratch == gitDir || strings.HasPrefix(scratch, gitDir+"/")) {
		return
	}
	if scratch != root && !strings.HasPrefix(scratch, root+"/") {
		return
	}
	r.refuse("error: the idsd scratch location "+r.idsdDir+" is inside this checkout's working tree — nothing was read or written.",
		"  Throwaway scratch must sit where `git add -A` cannot reach it, or every report lands inside the tree it fingerprints",
		"  and no stamp can ever be fresh. Point `root` in "+r.overrideConfigPath()+" outside "+root+", or remove that file to use the default.")
}

// The canonical form of a path that may not exist yet: resolve the deepest ancestor that does, then
// re-append what was below it. Canonicalising only an existing path would answer empty for a scratch
// dir on its first run, and a containment test against empty passes everything.
func resolveExisting(path string) string {
	path = filepath.Clean(path)
	suffix := ""
	for {
		if real := shell.CanonicalDir(path); real != "" {
			return filepath.Clean(real + suffix)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return filepath.Clean(path + suffix)
		}
		suffix = "/" + filepath.Base(path) + suffix
		path = parent
	}
}
