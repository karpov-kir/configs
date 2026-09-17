package gate

import (
	"bufio"
	"os"
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
	goTree = "ai/tools"
	// The main package `ai/guide.sh --check` builds and runs — resolve.sh picks `./cmd/<tool>/` first.
	ecoGuideCommand = goTree + "/cmd/eco-guide"
	extFlavor       = "ai/kk-flavor/scripts/tree-fingerprint.sh"
	// The audience marker is read twice — as a Go regexp in shell/markdown.go, and as awk in this
	// library, which both installers source and which runs before the machine has a Go binary at all.
	// shell's suite holds the two spellings to each other, so it is keyed on the file it reads them
	// out of: key it on anything else and an edit to the awk leaves that suite fresh from cache.
	extAudience  = "lib/skill-audience.sh"
	extReduce    = "ai/kk-flavor/skills/kk-reduce/stats.md"
	extWorkflows = ".github/workflows"
	extModels    = "ai/kk-flavor/models.json"
)

// The marker opening the shared stub region. It is what actually defines this input set:
// `ai/tools/tool-stub-test.sh` enumerates every tracked `.sh` carrying it and reads the offset out of
// each, so keying the unit on anything narrower leaves the suite measuring files the unit has stopped
// watching — and the drift it exists to catch then arrives as a green from cache.
//
// Derived rather than globbed for that reason. A glob list has to be edited whenever a stub moves
// between directories, and the edit that is forgotten is silent in exactly the direction that hurts:
// the suite still covers the file, the cache no longer does. This change's own move of
// `dup-literals.sh` out of `skills/*/scripts/` is the case in point, and a list would still be
// missing the three under `kk-flavor/scripts/` and the two at `ai/`'s own root.
const stubRegionMarker = "# --- shared:tool-stub ---"

// The two directories eco-report's harness copies from: scripts/ for todo-gate.sh, templates/ for the
// report template. Directories rather than the two files, so a third thing copied in later is still
// keyed on — and not the whole skill, whose SKILL.md is prose no fixture reads.
var extQualify = []string{"ai/kk-flavor/skills/idsd-qualify/scripts", "ai/kk-flavor/skills/idsd-qualify/templates"}

// The stubs `stub_usage_test.go` opens, to hold each one's documented usage line against the line its
// binary prints. Every stub, because that suite discovers them rather than naming three. The paths
// rather than the directories holding them: it reads these files and nothing else out of those trees.
// Without them the drift check is a check that cannot fire — a stub-header edit leaves this unit fresh,
// and Go's own cache answers over a file outside the module — which is the shape the block above exists
// to close.
//
// The cost is real and deliberate: gotest is the slowest unit here, and a comment-only edit in any of
// these sixteen files now re-runs the whole Go suite. The alternative is a key narrower than what the
// suite reads, and that one answers green over a stub nothing looked at.
var extStubs = []string{
	"ai/gate.sh",
	"ai/guide.sh",
	"ai/kk-flavor/scripts/bloat-judge.sh",
	"ai/kk-flavor/scripts/model-check.sh",
	"ai/kk-flavor/scripts/model-policy.sh",
	"ai/kk-flavor/scripts/repo-key.sh",
	"ai/kk-flavor/scripts/tree-fingerprint.sh",
	"ai/kk-flavor/skills/idsd-qualify/scripts/report.sh",
	"ai/kk-flavor/skills/idsd-ship/scripts/cadence.sh",
	"ai/kk-flavor/skills/kk-ecosystem/scripts/check.sh",
	"ai/kk-flavor/skills/kk-ecosystem/scripts/cite-graph.sh",
	"ai/kk-flavor/skills/kk-ecosystem/scripts/ruleecho.sh",
	"ai/kk-flavor/skills/kk-edit/scripts/comment-density.sh",
	"ai/kk-flavor/skills/kk-handoff/scripts/handoff-check.sh",
	"ai/kk-flavor/skills/kk-reduce/scripts/stats.sh",
	"ai/kk-flavor/skills/kk-refactor/scripts/dup-literals.sh",
}

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

