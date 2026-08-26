package ecocheck

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

var (
	caseLabel         = regexp.MustCompilePOSIX(`^  [a-z][a-z0-9|_-]*\)$`)
	namedTestSuite    = regexp.MustCompilePOSIX(`[A-Za-z0-9_.-]+-test\.sh`)
	untestedDeclared  = regexp.MustCompilePOSIX(`^#[[:space:]]*untested:[[:space:]]*[^[:space:]]`)
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

// Skills reach their scripts by path (`scripts/report.sh …`), so a lost exec bit is a stage that
// cannot run at all. And a script is parsed under every bash `#!/usr/bin/env bash` could resolve to:
// macOS still ships 3.2 as /bin/bash, and it rejects constructs bash 5 accepts.
func (c *checker) scanScriptsParse() {
	scripts := c.filesNamed(c.root, "*.sh")
	for _, script := range scripts {
		if !isExecutable(script) {
			c.add("script not executable: " + oneline(script))
		}
	}
	// Two forks per script is the dominant cost of the whole check, and each is independent of
	// every other. Findings are sorted before they are printed, so running them out of order
	// changes no byte of the output.
	binaries := bashBinaries()
	results := make([]parseResult, len(scripts))
	work := make(chan int)
	var group sync.WaitGroup
	for range runtime.NumCPU() {
		group.Add(1)
		go func() {
			defer group.Done()
			for i := range work {
				results[i] = parseResult{script: scripts[i], findings: parseErrors(binaries, scripts[i])}
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

func parseErrors(binaries []string, script string) []string {
	var findings []string
	for _, binary := range binaries {
		command := exec.Command(binary, "-n", script)
		command.Env = append(os.Environ(), "LC_ALL=C")
		output, _ := command.CombinedOutput()
		// Every control byte but the newline goes: the message quotes the script's own text, and a
		// finding is one physical line. The newline stays because a parse error is reported over
		// several lines and each of them becomes its own finding.
		for i, b := range output {
			if b >= 0x01 && b <= 0x09 || b >= 0x0b && b <= 0x1f || b == 0x7f {
				output[i] = ' '
			}
		}
		for _, line := range splitLines(string(output)) {
			findings = append(findings, "syntax: "+line)
		}
	}
	return findings
}

// Both are run even when they resolve to the same file, as the shell version ran them: the
// duplicate findings collapse in the sort, and dropping one would silently stop checking the older
// bash on a machine where PATH happens to hold it.
func bashBinaries() []string {
	var found []string
	if path, err := exec.LookPath("bash"); err == nil {
		found = append(found, path)
	}
	if isExecutable("/bin/bash") {
		found = append(found, "/bin/bash")
	}
	return found
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode()&0o111 != 0
}

// Enforces ecosystem.md → **Prefer the mechanism**: every case label of a top-level
// `case "${1:-}" in` dispatch needs one `<script> <subcommand>` somewhere an agent reads. Labels are
// read at the case arm's own indentation, so a nested `done)` is not one.
func (c *checker) scanSubcommandCallSites() {
	type callSite struct{ script, subcommand string }
	var wanted []callSite
	queries := map[string]bool{}
	for _, script := range c.filesNamed(c.root, "*.sh") {
		lines, err := c.readLines(script)
		if err != nil {
			continue
		}
		base := baseName(script)
		for _, subcommand := range dispatchLabels(lines) {
			wanted = append(wanted, callSite{script: base, subcommand: subcommand})
			queries[base+" "+subcommand] = false
		}
	}
	if len(queries) == 0 {
		return
	}
	// One pass over the tree for every subcommand at once. The shell version walked the whole tree
	// per label, which is the shape that turns a script with many arms into many whole-tree walks.
	for _, file := range c.filesNamed(c.root, "*.md", "*.sh", "*.yaml") {
		body, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		text := string(body)
		for query, seen := range queries {
			if !seen && strings.Contains(text, query) {
				queries[query] = true
			}
		}
	}
	for _, site := range wanted {
		if !queries[site.script+" "+site.subcommand] {
			c.add(oneline(site.script) + " subcommand with no call site: " + oneline(site.subcommand))
		}
	}
}

func dispatchLabels(lines []string) []string {
	var labels []string
	inside := false
	for _, line := range lines {
		if strings.HasPrefix(line, `case "${1:-}" in`) {
			inside = true
			continue
		}
		if inside && strings.HasPrefix(line, "esac") {
			break
		}
		if !inside || !caseLabel.MatchString(line) {
			continue
		}
		trimmed := strings.NewReplacer(" ", "", ")", "").Replace(line)
		labels = append(labels, strings.Split(trimmed, "|")...)
	}
	return sortUnique(labels)
}

// Also ecosystem.md → **Prefer the mechanism**: a script is prose turned into enforcement, so
// something has to prove the enforcement still fires. Each script states its test position in its
// header, either the `-test.sh` covering it or `# untested: <why>`, and that header is what
// kk-reduce's Phase 6 reads to pick what to run. Stating neither hides the script from that phase;
// naming a suite that is not there is the worse half, because the phase finds nothing to run and the
// script counts as covered by a suite that does not exist.
func (c *checker) scanTestPositions() {
	// Built once, and from basenames the reviewed tree cannot forge: a filename holding a newline
	// would otherwise contribute its second line as a bare suite name, and a header naming a missing
	// suite would then pass the existence check.
	suites := map[string]bool{}
	for _, path := range c.filesNamed(c.root, "*-test.sh") {
		if name := baseName(path); isCleanBasename(name) {
			suites[name] = true
		}
	}
	for _, script := range c.filesNamed(c.root, "*.sh") {
		base := baseName(script)
		// The harness is exempt: asking a test file to name its own test makes every one a finding.
		if strings.HasSuffix(base, "-test.sh") || strings.HasSuffix(base, "-mutate.sh") {
			continue
		}
		lines, err := c.readLines(script)
		if err != nil {
			continue
		}
		header := leadingCommentBlock(lines)
		named := sortUnique(allMatches(header, namedTestSuite))
		// The count is of the 200-line window, never of the file: past that bound nothing was read,
		// so a header carrying thousands of names would report the window's total as the file's.
		if len(named) > 8 {
			c.add(fmt.Sprintf("script names more suites than the scan reads: %s names %d in its first 200 lines, of which 8 are read",
				oneline(base), len(named)))
		}
		if len(named) > 8 {
			named = named[:8]
		}
		if len(named) > 0 {
			for _, suite := range named {
				if !suites[suite] {
					c.add("script names a missing test: " + oneline(base) + " names " + oneline(suite))
				}
			}
			continue
		}
		if !anyMatch(header, untestedDeclared) {
			c.add("script declares no test position: " + oneline(base) +
				" names no -test.sh and carries no '# untested: <why>'")
		}
	}
}

// Reading past the leading comment block would let a `-test.sh` named anywhere in the body clear the
// check. Bounded at 200 lines, so a header past that bound is reported rather than silently read as
// a declaration.
func leadingCommentBlock(lines []string) []string {
	var header []string
	for i, line := range lines {
		if i >= 200 {
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

// A block fenced `# --- shared:<name> ---` … `# --- end shared:<name> ---` must be byte-identical
// everywhere that name appears. Two scripts in different skills duplicate these on purpose — a
// shared file would make one skill's tooling depend on another's, and this runs inside a worktree of
// code it did not write, where sourcing a file is executing it. That tolerance holds only while
// drift is *detected* (ecosystem.md → **Prefer the mechanism**).
func (c *checker) scanSharedRegions() {
	copies := map[string][]regionBody{}
	for _, script := range c.filesNamed(c.root, "*.sh") {
		lines, err := c.readLines(script)
		if err != nil {
			continue
		}
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
		switch {
		case isOversize:
			c.add("shared region " + name + " is too large to compare — it was NOT checked for drift")
		case len(found) < 2:
			c.add(fmt.Sprintf("shared region %s has %d copy — the marker names a counterpart no file carries", name, len(found)))
		case len(distinct) > 1:
			c.add(fmt.Sprintf("shared region %s has drifted: %d copies, %d distinct versions", name, len(found), len(distinct)))
		}
	}
}

type regionBody struct {
	body       string
	isOversize bool
}

// One file's regions, keyed by name. A name opening twice in one file accumulates into one body, the
// way one pass over the file has to.
func regionsIn(lines []string) map[string]regionBody {
	found := map[string]regionBody{}
	region := ""
	for _, line := range lines {
		if sharedRegionOpen.MatchString(line) {
			region = sharedRegionTail.ReplaceAllString(sharedRegionName.ReplaceAllString(line, ""), "")
			if _, seen := found[region]; !seen {
				found[region] = regionBody{}
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
		current := found[region]
		// Bounded: a region body is as attacker-controlled as the file it sits in, and an
		// *unterminated* fence swallows the rest of the file. Past the cap the region is reported
		// rather than compared — an unchecked region must never read as a matching one.
		if len(current.body) >= 262144 {
			current.isOversize = true
			found[region] = current
			continue
		}
		current.body += escapeRegionLine(line) + "\x01"
		found[region] = current
	}
	return found
}

// Escaping first is what makes the join reversible: an unescaped \001 planted in a region body would
// let two copies compare equal with a guard deleted from one of them.
func escapeRegionLine(line string) string {
	return strings.NewReplacer(`\`, `\\`, "\x01", `\001`, "\t", `\t`).Replace(line)
}
