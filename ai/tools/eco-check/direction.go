package ecocheck

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"kk-flavor/tools/shell"
)

// The four bounded heads are passed to addBounded as the class, so each also heads its own bound
// notice.
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
	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		file := shell.Join(c.root.Named(), name)
		if shell.IsRegularFile(file) {
			targets = append(targets, file)
		}
	}

	lanes := c.laneAlternation()
	citesPattern := laneCitationPattern(lanes, c.laneFileAlternation())
	namesPattern := laneNamePattern(lanes)
	basenames := c.laneBasenames(targets)

	counters := &directionCounters{}
	wasFlavorScanned := false
	for _, target := range targets {
		for _, file := range c.sharedFilesNamed(target, "*.md") {
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

// The shared layer's own files: everything under a target except the lane trees. `skills/` sits inside
// `kk-flavor/` so the two install as one mount, which means a plain walk of the bucket reaches every
// SKILL.md as well. Read as shared layer, every skill's citation of another skill becomes a `cites
// into a lane` finding, and — worse, because it is silent — every lane basename also appears in the
// shared set, so the uniqueness gate drops all of them and the basename scan goes dark.
//
// Excluding the lane tree rather than listing the shared directories by name is what keeps a
// directory added under kk-flavor/ tomorrow scanned by default. A list would let it escape with
// nothing reporting the gap.
func (c *checker) sharedFilesNamed(target string, globs ...string) []string {
	var kept []string
	for _, file := range c.filesNamed(target, globs...) {
		if c.underALane(file) {
			continue
		}
		kept = append(kept, file)
	}
	return kept
}

// The lane trees. A worker prompt is addressed to one agent doing one skill's work, so its mention of
// that skill is the ordinary skill-to-skill citation ecosystem.md → **One home** permits, not the
// shared layer reaching into a lane. A third lane tree has to be named here by hand.
func (c *checker) laneTrees() []string {
	return []string{c.root.Skills(), c.root.Flavor() + "/workers"}
}

func (c *checker) underALane(file string) bool {
	for _, lane := range c.laneTrees() {
		if strings.HasPrefix(file, lane+"/") {
			return true
		}
	}
	return false
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
		if name == "kk-flavor" || !isCleanBasename(name) {
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
		escaped = append(escaped, regexp.QuoteMeta(name))
	}
	if len(escaped) == 0 {
		return `$^`
	}
	return strings.Join(escaped, "|")
}

// `(/seg)+`, not `/seg`: one segment stops the echoed path at `.../kk-drive/scripts` and drops the
// file the citation was actually about, which is the half a reader needs to find it and move it.
//
// The third arm is the entrance to the exemption: a tree exempt from being *read* as shared layer
// still needs the shared files that *steer a reader into* it caught, or a rule inside a worker prompt,
// cited from the router's always-read block, loads in every session while this scan says nothing.
//
// This arm's prefix is optional because its members already begin at the tree's own directory:
// requiring one would catch the `~/.kk-flavor/...` spelling and miss `workers/patrol/scout.md`, the
// form the router actually writes.
func laneCitationPattern(lanes, laneFiles string) *regexp.Regexp {
	arms := []string{
		`[A-Za-z0-9._~-][A-Za-z0-9._/~-]*/SKILL\.md`,
		fmt.Sprintf(`[A-Za-z0-9._~-][A-Za-z0-9._/~-]*/(%s)(/[A-Za-z0-9._-]+)+`, lanes),
	}
	// Omitted rather than given a never-matching branch. The `$^` the other arms fall back to relies
	// on a required prefix to stay quiet: with this arm's prefix optional, `$^` alone matches every
	// empty line, because position zero of one is both its start and its end.
	if laneFiles != "" {
		arms = append(arms, fmt.Sprintf(`([A-Za-z0-9._~-][A-Za-z0-9._/~-]*/)?(%s)`, laneFiles))
	}
	return regexp.MustCompilePOSIX(strings.Join(arms, "|"))
}

// The entrance arm's alternation: every file this tree carries, by whole path — the exemption covers
// all of them, so the entrance must too. Not a bare `workers` segment: `--gate` must read no
// uncommitted path as a lane, and the standards name the layer by that bare directory. Read off
// laneTrees, so no exempted tree loses its guard; skills is skipped, the arm above holding its names.
func (c *checker) laneFileAlternation() string {
	var escaped []string
	for _, lane := range c.laneTrees() {
		if lane == c.root.Skills() {
			continue
		}
		for _, path := range c.filesNamed(lane, "*") {
			relative := strings.TrimPrefix(path, c.root.Flavor()+"/")
			// Dropped rather than escaped: QuoteMeta passes a non-ASCII byte through untouched and
			// MustCompilePOSIX refuses invalid UTF-8, so one committed filename holding a stray byte
			// would panic this tool before any finding printed, taking the mounts, dangling-link and
			// home-ref scans with it. Such a name loses its entrance guard — the trade laneNames makes.
			if !isCleanRelativePath(relative) {
				continue
			}
			escaped = append(escaped, regexp.QuoteMeta(relative))
		}
	}
	return strings.Join(escaped, "|")
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
			var owned []string
			for _, lane := range c.laneTrees() {
				owned = append(owned, c.walkTree(lane).matchPath(named)...)
			}
			owner := strings.Join(owned, "\n")
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
			" names a file under two lane trees, or under a lane and the shared layer, so this scan cannot tell which was meant; rename one of them (ecosystem.md → **One home**)"
	})
}

