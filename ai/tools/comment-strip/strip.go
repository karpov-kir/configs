// Removes a file's comment blocks before a writer reads the code, so the block it then writes is the
// code's alone. It runs without a model: the judge's Apply with every offered unit gone, plus a
// record of what went.
//
//	usage: comment-strip.sh --facts=<dir> [--changed[=<revisions>]] <path>
//
// The file is rewritten in place. Each removed block is written to `<dir>/<n>.facts` under the site
// it sat on, which the writer opens when it asks whether a note is owed. That site is the first code
// line after the block AS THE STRIPPED FILE NUMBERS IT, the file the writer reads. Stdout lists the
// same sites. Exit 1 removed something, 0 removed none, 2 did not run.
package commentstrip

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	readerjudge "configs/ai/tools/reader-judge"
	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

const factsOption = "--facts="
const archiveOption = "--archive="

// The grammar. It carries the stub's name where argv[0] would carry the binary's. A caller then
// reads a usage line they can retype. A refusal states it: an argument this tool refuses comes from
// a caller who needs the form, and the refusal alone gives them half of it.
const usage = "usage: comment-strip.sh --facts=<dir> [--archive=<dir>] [--changed[=<revisions>]] <path>"

const (
	exitClean     = 0
	exitCut       = 1
	exitDidNotRun = 2
)

// FactsRequested says whether the arguments open with the facts directory this tool requires.
func FactsRequested(args []string) bool {
	return len(args) > 0 && strings.HasPrefix(args[0], factsOption)
}

// directive is a comment the toolchain reads: a lint suppression, a compiler pragma, a build tag, a
// coverage marker or an interpreter line. Its text instructs a program, so a block holding one stays
// whole and stderr names it. Its removal changes what the code does.
var directive = regexp.MustCompile(`^(eslint-|@ts-|prettier-|istanbul |biome-|tslint:|noqa|pylint:|type: |nolint|go:|\+build|#!|/// <reference|@jsx|c8 |v8 |webpack|@vitest-|@jest-|jscpd:)`)

// These are the lines the checks in ai/tools/ read out of a comment. The list of directives holds
// what a language's toolchain reads, and these are the same thing one layer in. The header spellings
// cover the block opening the file, since eco-check's header scans read no further. A region marker
// has a reader wherever it sits, so the strip keeps it there.
var (
	headerInput = regexp.MustCompile(`^(usage:|untested:)`)
	suiteInput  = regexp.MustCompile(`[A-Za-z0-9_.-]+-test\.sh`)
	regionInput = regexp.MustCompile(`^--- (end )?shared:[A-Za-z0-9_-]+ ---$`)
)

// echoable bounds a path this tool echoes back, the way every tool here bounds one. A path comes off
// the command line, and a refusal naming it reaches a terminal.
func echoable(arg string) string {
	return shell.CutBytesMarked(shell.Oneline(arg), 80)
}

func isDirective(raw string) bool {
	return directive.MatchString(commentText(raw))
}

// commentText is a comment line with its marker and the space around it taken off, which is the text
// a reader of that line matches against.
func commentText(raw string) string {
	line := strings.TrimLeft(raw, shell.SpaceBytes)
	for _, marker := range []string{"//", "/*", "*/", "*", "#"} {
		if strings.HasPrefix(line, marker) {
			line = line[len(marker):]
			break
		}
	}
	return strings.TrimRight(strings.TrimLeft(line, shell.SpaceBytes), shell.SpaceBytes)
}

// isToolInput says a check reads this line. `leading` says the line stands in the block opening the
// file, and a header scan reads no further.
func isToolInput(raw string, leading bool) bool {
	text := commentText(raw)
	if regionInput.MatchString(text) {
		return true
	}
	return leading && (headerInput.MatchString(text) || suiteInput.MatchString(text))
}

func holdsDirective(lines []string, u readerjudge.Unit) bool {
	leading := opensTheFile(lines, u)
	for at := u.Line; at < u.Line+u.Span && at <= len(lines); at++ {
		if isDirective(lines[at-1]) || isToolInput(lines[at-1], leading) {
			return true
		}
	}
	return false
}

// opensTheFile says a header scan reads this block. Blank lines and an interpreter line may precede
// it, and code may not. A block further into the file carrying the same words is prose.
func opensTheFile(lines []string, u readerjudge.Unit) bool {
	for at := 1; at < u.Line && at <= len(lines); at++ {
		line := strings.TrimSpace(lines[at-1])
		if line == "" || strings.HasPrefix(line, "#!") {
			continue
		}
		return false
	}
	return true
}

