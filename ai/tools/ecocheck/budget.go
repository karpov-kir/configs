package ecocheck

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var (
	readAlwaysLink = regexp.MustCompilePOSIX(`\]\([^)#]+\)`)
	backtickSpan   = regexp.MustCompilePOSIX("`[^`]*`")
	// Any extension counts, but one is required: that and the non-word character before the `@` are
	// what keep `@param`, a package scope and an email address out of the import list.
	importToken = regexp.MustCompilePOSIX(`[^A-Za-z0-9_]@[~A-Za-z0-9._/-]+\.[A-Za-z0-9]+`)
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
	for _, file := range sortUnique(files) {
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
	claudeMd := join(c.root, "CLAUDE.md")
	if pathExists(claudeMd) || isSymlink(claudeMd) {
		if c.isContainedInRoot(claudeMd) {
			files = append(files, claudeMd)
		} else {
			c.refuseBudgetFile(claudeMd)
		}
	}
	inject := join(c.flavor, "inject.md")
	if !c.isContainedInRoot(inject) {
		c.refuseBudgetFile(inject)
		return files
	}
	files = append(files, inject)

	// A listed doc that does not exist is skipped by name, not read, or the printed file total
	// disagrees with what was measured. Guarded by the same containment test as the count: refusing
	// to count a file this then reads anyway refuses nothing.
	injectLines, _ := readLines(inject)
	for _, doc := range readAlwaysTargets(injectLines) {
		listed := join(c.flavor, doc)
		switch {
		case !pathExists(listed) && !isSymlink(listed):
			c.add("inject.md lists '" + oneline(doc) + "' under Read always, but " + c.flavor + "/" +
				oneline(doc) + " does not exist")
		case !c.isContainedInRoot(listed):
			c.refuseBudgetFile("inject.md Read-always target " + doc)
		default:
			files = append(files, listed)
		}
	}
	return files
}

// The block sed read as `/^## Read always/,/^## /` — from the heading to the next one, the closing
// heading included.
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
		for _, match := range readAlwaysLink.FindAllString(line, -1) {
			targets = append(targets, strings.TrimSuffix(strings.TrimPrefix(match, "]("), ")"))
		}
	}
	return targets
}

// A symlink is refused rather than resolved: canonicalDir canonicalises a *directory*, so it never
// sees the final component, and a link at a budget path would walk through a check that only tested
// its parent.
func (c *checker) isContainedInRoot(path string) bool {
	// A regular file, or nothing. Existence alone admits a FIFO or a device, which a read then
	// blocks on forever. Readable too, not just regular: a mode-000 file passes every type test,
	// and the read behind the figure then fails, leaving a file counted whose words are not.
	if isSymlink(path) || !isRegularFile(path) || !isReadable(path) {
		return false
	}
	dir := canonicalDir(dirName(path))
	if c.rootCanon == "" || dir == "" {
		return false
	}
	return dir == c.rootCanon || strings.HasPrefix(dir, c.rootCanon+"/")
}

// The refused file's **name** is attacker-chosen and is printed, so the name and the number of these
// lines are both bounded.
func (c *checker) refuseBudgetFile(name string) {
	c.budgetRefusals++
	if c.budgetRefusals <= 5 {
		c.add("budget file refused (symlink, unreadable, or resolves outside " + c.root +
			") — not read, not counted: " + cutBytes(oneline(name), 80))
	} else if c.budgetRefusals == 6 {
		c.add("further budget-file refusals suppressed; the count above is not the total")
	}
}

// An `@path` import inside a budget file loads with it, so a budget blind to one under-reports the
// tier it exists to measure. Resolved ones join the budget before it is counted; the rest come back
// to be named in the census note.
func (c *checker) resolveImports(files *[]string) []string {
	imports := importsIn(*files)
	if len(imports) == 0 {
		return nil
	}
	mount := c.newImportMount()
	var uncounted []string
	attempts := 0
	for _, name := range imports {
		if name == "" {
			continue
		}
		target, refusal := "", ""
		// Attempts are capped, and past the cap every remaining name goes to the note rather than
		// dropping out silently. Nothing is carried over from the last name the resolver examined:
		// past the cap it is not called, so its own reset never runs.
		if attempts < 64 {
			attempts++
			target, refusal = mount.resolve(name)
		}
		if target != "" {
			*files = append(*files, target)
			continue
		}
		if refusal != "" {
			c.add("import refused (" + refusal + "), named but not counted: " + cutBytes(oneline(name), 80))
		}
		uncounted = append(uncounted, name)
	}
	return uncounted
}

