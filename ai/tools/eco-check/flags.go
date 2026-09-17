package ecocheck

// Closes the prose half of a chain whose other half is already held shut: `ai/tools/stub_usage_test.go`
// holds a stub's documented usage line against the one its binary prints, so a flag the stub names is
// a flag the tool takes. Nothing held the step before it. An instruction file could tell every session
// to run `repo-key.sh --abbrev`, the script could document no such flag, and the check that reads this
// tree said "wiring: clean". With both halves the chain runs prose → stub → binary.
//
// subcommands.go scans call sites too and a flag is invisible to it twice over: it reads only a braced
// `usage: <base> {…}` grammar, and its name charset has no room for the leading dashes. Reshaping a
// flag into a subcommand so that scan bites was considered and rejected — `ai/tools/repo-key/repokey.go`
// keeps flags distinguishable from paths on purpose, because a directory may legitimately be named
// `-rf` and a bare subcommand cannot be told from a path argument.
//
// What this scan reads is the flag's *name*, never its value: `--agent=claude` and `--agent codex`
// name one flag, and the usage line writes the same flag a third way again.
//
// Every finding here leads with its kind and puts the located `file:line` after it, the shape the
// citation scan beside it already prints. The kind stays first for report.go's reason — a class is
// decided on the head of the line, and no table can name a head the reviewed branch wrote — and the
// location goes next because the reader's first question is which of a hundred instruction files
// carries the call, and a finding naming only the script leaves them to grep for it.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"kk-flavor/tools/shell"
)

const (
	flagUsageDoesNotName    = "flag its usage does not name: "
	flagCallSitesNotChecked = "flag call sites not checked: "
	flagScanAtItsBound      = "flag call-site scan is at its"
)

// The most call sites this scan carries. Each one costs a map entry and, for a script not yet read, one
// bounded read of that script's head, and nothing past the bound costs either. Past it the rest are
// reported and NOT checked — an unchecked call site must never read as one that was checked and passed.
const flagCallSiteCap = 256

var (
	// A token naming a script: a `*.sh` basename, with the path an instruction file usually writes in
	// front of it. `~` is in that charset because `~/.kk-flavor/scripts/model-policy.sh` is how this
	// tree names an installed script, and `$` because a fenced example writes `"$HOME/…"`.
	//
	// Only the basename is taken, and no path here is ever opened: a call site is prose, so it names
	// the script the way an agent types it. That also keeps this scan out of the probe tree.go's
	// underRoot exists to close — there is no path question for a committed `../../../etc/passwd` to
	// ask, because this never asks the filesystem about one.
	scriptInACommand = regexp.MustCompilePOSIX(`^([A-Za-z0-9._~$/-]*/)?([A-Za-z0-9][A-Za-z0-9._-]*\.sh)$`)
	// `--abbrev`, with the `=value` half dropped. Unanchored at the tail so `--agent=claude|codex`
	// answers `agent`; POSIX leftmost-longest is what makes that the whole name and not one letter.
	flagInACommand = regexp.MustCompilePOSIX(`^--([a-z0-9][a-z0-9-]*)`)
	// The same names, anywhere on a usage line: `[--gate]`, `--agent=claude|codex`, `--changed[=<rev>]`.
	flagInAUsage = regexp.MustCompilePOSIX(`--([a-z0-9][a-z0-9-]*)`)
)

// The punctuation that is the markdown around a command rather than part of it: emphasis, the brackets
// a usage grammar writes an optional argument in, and the sentence a code span sits inside. Trimmed
// from both ends of every token, so a bracketed `[--gate]` and one left holding the comma and the
// closing backtick of the span it ended both name `--gate`.
const commandTokenMarks = "*_,.;:!?()[]{}\"'`"

// One `<script> --<flag>` an instruction file names, and the file and line it was read from.
type flagCallSite struct {
	script, flag string
	file         string
	line         int
}

// What makes two call sites the same defect: the pair, never the place it was read from. A flag a
// script's usage line does not name is one defect however many files pass it, and one edit to that
// usage line closes every one of them — so the same pair read in twenty files is one finding, not
// twenty copies of one sentence crowding its rank out of report.go's per-class room. It also keeps
// flagCallSiteCap counting what its own comment says it counts, rather than silently becoming a bound
// on call sites the day a location was added to them.
//
// The location a finding carries is therefore the first one the walk reached, and that is stable
// rather than arbitrary: the walk is ReadDir-ordered, so a rerun over an unchanged tree names the
// same line again.
type flagPair struct{ script, flag string }

