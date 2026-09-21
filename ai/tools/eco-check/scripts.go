package ecocheck

import (
	"crypto/sha256"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"configs/ai/tools/shell"
)

// How much of a script's head is read as its test-position declaration, and how many suite names are
// taken out of it. Both are named because the finding beside each one states the same number back to
// its reader, and a bound that drifts from what the message claims is a message that lies.
const (
	headerLineCap = 200
	namedSuiteCap = 8
)

const (
	syntaxError                    = "syntax: "
	scriptNotExecutable            = "script not executable"
	scriptNamesTooManySuites       = "script names more suites than the scan reads"
	scriptDeclaresNoTestPosition   = "script declares no test position: "
	scriptNamesMissingTest         = "script names a missing test"
	scriptNamesAmbiguousTest       = "script names an ambiguous test"
	scriptUsageSpellingUnread      = "script states its usage in a spelling no scan reads: "
	sharedRegionNotChecked         = "shared region not checked for drift: "
	sharedRegionWithoutCounterpart = "shared region without a counterpart: "
	sharedRegionHasDrifted         = "shared region has drifted: "
)

var (
	namedTestSuite   = regexp.MustCompilePOSIX(`[A-Za-z0-9_.-]+-test\.sh`)
	untestedDeclared = regexp.MustCompilePOSIX(`^#[[:space:]]*untested:[[:space:]]*[^[:space:]]`)
	// `ai/tools` itself is a package and an answer. A case whose subject sits outside that Go module,
	// as this script does, belongs in the root package. Go keys a test cache on the module, and a
	// package under it would answer `ok (cached)` over a file that had moved.
	goSuiteDeclared   = regexp.MustCompilePOSIX(`the Go suite in (ai/tools[A-Za-z0-9_/-]*)/`)
	sharedRegionOpen  = regexp.MustCompilePOSIX(`^[[:space:]]*# --- shared:[A-Za-z0-9_-]+ ---[[:space:]]*$`)
	sharedRegionClose = regexp.MustCompilePOSIX(`^[[:space:]]*# --- end shared:[A-Za-z0-9_-]+ ---[[:space:]]*$`)
	sharedRegionName  = regexp.MustCompilePOSIX(`^[[:space:]]*# --- shared:`)
	sharedRegionTail  = regexp.MustCompilePOSIX(`[[:space:]]*---[[:space:]]*$`)
)

// The whole `bash -n` output for one script, already sanitised and prefixed.
type parseResult struct {
	script   string
	findings []string
}

// The map is held for the process, and the saving is there.

// A single check cannot use the map. The parse workers reach every copy of a script before any worker
// has stored anything. One check of this tree parses 88 times over 44 distinct scripts whatever this
// map holds.

// What it answers is a SECOND check in the same process, and TestRepeatedScriptContentIsParsedOnce
// holds that second check to zero parses. Two runs sharing this map cannot see each other, because a
// hit is keyed on the bytes and returns what the parse would have returned for them.

// Every (binary, script content) pair already seen to parse clean. `bash -n` reads the file and
// nothing else, so under a fixed binary and a fixed locale it is a function of the script's bytes: a
// file whose content another file has already parsed clean needs no process of its own.
//
// Keyed on SHA-256 and not on a cheaper digest: the reviewed tree writes the scripts, and a digest it
// could collide would let a broken script inherit a clean one's answer.
//
// Unbounded in entries, deliberately. One entry costs a digest and a path, and one *miss* costs a
// process — so a tree big enough to make this map matter has already spent hours forking, and a bound
// here would be a guard nothing could ever observe.
var cleanParses sync.Map

