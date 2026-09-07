package gate

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"kk-flavor/tools/shell"
)

// The files the Go suites read from outside their own module. Go's test cache is keyed on the module
// and cannot see them, so a plain `go test` reports a cached pass over a changed template; the gate
// keys on them itself. Named file by file, from the suites' own `../../` constants: keying on all of
// kk-flavor made editing tree-fingerprint.sh force eco-report — 233s for a package that cannot read it.
const (
	goTree    = "ai/tools"
	extFlavor = "ai/kk-flavor/scripts/tree-fingerprint.sh"
	// The audience marker is read twice — as a Go regexp in shell/markdown.go, and as awk in this
	// script, which runs before the machine has a Go binary at all. shell's suite holds the two
	// spellings to each other, so it is keyed on the script it reads them out of.
	extBootstrap = "ai/bootstrap.sh"
	extReduce    = "ai/skills/kk-reduce/stats.md"
	extWorkflows = ".github/workflows"
)

// The two directories eco-report's harness copies from: scripts/ for todo-gate.sh, templates/ for the
// report template. Directories rather than the two files, so a third thing copied in later is still
// keyed on — and not the whole skill, whose SKILL.md is prose no fixture reads.
var extQualify = []string{"ai/skills/idsd-qualify/scripts", "ai/skills/idsd-qualify/templates"}

// A suite that runs the Go module's own suites, rather than only a binary built from it. `go test` and
// `go vet` both compile `_test.go`, so a suite reaching for either sees those files and must stay
// keyed on them.
var goSuiteRun = regexp.MustCompile(`\bgo (test|vet)\b`)

// Keyed on, or an edit under lib/ moves what the suite measures while the suite and its script sit
// still. The two prefixes written here, `$repo/../lib/` and `$checkout/lib/`, both land on the
// repository's own lib/.
var sourcedLibLine = regexp.MustCompile(`(?m)^[ \t]*\.[ \t]+"[^"]*/lib/([^"/]+\.sh)"`)

// One shell-word wider, to refuse what the pattern above cannot key on rather than key on nothing.
// Neither reads a source line buried mid-line, nor a library some third file sources.
var anySourcedLibLine = regexp.MustCompile(`(?m)^[ \t]*(?:\.|source)[ \t]+[^\n]*?\blib/([A-Za-z0-9._-]+\.sh)`)

// Copying is the other way a suite reaches a repository file: env/bootstrap-test.sh copies
// ai/bootstrap-test.sh into a fixture and runs it there, so deleting that file's harness guard is
// meant to redden this suite. Keyed on, or the edit lands and the unit answers from its cache record.
// Anchored to the operand right after `cp`, so a destination inside the fixture keys nothing. What
// names a file is the literal tail after the variable; a copy whose basename is itself a variable —
// and a whole fixture tree held in one — names none, and stays outside this scan the way a sourced
// library held in a variable stays outside the two above.
var copiedFileLine = regexp.MustCompile(`\bcp[ \t]+(?:-[-A-Za-z]+[ \t]+)*"\$\{?[A-Za-z_][A-Za-z0-9_]*\}?/([^"$*?\n]+)"`)

// The repository's own files, held both ways discovery reads them: as a set, for a rooted lookup, and
// in listing order, for the suffix scan.
type repoListing struct {
	all []string
	set map[string]bool
}

func (g *gate) add(id, kind string, inputs []string, cmd string) {
	g.units = append(g.units, unit{id: id, kind: kind, inputs: inputs, cmd: cmd})
}

func (g *gate) addBlindToGoTests(id, kind string, inputs []string, cmd string) {
	g.units = append(g.units, unit{id: id, kind: kind, inputs: inputs, cmd: cmd, blindToGoTests: true})
}

func (g *gate) buildUnits() int {
	if g.env.UnitsFile != "" {
		return g.unitsFromFile()
	}
	return g.discoverUnits()
}

func (g *gate) unitsFromFile() int {
	file, err := os.Open(g.env.UnitsFile)
	if err != nil {
		return g.fail("GATE_UNITS_FILE names %s, which is not a file — nothing ran", g.env.UnitsFile)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), "\t", 4)
		if len(fields) < 4 || fields[0] == "" {
			continue
		}
		g.add(fields[0], fields[1], strings.Fields(fields[2]), fields[3])
	}
	return 0
}