func (s flagCallSite) pair() flagPair { return flagPair{script: s.script, flag: s.flag} }

// A command the tree writes, and the 1-based line it sits on.
type commandSpan struct {
	text string
	line int
}

// The flags one script's usage block names, and whether it has one at all. The two answers are separate
// because they fail separately: a script with no usage line has every flag its call sites pass checked
// against nothing, and saying "its usage does not name it" of such a script names the wrong defect.
type documentedFlags struct {
	// readLines refused the file and named it doing so, so this scan adds nothing and checks nothing.
	unread bool
	stated bool
	flags  map[string]bool
}

func (c *checker) scanFlagCallSites() {
	c.indexScriptOwners()
	sites, capped := c.flagCallSites()
	usages := map[string]documentedFlags{}
	unstated := map[string]bool{}
	for _, site := range sites {
		paths := c.scriptOwners[site.script]
		// No script under that basename, or more than one. Neither is this scan's finding to make.
		// With no script there is no usage line to check the flag against, and a bare basename is how
		// prose names a command — in an installed project whose root is the whole repository, the
		// project's own prose names scripts this tree does not hold, and every one of them would fire
		// here. With two, subcommands.go's reportWeldedScriptNames already names both files, and a
		// flag finding attributed to a basename could not say which of them it was about.
		if len(paths) != 1 {
			continue
		}
		usage, read := usages[paths[0]]
		if !read {
			usage = c.documentedFlagsOf(paths[0])
			usages[paths[0]] = usage
		}
		// Both paths in a finding are the reviewed tree's own text, and each takes only the guard its
		// own source needs. The located file and the script path are sanitised; the flag name is cut
		// instead, because flagInACommand's charset already excludes every byte Oneline would replace
		// but bounds no length, and an instruction file can write a flag a megabyte long.
		at := shell.Oneline(site.file) + ":" + strconv.Itoa(site.line)
		switch {
		case usage.unread:
		case !usage.stated:
			// Once per script, never once per flag it could not check. The defect is the missing usage
			// line, so the same sentence repeated for each flag passed to that script says nothing new
			// and spends room its rank has to hold other findings in.
			if !unstated[paths[0]] {
				unstated[paths[0]] = true
				c.add(flagCallSitesNotChecked + at + " — " + shell.Oneline(paths[0]) +
					" states no lowercase 'usage:' line, so every flag a call site passes it is checked against nothing")
			}
		case !usage.flags[site.flag]:
			c.add(flagUsageDoesNotName + at + " — " + shell.CutBytesMarked("--"+site.flag, findingNameCap) +
				" is passed to " + shell.Oneline(paths[0]) + ", whose usage line does not name it")
		}
	}
	if capped > 0 {
		c.add(fmt.Sprintf(flagScanAtItsBound+" %d-call-site bound: %d more were NOT checked",
			flagCallSiteCap, capped))
	}
}

// Every `<script> --<flag>` the instruction files name, deduplicated, and how many the bound withheld.
//
// Markdown only. The requirement is about what an instruction file tells a session to run; a `.sh`
// naming its own flags is its own argument parser, and reading those would report every script in the
// tree against itself. A workflow that runs a script with a flag is left to CI, which fails on its own
// and loudly.
func (c *checker) flagCallSites() (sites []flagCallSite, capped int) {
	seen := map[flagPair]bool{}
	for file, lines := range c.filesWithLines(c.root.Named(), "*.md") {
		for _, span := range commandSpans(lines) {
			for _, site := range flagsNamedInSpan(span, file) {
				if seen[site.pair()] {
					continue
				}
				// The bound is tested before the insert, so it bounds the map as well as the slice —
				// otherwise the one thing that grows with the tree is the one thing nothing bounds.
				// subcommandsToFind already reads in this order. What the count then counts is call
				// sites rather than distinct pairs, which is what its own message says: past the bound
				// a pair is never recorded, so each of its call sites is one more nobody checked.
				if len(sites) >= flagCallSiteCap {
					capped++
					continue
				}
				seen[site.pair()] = true
				sites = append(sites, site)
			}
		}
	}
	return sites, capped
}

