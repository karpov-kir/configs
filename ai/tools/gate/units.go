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
	// library, which both installers source and which runs before the machine has a Go binary at all.
	// shell's suite holds the two spellings to each other, so it is keyed on the file it reads them
	// out of: key it on anything else and an edit to the awk leaves that suite fresh from cache.
	extAudience  = "lib/skill-audience.sh"
	extReduce    = "ai/kk-flavor/skills/kk-reduce/stats.md"
	extWorkflows = ".github/workflows"
	// The shared shell libraries every installer sources.
	// The stub scripts ai/tools/tool-stub-test.sh copies into fixtures and runs. copiedRepoFiles finds
	// only the one path that suite spells out literally; the other six live in its `stubs()` table,
	// which no text scan parses. Globbed at DISCOVERY, so what lands in `inputs` is concrete paths —
	// a pattern stored as an input would match nothing once git is asked with literal pathspecs.
	skillScripts = "ai/kk-flavor/skills/*/scripts/*.sh"
	// Where the skills keep the stub scripts that reach the Go tools. ai/tools/tool-stub-test.sh copies
	// the stubs in its own table into fixtures and executes them, so they are that suite's subject —
	// see the keying below for why naming the tree is not enough.
)

// The two directories eco-report's harness copies from: scripts/ for todo-gate.sh, templates/ for the
// report template. Directories rather than the two files, so a third thing copied in later is still
// keyed on — and not the whole skill, whose SKILL.md is prose no fixture reads.
var extQualify = []string{"ai/kk-flavor/skills/idsd-qualify/scripts", "ai/kk-flavor/skills/idsd-qualify/templates"}

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

// Copying is the other way a suite reaches a repository file, and it has to be keyed on too: edit the
// copied file and the unit still answers out of its cache. Only the operand right after `cp` counts,
// which leaves a destination inside the fixture unkeyed. A file is named by the literal tail after the
// variable, so a copy whose basename is itself a variable names nothing.
var copiedFileLine = regexp.MustCompile(`\bcp[ \t]+(?:-[-A-Za-z]+(?:=[^ \t\n]*)?[ \t]+)*"\$\{?[A-Za-z_][A-Za-z0-9_]*\}?/([^"$*?\n]+)"`)

// Anchoring the pattern above to the start of a line would be the tighter guard, but
// ai/mcp-sync-test.sh puts a real cp after a `case` arm.
var commentedLine = regexp.MustCompile(`^[ \t]*#`)

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
	gotestInputs = append(gotestInputs, extAudience, extReduce, extWorkflows)
	g.add("gotest", "check", gotestInputs, "@gotest")
	// --gate, because this unit's verdict has to be about the commit and nothing else. Without it the
	// check walks whatever sits on disk, gitignored files included, and two checkouts of one commit
	// disagree. `.gitignore` is an input BECAUSE of the flag: the rules decide which files the check
	// judges, so editing them moves this unit's verdict — and a verdict that moves without its key is
	// the stale green this whole thing exists not to serve. Blind to the module's test files for a
	// reason of its own: eco-check reads Go sources only to find subcommand dispatches, and skips
	// `_test.go` by name, because a test file's fixtures hold dispatch switches of their own.
	g.addBlindToGoTests("wiring", "check", []string{"ai/kk-flavor", "ai/tools", "lib", ".gitignore"},
		"ECO_TOOLS_BUILD=1 ai/kk-flavor/skills/kk-ecosystem/scripts/check.sh --gate")
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
		[]string{"ai/kk-flavor/skills", "ai/field-guide.html", "ai/tools/eco-guide", "ai/tools/eco-root", "ai/tools/shell", "ai/guide.sh"},
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
	repo, code := g.readRepoListing()
	if code != 0 {
		return code
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
			// The only input derived from the text of a file rather than from git's own listing, so the
			// only one that has not already been through here.
			if err := safeToken("copied path", file); err != nil {
				return g.fail("%s: %s", suite, err)
			}
			if !slices.Contains(inputs, file) {
				inputs = append(inputs, file)
			}
		}
		// The suites that drive a Go tool also take the tool tree, since a change there moves what they
		// observe. What they observe is a compiled binary, though, so the key drops the module's own
		// `_test.go` files — 66 of the 150 files these units were keyed on, none of which `go build`
		// puts in a binary.
		viaBinary := false
		if strings.Contains(body, "kk-flavor/skills") {
			matches, globErr := filepath.Glob(filepath.Join(g.root, skillScripts))
			if globErr == nil {
				for _, match := range matches {
					rel, relErr := filepath.Rel(g.root, match)
					if relErr != nil {
						continue
					}
					if err := safeToken("skill script", rel); err != nil {
						return g.fail("%s: %s", suite, err)
					}
					if !slices.Contains(inputs, rel) {
						inputs = append(inputs, rel)
					}
				}
			}
		}
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