// A line that runs nothing. Matches only a line that STARTS with `#`, so a marker after a real
// command still counts. Read by drivesGoTool below and by copiedRepoFiles in copies.go.
var commentedLine = regexp.MustCompile(`^[ \t]*#`)

func (g *gate) add(id, kind string, inputs []string, cmd string) {
	g.addUnit(unit{id: id, kind: kind, inputs: inputs, cmd: cmd})
}

func (g *gate) addBlindToGoTests(id, kind string, inputs []string, cmd string) {
	g.addUnit(unit{id: id, kind: kind, inputs: inputs, cmd: cmd, blindToGoTests: true})
}

// The one place a unit is built, so the one place its declared inputs are made a set. Six append
// sites below can reach one file — a sibling script is often also the library it sources, or a file
// the suite copies into its fixture — and a path listed twice made `--units` report a count `--why`
// disagreed with. Guarding at the site instead is what let the other sites collide unnoticed.
//
// Deduping moves no key. `linesUnder` walks the manifest and takes each line at most once however
// many times a path is declared, then sorts what it takes, so neither a duplicate nor the order this
// leaves behind ever reached a key. Nothing indexes a unit's inputs.
func (g *gate) addUnit(u unit) {
	u.inputs = shell.SortUnique(u.inputs)
	g.units = append(g.units, u)
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
	gotestInputs = append(gotestInputs, extAudience, extReduce, extWorkflows, extModels)
	gotestInputs = append(gotestInputs, extStubs...)
	g.add("gotest", "check", gotestInputs, "@gotest")
	// --gate, because this unit's verdict has to be about the commit and nothing else. Without it the
	// check walks whatever sits on disk, gitignored files included, and two checkouts of one commit
	// disagree. `.gitignore` is an input BECAUSE of the flag: the rules decide which files the check
	// judges, so editing them moves this unit's verdict — and a verdict that moves without its key is
	// the stale green this whole thing exists not to serve. Blind to the module's test files for a
	// reason of its own: eco-check reads Go sources only to find subcommand dispatches, and skips
	// `_test.go` by name, because a test file's fixtures hold dispatch switches of their own.
	g.addBlindToGoTests("wiring", "check", []string{"ai/kk-flavor", "ai/tools", "lib", ".gitignore"},
		"ECO_TOOLS_BUILD=1 ai/kk-flavor/skills/kk-ecosystem/scripts/check.sh --agent=claude --gate && ECO_TOOLS_BUILD=1 ai/kk-flavor/skills/kk-ecosystem/scripts/check.sh --agent=codex --gate")
}

// The field guide is generated, so the committed page can fall behind the skills without anyone
// touching it — a skill added, renamed or retired is enough. This unit regenerates into memory and
// diffs, which is the only thing that notices.
//
// Keyed on the skills because their frontmatter IS the inventory, on the page itself so hand-editing
// the committed file re-runs the check that would catch it, on the stub, and on every package the
// command is built from. Those packages come out of the import graph rather than a list written here:
// hand-listed, this unit named eco-guide, eco-root and shell but not `cmd/eco-guide` — the main
// package the binary is built from — so editing main.go left the verdict fresh over a tool nothing
// rebuilt.
//
// The graph answers what a `go test` compiles, a superset for a binary: a test-only import would key
// this on a package the build never reads. Wide is the safe direction; narrow is the defect.
//
// `extModels` is seeded by hand because no import graph reaches it: the page prints the tier each
// dispatch buys, resolved through the policy's own resolver, so the file is read at run time and
// imported by nothing. Leave it out and editing a row changes the page the generator would write
// while this unit answers from cache.
//
// Blind to the module's test files, like every other unit that observes a compiled binary.
// ECO_TOOLS_BUILD=1 for the reason the wiring unit sets it: a gate has to measure the source in this
// tree, never a binary that came from somewhere else.
func (g *gate) addGuideCheck(imports map[string][]string) int {
	compiles, known := imports[ecoGuideCommand]
	if !known {
		return g.fail("the gate cannot say which packages %s imports, so the guide unit would be keyed "+
			"on less than `ai/guide.sh --check` builds — nothing ran", ecoGuideCommand)
	}
	inputs := append([]string{"ai/kk-flavor/skills", "ai/field-guide.html", "ai/guide.sh", extModels}, compiles...)
	g.addBlindToGoTests("guide", "check", inputs, "ECO_TOOLS_BUILD=1 ai/guide.sh --check")
	return 0
}

