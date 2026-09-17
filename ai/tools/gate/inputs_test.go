// A unit's declared inputs are a set. `--units` prints them and `--why` prints what they resolve to,
// so a count off either is usable only while the two agree.
package gate

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kk-flavor/tools/shell"
)

// The real table, not a fixture. The collision guarded against here is two append sites reaching one
// file, and only this repository's own suites produce it: a sibling script is often also the library
// it sources, and a fixture copy is a second path to the same input. A staged fixture would assert
// what the case handed it rather than what discovery does.
//
// The Go checks and the shell suites, not discoverGoMutants: it runs `go build` and writes a binary
// into the tree. The case below covers the mutation units' own append site directly instead.
// Returns the table, the suite count it should hold one unit for, and how many checks were already
// registered before those units — so a caller compares two counted numbers, never a written-down one.
func discoveredOverThisRepo(t *testing.T) (*gate, int, int) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ai", "run-tests.sh")); err != nil {
		t.Fatalf("%s is not this repository's root, so nothing below measures its units: %v", root, err)
	}
	said := &strings.Builder{}
	g := &gate{root: root, env: Env{Root: root}, errOut: said}
	listed, err := g.listFiles("*-test.sh")
	if err != nil || len(listed) == 0 {
		t.Fatalf("listing this repository's suites: %v", err)
	}
	// Deduplicated the way discoverShellSuites deduplicates its own listing. `git ls-files --cached`
	// names a path once per stage, so during an unmerged suite the raw count runs ahead of the table
	// and the control below would blame a table that is correct.
	suites := len(shell.SortUnique(listed))
	if code := g.addChecks(thisModulesImports(t, g)); code != 0 {
		t.Fatalf("the checks did not register: %s", said.String())
	}
	checks := len(g.units)
	if code := g.discoverShellSuites(); code != 0 {
		t.Fatalf("discovery over this repository exited %d: %s", code, said.String())
	}
	return g, suites, checks
}

// The guide unit has to be keyed on the main package its command builds. Hand-listed, it named the
// library packages and not `cmd/eco-guide`, so editing main.go left the verdict fresh.
func TestTheGuideUnitIsKeyedOnTheCommandItRuns(t *testing.T) {
	g, _, _ := discoveredOverThisRepo(t)

	var guide *unit
	for i := range g.units {
		if g.units[i].id == "guide" {
			guide = &g.units[i]
		}
	}
	if guide == nil {
		t.Fatal("no unit called guide, so this case would pass against any key at all")
	}
	for _, want := range []string{ecoGuideCommand, "ai/tools/eco-guide", "ai/tools/eco-root", "ai/tools/shell"} {
		if !slices.Contains(guide.inputs, want) {
			t.Errorf("guide is not keyed on %s, which its command is built from, so an edit there leaves "+
				"the verdict fresh over a binary nothing rebuilt", want)
		}
	}
	// The narrowness half: this check builds one command, not the module.
	if slices.Contains(guide.inputs, goTree) {
		t.Error("guide is keyed on the whole tool tree, so any Go edit at all re-runs it")
	}
}

// A graph that cannot answer for that command refuses, rather than keying on the three paths left.
func TestAGuideUnitTheGraphCannotAnswerForRefuses(t *testing.T) {
	said := &strings.Builder{}
	g := &gate{errOut: said}
	if code := g.addGuideCheck(map[string][]string{}); code != 2 {
		t.Fatalf("addGuideCheck exited %d over a graph naming no package, want 2", code)
	}
	if len(g.units) != 0 {
		t.Errorf("it registered %d unit(s) anyway, keyed on less than the command builds", len(g.units))
	}
}

// This repository's own graph, read as discovery reads it. `go list` writes nothing, so a case may
// take this where it may not call discoverGoMutants.
func thisModulesImports(t *testing.T, g *gate) map[string][]string {
	t.Helper()
	listing, err := g.listModulePackages()
	if err != nil {
		t.Fatalf("listing this module's packages: %v", err)
	}
	reached, err := moduleImports(listing, g.root)
	if err != nil {
		t.Fatalf("reading this module's import graph: %v", err)
	}
	return reached
}