// Everything one shell suite's verdict can turn on, and whether it observes a compiled Go binary
// rather than the module's sources — which is what decides that its key drops `_test.go`.
//
// Every rule here was a stale green: a file the suite reads, that no unit was keyed on, so an edit to
// it left the unit answering from cache.

// A file this tree is expected to hold, as text. Unreadable comes back empty, and every caller above
// reads that as "this rule does not apply" — a suite that cannot be read keys on nothing extra rather
// than taking the run down.
func (g *gate) readOrEmpty(rel string) string {
	body, err := os.ReadFile(filepath.Join(g.root, rel))
	if err != nil {
		return ""
	}
	return string(body)
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

// The repository files a suite copies into its fixture, plus the first copy naming a file the gate
// cannot resolve. Keyed whether the copy is run or only read: a regexp cannot tell those apart, and
// either way the file moves what the suite measures. ai/mcp-sync-test.sh copies ai/mcp.jsonc in and
// asserts every command it names is executable.
func (g *gate) copiedRepoFiles(repo repoListing, suiteDir string, bodies ...string) ([]string, string) {
	var copied []string
	for _, body := range bodies {
		for _, line := range strings.Split(body, "\n") {
			if commentedLine.MatchString(line) {
				continue
			}
			for _, match := range copiedFileLine.FindAllStringSubmatch(line, -1) {
				tail := match[1]
				files, unresolved := resolveCopiedPath(repo, suiteDir, tail)
				if unresolved == "" {
					copied = append(copied, files...)
					continue
				}
				return nil, tail + ", which " + unresolved
			}
		}
	}
	return shell.SortUnique(copied), ""
}

// Three tries, tightest first: the tail as a repository path, then under the directory the suite lives
// in, and only then the tracked files it is a suffix of. Each try returns every path it hit, not the
// first. Two paths matching means the gate cannot say which one the suite copies. Keying on both
// costs one file too many, and picking one could key the unit on the wrong file altogether.
func resolveCopiedPath(repo repoListing, suiteDir, tail string) ([]string, string) {
	candidates := copyCandidates(suiteDir, tail)
	if resolved := allMatching(candidates, func(c string) bool { return repo.byPath[c] }); len(resolved) > 0 {
		return resolved, ""
	}
	// A directory only where the tail spells one out, never through the suffix scan below: a tail
	// landing on some deep directory by accident would key the unit on everything under it.
	if resolved := allMatching(candidates, repo.holdsDirectory); len(resolved) > 0 {
		return resolved, ""
	}
	var matches []string
	for _, file := range repo.all {
		if strings.HasSuffix(file, "/"+tail) {
			matches = append(matches, file)
		}
	}
	if len(matches) > 0 {
		return matches, ""
	}
	// Nothing resolved, and the tail does not start at a top-level entry this repository has. So it points
	// inside the fixture, which the suite built itself and no edit here can move. Keyed on nothing rather
	// than refused, because refusing would let an ordinary line like
	// `cp "$fixture/config.json" "$other/config.json"` stop the whole gate for everyone.
	if !repo.topLevel[firstSegment(tail)] {
		return nil, ""
	}
	return nil, "names no file in this repository"
}

// Both spellings the suites use: `$checkout/ai/bootstrap-test.sh` is rooted at the repository, while
// `$script_dir/mcp-env.sh` names a sibling of the suite.
func copyCandidates(suiteDir, tail string) []string {
	if suiteDir == "" || suiteDir == "." {
		return []string{tail}
	}
	return []string{tail, path.Join(suiteDir, tail)}
}

func allMatching(candidates []string, holds func(string) bool) []string {
	var resolved []string
	for _, candidate := range candidates {
		if holds(candidate) {
			resolved = append(resolved, candidate)
		}
	}
	return resolved
}

func firstSegment(tail string) string {
	first, _, _ := strings.Cut(tail, "/")
	return first
}

// The repository's own files. `all` keeps the listing order the suffix scan walks, and `topLevel`
// separates a path this repository could hold from one that exists only inside a fixture.
type repoListing struct {
	all      []string
	byPath   map[string]bool
	topLevel map[string]bool
}

func (g *gate) readRepoListing() (repoListing, int) {
	all, err := g.listFiles(".")
	if err != nil || len(all) == 0 {
		return repoListing{}, g.fail("discovery could not list the repository's files, so it cannot " +
			"say which of them a suite copies into its fixture — nothing ran")
	}
	repo := repoListing{all: all, byPath: map[string]bool{}, topLevel: map[string]bool{}}
	for _, file := range all {
		repo.byPath[file] = true
		repo.topLevel[firstSegment(file)] = true
	}
	return repo, 0
}

func (r repoListing) holdsDirectory(candidate string) bool {
	prefix := candidate + "/"
	for _, file := range r.all {
		if strings.HasPrefix(file, prefix) {
			return true
		}
	}
	return false
}

// `-z` and `core.quotePath=false`, the rule ai/run-tests.sh lives by. Drop either and a name reaches
// the split below newline-separated or C-quoted, leaving a token safeToken refuses — the run exits 2
// blaming a name nothing is wrong with.
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