// The import graph is read once here, for the `guide` unit that keys on it.
func (g *gate) discoverUnits() int {
	packages, err := g.listModulePackages()
	if err != nil {
		return g.fail("%s", err)
	}
	imports, err := moduleImports(packages, g.root)
	if err != nil {
		return g.fail("%s", err)
	}
	if code := g.addChecks(imports); code != 0 {
		return code
	}
	return g.discoverShellSuites()
}

// The units that are not discovered from the tree. One list, because the suite counting units has to
// register the same set before it counts the discovered ones, and a check added to only one of two
// places leaves that control measuring a total it cannot attribute.
func (g *gate) addChecks(imports map[string][]string) int {
	g.addGoChecks()
	return g.addGuideCheck(imports)
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
		switch suite {
		case "ai/bootstrap-test.sh":
			inputs = append(inputs, "ai/bootstrap-owner.sh", "ai/owner-instructions.md")
		case "ai/rtk-bootstrap-test.sh":
			sibling = "ai/bootstrap.sh"
			inputs = append(inputs, "ai/owner-instructions.md")
		// No ai/run-tests-concurrency.sh exists; this suite covers ai/run-tests.sh. Being an input keys
		// the unit on that file; naming it the sibling is what gets its text SCANNED.
		case "ai/run-tests-concurrency-test.sh":
			sibling = "ai/run-tests.sh"
		case "ai/install-project-test.sh", "ai/project-skills-test.sh":
			sibling = "ai/install-project.sh"
			inputs = append(inputs, "ai/project-skills.sh", "ai/project-dependencies.sh",
				"ai/project-mcp.sh", "ai/project-mcp.mjs", "ai/mcp.jsonc", "ai/mcp-env.sh")
		case "ai/project-mcp-test.sh":
			inputs = append(inputs, "ai/project-mcp.mjs", "ai/mcp.jsonc", "ai/mcp-env.sh")
		}
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
			inputs = append(inputs, file)
		}
		// The suites that drive a Go tool also take the tool tree, since a change there moves what they
		// observe. What they observe is a compiled binary, though, so the key drops the module's own
		// `_test.go` files — `go build` puts none of them in one.
		viaBinary := false
		if strings.Contains(body, "kk-flavor/skills") || strings.Contains(body, "kk-flavor/workers") {
			stubs, stubErr := g.stubScripts()
			if stubErr != nil {
				return g.fail("%s: cannot list the stub scripts it copies: %s", suite, stubErr)
			}
			for _, stub := range stubs {
				if err := safeToken("stub script", stub); err != nil {
					return g.fail("%s: %s", suite, err)
				}
				inputs = append(inputs, stub)
			}
		}
		// Three ways into one input: a suite inside the tool tree, one that compiles the module, one
		// whose text names a tool. The second also decides the blindness rather than only clearing it —
		// what it observes is the test files, not a binary built without them.
		//
		// Only the third is a scan, and only the third a suite may answer for itself. The first two are
		// facts about where the suite lives and what it runs, so a declaration beside either is a suite
		// contradicting its own text, and the gate refuses rather than picking one.
		runsGoSuites := goSuiteRun.MatchString(body) || goSuiteRun.MatchString(siblingBody)
		inTree := strings.HasPrefix(suite, goTree+"/")
		declaredNone, malformed := declaresNoGoTool(body)
		switch {
		case malformed:
			return g.fail("%s declares `# go-tools: none` with no reason after it. The reason is what a "+
				"later reader checks the declaration against, and without one the line narrows a key on "+
				"nobody's word — nothing ran", suite)
		case declaredNone && inTree:
			return g.fail("%s declares `# go-tools: none` and lives inside %s, where every file is one "+
				"it could read. Delete the declaration or move the suite — nothing ran", suite, goTree)
		case declaredNone && runsGoSuites:
			return g.fail("%s declares `# go-tools: none` and runs `go test` or `go vet`, which compiles "+
				"the module it says it does not reach. Delete the declaration — nothing ran", suite)
		}
		if inTree || runsGoSuites || (!declaredNone && drivesGoTool(body, siblingBody)) {
			inputs = append(inputs, goTree)
			viaBinary = !runsGoSuites
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

// What a suite or its script says when it reaches into the Go tool tree. `ai/tools/` and not a bare
// `tools/`: the loose form also matches `$tmp_real/tools/mise`, a fake mise shim two suites write
// into their own temp dir and put on PATH.
//
// `tools/install.sh` is spelt out because ai/bootstrap.sh reaches the installer through a variable —
// `"$repo/tools/install.sh"` — which `ai/tools/` never matches. Without it shell:ai/rtk-bootstrap,
// whose own suite names no tool at all, is keyed only through the diagnostic strings that quote the
// path.
var goToolMarkers = []string{goTree + "/", "tools/install.sh", "resolve.sh", "eco-check", "eco-report", "eco-stats", "cite-graph", "rule-echo", "ECO_TOOLS"}

// Both the suite and the script it covers, because either can be the one that runs the tool. Read one
// and not the other and two units over the same script get opposite answers.
//
// Comment lines do not count: a marker in prose runs nothing, and a header sentence naming a Go suite
// would take the whole tree on the strength of it.
//
// Silence here is an answer, not a scan that failed, so there is no refusal to fall back on the way
// unreadLib has one — which is why the units where a wrong answer costs most are decided by path.
func drivesGoTool(bodies ...string) bool {
	for _, body := range bodies {
		for _, line := range strings.Split(body, "\n") {
			if commentedLine.MatchString(line) {
				continue
			}
			for _, marker := range goToolMarkers {
				if strings.Contains(line, marker) {
					return true
				}
			}
		}
	}
	return false
}

// A suite's own answer to the scan below: the tool paths in its text go into fixtures it builds, never
// into the checkout's tree. `ai/bootstrap-test.sh` is the case — it writes a stub `ai/tools/install.sh`
// into a temp repo and runs its subject from copies, so no Go source it could name is one it can
// observe. Nothing in a suite's text separates that from a path into the real tree, which is why the
// suite says so itself.
//
// Silence is the scan's answer, not this one: a suite with no declaration keys on the whole tree exactly
// as before. The declaration only ever narrows, so it carries the reason a later reader checks it
// against, and a line without one is refused rather than honoured.
var goToolsNone = regexp.MustCompile(`(?m)^[ \t]*#[ \t]*go-tools:[ \t]*none[ \t]*(.*)$`)

// Whether the suite declared it, and whether the line is one the gate refuses. The reason itself is
// read by people and not by this: what the gate holds is that one was given.
func declaresNoGoTool(body string) (declared, malformed bool) {
	match := goToolsNone.FindStringSubmatch(body)
	if match == nil {
		return false, false
	}
	return true, strings.TrimSpace(strings.TrimLeft(match[1], " \t-\u2014:")) == ""
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

// Every tracked script carrying the shared stub region, read at DISCOVERY so that `inputs` holds
// concrete paths — a pattern stored as an input matches nothing once git is asked with literal
// pathspecs.
//
// A file that cannot be read is skipped rather than failing the run: git listed it, so it is tracked,
// and the one thing that would make it unreadable here is a permission the suite covering it will hit
// first and report with more to say.
func (g *gate) stubScripts() ([]string, error) {
	listed, err := g.listFiles("*.sh")
	if err != nil {
		return nil, err
	}
	var carrying []string
	for _, file := range listed {
		body, readErr := os.ReadFile(filepath.Join(g.root, file))
		if readErr != nil {
			continue
		}
		if strings.Contains(string(body), stubRegionMarker) {
			carrying = append(carrying, file)
		}
	}
	return carrying, nil
}