func (g *gate) addGoChecks() {
	g.add("gofmt", "check", []string{goTree}, "@gofmt")
	g.add("vet", "check", []string{goTree}, "cd ai/tools && go vet ./...")
	gotestInputs := append([]string{goTree, extFlavor}, extQualify...)
	gotestInputs = append(gotestInputs, extBootstrap, extReduce, extWorkflows)
	g.add("gotest", "check", gotestInputs, "@gotest")
	// --gate, because this unit's verdict has to be about the commit and nothing else. Without it the
	// check walks whatever sits on disk, gitignored files included, and two checkouts of one commit
	// disagree. `.gitignore` is an input BECAUSE of the flag: the rules decide which files the check
	// judges, so editing them moves this unit's verdict — and a verdict that moves without its key is
	// the stale green this whole thing exists not to serve. Blind to the module's test files for a
	// reason of its own: eco-check reads Go sources only to find subcommand dispatches, and skips
	// `_test.go` by name, because a test file's fixtures hold dispatch switches of their own.
	g.addBlindToGoTests("wiring", "check", []string{"ai/skills", "ai/kk-flavor", "ai/tools", ".gitignore"},
		"ECO_TOOLS_BUILD=1 ai/skills/kk-ecosystem/scripts/check.sh --gate")
}

// The field guide is generated, so the committed page can fall behind the skills without anyone
// touching it — a skill added, renamed or retired is enough. This unit regenerates into memory and
// diffs, which is the only thing that notices.
//
// Keyed on the skills because their frontmatter IS the inventory, on the tool and the two packages it
// reads that frontmatter through, and on the page itself so hand-editing the committed file re-runs
// the check that would catch it. Blind to the module's test files, like every other unit that
// observes a compiled binary. ECO_TOOLS_BUILD=1 for the reason the wiring unit sets it: a gate has to
// measure the source in this tree, never a binary that came from somewhere else.
func (g *gate) addGuideCheck() {
	g.addBlindToGoTests("guide", "check",
		[]string{"ai/skills", "ai/field-guide.html", "ai/tools/eco-guide", "ai/tools/eco-root", "ai/tools/shell", "ai/guide.sh"},
		"ECO_TOOLS_BUILD=1 ai/guide.sh --check")
}

func (g *gate) discoverUnits() int {
	g.addGoChecks()
	g.addGuideCheck()

	if code := g.discoverShellSuites(); code != 0 {
		return code
	}
	return g.discoverGoMutants()
}