// Skills reach their scripts by path (`scripts/report.sh …`), so a lost exec bit is a stage that
// cannot run at all. A script is also parsed under every bash `#!/usr/bin/env bash` could resolve to.
// bash.go's port holds which those are and why there is more than one.
func (c *checker) scanScriptsParse() {
	scripts := c.filesNamed(c.root.Named(), "*.sh")
	for _, script := range scripts {
		if !isExecutable(script) {
			c.add(scriptNotExecutable + ": " + shell.Oneline(script))
		}
	}
	// Two forks per script is the dominant cost of the whole check, and each is independent of
	// every other. Findings are sorted before they are printed, so running them out of order
	// changes no byte of the output.
	binaries := c.bash.Binaries()
	if len(binaries) == 0 {
		// Zero binaries means zero forks, and the loop below then produces no findings at all —
		// byte for byte what a tree of clean scripts produces. "every script still parses" is half
		// of what this package promises, and a run that could not ask is not a run that got an
		// answer, so this leaves through the exit rather than through silence.
		c.cannotRun(fmt.Sprintf("no bash on PATH and none at /bin/bash, so NO script was parsed — %d script(s) went unchecked", len(scripts)))
		return
	}
	results := make([]parseResult, len(scripts))
	work := make(chan int)
	var group sync.WaitGroup
	for range runtime.NumCPU() {
		group.Add(1)
		go func() {
			defer group.Done()
			for i := range work {
				results[i] = parseResult{script: scripts[i], findings: c.parseErrors(binaries, scripts[i])}
			}
		}()
	}
	for i := range scripts {
		work <- i
	}
	close(work)
	group.Wait()
	for _, result := range results {
		for _, finding := range result.findings {
			c.add(finding)
		}
	}
}

func (c *checker) parseErrors(binaries []string, script string) []string {
	var findings []string
	digest := scriptDigest(script)
	for _, binary := range binaries {
		key := binary + "\x00" + digest
		if digest != "" {
			if _, isClean := cleanParses.Load(key); isClean {
				continue
			}
		}
		output := c.bash.Parse(binary, script)
		// Stored only when the parse was clean and the digest is one this process held: an error
		// quotes the failing path back, so the next file with these bytes needs its own message.
		if len(output) == 0 {
			if digest != "" {
				cleanParses.Store(key, struct{}{})
			}
			continue
		}
		// A parse error is reported over several lines and each becomes its own finding, so the
		// split comes first and Oneline then sanitises what is left. One definition of a control
		// byte, the same one every other finding is echoed through: this message quotes the
		// script's own text and its path, both chosen by the reviewed tree.
		for _, line := range shell.SplitLines(output) {
			findings = append(findings, syntaxError+shell.Oneline(line))
		}
	}
	return findings
}

// The content digest a memo entry is keyed on, or empty for a file this process will not hold in
// memory — one over the read bound is parsed by its own process every time, exactly as before.
func scriptDigest(script string) string {
	info, err := os.Stat(script)
	if err != nil || info.Size() > maxFileBytes {
		return ""
	}
	content, err := os.ReadFile(script)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(content)
	return string(sum[:])
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode()&0o111 != 0
}

// ecosystem.md → **Prefer the mechanism**: a script is prose turned into enforcement, so something
// has to prove the enforcement still fires. Each script states its test position in its header,
// either the `-test.sh` covering it or `# untested: <why>`. That header is what kk-reduce's Phase 6
// reads to pick what to run. Stating neither hides the script from that phase. Naming a suite that is
// not there is worse: the phase finds nothing to run, and the script counts as covered by a suite
// that does not exist.
func (c *checker) scanTestPositions() {
	// Built once, and from basenames the reviewed tree cannot forge: a filename holding a newline
	// would otherwise contribute its second line as a bare suite name, and a header naming a missing
	// suite would then pass the existence check.
	//
	// Which files answer to a suite name, never merely whether any does. A header writes its suite as
	// a basename, and two lanes both carrying `report-test.sh` weld into one name. A header naming it
	// is then satisfied by a suite in the other lane that never sees this script. So the scan needs the
	// carriers to tell the two apart; their count answers the existence question too.
	carriers := map[string][]string{}
	for _, path := range c.filesNamed(c.root.Named(), "*-test.sh") {
		if name := shell.BaseName(path); isCleanBasename(name) {
			carriers[name] = append(carriers[name], path)
		}
	}
	for _, script := range c.filesNamed(c.root.Named(), "*.sh") {
		// The harness is exempt: asking a test file to name its own test makes every one a finding.
		if isTestHarness(script) {
			continue
		}
		if lines, err := c.readLines(script); err == nil {
			c.reportTestPosition(script, lines, carriers)
		}
	}
}

