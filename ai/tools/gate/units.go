package gate

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// The files the Go suites read from outside their own module. Go's test cache is keyed on the module
// and cannot see them, so a plain `go test` reports a cached pass over a changed template; the gate
// keys on them itself. Named file by file, from the suites' own `../../` constants: keying on all of
// kk-flavor made editing tree-fingerprint.sh force eco-report — 233s for a package that cannot read it.
const (
	goTree       = "ai/tools"
	extFlavor    = "ai/kk-flavor/scripts/tree-fingerprint.sh"
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
	gotestInputs = append(gotestInputs, extReduce, extWorkflows)
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

func (g *gate) discoverUnits() int {
	g.addGoChecks()

	if code := g.discoverShellSuites(); code != 0 {
		return code
	}
	return g.discoverGoMutants()
}

func (g *gate) discoverShellSuites() int {
	// `-z` and `core.quotePath=false`, the rule ai/run-tests.sh lives by. Without `-z` a name holding a
	// space arrives as two tokens: `strings.Fields` splits it, safeToken accepts both halves, and the
	// gate builds two units keyed on files that do not exist while the real suite is gated by nothing.
	// Without quotePath a non-ASCII name arrives C-quoted and takes the run to exit 2 blaming the name.
	out, err := g.capture("git", "-c", "core.quotePath=false", "ls-files", "-z",
		"--cached", "--others", "--exclude-standard", "--", "*-test.sh")
	if err != nil || strings.TrimSpace(out) == "" {
		return g.fail("discovery found no *-test.sh at all — read this as the gate broken, never as a clean run")
	}
	var listed []string
	for _, name := range strings.Split(out, "\x00") {
		if name != "" {
			listed = append(listed, name)
		}
	}
	suites := uniqueSorted(listed)
	for _, suite := range suites {
		if err := safeToken("suite", suite); err != nil {
			return g.fail("%s", err)
		}
		// A suite's inputs are itself, the script it covers, and ai/run-tests.sh. That last one because
		// it decides what the suite's exit status and summary line MEAN, so a change to it can flip this
		// unit's verdict with neither the suite nor its script moving a byte.
		inputs := []string{suite, "ai/run-tests.sh"}
		sibling := strings.TrimSuffix(suite, "-test.sh") + ".sh"
		if _, err := os.Stat(filepath.Join(g.root, sibling)); err == nil {
			inputs = append(inputs, sibling)
		}
		// The suites that drive a Go tool also take the tool tree, since a change there moves what they
		// observe. What they observe is a compiled binary, though, so the key drops the module's own
		// `_test.go` files — 66 of the 150 files these units were keyed on, none of which `go build`
		// puts in a binary.
		viaBinary := false
		if body, err := os.ReadFile(filepath.Join(g.root, suite)); err == nil {
			for _, marker := range []string{"tools/", "resolve.sh", "eco-check", "eco-report", "eco-stats", "cite-graph", "rule-echo", "ECO_TOOLS"} {
				if strings.Contains(string(body), marker) {
					inputs = append(inputs, goTree)
					viaBinary = true
					break
				}
			}
			if goSuiteRun.Match(body) {
				viaBinary = false
			}
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

// Single quotes, the one form a POSIX shell reads literally throughout. Written out rather than
// assumed safe: safeToken and the quoting are two defences, and an injection needs both to fail.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