func (g *gate) discoverShellSuites() int {
	listed, err := g.listFiles("*-test.sh")
	if err != nil || len(listed) == 0 {
		return g.fail("discovery found no *-test.sh at all — read this as the gate broken, never as a clean run")
	}
	tree, err := g.listFiles(".")
	if err != nil || len(tree) == 0 {
		return g.fail("discovery could not list the repository's files, so it cannot say which of them a suite copies into its fixture — nothing ran")
	}
	repo := repoListing{all: tree, set: map[string]bool{}}
	for _, file := range tree {
		repo.set[file] = true
	}
	suites := shell.SortUnique(listed)
	for _, suite := range suites {
		if err := safeToken("suite", suite); err != nil {
			return g.fail("%s", err)
		}
		// A suite's inputs are itself, the script it covers, and ai/run-tests.sh. That last one because
		// it decides what the suite's exit status and summary line MEAN, so a change to it can flip this
		// unit's verdict with neither the suite nor its script moving a byte.
		inputs := []string{suite, "ai/run-tests.sh"}
		sibling := strings.TrimSuffix(suite, "-test.sh") + ".sh"
		siblingPath := filepath.Join(g.root, sibling)
		if _, err := os.Stat(siblingPath); err == nil {
			inputs = append(inputs, sibling)
		}
		body := fileText(filepath.Join(g.root, suite))
		siblingBody := fileText(siblingPath)
		libs := sourcedLibs(body, siblingBody)
		if missed := unreadLib(libs, body, siblingBody); missed != "" {
			return g.fail("%s or the script it covers sources %s in a form the input scan does not "+
				"read, so the unit would run that library without being keyed on it — nothing ran",
				suite, missed)
		}
		inputs = append(inputs, libs...)
		copied, unresolved := g.copiedRepoFiles(repo, path.Dir(suite), body, siblingBody)
		if unresolved != "" {
			return g.fail("%s or the script it covers copies %s, so the unit would run a file the "+
				"gate cannot key on — nothing ran", suite, unresolved)
		}
		for _, file := range copied {
			if !slices.Contains(inputs, file) {
				inputs = append(inputs, file)
			}
		}
		// The suites that drive a Go tool also take the tool tree, since a change there moves what they
		// observe. What they observe is a compiled binary, though, so the key drops the module's own
		// `_test.go` files — 66 of the 150 files these units were keyed on, none of which `go build`
		// puts in a binary.
		viaBinary := false
		if drivesGoTool(body) {
			inputs = append(inputs, goTree)
			viaBinary = true
		}
		if goSuiteRun.MatchString(body) {
			viaBinary = false
		}
		// Through run-tests.sh, never `bash $suite`: that file owns the reading of a suite's result — exit 2
		// is "did not measure", and a suite exiting 0 having run no case is VACUOUS and a failure. Run
		// directly, a suite emptied to zero bytes exits 0 silently and reads as `ran ok`. Keyed on the
		// suite's path, not its basename: two `bootstrap-test.sh` under one id share one cache record.
		name := strings.TrimSuffix(suite, "-test.sh")
		addUnit := g.add
		if viaBinary {
			addUnit = g.addBlindToGoTests
		}
		addUnit("shell:"+name, "check", inputs, "ai/run-tests.sh -s "+shellQuote(suite))
	}
	return 0
}

func (g *gate) discoverGoMutants() int {
	g.goMutateBinary = "ai/tools/go-mutate/go-mutate"
	build := exec.Command("go", "build", "-o", "go-mutate/go-mutate", "./go-mutate")
	build.Dir = filepath.Join(g.root, "ai", "tools")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintln(g.errOut, "gate.sh: go-mutate does not build, so its units cannot be listed and its verdicts cannot be trusted — nothing ran")
		g.errOut.Write(out)
		return 2
	}
	listing, err := g.capture(filepath.Join(g.root, g.goMutateBinary), "-units")
	if err != nil {
		return g.fail("go-mutate could not list its units — nothing ran")
	}
	if strings.TrimSpace(listing) == "" {
		return g.fail("go-mutate listed no units — read this as the harness broken, never as nothing to check")
	}
	for _, line := range strings.Split(listing, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 4 || fields[0] == "" {
			continue
		}
		file, suites, resolved := fields[0], fields[1], fields[3]
		if err := safeToken("mutant file", file); err != nil {
			return g.fail("%s", err)
		}
		if resolved == "" {
			return g.fail("the mutation harness listed %s with no resolved path, so the gate cannot say which file the unit is keyed on — nothing ran", file)
		}
		target := strings.TrimPrefix(strings.TrimPrefix(resolved, g.root), "/")
		inputs := []string{target, "ai/tools/go-mutate"}
		for _, suite := range strings.Split(suites, ",") {
			if suite == "" {
				continue
			}
			dir := "ai/tools/" + strings.TrimPrefix(suite, "./")
			inputs = append(inputs, strings.TrimSuffix(dir, "/"))
		}
		// Keyed on the package-qualified path, never the basename: eco-check and eco-report both hold a
		// shell.go, and two units under one id would share one cache record.
		id := "mutants:go:" + strings.TrimPrefix(target, "ai/tools/")
		g.add(id, "mutation", inputs, g.goMutateBinary+" -file "+shellQuote(file))
	}
	return 0
}

func drivesGoTool(body string) bool {
	for _, marker := range []string{"tools/", "resolve.sh", "eco-check", "eco-report", "eco-stats", "cite-graph", "rule-echo", "ECO_TOOLS"} {
		if strings.Contains(body, marker) {
			return true
		}
	}
	return false
}

func unreadLib(keyed []string, bodies ...string) string {
	for _, body := range bodies {
		for _, match := range anySourcedLibLine.FindAllStringSubmatch(body, -1) {
			if lib := "lib/" + match[1]; !slices.Contains(keyed, lib) {
				return lib
			}
		}
	}
	return ""
}

