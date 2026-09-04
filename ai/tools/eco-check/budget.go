package ecocheck

import (
	"fmt"
	"io"

	ecoroot "kk-flavor/tools/eco-root"
	"kk-flavor/tools/shell"
)

// The always-loaded budget: the root CLAUDE.md every system prompt carries, inject.md, and every doc
// it lists under "Read always".
func (c *checker) reportBudget(out io.Writer) {
	files, uncounted := c.withImports(c.budgetFiles())

	// Counted per file and summed, never over a concatenation: gluing the files together fuses the
	// last word of one that has no final newline onto the first word of the next, and ecostats sums
	// per file, so the two would report different figures for one tree. Deduplicated first —
	// inject.md's Read-always list can name one file any number of times, and counting it twice
	// inflates the tier it exists to measure. The printed total counts the same set the figures were
	// summed over, or the line says "across 4 files" beside three files' worth of lines and words.
	counted := shell.SortUnique(files)
	budgetLines, budgetWords := 0, 0
	for _, file := range counted {
		lines, words := c.countLinesAndWords(file)
		budgetLines += lines
		budgetWords += words
	}
	writeLinef(out, "always-loaded: %d lines, %d words across %d files%s",
		budgetLines, budgetWords, len(counted), uncountedNote(uncounted))
}

// How a doc inject.md lists under Read always is named when it is refused. One spelling for the three
// sites that refuse one, for the reason reportAbsentBudgetDoc gives: a reader comparing two runs may
// not meet two wordings of one fact.
const readAlwaysTargetPrefix = "inject.md Read-always target "

// Every budget file is contained under the root before it is read — CLAUDE.md and inject.md
// included, not just the docs one of them lists. All three are attacker-authored when this runs as a
// PR review's ecosystem stage (quality-pipeline.md → **The stages**), and the import scan prints
// matched substrings, so a `../../` target reaches a reviewing agent's context.
func (c *checker) budgetFiles() []string {
	var files []string
	claudeMd := shell.Join(c.root.Named(), "CLAUDE.md")
	if c.holdsSomething(claudeMd) {
		if c.root.Contains(claudeMd) {
			files = append(files, claudeMd)
		} else {
			c.refuseBudgetFile(claudeMd)
		}
	}
	inject := shell.Join(c.root.Flavor(), "inject.md")
	if !c.root.Contains(inject) {
		c.refuseBudgetFile(inject)
		return files
	}
	files = append(files, inject)

	// A listed doc that does not exist is skipped by name, not read, or the printed file total
	// disagrees with what was measured. Guarded by the same containment test as the count: refusing
	// to count a file this then reads anyway refuses nothing.
	injectLines, _ := c.readLines(inject)
	for _, doc := range ecoroot.ReadAlwaysTargets(injectLines) {
		listed := shell.Join(c.root.Flavor(), doc)
		switch {
		// Lexically, and before anything here asks the filesystem — tree.go's underRoot holds the
		// reason. A Read-always target is a link the reviewed branch wrote and `..` is in its
		// charset, so the two arms below are an oracle: `[a](../../../../Users/x/.ssh/id_rsa)` came
		// back refused when the reviewing machine held that file and "does not exist" when it did
		// not.
		case !c.underRoot(listed):
			c.refuseBudgetFile(readAlwaysTargetPrefix + doc)
		// Under --gate a gitignored doc is not there as far as a commit is concerned. Ahead of
		// absentOrOutOfReach, whose Lstat would find the file sitting right where the router points and
		// report that nothing could answer for it.
		case c.isSkippedByGate(listed):
			c.reportAbsentBudgetDoc(doc)
		case !shell.PathExists(listed) && !shell.IsSymlink(listed):
			c.absentOrOutOfReach(doc, listed)
		case !c.root.Contains(listed):
			c.refuseBudgetFile(readAlwaysTargetPrefix + doc)
		default:
			files = append(files, listed)
		}
	}
	return files
}

// How this tool words the two, absent and out of reach. Which of them a failed existence test means
// is ecoroot.AbsentOrOutOfReach's, shared with ecostats because both describe one tree and may not
// answer that differently.
//
// Reached only for a path underRoot already accepted, so asking the filesystem here tells the reviewed
// branch nothing it did not already write.
func (c *checker) absentOrOutOfReach(doc, listed string) {
	isAbsent, reason := ecoroot.AbsentOrOutOfReach(listed)
	if isAbsent {
		c.reportAbsentBudgetDoc(doc)
		return
	}
	c.refuseBudgetFile(readAlwaysTargetPrefix + doc + ": " + reason)
}

// One wording for a listed doc this tree does not hold, because two callers reach it and a reader
// comparing a gated run against a bare one may not meet two spellings of one fact.
func (c *checker) reportAbsentBudgetDoc(doc string) {
	c.add("inject.md lists '" + shell.Oneline(doc) + "' under Read always, but " + c.root.Flavor() + "/" +
		shell.Oneline(doc) + " does not exist")
}

// The refused file's **name** is attacker-chosen and is printed, so the name and the number of these
// lines are both bounded. Marked when the bound bites: the name carries the refusal's reason on the
// Read-always path, where it reads "<target>: <what Lstat said>", and a reason cut without a mark
// reads as the whole of one.
func (c *checker) refuseBudgetFile(name string) {
	c.budgetRefusals++
	if c.budgetRefusals <= budgetRefusalCap {
		c.add("budget file refused (symlink, unreadable, or resolves outside " + c.root.Named() +
			") — not read, not counted: " + shell.CutBytesMarked(shell.Oneline(name), 80))
	} else if c.budgetRefusals == budgetRefusalCap+1 {
		c.add("further budget-file refusals suppressed; the count above is not the total")
	}
}

// An `@path` import inside a budget file loads with it, so a budget blind to one under-reports the
// tier it exists to measure. Resolved ones join the budget before it is counted; the rest come back
// to be named in the census note.
func (c *checker) withImports(files []string) (budget, uncounted []string) {
	budget = files
	uncounted = c.root.ResolveImports(ecoroot.ImportScan{
		Files:    files,
		Read:     c.readLines,
		Resolved: func(target string) { budget = append(budget, target) },
		Refused: func(name, reason string) {
			c.add("import refused (" + reason + "), named but not counted: " + shell.CutBytesMarked(shell.Oneline(name), 80))
		},
	})
	return budget, uncounted
}

// Capped in bytes, not just in entries: this line rides the exit-0 path, so an uncapped list prints
// attacker-chosen text under `wiring: clean`. The count stays exact; only the naming is trimmed.
func uncountedNote(uncounted []string) string {
	if len(uncounted) == 0 {
		return ""
	}
	return fmt.Sprintf(" + %d uncounted import(s): %s", len(uncounted), ecoroot.UncountedNames(uncounted))
}
