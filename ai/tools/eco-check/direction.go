package ecocheck

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"kk-flavor/tools/shell"
)

// A boundary character comes back with the match, so the basename scan strips it; the pattern
// excludes `/` and `~` so a token inside a path the cites scan already reported does not come back
// here as a second finding for the same text.
var laneBasenamePattern = regexp.MustCompilePOSIX(`(^|[^/~A-Za-z0-9._-])[A-Za-z0-9][A-Za-z0-9._-]*\.(sh|md)`)

// The per-shape emit counters. They live on a value the scan owns rather than on the package, so a
// second run in the same process starts them at zero.
type directionCounters struct {
	cites     int
	names     int
	basenames int
	ambiguous int
}

// Direction: the shared layer never cites into a lane, and never names one (ecosystem.md → **One
// home**). Three shapes are banned.
//
// A path into a skill needs one name character before the slash, or a glob's bare `/SKILL.md` tail
// matches. A bare skill name counts only when a skill of that name exists; that gate is not a
// licence for the prose it lets through, because every other `kk-*`/`idsd-*` token the shared layer
// carries is already a finding from the unknown-skill scan. The third is a lane file named by its
// basename alone — `report.sh`, `check.sh`: it carries neither a lane name nor a path, so the first
// two miss it, and it steers its reader into a lane just the same. Its message deliberately does
// not extend `shared layer names a lane`, because check-test.sh matches a finding by fixed
// substring and a message carrying another's whole text would satisfy that other one's
// assert_does_not_report cases and turn them into silent passes.
//
// Fences are not skipped, unlike in the scans that resolve a citation — a banned form steers its
// reader from inside one too.
func (c *checker) scanDirection() {
	targets := []string{c.flavor}
	if shell.IsRegularFile(shell.Join(c.root, "CLAUDE.md")) {
		targets = append(targets, shell.Join(c.root, "CLAUDE.md"))
	}

	lanes := c.laneAlternation()
	citesPattern := laneCitationPattern(lanes)
	namesPattern := laneNamePattern(lanes)
	laneBasenames, ambiguousBasenames := c.laneBasenames(targets)

	counters := &directionCounters{}
	wasFlavorScanned := false
	for _, target := range targets {
		for _, file := range c.filesNamed(target, "*.md") {
			// Set from flavor files alone: one flag over both tiers would let a readable CLAUDE.md
			// stand in for the tree and mute the guard below.
			if strings.HasPrefix(file, c.flavor+"/") {
				wasFlavorScanned = true
			}
			lines, err := c.readLines(file)
			if err != nil {
				continue
			}
			safeFile := shell.Oneline(file)
			c.reportLaneCitations(counters, safeFile, lines, citesPattern)
			c.reportLaneNames(counters, safeFile, lines, namesPattern)
			if len(laneBasenames) > 0 || len(ambiguousBasenames) > 0 {
				c.reportLaneBasenames(counters, safeFile, lines, laneBasenames, ambiguousBasenames)
			}
		}
	}
	if !wasFlavorScanned {
		c.add("direction scan read no files under " + c.flavor + " — a check that did not run is not a clean one")
	}
}

// The lane names come from the tree, never from the `kk-`/`idsd-` families. Nothing enforces that
// naming rule, so keying on the prefix would trust a convention no scan checks, and a skill named
// outside it could be cited and named freely. A name outside the characters below would reach the
// regexp as a metacharacter and match text no skill owns.
func (c *checker) laneNames() []string {
	var names []string
	for _, name := range c.skillDirNames() {
		if !shell.IsRegularFile(shell.Join(shell.Join(c.skills, name), "SKILL.md")) {
			continue
		}
		// `kk-flavor` is the shared layer itself, so a reviewed tree committing `skills/kk-flavor/`
		// would turn every mention of that layer inside its own standards into a finding.
		if name == "kk-flavor" || strings.ContainsFunc(name, isNotLaneNameRune) {
			continue
		}
		names = append(names, name)
	}
	return names
}

func isNotLaneNameRune(r rune) bool {
	return !(r < 0x80 && (isAlnumByte(byte(r)) || r == '.' || r == '_' || r == '-'))
}