// Strip runs the grammar this file's header states. The facts directory must be empty or absent: a
// file already there reads exactly like one this run wrote, and the writer would take another block's
// facts as this one's.
//
// The repository arrives as a parameter because only --changed asks it anything, and a case driving
// every other path should not have to build one.
func Strip(self string, args []string, cwd string, git repo.Git, stdout, stderr io.Writer) int {
	refuse := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "%s: %s — the strip did NOT run\n", self, fmt.Sprintf(format, a...))
		fmt.Fprintf(stderr, "%s\n", usage)
		return exitDidNotRun
	}
	if !FactsRequested(args) {
		return refuse("%s", "--facts=<dir> must come first")
	}
	dir := strings.TrimPrefix(args[0], factsOption)
	if dir == "" {
		return refuse("%s", "--facts needs a directory")
	}
	args = args[1:]
	archive := ""
	if len(args) > 0 && strings.HasPrefix(args[0], archiveOption) {
		archive = strings.TrimPrefix(args[0], archiveOption)
		if archive == "" {
			return refuse("%s", "--archive needs a directory")
		}
		args = args[1:]
	}
	changed := false
	var revisions []string
	switch {
	case len(args) > 0 && args[0] == "--changed":
		changed = true
		args = args[1:]
	case len(args) > 0 && strings.HasPrefix(args[0], "--changed="):
		changed = true
		revisions = strings.Fields(strings.TrimPrefix(args[0], "--changed="))
		args = args[1:]
	}
	if len(args) != 1 {
		return refuse("%s", "the strip takes one path, and only a source file has comment blocks")
	}
	path := args[0]
	readPath := path
	if !filepath.IsAbs(readPath) {
		readPath = filepath.Join(cwd, readPath)
	}
	info, err := os.Stat(readPath)
	if err != nil {
		return refuse("cannot read %s", echoable(path))
	}
	raw, err := os.ReadFile(readPath)
	if err != nil {
		return refuse("cannot read %s", echoable(path))
	}
	if archive != "" && !filepath.IsAbs(archive) {
		archive = filepath.Join(cwd, archive)
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(cwd, dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return refuse("cannot create %s", echoable(dir))
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) > 0 {
		return refuse("%s is not empty, and a facts file already there reads like one this run wrote", echoable(dir))
	}

	content := string(raw)
	lines := shell.SplitLines(content)
	offer := func(readerjudge.Unit) bool { return true }
	if changed {
		added, err := readerjudge.AddedLines(git, cwd, path, revisions)
		if err != nil {
			return refuse("%v", err)
		}
		offer = readerjudge.NarrowToDiff(offer, added)
	}
	var units []readerjudge.Unit
	for _, u := range readerjudge.CommentBlocks(lines) {
		if !offer(u) {
			continue
		}
		if holdsDirective(lines, u) {
			fmt.Fprintf(stderr, "%s:%d: a comment the toolchain reads, kept\n", path, u.Line)
			continue
		}
		units = append(units, u)
	}
	if len(units) == 0 {
		return exitClean
	}

	// Sites are numbered in the stripped file. The writer reads that file, and a site naming a line of
	// the old one would point it at the wrong declaration by the height of every block above it.
	removedBefore := 0
	type site struct {
		line   int
		facts  string
		record string
	}
	sites := make([]site, 0, len(units))
	for n, u := range units {
		next := u.Line + u.Span
		for next <= len(lines) && strings.TrimSpace(lines[next-1]) == "" {
			next++
		}
		removedBefore += u.Span
		at := next - removedBefore
		if next > len(lines) {
			at = len(lines) - removedBefore
		}
		var record strings.Builder
		for offset := 0; offset < u.Span; offset++ {
			record.WriteString(lines[u.Line-1+offset])
			record.WriteByte('\n')
		}
		sites = append(sites, site{line: at, facts: fmt.Sprintf("%d.facts", n+1), record: record.String()})
	}
	gone := make([]int, len(units))
	for i := range units {
		gone[i] = i + 1
	}
	stripped := readerjudge.Apply(lines, units, gone)
	// A file header sits above a blank line, and removing the header leaves that blank as line 1. The
	// writer then opens a file whose first line is empty. The formatter drops it at the gate, which
	// puts a line the change never wrote into the change set. Only blankness this run created goes,
	// and a file that already opened on a blank line keeps it.
	if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
		short := strings.TrimLeft(stripped, "\n")
		// What goes here sits over every site, so each site moves up by as many lines. The loop that
		// numbered them counted the blocks alone, so a site one line high names the declaration before
		// its own.
		for i := range sites {
			sites[i].line = max(sites[i].line-(len(stripped)-len(short)), 1)
		}
		stripped = short
	}
	if !strings.HasSuffix(content, "\n") {
		stripped = strings.TrimSuffix(stripped, "\n")
	}
	// A facts file carries its site, so it is written once the site is final. A refusal here leaves the
	// source file as the run read it.
	for _, s := range sites {
		record := fmt.Sprintf("%s:%d\n%s", path, s.line, s.record)
		// Every claim ever made at this site, beside the block standing now. A writer drops a fact by
		// its own rules on one run. The strip reads the block as it stands, so the dropped text lives
		// in that run's facts directory alone and a later run never weighs it.
		if archive != "" {
			carried, err := earlierFacts(archive, path, s.record)
			if err != nil {
				return refuse("%s", err.Error())
			}
			record += carried
		}
		if err := os.WriteFile(filepath.Join(dir, s.facts), []byte(record), 0o644); err != nil {
			return refuse("cannot write %s", echoable(filepath.Join(dir, s.facts)))
		}
		if archive != "" {
			// What is kept is the claims alone, with the site line left off: a later run writes its
			// own site, and the line a block sits on moves between runs.
			_, claims, _ := strings.Cut(record, "\n")
			if err := keepForLater(archive, path, s.line, claims); err != nil {
				return refuse("%s", err.Error())
			}
		}
	}
	if err := os.WriteFile(readPath, []byte(stripped), info.Mode().Perm()); err != nil {
		return refuse("cannot write %s", echoable(path))
	}
	// The writer's audit classifies a noun phrase as the code's word by looking it up here, so the
	// list sits beside the facts. Absent, every noun audits as none of the three and the writer
	// rewrites until it declines the site.
	if err := os.WriteFile(filepath.Join(dir, identifiersFile),
		[]byte(strings.Join(identifierWords(lines), "\n")+"\n"), 0o644); err != nil {
		return refuse("cannot write %s", echoable(filepath.Join(dir, identifiersFile)))
	}
	for _, s := range sites {
		fmt.Fprintf(stdout, "%s:%d %s\n", path, s.line, s.facts)
	}
	return exitCut
}