func TestNoUnitDeclaresAnInputTwice(t *testing.T) {
	g, suites, checks := discoveredOverThisRepo(t)

	// The control: one unit per discovered suite, over and above the checks registered before them.
	// Both numbers are counted here rather than written down, so adding a Go check does not fail this
	// case with a total that names the wrong cause. A table short of a unit would walk the loop below
	// having checked less than it looks like.
	if got := len(g.units) - checks; got != suites {
		t.Fatalf("discovery produced %d shell unit(s) over this repository's %d suite(s)", got, suites)
	}
	for _, u := range g.units {
		if len(u.inputs) == 0 {
			t.Errorf("%s declares no inputs at all, so it checked nothing here", u.id)
			continue
		}
		seen := map[string]bool{}
		var twice []string
		for _, in := range u.inputs {
			if seen[in] {
				twice = append(twice, in)
			}
			seen[in] = true
		}
		if len(twice) > 0 {
			t.Errorf("%s declares %v more than once, so a count read off `--units` is %d too high. "+
				"Dedupe where the unit is built — addUnit — and not at whichever append site collided",
				u.id, twice, len(u.inputs)-len(seen))
		}
	}
}

// The mutation units are built somewhere else entirely, and they left this file's reach when the
// helper above stopped calling buildUnits. groupMutants appends to one group's inputs once per mutant
// over that suite set, so a set holding many mutants over one file is where the widest duplication
// lives — and it is g.add, not groupMutants, that has to collapse it. Staged over the listing format
// the harness emits rather than run through `go build`.
func TestAMutantGroupReachesAUnitWithNoInputTwice(t *testing.T) {
	const line = "eco-report/records.go\t./eco-report/\tTestSomething\tai/tools/eco-report/records.go\n"
	groups, err := groupMutants(line+line+line, "", suiteCompiles)
	if err != nil {
		t.Fatalf("grouping three mutants over one file: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("three mutants over one suite set produced %d group(s), wanted 1", len(groups))
	}

	// The control, and it is the point of the case: grouping itself repeats the file, once per mutant.
	// If that ever stops being true this case proves nothing about g.add, so it has to be asserted.
	if count(groups[0].inputs, "ai/tools/eco-report/records.go") < 2 {
		t.Fatalf("grouping no longer repeats a file across its mutants, so registering it below cannot "+
			"show the dedupe doing anything: %v", groups[0].inputs)
	}

	g := &gate{}
	g.add(groups[0].id, "mutation", groups[0].inputs, "run")
	if len(g.units) != 1 {
		t.Fatalf("registering one group produced %d unit(s)", len(g.units))
	}
	if n := count(g.units[0].inputs, "ai/tools/eco-report/records.go"); n != 1 {
		t.Errorf("the mutation unit declares its file %d times. groupMutants appends per mutant, so a "+
			"suite set with many mutants over one file is the widest duplication in the table", n)
	}
}

func count(values []string, want string) int {
	n := 0
	for _, v := range values {
		if v == want {
			n++
		}
	}
	return n
}

// Why deduping is safe at all, and the half a display fix would leave unstated: a duplicate never
// reached a key. `linesUnder` walks the manifest and takes each line at most once however many times
// a path is declared. If that stops being true, deduping the declared inputs stops being tidying and
// silently retires every cached verdict in the table.
func TestADuplicatedInputDoesNotMoveAUnitsKey(t *testing.T) {
	g := &gate{stamp: "test-digest", manifest: []manifestLine{
		{hash: "aaa", path: goSource},
		{hash: "ccc", path: shellFile},
	}}
	once := unit{id: "shell:x", kind: "check", inputs: []string{goTree, shellFile}, cmd: "run"}
	twice := unit{id: "shell:x", kind: "check", inputs: []string{goTree, shellFile, shellFile, goTree}, cmd: "run"}

	onceKey, onceLines := g.keyMaterial(once)
	twiceKey, twiceLines := g.keyMaterial(twice)

	// The control: the key has to be built over something. Two units resolving to nothing would agree
	// here and the case would pass over a manifest that matched neither.
	if len(onceLines) != 2 {
		t.Fatalf("the singly-declared unit resolved to %v, wanted both fixture files", pathsIn(onceLines))
	}
	if onceKey != twiceKey {
		t.Errorf("declaring an input twice moved the key (%s against %s), so deduping declared inputs "+
			"retires every cached verdict rather than only correcting what --units prints",
			onceKey[:12], twiceKey[:12])
	}
	if len(twiceLines) != len(onceLines) {
		t.Errorf("the duplicate reached the key material: %v against %v",
			pathsIn(twiceLines), pathsIn(onceLines))
	}

	// The half the two comparisons above cannot show: a keyMaterial that ignored its lines entirely
	// would satisfy both. A unit over strictly fewer files has to key differently.
	narrower := unit{id: "shell:x", kind: "check", inputs: []string{shellFile}, cmd: "run"}
	if narrowKey, _ := g.keyMaterial(narrower); narrowKey == onceKey {
		t.Error("a unit keyed on one of the two files hashes the same as one keyed on both, so the key " +
			"is not built from the inputs at all and the comparisons above prove nothing")
	}
}

// The scripts carrying the shared stub region, found without asking the code under test. `git
// ls-files` and a substring search are the whole of it, so a mutation inside stubScripts moves the
// result and not this expectation — an oracle built by calling stubScripts would shrink and grow
// with the thing it is supposed to be pinning, and pass either way.
func stubsFoundIndependently(t *testing.T, root string) []string {
	t.Helper()
	// The same listing listFiles asks for, flags included: `--cached` alone leaves an untracked but
	// marked script out of `want` and in `got`, turning this test red over a code path that behaves.
	// The marker below is spelled out rather than referencing stubRegionMarker on purpose — that is
	// what keeps a mutation of the constant moving `got` and not `want`.
	out, err := exec.Command("git", "-C", root, "-c", "core.quotePath=false", "ls-files", "-z",
		"--cached", "--others", "--exclude-standard", "--", "*.sh").Output()
	if err != nil {
		t.Fatalf("listing this repository's scripts: %v", err)
	}
	var carrying []string
	for _, name := range strings.Split(string(out), "\x00") {
		if name == "" {
			continue
		}
		body, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			continue
		}
		if strings.Contains(string(body), "# --- shared:tool-stub ---") {
			carrying = append(carrying, name)
		}
	}
	slices.Sort(carrying)
	return carrying
}

