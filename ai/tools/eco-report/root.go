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
	r.assertOverrideConfigIsTrustworthy(path)
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

// The file that names the root needs the trust the root needs, and for a sharper reason: every guard
// below this one judges the value, and a value an attacker chose passes all of them. A world-writable
// `idsd.conf` — or a world-writable directory holding it — lets any local account name a root of their
// own, and the root they name will be a perfectly ordinary 0700 directory that satisfies every check
// downstream. A security review closed the root's own substitution routes and said so plainly: the
// guard it hardened is only as strong as the file that feeds it.
//
// A symlinked config is refused outright rather than judged by its target. `IsRegularFile` above
// follows the link, so a link to a good file passes there — but whoever can repoint the link chooses
// the root on the next run, which is the same substitution the ancestor walk exists to stop.
func (r *run) assertOverrideConfigIsTrustworthy(path string) {
	if shell.IsSymlink(path) {
		r.refuse("error: "+shell.Oneline(path)+" is a symlink -> "+shell.Oneline(readLink(path))+" — nothing was read or written.",
			"  Whoever can repoint it chooses where this repo's reports are written and what `discard` removes.",
			"  Replace it with a regular file.")
	}
	info, err := os.Lstat(path)
	if err != nil {
		// The same rule the root's own guard states fifty lines down: a permission this tool could not
		// read must not pass as one it checked. Reachable only as a race against the IsRegularFile above,
		// but accepting silently here is exactly the asymmetry that made the two guards disagree about
		// one fact — and this is the guard the other one's strength rests on.
		r.refuse("error: could not read " + shell.Oneline(path) + " (" + err.Error() + ") — whether the file naming the scratch root is safe to trust is unknown, so nothing was read or written.")
	}
	if info.Mode().Perm()&0o022 != 0 {
		r.refuse("error: "+shell.Oneline(path)+" is group- or world-writable (mode "+fmt.Sprintf("%04o", info.Mode().Perm())+") — nothing was read or written.",
			"  Another account could name a scratch root of its own here, and every check below this one would pass it,",
			"  because the root it names can be an ordinary private directory. `chmod go-w` the file, then re-run.")
	}
	// The same substitution, one level out: a writable directory holding the config lets it be replaced
	// wholesale rather than edited. Sticky is exempt for the reason the root's walk states.
	for _, above := range directoriesAbove(path) {
		info, err := os.Stat(above)
		if err != nil {
			continue
		}
		if !isSubstitutableDir(info) {
			continue
		}
		r.refuse("error: "+shell.Oneline(path)+" sits under a group- or world-writable directory ("+shell.Oneline(above)+", mode "+fmt.Sprintf("%04o", info.Mode().Perm())+") — nothing was read or written.",
			"  Whoever can write there can replace the config wholesale and name a scratch root of their own.",
			"  `chmod go-w` the directory named, or move the config somewhere every directory above it is yours alone.")
	}
}

