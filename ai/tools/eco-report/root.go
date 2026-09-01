package ecoreport

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
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
// is how the nine skills that used to hardcode `.idsd/` find it.

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
func (r *run) overrideConfigPath() string {
	config := r.configHome
	if config == "" {
		config = r.home + "/.config"
	}
	return config + "/kk-flavor/idsd.conf"
}

// The override's root, or empty when there is no override file.
//
// Strict by design: a file that is present but unusable refuses, and no path here falls back to the
// default. A silent fallback would write this clone's intents into a directory the human was never
// told about, which is the one failure an override must not have — the same rule score.sh applies to
// its own. Absent is different from broken, and only absent is quiet.
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
	return filepath.Clean(root)
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

// Throwaway mode holds nothing in the tree, so an in-tree `.idsd/` here is either a directory left by
// the older layout or one a human made by hand. Its content is the only copy of whatever it holds, so
// this refuses and says what to do rather than moving or merging anything.
//
// An empty directory is removed instead: it holds nothing to lose, and left standing it is a trace in
// the mode whose whole contract is leaving none.
func (r *run) reconcileTreeIdsdDir() {
	tree := r.treeIdsdDir()
	if r.idsdDir == tree || !shell.PathExists(tree) {
		return
	}
	if shell.IsSymlink(tree) {
		r.refuse("error: "+tree+" is a symlink -> "+readLink(tree)+" — nothing was read or written.",
			"  The scratch directory now lives at "+r.idsdDir+"; remove the link, then re-run.")
	}
	// Files, not directory entries. Moving the content out leaves `intents/` and `qualify-reports/`
	// standing as empty directories, and counting entries reads those as content — so a repo whose
	// migration is finished is refused, with a message saying it holds content when it holds none.
	count, sample, err := filesUnder(tree)
	if err != nil {
		r.refuse("error: could not read "+tree+" ("+err.Error()+") — whether it holds anything is unknown, so nothing was read or written.",
			"  The scratch directory now lives at "+r.idsdDir+".")
	}
	if count == 0 {
		// RemoveAll, because what is left is the empty directory skeleton the move left behind rather
		// than a single empty dir. It holds nothing to lose: the walk above found no file under it.
		_ = os.RemoveAll(tree)
		return
	}
	r.refuse("error: "+tree+" still holds "+strconv.Itoa(count)+" file(s), and throwaway idsd scratch no longer lives in the tree — nothing was read or written.",
		"  The scratch directory is now "+r.idsdDir+", shared by every branch and worktree of this clone.",
		"  Still there: "+strings.Join(sample, " ")+sampleTail(count, len(sample)),
		"  Nothing here moves your files for you: copy what you still want into that directory, delete the rest of "+tree+", then re-run.")
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

// The line the old in-tree layout wrote into `.git/info/exclude` to hide the scratch from
// `git add -A`. Nothing writes it any more — the scratch is not in the tree to hide — so every repo
// that ever ran a throwaway ship still carries a rule for a directory that is not there.
const staleExclusionEntry = ".idsd/"

// Remove that rule. Reached from check-ignore, which runs before anything else in every pass, so a
// repo cleans itself up the first time a ship touches it and no migration script has to remember.
//
// Safe unconditionally by the time this runs: reconcileTreeIdsdDir has already refused if the tree
// still holds scratch, so the entry can only be excluding a directory that no longer exists. It is
// also shared across every worktree, which used to make dropping it a decision requiring a worktree
// count — now there is nothing any worktree needs it for.
//
// A failure here is reported and does not stop the pass: nothing about correctness rests on the entry
// any more, so refusing would block real work over a tidy-up. Silence is the thing to avoid, not the
// failure itself.
func (r *run) removeStaleExclusion() {
	exclude := r.gitCommonPath("info/exclude")
	if !shell.IsRegularFile(exclude) {
		return
	}
	content, err := os.ReadFile(exclude)
	if err != nil {
		r.errLines("note: could not read " + exclude + " — a stale '" + staleExclusionEntry + "' rule may still be there; nothing was changed")
		return
	}
	var kept []string
	removed := 0
	for _, line := range shell.SplitLines(string(content)) {
		if line == staleExclusionEntry {
			removed++
			continue
		}
		kept = append(kept, line)
	}
	if removed == 0 {
		return
	}
	// Renamed over rather than written in place, for the reason the old teardown did it: an unwritable
	// .git/info must fail without truncating the file it could not replace — which holds the human's
	// own rules.
	temp, err := os.CreateTemp(shell.DirName(exclude), ".exclude.")
	if err != nil {
		r.errLines("note: could not rewrite " + exclude + " — the stale '" + staleExclusionEntry + "' rule is still there")
		return
	}
	_, err = temp.Write(joinRecords(kept))
	if closed := temp.Close(); err == nil {
		err = closed
	}
	if err == nil {
		err = moveFile(temp.Name(), exclude)
	}
	if err != nil {
		_ = os.Remove(temp.Name())
		r.errLines("note: could not replace " + exclude + " (" + err.Error() + ") — the stale '" + staleExclusionEntry + "' rule is still there")
		return
	}
	r.line("cleaned: removed the stale '%s' rule from %s (%d line(s)) — nothing writes there any more",
		staleExclusionEntry, exclude, removed)
}

// How many regular files a directory holds, and up to sampleBound of their paths relative to it. The
// count and the sample are separate returns because the sample is bounded: reporting len(sample) as the
// count would understate a directory of a hundred files as twenty, and a message whose number drifts
// from what it is counting is a message that lies.
//
// A symlink is not a regular file and is not followed: the point is what would be lost here, and a
// link's target lives somewhere else.
func filesUnder(dir string) (count int, sample []string, err error) {
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		count++
		if len(sample) < sampleBound {
			sample = append(sample, strings.TrimPrefix(path, dir+"/"))
		}
		return nil
	})
	return count, sample, err
}

// How many paths a refusal names before it stops.
const sampleBound = 20

// What the refusal adds when the sample is short of the count, so the two never read as disagreeing.
func sampleTail(count, shown int) string {
	if count <= shown {
		return ""
	}
	return " (and " + strconv.Itoa(count-shown) + " more)"
}