// The scan behind the stub unit's inputs finds every script carrying the region and nothing else.
// Both halves matter and they fail in opposite directions: too few and the unit stops watching a
// stub while `ai/tools/tool-stub-test.sh` keeps measuring it, so drift arrives as a green from cache;
// too many and every unrelated script edit re-runs the suites that copy stubs.
func TestTheStubScanFindsExactlyTheScriptsCarryingTheRegion(t *testing.T) {
	g, _, _ := discoveredOverThisRepo(t)

	want := stubsFoundIndependently(t, g.root)
	// Two controls, because the comparison is satisfied by two sets that are both empty, and by two
	// that never leave one directory.
	if len(want) < 5 {
		t.Fatalf("the marker found %d script(s) in this repository, so this case proves nothing", len(want))
	}
	directories := map[string]bool{}
	for _, stub := range want {
		directories[filepath.Dir(stub)] = true
	}
	if len(directories) < 3 {
		t.Fatalf("those stubs sit in %d director(ies), so a single-directory glob would satisfy this case", len(directories))
	}

	got, err := g.stubScripts()
	if err != nil {
		t.Fatalf("listing the scripts carrying the stub region: %v", err)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("the stub scan found\n  %v\nbut the region is carried by\n  %v", got, want)
	}
}

// And the unit is keyed on all of them, wherever they sit. The stubs are at four depths in this tree
// — `ai/`, `ai/kk-flavor/scripts/`, `ai/kk-flavor/skills/*/scripts/`, `ai/kk-flavor/workers/*/` — so
// this is asserted over the repository rather than a fixture, which would only assert the depths the
// case itself chose.
func TestTheStubUnitIsKeyedOnEveryScriptCarryingTheRegion(t *testing.T) {
	g, _, _ := discoveredOverThisRepo(t)

	want := stubsFoundIndependently(t, g.root)
	if len(want) < 5 {
		t.Fatalf("the marker found %d script(s), so this case proves nothing about coverage", len(want))
	}

	const stubSuite = "shell:ai/tools/tool-stub"
	found := false
	for _, u := range g.units {
		if u.id != stubSuite {
			continue
		}
		found = true
		for _, stub := range want {
			if !slices.Contains(u.inputs, stub) {
				t.Errorf("%s is not keyed on %s, so editing that stub leaves the unit answering from cache", stubSuite, stub)
			}
		}
	}
	if !found {
		t.Fatalf("%s is not among the discovered units, so nothing here was checked", stubSuite)
	}
}

// The guide prints the tier each dispatch buys, resolved through the model policy, so the policy is
// one of the files that decides the page's bytes. A unit not keyed on a file its command reads is
// the one failure a check like this cannot survive: the committed page goes stale and `--check`
// reports green from cache, which is precisely the state it exists to catch.
//
// Asserted over this repository because the claim is about this tree's wiring rather than about the
// gate's machinery — a fixture would assert whatever declaration the case itself wrote.
func TestTheGuideUnitIsKeyedOnEveryFileItsPageIsBuiltFrom(t *testing.T) {
	g, _, _ := discoveredOverThisRepo(t)

	// The skills tree and the template decide the page too, and were keyed on from the start; the
	// policy is the one that arrived with the second inventory. All three are asserted, so a later
	// edit that trims the declaration is caught whichever entry it takes.
	want := []string{"ai/kk-flavor/configs/models.json", "ai/kk-flavor/skills", "ai/tools/eco-guide"}
	found := false
	for _, u := range g.units {
		if u.id != "guide" {
			continue
		}
		found = true
		for _, input := range want {
			if !slices.Contains(u.inputs, input) {
				t.Errorf("the guide unit is not keyed on %s, so editing it leaves the committed page stale and the check green", input)
			}
		}
	}
	if !found {
		t.Fatal("no guide unit among the discovered units, so nothing here was checked")
	}
}

