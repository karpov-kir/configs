// Removes a file's comment blocks before a writer reads the code, so the block it then writes owes
// the old one nothing. It calls no model: this is the judge's Apply with every offered unit gone,
// plus a record of what went, one file per block, which the writer opens only when it asks whether a
// note is owed.
//
//	usage: comment-strip.sh --facts=<dir> [--changed[=<revisions>]] <path>
//
// The file is rewritten in place. Each removed block goes to `<dir>/<n>.facts`, headed by the site the
// block sat on, `<path>:<line>`, where the line is the first code line after the block AS THE STRIPPED
// FILE NUMBERS IT, which is the file the writer reads. Stdout lists the same sites with their facts
// file, one per line. Exit 1 when something was removed, 0 when nothing was, 2 when the strip did not
// run. A comment the toolchain reads (`eslint-disable`, `@ts-expect-error`, `go:generate`, ...) is
// never removed, since removing it changes what the code does, and is named on stderr instead.
package commentstrip

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	readerjudge "kk-flavor/tools/reader-judge"
	"kk-flavor/tools/shell"
)

const factsOption = "--facts="

const (
	exitClean     = 0
	exitCut       = 1
	exitDidNotRun = 2
)

// FactsRequested says whether the arguments ask for the strip rather than a judgement.
func FactsRequested(args []string) bool {
	return len(args) > 0 && strings.HasPrefix(args[0], factsOption)
}

// directive is a comment the toolchain reads: a lint suppression, a compiler pragma, a build tag, a
// coverage marker or an interpreter line. Its text is an instruction to a program, and a block holding
// one stays whole.
var directive = regexp.MustCompile(`^(eslint-|@ts-|prettier-|istanbul |biome-|tslint:|noqa|pylint:|type: |nolint|go:|\+build|#!|/// <reference|@jsx|c8 |v8 |webpack|@vitest-|@jest-|jscpd:)`)

// echoable bounds a path this tool echoes back, the way every tool here bounds one. A path comes off
// the command line, and a refusal naming it reaches a terminal.
func echoable(arg string) string {
	return shell.CutBytesMarked(shell.Oneline(arg), 80)
}

func isDirective(raw string) bool {
	line := strings.TrimLeft(raw, shell.SpaceBytes)
	for _, marker := range []string{"//", "/*", "*/", "*", "#"} {
		if strings.HasPrefix(line, marker) {
			line = line[len(marker):]
			break
		}
	}
	return directive.MatchString(strings.TrimLeft(line, shell.SpaceBytes))
}

func holdsDirective(lines []string, u readerjudge.Unit) bool {
	for at := u.Line; at < u.Line+u.Span && at <= len(lines); at++ {
		if isDirective(lines[at-1]) {
			return true
		}
	}
	return false
}

// Strip runs the grammar above. The facts directory must be empty or absent: a file already there
// reads exactly like one this run wrote, and the writer would take another block's facts as this one's.
func Strip(self string, args []string, cwd string, stdout, stderr io.Writer) int {
	refuse := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "%s: %s — the strip did NOT run\n", self, fmt.Sprintf(format, a...))
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
		added, err := readerjudge.AddedLines(cwd, path, revisions)
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

	// Sites are numbered in the stripped file: the writer reads that file, and a site naming a line of
	// the old one would point it at the wrong declaration by the height of every block above it.
	removedBefore := 0
	type site struct {
		line  int
		facts string
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
		facts := fmt.Sprintf("%d.facts", n+1)
		var record strings.Builder
		fmt.Fprintf(&record, "%s:%d\n", path, at)
		for offset := 0; offset < u.Span; offset++ {
			record.WriteString(lines[u.Line-1+offset])
			record.WriteByte('\n')
		}
		if err := os.WriteFile(filepath.Join(dir, facts), []byte(record.String()), 0o644); err != nil {
			return refuse("cannot write %s", echoable(filepath.Join(dir, facts)))
		}
		sites = append(sites, site{line: at, facts: facts})
	}
	gone := make([]int, len(units))
	for i := range units {
		gone[i] = i + 1
	}
	stripped := readerjudge.Apply(lines, units, gone)
	if !strings.HasSuffix(content, "\n") {
		stripped = strings.TrimSuffix(stripped, "\n")
	}
	if err := os.WriteFile(readPath, []byte(stripped), info.Mode().Perm()); err != nil {
		return refuse("cannot write %s", echoable(path))
	}
	for _, s := range sites {
		fmt.Fprintf(stdout, "%s:%d %s\n", path, s.line, s.facts)
	}
	return exitCut
}