// The alternation the two path-shaped scans embed. With no lanes it is `$^`, which matches nothing
// — the same never-matching branch the shell version substituted, so the SKILL.md half of the
// citation pattern still fires on a tree that mounts no skill at all.
func (c *checker) laneAlternation() string {
	var escaped []string
	for _, name := range c.laneNames() {
		escaped = append(escaped, strings.ReplaceAll(name, ".", `\.`))
	}
	if len(escaped) == 0 {
		return `$^`
	}
	return strings.Join(escaped, "|")
}

// `(/seg)+`, not `/seg`: one segment stops the echoed path at `.../kk-drive/scripts` and drops the
// file the citation was actually about, which is the half a reader needs to find it and move it.
func laneCitationPattern(lanes string) *regexp.Regexp {
	return regexp.MustCompilePOSIX(fmt.Sprintf(
		`[A-Za-z0-9._~-][A-Za-z0-9._/~-]*/SKILL\.md|[A-Za-z0-9._~-][A-Za-z0-9._/~-]*/(%s)(/[A-Za-z0-9._-]+)+`,
		lanes))
}

func laneNamePattern(lanes string) *regexp.Regexp {
	return regexp.MustCompile(`\b(` + lanes + `)[A-Za-z0-9._-]*`)
}

func (c *checker) reportLaneCitations(counters *directionCounters, safeFile string, lines []string, pattern *regexp.Regexp) {
	for _, hit := range grepNumbered(lines, pattern) {
		counters.cites++
		if counters.cites <= findingCap {
			c.add("shared layer cites into a lane: " + safeFile + ":" + shell.Oneline(hit) +
				" — move the rule to a standard (ecosystem.md → **One home**)")
		} else if counters.cites == findingCap+1 {
			c.reportBoundReached("shared layer cites into a lane", safeFile)
		}
	}
}

func (c *checker) reportLaneNames(counters *directionCounters, safeFile string, lines []string, pattern *regexp.Regexp) {
	for _, hit := range grepNumbered(lines, pattern) {
		_, matched, _ := strings.Cut(hit, ":")
		// The trailing run of `.`, `_` and `-` is punctuation the token ends on, not part of the
		// name: the suffix class carries all three, so the match keeps the hyphen a `kk-drive-*`
		// glob ends on and the full stop of a sentence that ends on the lane name alike, and either
		// would then match no skill directory. Trimmed as a run, never one character: a token can
		// end on more than one.
		named := strings.TrimRight(matched, "._-")
		// The whole token is tested, not the alternation's own match: `kk-drive-verified` starts
		// with a real lane name and is not one, so matching the prefix alone would report a skill
		// that does not exist as a lane the shared layer names.
		if !shell.IsRegularFile(shell.Join(shell.Join(c.skills, named), "SKILL.md")) {
			continue
		}
		counters.names++
		if counters.names <= findingCap {
			c.add("shared layer names a lane: " + safeFile + ":" + shell.Oneline(hit) +
				" — name the lane, and let the skill bind itself to it (ecosystem.md → **One home**)")
		} else if counters.names == findingCap+1 {
			c.reportBoundReached("shared layer names a lane", safeFile)
		}
	}
}

func (c *checker) reportLaneBasenames(counters *directionCounters, safeFile string, lines []string, laneBasenames, ambiguousBasenames map[string]bool) {
	for _, hit := range grepNumbered(lines, laneBasenamePattern) {
		lineNumber, matched, _ := strings.Cut(hit, ":")
		// The leading boundary character comes back with the match; a token starts on
		// `[A-Za-z0-9]`, so a first character outside that set is the boundary, never the name.
		named := matched
		if named != "" && !isAlnumByte(named[0]) {
			named = named[1:]
		}
		// **The ordering below is load-bearing**, and it is the guard rather than a convenience:
		// test the violation set first and every ambiguous name becomes a forged finding.
		if ambiguousBasenames[named] {
			c.reportUncheckedBasename(counters, safeFile, lineNumber, named)
			continue
		}
		if !laneBasenames[named] {
			continue
		}
		counters.basenames++
		if counters.basenames <= findingCap {
			// The line number alone, never the match: echoing it would carry the boundary character
			// the pattern consumed, so the finding would show an unbalanced tick for a name written
			// `` `doit.sh` ``.
			owner := strings.Join(c.walkTree(c.skills).matchPath(named), "\n")
			c.add("shared layer reaches into a lane by basename: " + safeFile + ":" + lineNumber +
				" — " + shell.Oneline(named) + " is " + shell.Oneline(owner) +
				"; move the rule to a standard (ecosystem.md → **One home**)")
		} else if counters.basenames == findingCap+1 {
			c.reportBoundReached("shared layer reaches into a lane by basename", safeFile)
		}
	}
}