func (c *checker) reportTestPosition(script string, lines []string, carriers map[string][]string) {
	header := leadingCommentBlock(lines)
	named := shell.SortUnique(allMatches(header, namedTestSuite))
	// The count is of the 200-line window, never of the file: past that bound nothing was read, so a
	// header carrying thousands of names would report the window's total as the file's.
	//
	// Every finding below names the script by its path and not by the basename it used to. Two lanes
	// carrying one basename produced byte-identical findings, and identical findings collapse in the
	// sort, so one of the two scripts went unmentioned by the check that had just found it.
	if len(named) > namedSuiteCap {
		c.add(fmt.Sprintf(scriptNamesTooManySuites+": %s names %d in its first %d lines, of which %d are read",
			shell.Oneline(script), len(named), headerLineCap, namedSuiteCap))
		named = named[:namedSuiteCap]
	}
	if len(named) == 0 {
		if c.namesGoSuite(script, header) {
			return
		}
		if !anyMatch(header, untestedDeclared) {
			c.add(scriptDeclaresNoTestPosition + shell.Oneline(script) +
				" names no -test.sh and carries no '# untested: <why>'")
		}
		return
	}
	for _, suite := range named {
		if len(carriers[suite]) == 0 {
			c.add(scriptNamesMissingTest + ": " + shell.Oneline(script) + " names " + shell.Oneline(suite))
			continue
		}
		if suiteIsAmbiguous(script, suite, carriers[suite]) {
			c.add(fmt.Sprintf(scriptNamesAmbiguousTest+": %s names %s, which %d files answer to and none of them sits beside it — nothing here says which one covers it",
				shell.Oneline(script), shell.Oneline(suite), len(carriers[suite])))
		}
	}
}

// Some scripts cannot have a `-test.sh`. `ai/mcp-env.sh` is launched by an MCP client from a path
// written into a config, so it stays shell, and a Go package execs it once per case.

// The named package is held to the standard a named `-test.sh` is: it has to be there and to hold a Go
// test file. A header naming a package that is absent leaves the script counted as covered by a suite
// that does not exist, which is the failure this scan prevents.

// Such a header gets the scriptNamesMissingTest finding. A fall through to
// scriptDeclaresNoTestPosition would ask the reader for a reason, and the script already states where
// its cases are.

// A script whose cases live in the Go module and in no `-test.sh` beside it.
func (c *checker) namesGoSuite(script string, header []string) bool {
	for _, line := range header {
		match := goSuiteDeclared.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if !c.holdsGoSuite(match[1]) {
			c.add(scriptNamesMissingTest + ": " + shell.Oneline(script) + " names " + shell.Oneline(match[1]))
		}
		return true
	}
	return false
}

// Whether any Go test file in the tree sits in the named package. The match is on the path's tail.
// That named root is the directory the check was pointed at, and a header writes the package as a
// repository-relative path. The two agree only where the check was pointed at the repository itself.
func (c *checker) holdsGoSuite(pkg string) bool {
	for _, path := range c.filesNamed(c.root.Named(), "*_test.go") {
		if strings.HasSuffix(shell.DirName(path), "/"+pkg) {
			return true
		}
	}
	return false
}

// A file of that name beside the script resolves the name, since "a case in <suite> beside it" is what
// the header says. A name carried by a single file anywhere under the root resolves it too.

// What is left is two or more files under one basename, with none of them a sibling. The tree says
// which covers this script in no other way. The check reports the name and leaves the choice to the
// reader.

// Whether a suite name in a header answers to more than one file.
func suiteIsAmbiguous(script, suite string, carriers []string) bool {
	if len(carriers) < 2 {
		return false
	}
	sibling := shell.Join(shell.DirName(script), suite)
	for _, path := range carriers {
		if path == sibling {
			return false
		}
	}
	return true
}