// The same claim as the guide case above, over the one unit whose command spends money. `addModelCheck`
// keys on direct imports by hand rather than on `ai/tools`, so that a Go edit anywhere does not buy a
// model call per name — which means the list is the only thing standing between an edit and a stale
// green.
//
// `ai/tools/model-policy` is the entry that shows why this is worth a case: `Selections()` lives there
// and decides which names the probe asks about. Drop that one and repricing a row changes the question
// while the unit answers from the last run's record — a provider never asked about the new name, and a
// green saying it was. `bloat-judge` is here for the same reason one step out: the probe runs through
// its callers, so the argv a name is proven against is built there.
//
// What this catches is an entry TRIMMED from the declaration, and only that — hence "stays keyed".
// `want` is a hand-copy, so a package newly imported and left out passes silently. The stub case below
// derives its oracle instead; that is not available here, because the only mechanical derivation is
// `go list -deps`, which over-reaches to diffscan for the reason `addModelCheck` gives.
func TestTheModelsUnitStaysKeyedOnThePackagesItsProbeIsBuiltFrom(t *testing.T) {
	g, _, _ := discoveredOverThisRepo(t)

	want := []string{"ai/kk-flavor/configs/models.json", "ai/kk-flavor/scripts/model-check.sh",
		"ai/tools/model-check", "ai/tools/cmd/model-check", "ai/tools/model-policy",
		"ai/tools/bloat-judge", "ai/tools/shell"}
	found := false
	for _, u := range g.units {
		if u.id != "models" {
			continue
		}
		found = true
		for _, input := range want {
			if !slices.Contains(u.inputs, input) {
				t.Errorf("the models unit is not keyed on %s, so editing it leaves the provider unasked and the check green from cache", input)
			}
		}
	}
	if !found {
		t.Fatal("no models unit among the discovered units, so nothing here was checked")
	}
}

// The gotest unit has to be keyed on the stubs its suite opens. `stub_usage_test.go` discovers every
// stub in the repository and reads each one, all of them outside this module, and Go's test cache
// cannot see any of them: without the key, editing a stub's header leaves the unit fresh and the drift
// check answers out of a cache over a file it never re-read — a check that cannot fire, reported as a
// pass.
//
// Asserted against the built unit rather than against the source text. `gate_script_test.go` holds the
// two halves of the wiring to each other by parsing this file; what that cannot say is whether a
// particular path ended up in the list, which is the fact a reader of `--why gotest` relies on.
func TestTheGotestUnitIsKeyedOnTheStubsItsSuiteReads(t *testing.T) {
	g, _, _ := discoveredOverThisRepo(t)

	var gotest *unit
	for i := range g.units {
		if g.units[i].id == "gotest" {
			gotest = &g.units[i]
		}
	}
	if gotest == nil {
		t.Fatal("no unit called gotest, so this case would pass against any key at all")
	}
	if len(extStubs) == 0 {
		t.Fatal("extStubs is empty, so the loop below asserts nothing")
	}
	for _, stub := range extStubs {
		if !slices.Contains(gotest.inputs, stub) {
			t.Errorf("gotest is not keyed on %s, so an edit to that stub's header leaves this unit fresh "+
				"and stub_usage_test.go compares a line nothing re-read", stub)
		}
	}
}

// `go list`'s answer over the real module, which is what every unit's key now rests on. Two directions,
// and only one is the cheap mistake: eco-report's suite compiles repo-key, so that pair must be there,
// while cadence compiles nothing but itself, so a graph answering "the whole module" fails here.
func TestThisModulesGraphSaysWhatASuiteCompiles(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	reached := thisModulesImports(t, &gate{root: root})
	if !slices.Contains(reached["ai/tools/eco-report"], "ai/tools/repo-key") {
		t.Errorf("the graph does not say eco-report's suite compiles repo-key, so editing repokey.go "+
			"leaves mutants:go:eco-report fresh: %v", reached["ai/tools/eco-report"])
	}
	if got := reached["ai/tools/cadence"]; len(got) != 1 || got[0] != "ai/tools/cadence" {
		t.Errorf("cadence's suite compiles nothing else in this module, and the graph answers %v — a "+
			"unit keyed on that re-runs on edits that cannot move its verdict", got)
	}
}

