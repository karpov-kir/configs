package ecocheck

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

// The commit gate's half of this check: judge only what a commit can carry.
//
// Without it two checkouts of one commit answer differently, because the walk reads whatever sits on
// disk. A gitignored file — a scratch record, a local note — is in one checkout and not another, and
// the scans have no way to tell it from a committed one.
//
// Ignored, never merely untracked. A blanket untracked-skip would hide a newly authored skill
// directory from the very scan that proves a new skill is mounted (ecosystem.md → **Conventions a new
// file joins**). `git check-ignore` consults the index, so a tracked file matching an ignore pattern
// stays under review and a new, unstaged skill is never filtered out.
const gateFlag = "--gate"

// How many skipped paths the report names before it stops at the count. Bounded for the reason
// uncountedNote is: this line rides the exit-0 path, so an unbounded list prints tree-chosen text
// under `wiring: clean`.
const gateSkipNameCap = 5

// The paths this run treats as absent, resolved once from the whole walk. A nil gateFilter on the
// checker is the flag being off, which is every reach's default.
//
// Two spellings of one set, because they answer different questions: a reach compares a walked path,
// which is built from the root as it was named, and the report prints what git was asked about, which
// is relative to that root and short enough to read beside a budget line.
type gateFilter struct {
	ignored  map[string]bool
	relative map[string]bool
}

// Whether the gate is on and holds this path — the one predicate every reach asks, so there is one
// place to change what the flag means. Two kinds of reach ask it: those that decide a *set* of files,
// and the ones below that answer for a path a scan named for itself.
func (c *checker) isSkippedByGate(path string) bool {
	return c.gate != nil && c.gate.holds(path)
}

// Whether the filter dropped this path, compared in cleaned form and never as the two strings were
// spelled. The walk and the ignored set are both built by concatenation, so those two always agree —
// but a citation is a path *prose* wrote, and `./notes.md` names the same file as `notes.md` while
// comparing unequal to it. Raw, a gitignored file went on answering every citation spelled any way
// but the walk's: the run printed the file on its own skipped list and then resolved a reference
// through it.
//
// Cleaning only the comparison, never what a finding prints: shell.Join concatenates so that a `./ai`
// root stays `./ai` in every echoed path, and that is still what the reader gets.
func (g *gateFilter) holds(path string) bool {
	return g.ignored[filepath.Clean(path)]
}

// The filesystem questions a scan asks about a path it *names* rather than one the walk handed it —
// a skill's own SKILL.md, the root CLAUDE.md. Each is the shell predicate that site already used with
// the gate term in front, so under the flag a gitignored path answers no and the scan sees the tree a
// commit carries rather than the directory it is running in. Without them a gitignored SKILL.md left
// `skill dir without SKILL.md` unreported.
//
// Two functions over six reaches, because everywhere else the walk filter already answers: a scan
// that stats a path and then reads it through filesNamed cannot see a file the walk dropped. Every
// reach that names a path is here.
func (c *checker) holdsRegularFile(path string) bool {
	return !c.isSkippedByGate(path) && shell.IsRegularFile(path)
}

// Exists-or-is-a-symlink, which is how the budget enters a path: a dangling symlink at a budget path
// is a refusal to make, never a file that is not there. ecoroot's containedInRoot holds why.
func (c *checker) holdsSomething(path string) bool {
	return !c.isSkippedByGate(path) && (shell.PathExists(path) || shell.IsSymlink(path))
}

// The tree with every ignored entry dropped, suffix index and all. Rebuilt rather than filtered in
// place, because the index a `*/<ref>` citation resolves through is built as entries are added: left
// alone it would still name a file this run has decided is not here.
//
// Copied from the walk rather than declared fresh, so whatever else the walk recorded about itself
// comes with it. A second `&tree{...}` literal here would be one a later field is added to in the
// walk and not in the filter, and the two trees would then differ only under --gate — a defect no
// bare run can show.
func (g *gateFilter) keepCommittable(walked *tree) *tree {
	kept := *walked
	kept.entries = nil
	kept.suffixes = map[string][]string{}
	for _, entry := range walked.entries {
		if g.holds(entry.path) {
			continue
		}
		kept.add(entry)
	}
	return &kept
}