// Two scans turn on this predicate, and one predicate keeps them agreeing on what a harness is. The
// scan for test position asks these files no question, and the citation scan tells their author what
// to do about a fixture it just read as a citation.

// Whether the tree treats this script as harness.
func isTestHarness(path string) bool {
	base := shell.BaseName(path)
	return strings.HasSuffix(base, "-test.sh")
}

// Every scan that reads a usage line anchors on a lowercase `usage:` — flags.go's usageFlags,
// subcommands.go's usageSubcommands, and ai/tools/stub_usage_test.go. So a header stating `Usage:` or
// `USAGE:` and nothing lowercase is in none of them: it reads to a human as documentation while
// holding nothing against anything. The flags its call sites pass are checked against nothing, the
// subcommands its dispatch accepts go unnamed, and no finding says so — because to those scans the
// file simply has no usage line at all.
//
// Widening them to take both spellings is the drift rather than the fix: it splits the anchor from
// subcommands.go and stub_usage_test.go, and leaves the tree with two spellings of one thing. The
// spelling is reported here instead, and the scans stay lowercase-only.
//
// Every `*.sh`, the harness included. A `-test.sh` header is as invisible to those scans as any
// other, and ai/tools/stub_usage_test.go, which refuses a capitalised `Usage:` too, reaches only the
// files carrying the tool-stub shared region.
func (c *checker) scanUsageSpelling() {
	for script, lines := range c.filesWithLines(c.root.Named(), "*.sh") {
		spelling := unreadUsageSpelling(lines)
		if spelling == "" {
			continue
		}
		c.add(scriptUsageSpellingUnread + shell.Oneline(script) + " writes '" + shell.Oneline(spelling) +
			"' and no lowercase 'usage:', so every scan that reads a usage line passes it by")
	}
}

// The spelling a header states its usage line in, when that spelling is one no scan reads — a header
// line opening on `usage: ` in some other case, and no header line opening on the lowercase one.
//
// The lowercase line wins wherever it sits, even below the capitalised one: those scans take the
// first lowercase `usage:` of the header and are satisfied by it, so a header carrying both is
// documented to them and there is nothing here to report.
//
// Read out of leadingCommentBlock and matched against the same trimmed text usageFlags matches, so
// what this reports is exactly the line that scan would have read had it been lowercase. The anchor
// is written here rather than shared: subcommands.go and ai/tools/stub_usage_test.go each hold their
// own too, and the comment above is what binds the four.
func unreadUsageSpelling(lines []string) string {
	const anchor = "usage: "
	spelling := ""
	for _, line := range leadingCommentBlock(lines) {
		trimmed := strings.TrimLeft(strings.TrimPrefix(line, "#"), " \t")
		if len(trimmed) < len(anchor) {
			continue
		}
		head := trimmed[:len(anchor)]
		if head == anchor {
			return ""
		}
		// The first capitalised line is the one reported, and the loop runs on: a lowercase line
		// under it still clears the check, which is what makes this a spelling finding rather than a
		// second way to say the header is missing one.
		if spelling == "" && strings.EqualFold(head, anchor) {
			spelling = strings.TrimSuffix(head, " ")
		}
	}
	return spelling
}

// Reading past the leading comment block would let a `-test.sh` named anywhere in the body clear the
// check. Bounded, so a header past that bound is reported rather than silently read as a declaration.
func leadingCommentBlock(lines []string) []string {
	var header []string
	for i, line := range lines {
		if i >= headerLineCap {
			break
		}
		if i == 0 && strings.HasPrefix(line, "#!") {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			break
		}
		header = append(header, line)
	}
	return header
}

func allMatches(lines []string, pattern *regexp.Regexp) []string {
	var matches []string
	for _, line := range lines {
		matches = append(matches, pattern.FindAllString(line, -1)...)
	}
	return matches
}