// identifiersFile is what the writer's audit reads to tell the code's own words from English.
const identifiersFile = "identifiers.txt"

var identifierToken = regexp.MustCompile(`[A-Za-z_$][\w$]*`)
var camelHump = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// identifierWords is every word a file's identifiers spell, split at the camel humps, lowercased and
// deduplicated. A reader meets `entryType` in prose as "entry type", so the audit needs the humps
// beside the whole. Comment lines stay out, since a word taken from them would audit as the code's
// own and the comments are what the writer replaces.
func identifierWords(lines []string) []string {
	inComment := map[int]bool{}
	for _, u := range readerjudge.CommentBlocks(lines) {
		for offset := 0; offset < u.Span; offset++ {
			inComment[u.Line+offset] = true
		}
	}
	seen := map[string]bool{}
	for at, line := range lines {
		if inComment[at+1] {
			continue
		}
		for _, token := range identifierToken.FindAllString(line, -1) {
			// Both spellings. The list held the lowercased form alone, so a writer looking a name up as
			// prose spells it missed one the file imports.
			seen[token] = true
			seen[strings.ToLower(token)] = true
			for _, hump := range strings.Fields(camelHump.ReplaceAllString(token, "$1 $2")) {
				seen[hump] = true
				seen[strings.ToLower(hump)] = true
			}
		}
	}
	words := make([]string, 0, len(seen))
	for word := range seen {
		words = append(words, word)
	}
	sort.Strings(words)
	return words
}

// archiveName is where a site's history lives: one file per file and line, under the archive a caller
// keys by the change set's base revision.
func archiveName(path string, line int) string {
	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' {
			return '_'
		}
		return r
	}, path)
	return fmt.Sprintf("%s@%d.facts", safe, line)
}

// earlierFacts is every claim an earlier run recorded at this site, with the block standing now left
// out. A block byte-identical to one already held is dropped. A site stripped twice with one block
// between hands the writer that block once.
func earlierFacts(archive, path, standing string) (string, error) {
	entries, err := os.ReadDir(archive)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("cannot read the archive at %s", echoable(archive))
	}
	head := archiveName(path, 0)
	head = strings.TrimSuffix(head, "@0.facts") + "@"
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), head) {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	seen := map[string]bool{strings.TrimSpace(standing): true}
	var out strings.Builder
	for _, name := range names {
		body, err := os.ReadFile(filepath.Join(archive, name))
		if err != nil {
			return "", fmt.Errorf("cannot read %s", echoable(filepath.Join(archive, name)))
		}
		claims := string(body)
		// An archived record holds one claim block per section. A site stripped three times hands over
		// three claims, each on its own.
		for _, block := range strings.Split(claims, earlierMarker) {
			block = strings.TrimSpace(block)
			if block == "" || seen[block] {
				continue
			}
			seen[block] = true
			fmt.Fprintf(&out, "\n%s\n%s\n", earlierMarker, block)
		}
	}
	return out.String(), nil
}

// earlierMarker tells the writer which claims came from a run before this one. Question 3 weighs
// them the way it weighs a standing claim, and a reader of the facts file can see which of them the
// block standing now leaves out.
const earlierMarker = "# claimed at this site by an earlier run:"

// keepForLater records this run's facts for the runs after it.
func keepForLater(archive, path string, line int, record string) error {
	if err := os.MkdirAll(archive, 0o755); err != nil {
		return fmt.Errorf("cannot create the archive at %s", echoable(archive))
	}
	name := filepath.Join(archive, archiveName(path, line))
	if err := os.WriteFile(name, []byte(record), 0o644); err != nil {
		return fmt.Errorf("cannot write %s", echoable(name))
	}
	return nil
}