// The ignored paths named apart from the ones under them: a wholly ignored directory is one line, not
// one per file inside it. Sorted, so two runs over one tree print the same line.
func (g *gateFilter) skippedRoots() []string {
	var roots []string
	for path := range g.relative {
		if !g.relative[shell.DirName(path)] {
			roots = append(roots, path)
		}
	}
	return shell.SortUnique(roots)
}

// Turn the gate on, or say what stopped it. Called before any scan, so a run that could not ask the
// question reports nothing at all rather than a page of findings over an unfiltered tree.
func (c *checker) enableGate() error {
	walked := newWalk(c.root.Named())
	paths := make([]string, 0, len(walked.entries))
	for _, entry := range walked.entries {
		paths = append(paths, entry.path)
	}
	relative, err := newIgnoredPaths(c.git, c.root.Named(), paths)
	if err != nil {
		return err
	}
	ignored := map[string]bool{}
	for path := range relative {
		ignored[filepath.Clean(shell.Join(c.root.Named(), path))] = true
	}
	c.gate = &gateFilter{ignored: ignored, relative: relative}
	return nil
}

// Which of the walked paths git says this checkout ignores, asked once over all of them.
//
// One question and never one per file. On a machine whose endpoint agent inspects every exec a spawn
// costs ~250ms against the 1-3ms an ordinary Unix charges, so a per-file call would price this flag
// out of the gate it exists for — and the tree under review chooses how many files there are. That is
// why the port takes the whole list in one call.
//
// Paths go in relative to the root and come back the same way, so the answer is echoed rather than
// re-derived: git's own spelling of a path it was handed cannot disagree with the walk's.
func newIgnoredPaths(git repo.Git, root string, walked []string) (map[string]bool, error) {
	prefix := root + "/"
	var asked []string
	for _, path := range walked {
		// The root entry itself carries no prefix to cut, and git has no question to answer about
		// the directory it is being run in.
		if rel, ok := strings.CutPrefix(path, prefix); ok {
			asked = append(asked, rel)
		}
	}
	if len(asked) == 0 {
		return nil, nil
	}

	ignored, err := git.Ignored(root, asked)
	if err != nil {
		// The root is bounded because git's reason follows it on the one line refuseToRun prints, and a
		// root long enough to spend that line takes the reason with it —
		// TestTheGateRefusalStillNamesGitsReasonUnderALongRoot is that bound. Nothing follows git's own
		// words, so the bound on them only saves carrying the rest.
		//
		// Neither is escaped here, deliberately: the case proving the printer escapes anything observes
		// that guard only in a tree where this site has none, so putting one back blinds the arm it owns.
		return nil, fmt.Errorf("git check-ignore could not answer for %s, so %s filtered nothing and no scan ran: %s",
			shell.CutBytesMarked(root, 120), gateFlag,
			shell.CutBytesMarked(strings.TrimSpace(err.Error()), 200))
	}
	return ignored, nil
}

// What the filter took out of the walk, above the two budget lines, because it decides what those
// figures and every finding below them are about. Printed only with the flag on: the line's presence
// is what tells a reader this run judged less than the directory holds.
func (c *checker) reportGate(out io.Writer) {
	if c.gate == nil {
		return
	}
	skipped := c.gate.skippedRoots()
	writeLinef(out, "gate: %d gitignored path(s) skipped, so only committable content was checked%s",
		len(skipped), skippedNames(skipped))
}

func skippedNames(skipped []string) string {
	if len(skipped) == 0 {
		return ""
	}
	var named []string
	for _, path := range skipped {
		if len(named) == gateSkipNameCap {
			break
		}
		named = append(named, shell.CutBytesMarked(shell.Oneline(path), 120))
	}
	line := ": " + strings.Join(named, ", ")
	if len(skipped) > gateSkipNameCap {
		line += fmt.Sprintf(", and %d more", len(skipped)-gateSkipNameCap)
	}
	return line
}