// The suites read the shipped configs through the real paths, from outside their own module, so
// `go test` keys on none of them: edit a config and the answer comes back from cache, over a file the
// run never opened. The directory is what the unit carries, so a config added later is keyed too.
func TestTheGoTestUnitIsKeyedOnTheShippedConfigs(t *testing.T) {
	g, _, _ := discoveredOverThisRepo(t)

	for _, u := range g.units {
		if u.id != "gotest" {
			continue
		}
		if !slices.Contains(u.inputs, "ai/kk-flavor/configs") {
			t.Fatalf("the gotest unit is not keyed on ai/kk-flavor/configs, so editing a shipped config leaves the suites green from cache: %v", u.inputs)
		}
		return
	}
	t.Fatal("no gotest unit was discovered, so this case measured nothing")
}

// Every package whose suite opens a shipped config through the real path must be forced when one
// moves, or `go test` answers from a cache that cannot see the file it read. The forced list is read
// out of run.go and the expected list is DERIVED from the suites, because a hand-copied oracle passes
// for a package somebody forgot to add.
func TestEveryShippedConfigReaderIsForcedWhenAConfigMoves(t *testing.T) {
	forced := groupsForcedOn(t, "extConfigs")
	for _, pkg := range shippedConfigReaders(t) {
		if !slices.Contains(forced, pkg) {
			t.Errorf("%s reads a shipped config through the real path but is not forced on extConfigs, so editing one leaves its suite green from cache: forced %v", pkg, forced)
		}
	}
	for _, pkg := range forced {
		if _, err := os.Stat(filepath.Join("..", pkg)); err != nil {
			t.Errorf("extConfigs forces %q, which is no package under ai/tools: %v", pkg, err)
		}
	}
}

// The group names one `changedSinceGreen` branch appends. This reads it out of the source, so the case
// cannot fall out of step with the branch it describes.
func groupsForcedOn(t *testing.T, constName string) []string {
	t.Helper()
	body, err := os.ReadFile("run.go")
	if err != nil {
		t.Fatal(err)
	}
	marker := "if g.changedSinceGreen([]string{" + constName + "}) {"
	_, after, found := strings.Cut(string(body), marker)
	if !found {
		t.Fatalf("run.go has no %s branch, so this case measured nothing", constName)
	}
	// Cut at the branch's own closing brace — one tab in — or this reads on through the rest of the
	// function and picks the `go test` argv up as group names.
	line, _, closed := strings.Cut(after, "\n\t}")
	if !closed {
		t.Fatalf("the %s branch in run.go is not closed where this expects it: %.200q", constName, after)
	}
	// Odd segments of a split on the quote character are what sat inside quotes; the even ones are the
	// commas and the closing paren between them.
	var groups []string
	segments := strings.Split(line, `"`)
	for i := 1; i < len(segments); i += 2 {
		groups = append(groups, segments[i])
	}
	if len(groups) == 0 {
		t.Fatalf("read no groups out of the %s branch: %q", constName, line)
	}
	return groups
}

// A package whose own suite resolves the real flavor tree AND names a `.conf` reads a shipped config
// through the path a real run uses. That pair is the signal; either alone is not (a package may hold
// the tree for other fixtures, or write `.conf` files of its own under t.TempDir()).
func shippedConfigReaders(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir("..")
	if err != nil {
		t.Fatal(err)
	}
	var readers []string
	for _, entry := range entries {
		// This package does the deriving, so its own source carries both signals verbatim — the tree
		// literal and the `.conf` suffix are what the scan searches FOR. It opens no shipped config, and
		// counting it would make this case demand that the gate force itself.
		if !entry.IsDir() || entry.Name() == "gate" {
			continue
		}
		tests, err := filepath.Glob(filepath.Join("..", entry.Name(), "*_test.go"))
		if err != nil {
			t.Fatal(err)
		}
		resolvesTree, namesConf := false, false
		for _, test := range tests {
			body, err := os.ReadFile(test)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(body), `"../../kk-flavor"`) {
				resolvesTree = true
			}
			if strings.Contains(string(body), ".conf") {
				namesConf = true
			}
		}
		if resolvesTree && namesConf {
			readers = append(readers, entry.Name())
		}
	}
	if len(readers) == 0 {
		t.Fatal("found no package reading a shipped config, so this case measured nothing")
	}
	return readers
}