func anyMatch(lines []string, pattern *regexp.Regexp) bool {
	for _, line := range lines {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

// How much of a shared region is held for comparison. A region body is as attacker-controlled as the
// file it sits in, and an *unterminated* fence swallows the rest of that file.
const sharedRegionBodyCap = 256 << 10

// A block fenced `# --- shared:<name> ---` … `# --- end shared:<name> ---` must be byte-identical
// everywhere that name appears. Two scripts in different skills duplicate these on purpose: a shared
// file would make one skill's tooling depend on another's, and this runs inside a worktree of code it
// did not write, where sourcing a file is executing it. That tolerance holds only while drift is
// *detected* (ecosystem.md → **Prefer the mechanism**).
func (c *checker) scanSharedRegions() {
	copies := map[string][]regionBody{}
	for _, lines := range c.filesWithLines(c.root.Named(), "*.sh") {
		for name, region := range regionsIn(lines) {
			copies[name] = append(copies[name], region)
		}
	}
	for name, found := range copies {
		distinct := map[string]bool{}
		isOversize := false
		for _, region := range found {
			distinct[region.body] = true
			isOversize = isOversize || region.isOversize
		}
		// Three kinds, not one, and the region's name is the branch's own text. So each finding leads
		// with its kind and puts the name after it; report.go's rank table has the rest.
		//
		// The marker charset bounds what a name may contain and never how long it may be. So the name is
		// cut at findingNameCap, with the mark that says it cut. Uncut, the printer's 500-byte bound
		// reaches the line first, and the tail it takes is the detail saying what is wrong with the
		// region.
		named := shell.CutBytesMarked(name, findingNameCap)
		switch {
		case isOversize:
			c.add(sharedRegionNotChecked + named + " — it is too large to compare")
		case len(found) < 2:
			c.add(fmt.Sprintf(sharedRegionWithoutCounterpart+"%s — %d copy, and the marker names one no file carries", named, len(found)))
		case len(distinct) > 1:
			c.add(fmt.Sprintf(sharedRegionHasDrifted+"%s — %d copies, %d distinct versions", named, len(found), len(distinct)))
		}
	}
}

type regionBody struct {
	body       string
	isOversize bool
}

// One file's regions, keyed by name. A name opening twice in one file accumulates into one body, the
// way one pass over the file has to.
//
// Appended to, never concatenated. `body += line` re-copies everything gathered so far on every line,
// which is quadratic in a line count the reviewed tree picks — the same trade imports.go took when it
// dropped the shell version's scratch file. The cap below bounds the bytes held, not the copying done
// to reach them: one committed 300 KB region cost 13.2s of CPU against 0.15s for the same file with
// no markers in it, and nothing bounds how many regions or how many scripts a branch commits.
func regionsIn(lines []string) map[string]regionBody {
	bodies := map[string]*strings.Builder{}
	oversize := map[string]bool{}
	region := ""
	for _, line := range lines {
		if sharedRegionOpen.MatchString(line) {
			region = sharedRegionTail.ReplaceAllString(sharedRegionName.ReplaceAllString(line, ""), "")
			if _, seen := bodies[region]; !seen {
				bodies[region] = &strings.Builder{}
			}
			continue
		}
		if sharedRegionClose.MatchString(line) {
			region = ""
			continue
		}
		if region == "" {
			continue
		}
		body := bodies[region]
		// Past the cap the region is reported rather than compared: an unchecked region must never
		// read as a matching one.
		if body.Len() >= sharedRegionBodyCap {
			oversize[region] = true
			continue
		}
		body.WriteString(escapeRegionLine(line))
		body.WriteByte('\x01')
	}
	found := map[string]regionBody{}
	for name, body := range bodies {
		found[name] = regionBody{body: body.String(), isOversize: oversize[name]}
	}
	return found
}

// Escaping first is what makes the join reversible: an unescaped \001 planted in a region body would
// let two copies compare equal with a guard deleted from one of them.
func escapeRegionLine(line string) string {
	return strings.NewReplacer(`\`, `\\`, "\x01", `\001`, "\t", `\t`).Replace(line)
}
