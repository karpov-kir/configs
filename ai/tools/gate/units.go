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
	goTree    = "ai/tools"
	extFlavor = "ai/kk-flavor/scripts/tree-fingerprint.sh"
	// The audience marker is read twice — as a Go regexp in shell/markdown.go, and as awk in this
	// library, which both installers source and which runs before the machine has a Go binary at all.
	// shell's suite holds the two spellings to each other, so it is keyed on the file it reads them
	// out of: key it on anything else and an edit to the awk leaves that suite fresh from cache.
	extAudience  = "lib/skill-audience.sh"
	extReduce    = "ai/kk-flavor/skills/kk-reduce/stats.md"
	extWorkflows = ".github/workflows"
	extModels    = "ai/kk-flavor/models.json"
	// The stub scripts ai/tools/tool-stub-test.sh copies into fixtures and runs. copiedRepoFiles finds
	// only the one path that suite spells out literally; the other six live in its `stubs()` table,
	// which no text scan parses. Globbed at DISCOVERY, so what lands in `inputs` is concrete paths —
	// a pattern stored as an input would match nothing once git is asked with literal pathspecs.
	skillScripts = "ai/kk-flavor/skills/*/scripts/*.sh"
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
					inputs = append(inputs, rel)
				}
			}
		}
		// Three ways into one input. A suite inside the tool tree observes it whatever its text says,
		// which is what lets the scan below ask for `ai/tools/` rather than a bare `tools/`. Running the
		// module's own suites is the third: such a suite compiles the tree, so keying it on nothing is a
		// cached pass over a tool that changed, and it decides the blindness rather than only clearing
		// it — what it observes is the test files, not a binary built without them.
		runsGoSuites := goSuiteRun.MatchString(body) || goSuiteRun.MatchString(siblingBody)
		if strings.HasPrefix(suite, goTree+"/") || runsGoSuites || drivesGoTool(body, siblingBody) {
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
