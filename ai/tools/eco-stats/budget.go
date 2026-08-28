package ecostats

import (
	"fmt"
	"io"
	"os"
	"strings"

	ecoroot "kk-flavor/tools/eco-root"
	"kk-flavor/tools/shell"
)

// The always-loaded tier, in two parts: the router's own "Read always" targets, and every skill
// description the harness keeps in context. ecocheck counts the same tier and the two must agree, so
// what decides membership is shared with it rather than written twice.
func (s *stats) budgetFiles(errOut io.Writer) []string {
	var files []string
	claudeMd := shell.Join(s.root.Named(), "CLAUDE.md")
	if shell.PathExists(claudeMd) || shell.IsSymlink(claudeMd) {
		if s.root.Contains(claudeMd) {
			s.alwaysLoadedWords += wordsInFile(claudeMd)
			files = append(files, claudeMd)
		} else {
			s.refuseBudgetFile(errOut, claudeMd)
		}
	}

	inject := shell.Join(s.root.Flavor(), "inject.md")
	switch {
	// Entered on exists-or-is-a-symlink, not on `[ -f ]`: a symlink to a FIFO is neither, and would
	// fall through both branches, dropping inject.md and every target it lists from the budget in
	// silence.
	case (shell.PathExists(inject) || shell.IsSymlink(inject)) && !s.root.Contains(inject):
		s.refuseBudgetFile(errOut, inject)
	case shell.IsRegularFile(inject):
		lines, _ := readLines(inject)
		for _, target := range readAlwaysTargets(lines) {
			if target == "" {
				continue
			}
			file := shell.Join(s.root.Flavor(), target)
			switch {
			case !shell.PathExists(file) && !shell.IsSymlink(file):
				// Sanitised like every other name from the tree: the Read-always list is
				// attacker-authored when this runs over a branch someone else wrote, and an ESC byte
				// in a link target erases whatever this message was printed beside. ecocheck puts the
				// same name through Oneline where it reports the same missing target.
				fmt.Fprintf(errOut, "stats.sh: inject.md lists '%s' under Read always, but %s does not exist\n",
					shell.Oneline(target), shell.Oneline(file))
			case !s.root.Contains(file):
				s.refuseBudgetFile(errOut, "inject.md Read-always target "+target)
			default:
				s.alwaysLoadedWords += wordsInFile(file)
				files = append(files, file)
			}
		}
		s.alwaysLoadedWords += wordsInFile(inject)
		files = append(files, inject)
	}
	return files
}

// The block awk selected with `/^## Read always/{f=1;next} /^## /{f=0} f` — between the headings,
// neither one included. ecocheck selects the same block with a sed range, which takes both boundary
// lines, so this is the one part of the budget scan the two tools do not share: a link on either
// heading line is inside ecocheck's block and outside this one.
func readAlwaysTargets(lines []string) []string {
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

func (s *stats) refuseBudgetFile(errOut io.Writer, name string) {
	s.budgetRefusals++
	if s.budgetRefusals <= 5 {
		fmt.Fprintf(errOut, "stats.sh: budget file refused (symlink, unreadable, or resolves outside %s) — not read, not counted: %s\n",
			s.root.Named(), shell.CutBytes(shell.Oneline(name), 80))
	} else if s.budgetRefusals == 6 {
		fmt.Fprintln(errOut, "stats.sh: further budget-file refusals suppressed; the count in the exit message is the total")
	}
}

// An `@path` import inside a budget file loads with it, so one resolved at the installed mount is
// counted into this tier. The rest stay named in the note, and how much of the figure came from the
// resolved ones is carried into the row: without it a reader comparing two rows cannot tell a tier
// that grew from one this script merely started seeing.
//
// An import refusal is not a budget refusal: a probe-shaped name was never a member of the tier, so
// the figure is not short and no row is withheld.
func (s *stats) resolveImports(budget []string, errOut io.Writer) {
	s.uncounted = s.root.ResolveImports(ecoroot.ImportScan{
		Files: budget,
		Read:  readLines,
		Resolved: func(target string) {
			words := wordsInFile(target)
			s.alwaysLoadedWords += words
			s.importResolvedWords += words
		},
		Refused: func(name, reason string) {
			fmt.Fprintf(errOut, "stats.sh: import refused (%s), named but not counted: %s\n",
				reason, shell.CutBytes(shell.Oneline(name), 80))
		},
	})
}

// Every skill description the router keeps in context, and how many of the tree's skills are routed
// at all. Read exactly as ecocheck reads it, because that tool reports this same budget.
func (s *stats) census() {
	for _, file := range skillFiles(s.root.Skills()) {
		// A SKILL.md that cannot be read still counts as routed, with no description words: the awk
		// behind the shell version failed to open it and its `&&` fell through to exactly that. Read as
		// a skip instead, the "R of T skills" figure would quietly shrink on a file the tree can see.
		lines, _ := readLines(file)
		if shell.IsOptedOutOfModelInvocation(lines) {
			continue
		}
		s.routedSkills++
		s.descriptionWords += len(shell.SplitFields(shell.FrontmatterDescription(lines)))
	}
}

// Skills mounted at `~/.claude/skills` from outside this tree cost the same tier and no pass here can
// shrink them, so they are counted apart.
//
// Only when this tree is the installed one: anywhere else — a clone, or a PR review's worktree — the
// mounts resolve to the *installed* checkout, the exclusion below matches nothing, and the figure
// publishes the reviewer's own local skill inventory into something an agent may quote.
func (s *stats) mountedOutside() {
	if !s.root.IsInstalled() {
		return
	}
	for _, file := range skillFiles(s.root.SkillsMount()) {
		if s.root.HoldsSkillFile(file) {
			continue
		}
		lines, _ := readLines(file)
		if shell.IsOptedOutOfModelInvocation(lines) {
			continue
		}
		s.outsideSkills++
		s.outsideWords += len(shell.SplitFields(shell.FrontmatterDescription(lines)))
	}
}

// The `<dir>/*/SKILL.md` glob, byte-sorted the way bash sorted it under LC_ALL=C, dotfiles excluded
// the way a glob excludes them. A glob, not a `find`: it resolves *through* a symlinked skill
// directory, which is what lets a mount outside the tree be read at all.
func skillFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		file := shell.Join(shell.Join(dir, entry.Name()), "SKILL.md")
		if shell.IsRegularFile(file) {
			files = append(files, file)
		}
	}
	return files
}