// The bound every finding in this scan reports under, and the notice each of them ends on. The
// notice leads with the file and a space, which sorts ahead of that file's own `file:line:` hits,
// so the printer's per-rank cap drops those hits before it drops the notice.
func (c *checker) reportBoundReached(class, file string) {
	c.add(class + ": " + file + " — " + strconv.Itoa(findingCap) +
		" already shown across the shared layer; the rest are not listed")
}

// The basenames that name exactly one file across the lane trees, and the subset this scan cannot
// attribute to one of them — a name a second lane tree holds too, or one the shared layer carries.
//
// Uniqueness is the whole gate on the first set: a basename several lanes carry names the *kind* of
// file rather than one of them, which is why `SKILL.md` (every lane has one) does not fire while
// `report.sh` (one lane has it) does. The second set is ambiguous rather than violating — the
// reviewed tree fills the lane trees, so one committed file under a lane named after a standard would
// otherwise report every standard citing that sibling, findings aimed at files the branch never
// touched.
//
// A basename carrying a character outside the set below is dropped from both: a committed filename
// holding a newline reaches a line-oriented reader as two names, one forging a finding against a file
// the branch never touched and the other muting a real violation through the uniqueness gate.
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
	holders := map[string]map[string]bool{}
	for _, lane := range c.laneTrees() {
		for _, path := range c.filesNamed(lane, "*.sh", "*.md") {
			if name := shell.BaseName(path); isCleanBasename(name) {
				counts[name]++
				if holders[name] == nil {
					holders[name] = map[string]bool{}
				}
				holders[name][lane] = true
			}
		}
	}
	shared := map[string]bool{}
	for _, target := range sharedTargets {
		for _, path := range c.sharedFilesNamed(target, "*.sh", "*.md") {
			if name := shell.BaseName(path); isCleanBasename(name) {
				shared[name] = true
			}
		}
	}
	sets := laneBasenameSets{underOneLane: map[string]bool{}, ambiguous: map[string]bool{}}
	for name, count := range counts {
		if count != 1 {
			// Repeated inside one lane tree is the kind-name case the scan passes over — `SKILL.md` sits
			// under every skill. Repeated across two is not: that name used to reach the shared set and
			// get reported, so dropping it silently would be the mute this scan exists to deny.
			if len(holders[name]) > 1 {
				sets.ambiguous[name] = true
			}
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

// The same character gate over a relative path: every segment of it has to be a clean basename.
func isCleanRelativePath(path string) bool {
	return path != "" && !strings.ContainsFunc(path, func(r rune) bool { return r != '/' && isNotLaneNameRune(r) })
}

// One match of a pattern, kept as the two things it is: the boundary character a pattern consumed
// makes the halves mean different things.
type lineMatch struct {
	line  int
	match string
}

// The `<line>:<match>` form a finding echoes.
func (m lineMatch) String() string { return strconv.Itoa(m.line) + ":" + m.match }

func grepNumbered(lines []string, pattern *regexp.Regexp) []lineMatch {
	var hits []lineMatch
	for i, line := range lines {
		for _, match := range pattern.FindAllString(line, -1) {
			hits = append(hits, lineMatch{line: i + 1, match: match})
		}
	}
	return hits
}
