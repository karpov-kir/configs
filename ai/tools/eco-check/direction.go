package ecocheck

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"kk-flavor/tools/shell"
)

// The heads this scan's findings lead with, which report.go's rankTable ranks them on. The four
// bounded ones are passed to addBounded as the class, so each also heads its own bound notice.
const (
	directionScanReadNoFiles         = "direction scan read no files"
	sharedLayerCitesLane             = "shared layer cites into a lane"
	sharedLayerNamesLane             = "shared layer names a lane"
	sharedLayerReachesLaneByBasename = "shared layer reaches into a lane by basename"
	basenameNotChecked               = "basename not checked"
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
// matches. A bare skill name counts only when a skill of that name exists. That gate is no licence
// for the prose it lets through: every other `kk-*`/`idsd-*` token the shared layer carries is
// already a finding from the unknown-skill scan.
//
// The third shape is a lane file named by its basename alone — `report.sh`, `check.sh`. It carries
// neither a lane name nor a path, so the first two miss it, and it steers its reader into a lane
// just the same. Its message deliberately does not extend `shared layer names a lane`: the suite
// matches a finding by fixed substring, so a message carrying another's whole text would satisfy
// that other one's does-not-report cases and turn them into silent passes.
//
// Fences are not skipped, unlike in the scans that resolve a citation — a banned form steers its
// reader from inside one too.
func (c *checker) scanDirection() {
	targets := []string{c.root.Flavor()}
	if shell.IsRegularFile(shell.Join(c.root.Named(), "CLAUDE.md")) {
		targets = append(targets, shell.Join(c.root.Named(), "CLAUDE.md"))
	}

	lanes := c.laneAlternation()
	citesPattern := laneCitationPattern(lanes)
	namesPattern := laneNamePattern(lanes)
	basenames := c.laneBasenames(targets)

	counters := &directionCounters{}
	wasFlavorScanned := false
	for _, target := range targets {
		for _, file := range c.filesNamed(target, "*.md") {
			// Set from flavor files alone: one flag over both tiers would let a readable CLAUDE.md
			// stand in for the tree and mute the guard below.
			if strings.HasPrefix(file, c.root.Flavor()+"/") {
				wasFlavorScanned = true
			}
			lines, err := c.readLines(file)
			if err != nil {
				continue
			}
			safeFile := shell.Oneline(file)
			c.reportLaneCitations(counters, safeFile, lines, citesPattern)
			c.reportLaneNames(counters, safeFile, lines, namesPattern)
			if basenames.any() {
				c.reportLaneBasenames(counters, safeFile, lines, basenames)
			}
		}
	}
	if !wasFlavorScanned {
		c.add(directionScanReadNoFiles + " under " + c.root.Flavor() + " — a check that did not run is not a clean one")
	}
}

// The lane names come from the tree, never from the `kk-`/`idsd-` families. Nothing enforces that
// naming rule, so keying on the prefix would trust a convention no scan checks, and a skill named
// outside it could be cited and named freely. A name outside the characters below would reach the
// regexp as a metacharacter and match text no skill owns.
func (c *checker) laneNames() []string {
	var names []string
	for _, name := range c.skillDirNames() {
		if !c.holdsRegularFile(c.skillFilePath(name)) {
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

// One finding under this scan's shared bound: always counted, printed while under the cap, and
// announced once at the boundary. `class` both prefixes the finding and names it in the announcement,
// so the two cannot drift apart. `detail` is a closure because the basename shape walks the skills
// tree to build its text, and must not do that for a finding the cap has already dropped.
func (c *checker) addBounded(count *int, class, file string, detail func() string) {
	*count++
	if *count <= findingCap {
		c.add(class + ": " + detail())
	} else if *count == findingCap+1 {
		c.reportBoundReached(class, file)
	}
}

func (c *checker) reportLaneCitations(counters *directionCounters, safeFile string, lines []string, pattern *regexp.Regexp) {
	for _, hit := range grepNumbered(lines, pattern) {
		c.addBounded(&counters.cites, sharedLayerCitesLane, safeFile, func() string {
			return safeFile + ":" + shell.Oneline(hit.String()) +
				" — move the rule to a standard (ecosystem.md → **One home**)"
		})
	}
}

func (c *checker) reportLaneNames(counters *directionCounters, safeFile string, lines []string, pattern *regexp.Regexp) {
	for _, hit := range grepNumbered(lines, pattern) {
		// The trailing run of `.`, `_` and `-` is punctuation the token ends on, not part of the name —
		// the hyphen of a `kk-drive-*` glob, the full stop of a sentence ending on the lane name. Either
		// would match no skill directory. Trimmed as a run, never one character: a token can end on more
		// than one.
		named := strings.TrimRight(hit.match, "._-")
		// The whole token is tested, not the alternation's own match: `kk-drive-verified` starts
		// with a real lane name and is not one, so matching the prefix alone would report a skill
		// that does not exist as a lane the shared layer names.
		if !c.holdsRegularFile(c.skillFilePath(named)) {
			continue
		}
		c.addBounded(&counters.names, sharedLayerNamesLane, safeFile, func() string {
			return safeFile + ":" + shell.Oneline(hit.String()) +
				" — name the lane, and let the skill bind itself to it (ecosystem.md → **One home**)"
		})
	}
}

func (c *checker) reportLaneBasenames(counters *directionCounters, safeFile string, lines []string, basenames laneBasenameSets) {
	for _, hit := range grepNumbered(lines, laneBasenamePattern) {
		lineNumber := strconv.Itoa(hit.line)
		// The leading boundary character comes back with the match; a token starts on
		// `[A-Za-z0-9]`, so a first character outside that set is the boundary, never the name.
		named := hit.match
		if named != "" && !isAlnumByte(named[0]) {
			named = named[1:]
		}
		// The order below is the guard, not a tidy-up. Test the violation set first and every
		// ambiguous name becomes a forged finding.
		if basenames.ambiguous[named] {
			c.reportUncheckedBasename(counters, safeFile, lineNumber, named)
			continue
		}
		if !basenames.underOneLane[named] {
			continue
		}
		c.addBounded(&counters.basenames, sharedLayerReachesLaneByBasename, safeFile, func() string {
			// The line number alone, never the match: echoing it would carry the boundary character
			// the pattern consumed, so the finding would show an unbalanced tick for a name written
			// `` `doit.sh` ``.
			owner := strings.Join(c.walkTree(c.root.Skills()).matchPath(named), "\n")
			return safeFile + ":" + lineNumber +
				" — " + shell.Oneline(named) + " is " + shell.Oneline(owner) +
				"; move the rule to a standard (ecosystem.md → **One home**)"
		})
	}
}

// Silence here would be the cheapest mute the reviewed tree has — commit any `.md` under
// `kk-flavor/` named after a lane file and every mention of that file stops being checked, while no
// other scan names the file the branch committed. A narrowed scan reports that it narrowed.
func (c *checker) reportUncheckedBasename(counters *directionCounters, safeFile, lineNumber, named string) {
	c.addBounded(&counters.ambiguous, basenameNotChecked, safeFile, func() string {
		return safeFile + ":" + lineNumber + " — " + shell.Oneline(named) +
			" names a file under both a lane and the shared layer, so this scan cannot tell which was meant; rename one of them (ecosystem.md → **One home**)"
	})
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
// gets both halves of a forgery. The tail is a basename no lane carries, so a standard naming a file
// nothing under `skills/` holds is reported against a file the branch never touched. The head is a
// second copy of a real basename, so the uniqueness gate drops it and a genuine violation goes quiet.
//
// The two sets travel together because the order they are tested in is the guard, and two bare maps
// in a signature can be transposed silently where one value cannot.
type laneBasenameSets struct {
	underOneLane map[string]bool
	ambiguous    map[string]bool
}

func (s laneBasenameSets) any() bool { return len(s.underOneLane) > 0 || len(s.ambiguous) > 0 }

func (c *checker) laneBasenames(sharedTargets []string) laneBasenameSets {
	counts := map[string]int{}
	for _, path := range c.filesNamed(c.root.Skills(), "*.sh", "*.md") {
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
	sets := laneBasenameSets{underOneLane: map[string]bool{}, ambiguous: map[string]bool{}}
	for name, count := range counts {
		if count != 1 {
			continue
		}
		sets.underOneLane[name] = true
		if shared[name] {
			sets.ambiguous[name] = true
		}
	}
	return sets
}

func isCleanBasename(name string) bool {
	return name != "" && !strings.ContainsFunc(name, isNotLaneNameRune)
}

// One match of a pattern, kept as the two things it is. Packed into `<line>:<match>` it was cut back
// apart at three of the four call sites, and the boundary character a pattern consumed makes the
// halves mean different things.
type lineMatch struct {
	line  int
	match string
}

// The `<line>:<match>` form a finding echoes, which is what `grep -no` printed.
func (m lineMatch) String() string { return strconv.Itoa(m.line) + ":" + m.match }

// `grep -no`: every match on every line. The shell version needed `-a` on every grep, and the hazard
// behind that is gone with it: one committed NUL byte made grep call the file binary and print
// nothing at all, so a violating shared layer read as clean while the did-not-run guard still counted
// the file as scanned.
func grepNumbered(lines []string, pattern *regexp.Regexp) []lineMatch {
	var hits []lineMatch
	for i, line := range lines {
		for _, match := range pattern.FindAllString(line, -1) {
			hits = append(hits, lineMatch{line: i + 1, match: match})
		}
	}
	return hits
}