// The repository files a suite copies into its fixture, and the first copy naming one the gate cannot
// resolve to a single path. Keyed whether the copy is then run or only read: a regexp cannot tell the
// two apart, and both move what the suite measures — ai/mcp-sync-test.sh copies ai/mcp.jsonc in and
// asserts that every command it names is executable.
func (g *gate) copiedRepoFiles(repo repoListing, suiteDir string, bodies ...string) ([]string, string) {
	var copied []string
	for _, body := range bodies {
		for _, match := range copiedFileLine.FindAllStringSubmatch(body, -1) {
			tail := match[1]
			file, unresolved := resolveCopiedPath(repo, suiteDir, tail)
			if unresolved == "" {
				copied = append(copied, file)
				continue
			}
			// A path git is told to ignore is build output, not a repository file: the source tree that
			// builds it already keys the unit, and hashing the artifact would key the unit on whatever
			// the last build left behind. Asked of git rather than of the disk, so an unbuilt checkout
			// answers the same as a built one.
			if g.isIgnored(suiteDir, tail) {
				continue
			}
			return nil, tail + ", which " + unresolved
		}
	}
	return shell.SortUnique(copied), ""
}

// Rooted before loose: the tail as a repository path, then under the directory the suite lives in,
// and only then the one listed file it is a suffix of. Loose last, because a second file sharing the
// tail leaves nothing to break the tie — which is a refusal rather than a guess.
func resolveCopiedPath(repo repoListing, suiteDir, tail string) (string, string) {
	for _, candidate := range copyCandidates(suiteDir, tail) {
		if repo.set[candidate] {
			return candidate, ""
		}
	}
	var matches []string
	for _, file := range repo.all {
		if strings.HasSuffix(file, "/"+tail) {
			matches = append(matches, file)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], ""
	case 0:
		return "", "names no file in this repository"
	default:
		return "", fmt.Sprintf("names %d files in this repository", len(matches))
	}
}

// Both spellings the suites use: `$checkout/ai/bootstrap-test.sh` is rooted at the repository, while
// `$script_dir/mcp-env.sh` names a sibling of the suite.
func copyCandidates(suiteDir, tail string) []string {
	if suiteDir == "" || suiteDir == "." {
		return []string{tail}
	}
	return []string{tail, path.Join(suiteDir, tail)}
}

func (g *gate) isIgnored(suiteDir, tail string) bool {
	for _, candidate := range copyCandidates(suiteDir, tail) {
		if _, err := g.capture("git", "check-ignore", "-q", "--", candidate); err == nil {
			return true
		}
	}
	return false
}

// `-z` and `core.quotePath=false`, the rule ai/run-tests.sh lives by. Without `-z` a name holding a
// space arrives as two tokens: `strings.Fields` splits it, safeToken accepts both halves, and the
// gate builds two units keyed on files that do not exist while the real suite is gated by nothing.
// Without quotePath a non-ASCII name arrives C-quoted and takes the run to exit 2 blaming the name.
func (g *gate) listFiles(pathspec string) ([]string, error) {
	out, err := g.capture("git", "-c", "core.quotePath=false", "ls-files", "-z",
		"--cached", "--others", "--exclude-standard", "--", pathspec)
	if err != nil {
		return nil, err
	}
	var listed []string
	for _, name := range strings.Split(out, "\x00") {
		if name != "" {
			listed = append(listed, name)
		}
	}
	return listed, nil
}

func sourcedLibs(bodies ...string) []string {
	var libs []string
	for _, body := range bodies {
		for _, match := range sourcedLibLine.FindAllStringSubmatch(body, -1) {
			libs = append(libs, "lib/"+match[1])
		}
	}
	return shell.SortUnique(libs)
}

// Empty rather than a refusal where the file cannot be read: the unit stays keyed on the suite either
// way, and run-tests.sh is what reports a suite it cannot run.
func fileText(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(body)
}

// Single quotes, the one form a POSIX shell reads literally throughout. Written out rather than
// assumed safe: safeToken and the quoting are two defences, and an injection needs both to fail.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
