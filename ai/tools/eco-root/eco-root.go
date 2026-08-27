// Package ecoroot is the checkout both ecosystem tools measure: the directory holding kk-flavor/ and
// skills/, the paths derived from it, and the mount those paths are compared against.
//
// It exists because check.sh and stats.sh resolve that directory the same way on purpose — each
// tool's report describes a tree, and two tools describing different trees for one invocation is a
// disagreement neither of them can report. The shell side keeps its copies honest by hand; the Go
// side had two, so a candidate added to one would have been a silent divergence.
//
// Nothing here holds state between calls or writes to a stream. A Root is a value: resolving one
// answers whether the directory is an ecosystem checkout at all, and every path a caller then builds
// from it is spelled here once rather than at each call site.
package ecoroot

import (
	"os"
	"strings"

	"kk-flavor/tools/shell"
)

// The directories that make a checkout an ecosystem one, and the two places a bare invocation looks
// for them, in the order the shell version tried.
const (
	flavorDir = "kk-flavor"
	skillsDir = "skills"
)

var candidates = []string{".", "./ai"}

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

// New resolves the root a tool was pointed at. An empty name means the two candidates the shell
// version tried, in order. The second return is false when no candidate holds both directories,
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

// Contains reports whether a file may be read as part of this tree — see shell.ContainedInRoot for
// what a refusal covers and why existence alone is not enough.
func (r Root) Contains(file string) bool {
	return shell.ContainedInRoot(r.canon, file)
}

// HoldsSkillFile reports whether a skill file found at the mount is one of this tree's own, so that
// the skills a tool cannot shrink are counted apart from the ones it can. Strictly under the root:
// the directory compared is the skill's own, which is never the root itself.
func (r Root) HoldsSkillFile(file string) bool {
	return strings.HasPrefix(shell.CanonicalDir(shell.DirName(file)), r.canon+"/")
}

// NewImportMount is shell.NewImportMount over this root's own kk-flavor and CLAUDE.md. The two
// arguments are what decide whether imports resolve at all, so they are spelled here rather than at
// each tool: a tool passing a different pair would resolve a different set and report a budget the
// other tool cannot reproduce.
func (r Root) NewImportMount(read shell.ReadLines) shell.ImportMount {
	return shell.NewImportMount(r.home, r.flavor, shell.Join(r.named, "CLAUDE.md"), read)
}