// Silence here would be the cheapest mute the reviewed tree has — commit any `.md` under
// `kk-flavor/` named after a lane file and every mention of that file stops being checked, while no
// other scan names the file the branch committed. A narrowed scan reports that it narrowed.
func (c *checker) reportUncheckedBasename(counters *directionCounters, safeFile, lineNumber, named string) {
	counters.ambiguous++
	if counters.ambiguous <= findingCap {
		c.add("basename not checked: " + safeFile + ":" + lineNumber + " — " + shell.Oneline(named) +
			" names a file under both a lane and the shared layer, so this scan cannot tell which was meant; rename one of them (ecosystem.md → **One home**)")
	} else if counters.ambiguous == findingCap+1 {
		c.reportBoundReached("basename not checked", safeFile)
	}
}

// The bound every finding in this scan reports under, and the notice each of them ends on. The
// notice leads with the file and a space, which sorts ahead of that file's own `file:line:` hits,
// so the printer's per-rank cap drops those hits before it drops the notice.
func (c *checker) reportBoundReached(class, file string) {
	c.add(class + ": " + file + " — " + strconv.Itoa(findingCap) +
		" already shown across the shared layer; the rest are not listed")
}

// The basenames that name exactly one file under `$skills`, and the subset the shared layer also
// carries.
//
// Uniqueness is the whole gate on the first set: a basename several lanes carry names the *kind* of
// file rather than one of them, which is why `SKILL.md` (every lane has one) does not fire while
// `report.sh` (one lane has it) does. The second set is ambiguous rather than violating — the
// reviewed tree fills `$skills`, so one committed file under a lane named after a standard would
// otherwise report every standard citing that sibling, findings aimed at files the branch never
// touched.
//
// A basename carrying a character outside the set below is dropped from both. A committed filename
// holding a newline would otherwise reach a line-oriented reader as two names, and the reviewed tree
// gets both halves of a forgery: the tail is a basename no lane carries, so a standard naming a file
// nothing under `skills/` holds is reported against a file the branch never touched, and the head is
// a second copy of a real basename, so the uniqueness gate drops it and a genuine violation goes
// quiet.
func (c *checker) laneBasenames(sharedTargets []string) (lane, ambiguous map[string]bool) {
	counts := map[string]int{}
	for _, path := range c.filesNamed(c.skills, "*.sh", "*.md") {
		if name := shell.BaseName(path); isCleanBasename(name) {
			counts[name]++
		}
	}
	shared := map[string]bool{}
	for _, target := range sharedTargets {
		for _, path := range c.filesNamed(target, "*.sh", "*.md") {
			if name := shell.BaseName(path); isCleanBasename(name) {
				shared[name] = true
			}
		}
	}
	lane = map[string]bool{}
	ambiguous = map[string]bool{}
	for name, count := range counts {
		if count != 1 {
			continue
		}
		lane[name] = true
		if shared[name] {
			ambiguous[name] = true
		}
	}
	return lane, ambiguous
}

func isCleanBasename(name string) bool {
	return name != "" && !strings.ContainsFunc(name, isNotLaneNameRune)
}

// `grep -no`: every match on every line, each as the `<line>:<match>` grep would have printed. The
// per-grep `-a` the shell version could not drop is gone with the hazard behind it — one committed
// NUL byte made grep call the file binary and print nothing at all, so a violating shared layer read
// as clean while the did-not-run guard still counted the file as scanned.
func grepNumbered(lines []string, pattern *regexp.Regexp) []string {
	var hits []string
	for i, line := range lines {
		for _, match := range pattern.FindAllString(line, -1) {
			hits = append(hits, strconv.Itoa(i+1)+":"+match)
		}
	}
	return hits
}
