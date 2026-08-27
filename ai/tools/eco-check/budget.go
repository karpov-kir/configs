package ecocheck

import (
	"fmt"
	"io"
	"strings"

	"kk-flavor/tools/shell"
)

// The always-loaded budget: the root CLAUDE.md every system prompt carries, inject.md, and every doc
// it lists under "Read always".
func (c *checker) reportBudget(out io.Writer) {
	files := c.budgetFiles()
	uncounted := c.resolveImports(&files)

	// Counted per file and summed, never over a concatenation: gluing the files together fuses the
	// last word of one that has no final newline onto the first word of the next, and stats.sh sums
	// per file, so the two would report different figures for one tree. Deduplicated first —
	// inject.md's Read-always list can name one file any number of times, and counting it twice
	// inflates the tier it exists to measure.
	budgetLines, budgetWords := 0, 0
	for _, file := range shell.SortUnique(files) {
		lines, words := countLinesAndWords(file)
		budgetLines += lines
		budgetWords += words
	}
	writeLinef(out, "always-loaded: %d lines, %d words across %d files%s",
		budgetLines, budgetWords, len(files), uncountedNote(uncounted))
}

// Every budget file is contained under the root before it is read — CLAUDE.md and inject.md
// included, not just the docs one of them lists. All three are attacker-authored when this runs as a
// PR review's ecosystem stage (quality-pipeline.md → **The stages**), and the import scan prints
// matched substrings, so a `../../` target reaches a reviewing agent's context.
func (c *checker) budgetFiles() []string {
	var files []string
	claudeMd := shell.Join(c.root, "CLAUDE.md")
	if shell.PathExists(claudeMd) || shell.IsSymlink(claudeMd) {
		if c.isContainedInRoot(claudeMd) {
			files = append(files, claudeMd)
		} else {
			c.refuseBudgetFile(claudeMd)
		}
	}
	inject := shell.Join(c.flavor, "inject.md")
	if !c.isContainedInRoot(inject) {
		c.refuseBudgetFile(inject)
		return files
	}
	files = append(files, inject)

	// A listed doc that does not exist is skipped by name, not read, or the printed file total
	// disagrees with what was measured. Guarded by the same containment test as the count: refusing
	// to count a file this then reads anyway refuses nothing.
	injectLines, _ := c.readLines(inject)
	for _, doc := range readAlwaysTargets(injectLines) {
		listed := shell.Join(c.flavor, doc)
		switch {
		case !shell.PathExists(listed) && !shell.IsSymlink(listed):
			c.add("inject.md lists '" + shell.Oneline(doc) + "' under Read always, but " + c.flavor + "/" +
				shell.Oneline(doc) + " does not exist")
		case !c.isContainedInRoot(listed):
			c.refuseBudgetFile("inject.md Read-always target " + doc)
		default:
			files = append(files, listed)
		}
	}
	return files
}

// The block sed read as `/^## Read always/,/^## /` — from the heading to the next one, the closing
// heading included. stats.sh selects the block with an awk flag instead and takes neither boundary
// line, so this is the one part of the budget scan the two do not share.
func readAlwaysTargets(lines []string) []string {
	var targets []string
	inBlock := false
	for _, line := range lines {
		if !inBlock {
			if !strings.HasPrefix(line, "## Read always") {
				continue
			}
			inBlock = true
		} else if strings.HasPrefix(line, "## ") {
			inBlock = false
		}
		targets = append(targets, shell.LinkTargets(line)...)
	}
	return targets
}

func (c *checker) isContainedInRoot(path string) bool {
	return shell.ContainedInRoot(c.rootCanon, path)
}

// The refused file's **name** is attacker-chosen and is printed, so the name and the number of these
// lines are both bounded.
func (c *checker) refuseBudgetFile(name string) {
	c.budgetRefusals++
	if c.budgetRefusals <= 5 {
		c.add("budget file refused (symlink, unreadable, or resolves outside " + c.root +
			") — not read, not counted: " + shell.CutBytes(shell.Oneline(name), 80))
	} else if c.budgetRefusals == 6 {
		c.add("further budget-file refusals suppressed; the count above is not the total")
	}
}

// An `@path` import inside a budget file loads with it, so a budget blind to one under-reports the
// tier it exists to measure. Resolved ones join the budget before it is counted; the rest come back
// to be named in the census note.
func (c *checker) resolveImports(files *[]string) []string {
	imports := shell.ImportsIn(c.readLines, *files)
	if len(imports) == 0 {
		return nil
	}
	mount := shell.NewImportMount(c.home, shell.Join(c.root, "kk-flavor"), shell.Join(c.root, "CLAUDE.md"), c.readLines)
	return shell.ResolveImports(imports, mount,
		func(target string) { *files = append(*files, target) },
		func(name, reason string) {
			c.add("import refused (" + reason + "), named but not counted: " + shell.CutBytes(shell.Oneline(name), 80))
		})
}

// Capped in bytes, not just in entries: this line rides the exit-0 path, so an uncapped list prints
// attacker-chosen text under `wiring: clean`. The count stays exact; only the naming is trimmed.
func uncountedNote(uncounted []string) string {
	if len(uncounted) == 0 {
		return ""
	}
	return fmt.Sprintf(" + %d uncounted import(s): %s", len(uncounted), shell.UncountedNames(uncounted))
}