// An import loads from beside the *installed* copy of the file carrying it, so `@RTK.md` in
// CLAUDE.md is `~/.claude/RTK.md`. That file is **not** one this repo forgot: the rtk installer puts
// it there and verifies it, so moving it into the tree fights the installer.
//
// Only CLAUDE.md's own imports resolve here — an inject.md import loads from `~/.kk-flavor/`, so
// resolving one here would count whatever file shares the name. Resolution also needs this checkout
// to be the installed one, or a branch someone else wrote names files in the invoking user's real
// `~/.claude/` and folds their sizes into a number it also authored. Canonicalising follows a
// symlinked *directory*, so refusing a symlinked `$root/kk-flavor` is what stops a branch committing
// one to the real install and opening that gate.
type importMount struct {
	home        string
	isInstalled bool
	// Depth 1: an import nested inside a resolved file is neither counted nor named.
	declared map[string]bool
}

func (c *checker) newImportMount() importMount {
	flavorInRoot := join(c.root, "kk-flavor")
	isInstalled := c.home != "" && !isSymlink(flavorInRoot) &&
		canonicalDir(flavorInRoot) != "" &&
		canonicalDir(join(c.home, ".kk-flavor")) == canonicalDir(flavorInRoot)

	declared := map[string]bool{}
	claudeMd := join(c.root, "CLAUDE.md")
	if !isSymlink(claudeMd) && isRegularFile(claudeMd) && isReadable(claudeMd) {
		for _, name := range importsIn([]string{claudeMd}) {
			declared[name] = true
		}
	}
	return importMount{home: c.home, isInstalled: isInstalled, declared: declared}
}

// A refusal reason is carried only for the shapes nothing legitimate produces — a traversal, a
// symlink planted at the mount path, a file present and deliberately unreadable. An import simply
// absent from the mount, a checkout that isn't the installed one, and a subdirectory import this
// resolver does not handle are the ordinary cases: they stay quiet names in the note.
func (m importMount) resolve(name string) (target, refusal string) {
	if !m.isInstalled || name == "" {
		return "", ""
	}
	// Bare filenames only — `@../../.ssh/id_rsa` must not resolve. `@dir/file.md` is a legitimate
	// import form, so a plain subdirectory name is refused here too, but quietly: reporting it
	// would take an honest run to exit 1.
	switch {
	case strings.HasPrefix(name, "~"), strings.HasPrefix(name, "/"),
		strings.HasPrefix(name, "../"), strings.Contains(name, "/../"), strings.HasSuffix(name, "/.."):
		return "", "a traversal, not a bare filename"
	case strings.Contains(name, "/"):
		return "", ""
	}
	if !m.declared[name] {
		return "", ""
	}
	mounted := join(join(m.home, ".claude"), name)
	if isSymlink(mounted) {
		return "", "a symlink at the mount"
	}
	if !isRegularFile(mounted) {
		return "", ""
	}
	if !isReadable(mounted) {
		return "", "unreadable at the mount"
	}
	return mounted, ""
}

// Every `@name.ext` outside a fence and outside backticks, across the given files, byte-sorted and
// deduplicated. Two bounds keep it linear in text the tree chose: a field length cap, and a match
// cap within one field.
func importsIn(files []string) []string {
	var found []string
	for _, file := range files {
		lines, err := readLines(file)
		if err != nil {
			continue
		}
		inFence := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			for _, field := range splitFields(backtickSpan.ReplaceAllString(line, " ")) {
				if len(field) > 4096 {
					continue
				}
				// Prefixed with a space so the leading non-word character the pattern requires has
				// something to match at the head of the field.
				token := " " + field
				for hits := 0; hits < 64; hits++ {
					at := importToken.FindStringIndex(token)
					if at == nil {
						break
					}
					found = append(found, token[at[0]+2:at[1]])
					token = token[at[1]:]
				}
			}
		}
	}
	return sortUnique(found)
}

// Capped in bytes, not just in entries: this line rides the exit-0 path, so an uncapped list prints
// attacker-chosen text under `wiring: clean`. The count stays exact; only the naming is trimmed.
func uncountedNote(uncounted []string) string {
	if len(uncounted) == 0 {
		return ""
	}
	shown := uncounted
	if len(shown) > 10 {
		shown = shown[:10]
	}
	var joined strings.Builder
	for _, name := range shown {
		// Sanitised like every other name a finding echoes, even though the import pattern's charset
		// admits no control byte today: this line rides the exit-0 path, so a scan added later that
		// widens that charset must not reopen the injection here.
		joined.WriteString(cutBytes(oneline(name), 60))
		joined.WriteString(" ")
	}
	names := strings.TrimSuffix(cutBytes(joined.String(), 200), " ")
	if len(uncounted) > 10 {
		names += fmt.Sprintf(" … and %d more", len(uncounted)-10)
	}
	return fmt.Sprintf(" + %d uncounted import(s): %s", len(uncounted), names)
}

func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}