// An override root has to be somewhere only its owner can steer. `discard` calls RemoveAll on what this
// resolves, and every report beneath it carries a pass's security findings — so a directory another
// account can replace or rename is a directory another account can aim that deletion with.
//
// The root AND every directory above it, because a root nobody else can write is still substitutable
// through a parent they can: `mv root root.stolen && mkdir root` needs write on the parent alone, and
// repointing a symlinked ancestor needs write on the directory holding the link. Both were demonstrated
// against the earlier form of this check, which looked at the final component only. Each landed the
// reports in a directory of the attacker's choosing, and every command reported success throughout.
//
// Ancestors are held to a looser rule than the root, which is what keeps ordinary setups working: a
// check nobody can live with gets the config file deleted rather than the root moved. A symlinked
// ancestor is fine — a home directory that is itself a symlink is common, and on macOS `/tmp` is one —
// because the walk follows it and judges what it points at rather than refusing the link. A
// world-writable ancestor is fine when it is sticky; assertNothingAboveTheRootCanSubstituteIt holds
// why. The root itself gets neither allowance: it is the value the human wrote in the config, and the
// one this tool can fairly hold them to.
func (r *run) assertOverrideRootIsTrustworthy(configPath, root string) {
	if shell.IsSymlink(root) {
		r.refuse("error: "+configPath+" sets a root that is a symlink ("+shell.Oneline(root)+" -> "+shell.Oneline(readLink(root))+") — nothing was read or written.",
			"  `discard` removes this directory, so whoever can repoint the link chooses what that removes.",
			"  Point `root` at a real directory.")
	}
	// Before the root's own mode, and reached whether or not the root exists yet: an absent root is
	// about to be CREATED under these directories, so who can write them decides what gets created.
	r.assertNothingAboveTheRootCanSubstituteIt(configPath, root)
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

// The half of the trust check that is about who else can reach the root, rather than about the root.
func (r *run) assertNothingAboveTheRootCanSubstituteIt(configPath, root string) {
	for _, above := range directoriesAbove(root) {
		info, err := os.Stat(above)
		if err != nil {
			// Deliberately not a refusal. The root's own stat, further down, walks into the same wall and
			// says so in the words that name the path the human actually wrote in the config.
			continue
		}
		// Sticky is exactly what stops another account renaming an entry it does not own, which is the
		// whole of the substitution this walk looks for — so a sticky world-writable ancestor is not one.
		// Without the exemption `/tmp` would be refused as an ancestor, on macOS and Linux alike, and a
		// scratch root under it is an ordinary thing to want.
		if !isSubstitutableDir(info) {
			continue
		}
		r.refuse("error: "+configPath+" sets a root under a group- or world-writable directory ("+shell.Oneline(above)+", mode "+fmt.Sprintf("%04o", info.Mode().Perm())+") — nothing was read or written.",
			"  Whoever can write there can rename "+shell.Oneline(root)+" away and leave a directory of their own in its place:",
			"  every report would then be written into theirs, and `discard` would remove theirs. `chmod go-w` the directory named,",
			"  or point `root` somewhere every directory above it is yours alone.")
	}
}

// Whether another account could rename an entry out of this directory and leave one of their own in
// its place — the whole of what both walks above look for. Sticky is exactly what stops that for an
// entry you do not own, so a sticky world-writable directory is not substitutable; without the
// exemption `/tmp` would be refused on macOS and Linux alike, and a scratch root under it is an
// ordinary thing to want.
//
// One predicate rather than the same condition in two walks: they judge different paths for the same
// reason, and a rule stated twice drifts.
func isSubstitutableDir(info fs.FileInfo) bool {
	mode := info.Mode()
	return mode.Perm()&0o022 != 0 && mode&os.ModeSticky == 0
}

// Every directory a substitution would have to be written into, from `/` down to the root's own parent.
// The root itself is never among them — its own checks are the caller's.
//
// Walked a component at a time rather than resolved in one step, because a symlinked ancestor has two
// halves that both matter: the directory holding the link, which is where a repointing gets written, and
// the directories holding the target. Resolving the path first would drop the former, which is the half
// the attack uses.
func directoriesAbove(root string) []string {
	var above []string
	remaining := pathComponents(root)
	current := "/"
	followed := 0
	for len(remaining) > 0 {
		above = append(above, current)
		next := filepath.Join(current, remaining[0])
		remaining = remaining[1:]
		info, err := os.Lstat(next)
		if err != nil {
			// Nothing below this point exists yet, which is ordinary on a first run: MkdirAll builds it,
			// and what decides whether the result can be swapped out is what already stands above it.
			break
		}
		if info.Mode()&os.ModeSymlink == 0 {
			current = next
			continue
		}
		followed++
		if followed > maxSymlinksFollowed {
			break
		}
		target, err := os.Readlink(next)
		if err != nil {
			break
		}
		if filepath.IsAbs(target) {
			current = "/"
		}
		remaining = append(pathComponents(target), remaining...)
	}
	return above
}

// How many links the walk follows before it gives up, which is the only thing here that can run away:
// a loop among ancestors would otherwise spin, while plain components are finite by the length of the
// path. Set at or above every kernel's own ELOOP limit — 32 on macOS, 40 on Linux — so that giving up
// is safe rather than permissive: a chain this walk abandons is one os.Stat cannot resolve either, and
// the root's own "could not read" refusal is what the caller reaches next.
const maxSymlinksFollowed = 40

// A path split into the names between its slashes, with the empties and `.` dropped. A `..` is kept,
// because filepath.Join resolves it against the directory the walk has actually reached — which is not
// the same directory a lexical clean would have picked once a symlink has been followed.
func pathComponents(path string) []string {
	var parts []string
	for _, part := range strings.Split(path, "/") {
		if part != "" && part != "." {
			parts = append(parts, part)
		}
	}
	return parts
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