// Where this tree writes a command: every line inside a fenced block, and every backticked span outside
// one. A call site is a command, and prose is not — a sentence naming `check.sh` and then `--gate` of
// something else would otherwise read as one.
//
// Nothing here excludes an example. A fenced block is exactly what a session copies and a backticked
// span is how a skill states the command it must run, so a scan that skipped them would read almost
// nothing in this tree and report a clean result over it.
func commandSpans(lines []string) []commandSpan {
	var spans []commandSpan
	inFence := false
	for at, line := range lines {
		if shell.IsFenceDelimiter(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			spans = append(spans, commandSpan{text: line, line: at + 1})
			continue
		}
		for _, span := range delimitedSpans(line, "`") {
			spans = append(spans, commandSpan{text: span, line: at + 1})
		}
	}
	return spans
}

// The `<script> --<flag>` pairs one command span names. The last `*.sh` token opens a command and every
// `--flag` after it belongs to that one, until a separator starts another: `a.sh --x | b.sh --y` names
// two commands, and reading the second flag as the first script's would report a defect in the wrong
// file.
func flagsNamedInSpan(span commandSpan, file string) []flagCallSite {
	var found []flagCallSite
	script := ""
	for _, token := range shell.SplitFields(span.text) {
		token = strings.Trim(token, commandTokenMarks)
		if isCommandSeparator(token) {
			script = ""
			continue
		}
		if match := scriptInACommand.FindStringSubmatch(token); match != nil {
			script = match[2]
			continue
		}
		if script == "" {
			continue
		}
		if match := flagInACommand.FindStringSubmatch(token); match != nil {
			found = append(found, flagCallSite{script: script, flag: match[1], file: file, line: span.line})
		}
	}
	return found
}

// Whether a token ends the command before it. Read on the first byte alone, because `--agent=claude|codex`
// carries a separator byte in the middle of a value and is one argument.
func isCommandSeparator(token string) bool {
	return token != "" && strings.IndexByte("|;&", token[0]) >= 0
}

// The flags one script documents, or the fact that this scan never got to look. Both ways readLines
// declines are read, and the nil error is the one that matters: a file over the read bound comes back
// with no lines and no error, and a scan testing only the error would say "states no lowercase 'usage:'
// line" about a file the line above it says was NOT checked. A run may refuse to read a script or
// describe it, never both.
//
// The cost, taken deliberately: shell.SplitLines also answers nil for a 0-byte script, so an empty
// script is silent here too. It is already thin cover — `bash -n` passes an empty file — and
// scriptNotExecutable still names one. Telling the two apart needs a third signal out of readLines
// that no other caller would use, which is a larger structure than the case is worth.
func (c *checker) documentedFlagsOf(path string) documentedFlags {
	lines, err := c.readLines(path)
	if err != nil || lines == nil {
		return documentedFlags{unread: true}
	}
	return usageFlags(lines)
}

// The flags a script's own usage block names. The block is the first `usage:` line of the leading
// comment header plus the lines under it indented past it — a grammar wrapped over several lines is one
// usage line, and the prose line beside it at the header's own indent is not part of it. That boundary
// is what keeps `# Run --help for …` from documenting a flag the grammar never names.
//
// Lowercase `usage:` only, the same anchor subcommands.go's usageSubcommands and
// ai/tools/stub_usage_test.go both take, and the one tool-stub-test.sh already refuses a `Usage:`
// against. One spelling per thing, or this scan and those two disagree about which line is the usage.
//
// Read out of leadingCommentBlock, so a `# usage:` written inside a function body is not one: that
// helper stops at the first line the header does not hold, and bounds what it reads.
//
// The trailing `   #` prose ai/tools/stub_usage_test.go cuts is cut here too. A run of spaces before a
// second `#` is where the grammar ended and the commentary began, and a flag named only in that
// commentary is named in prose rather than documented.
func usageFlags(lines []string) documentedFlags {
	found := documentedFlags{flags: map[string]bool{}}
	indent := 0
	for _, line := range leadingCommentBlock(lines) {
		text := strings.TrimPrefix(line, "#")
		trimmed := strings.TrimLeft(text, " \t")
		at := len(text) - len(trimmed)
		if !found.stated {
			if !strings.HasPrefix(trimmed, "usage: ") {
				continue
			}
			found.stated, indent = true, at
		} else if at <= indent {
			break
		}
		if cut := strings.Index(trimmed, "   #"); cut >= 0 {
			trimmed = trimmed[:cut]
		}
		for _, match := range flagInAUsage.FindAllStringSubmatch(trimmed, -1) {
			found.flags[match[1]] = true
		}
	}
	return found
}
