// Package ecoroot is the checkout both ecosystem tools measure: the directory holding kk-flavor/ and
// skills/, the paths derived from it, the mount those paths are compared against, and the `@import`
// names that load alongside them.
//
// It exists so ecocheck and ecostats cannot describe different trees for one invocation, which is a
// disagreement neither tool's report can express. A Root is a value: resolving one answers whether
// the directory is a checkout at all, and nothing here holds state between calls or writes to a
// stream.
package ecoroot

import (
	"os"
	"strings"

	"kk-flavor/tools/shell"
)

const (
	flavorDir = "kk-flavor"
	skillsDir = "skills"
)

// ReadLines is how a caller hands this package a file: its lines, or an error that means "skip it".
// It is supplied rather than read here because the two tools read a file under different bounds, and
// which bound applies is the caller's decision — ecocheck reports a file over its bound and skips it,
// ecostats has no such report to make and reads unbounded.
type ReadLines func(path string) ([]string, error)

// ImportScan is one resolution pass: the files whose `@import` lines are scanned, how to read one,
// and what to do with each outcome. Resolved is handed the file an import loads from; Refused is
// handed only the shapes nothing legitimate produces, because it rides a path that has to stay quiet
// on an ordinary miss.
type ImportScan struct {
	Files    []string
	Read     ReadLines
	Resolved func(target string)
	Refused  func(name, reason string)
}

type Root struct {
	// The root exactly as it was named, because every message a tool prints echoes a path built
	// from it: a `./ai` root has to stay `./ai` rather than be cleaned to `ai`.
	named  string
	flavor string
	skills string
	// Symlinks followed, which is what every containment test compares against. Empty when the
	// root cannot be resolved, and every caller reads that emptiness as "not there".
	canon string
	home  string
}

// The same two candidates, tried in the same order for both tools, so a bare invocation of either
// lands on the same tree.
var candidates = []string{".", "./ai"}

// New resolves the root a tool was pointed at. An empty name means the two candidates above, in
// order. The second return is false when no candidate holds both directories,
// which each tool reports in its own words — a check or a measurement that did not run is not a
// clean one, so neither may fold this into an ordinary result.
func New(named string) (Root, bool) {
	if named == "" {
		for _, candidate := range candidates {
			if holdsBoth(candidate) {
				named = candidate
				break
			}
		}
	}
	// Every path in the walk is joined onto `named`, so a root of `ai/` produces
	// `ai//tools/resolve.sh` and a citation of `ai/tools/resolve.sh` then matches nothing — the same
	// finding set answering clean through `ai` and dangling through `ai/`. `/` keeps its slash: it is
	// the whole path, not a separator on the end of one.
	for len(named) > 1 && strings.HasSuffix(named, "/") {
		named = strings.TrimSuffix(named, "/")
	}
	if named == "" || !holdsBoth(named) {
		return Root{}, false
	}
	return Root{
		named:  named,
		flavor: shell.Join(named, flavorDir),
		skills: shell.Join(named, skillsDir),
		canon:  shell.CanonicalDir(named),
		home:   os.Getenv("HOME"),
	}, true
}

// ReadAlwaysTargets is every link the router lists under its `## Read always` heading — the files
// that load with every session, whatever the task.
//
// Neither heading line is in the block: the opening one names the tier rather than listing it, and
// the closing one belongs to the section it opens.
func ReadAlwaysTargets(lines []string) []string {
	var targets []string
	inBlock := false
	for _, line := range lines {
		if strings.HasPrefix(line, "## Read always") {
			inBlock = true
			continue
		}
		if strings.HasPrefix(line, "## ") {
			inBlock = false
		}
		if !inBlock {
			continue
		}
		targets = append(targets, shell.LinkTargets(line)...)
	}
	return targets
}

func holdsBoth(dir string) bool {
	return shell.IsDir(shell.Join(dir, flavorDir)) && shell.IsDir(shell.Join(dir, skillsDir))
}

// Named is the root as the caller spelled it; Flavor and Skills are the two directories that made it
// one. Every path a tool prints is built from these, so they are concatenated rather than cleaned.
func (r Root) Named() string { return r.named }

func (r Root) Flavor() string { return r.flavor }

func (r Root) Skills() string { return r.skills }

// The paths this checkout is mounted at when it is the installed one. A tool compares its own
// directories against these to tell an installed checkout from a clone, and every `~/...` citation
// in the tree resolves through them.
func (r Root) FlavorMount() string { return shell.Join(r.home, ".kk-flavor") }

func (r Root) SkillsMount() string { return shell.Join(r.home, ".claude/skills") }

// IsInstalled reports whether this checkout is the one $HOME mounts — the gate on every figure that
// would otherwise be taken from outside the tree. Anywhere else, in a clone or a PR review's
// worktree, the mounts resolve to the *installed* checkout. A branch someone else wrote then names
// files in the invoking user's real `~/.claude/` and folds their sizes into a number it also
// authored.
//
// Canonicalising follows a symlinked *directory*, so refusing a symlinked `$root/kk-flavor` is what
// stops a branch committing one to the real install and opening that gate.
func (r Root) IsInstalled() bool {
	if r.home == "" || shell.IsSymlink(r.flavor) {
		return false
	}
	flavorCanon := shell.CanonicalDir(r.flavor)
	return flavorCanon != "" && shell.CanonicalDir(r.FlavorMount()) == flavorCanon
}

// Contains reports whether a file may be read as part of this tree. A refusal covers more than
// "outside the root": a symlink, anything that is not a regular file, and a file this process cannot
// open are all turned away wherever they sit, because existence alone is not enough to promise a read.
func (r Root) Contains(file string) bool {
	return containedInRoot(r.canon, file)
}

// HoldsSkillFile reports whether a skill file found at the mount is one of this tree's own, so that
// the skills a tool cannot shrink are counted apart from the ones it can. Strictly under the root:
// the directory compared is the skill's own, which is never the root itself.
func (r Root) HoldsSkillFile(file string) bool {
	return strings.HasPrefix(shell.CanonicalDir(shell.DirName(file)), r.canon+"/")
}
