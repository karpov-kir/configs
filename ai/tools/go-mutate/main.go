// Mutation testing for the Go checker: break one guard at a time and require the suite to notice.
//
//	usage: gomutate [-jobs N] [-preflight] [-run <substring>]
//
// The shell harness this replaces spent two hours because every mutant paid for a whole shell suite.
// Three things remove that, and the third is why a Go source tree is cheaper to mutate than a shell one:
//
//   - `go build -overlay` swaps one file's content without copying the module or touching the tree, so a
//     mutant costs a compile of one package rather than a checkout.
//   - The suite is in-process, so a mutant costs a test binary run rather than 85 process spawns.
//   - `go test -failfast` stops at the first red case, which is all a mutant has to prove.
//
// The mutants live here rather than beside the code, because unlike the shell harness there is no sed
// expression to keep in step with a line — each one names a file, a search string and its replacement,
// and preflight refuses any that no longer matches exactly once.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type mutant struct {
	label string
	file  string
	// The suite that has to notice. It is not always the package the edit lands in: shell and ecoroot
	// are read by both ports, and a figure ecocheck alone prints is asserted by the agreement cases,
	// which live in ecostats' suite.
	suite string
	// The test within that suite that has to notice, as `go test -run` reads it. Empty runs the whole
	// suite, which is what every mutant did before this field: a mutant costs the suite it names, and
	// ecocheck's is the one that forks two `bash -n` per script, so naming the test is what keeps a
	// list this long inside the gate's budget.
	//
	// Named at test-function granularity and never at the subtest's: a t.Run name is prose and gets
	// reworded, and this field would then be the stale anchor the `from` string exists to not be. The
	// *case* is named in the comment above any mutant where which one it is not obvious.
	by   string
	from string
	to   string
}

// Every one of these aims at a guard with a case behind it. A mutation that kills nothing means the
// guard is unobserved — the finding the shell harness surfaced twice, both times correctly.
var mutants = []mutant{
	{"direction: cites bound removed", "direction.go", "./eco-check/", "TestDirectionScan", "counters.cites <= findingCap", "counters.cites <= 100000"},
	{"direction: names bound removed", "direction.go", "./eco-check/", "TestDirectionScan", "counters.names <= findingCap", "counters.names <= 100000"},
	{"direction: basename bound removed", "direction.go", "./eco-check/", "TestDirectionScan", "counters.basenames <= findingCap", "counters.basenames <= 100000"},
	{"direction: unchecked notice unbounded", "direction.go", "./eco-check/", "TestDirectionScan", "counters.ambiguous <= findingCap", "counters.ambiguous <= 100000"},
	{"report: per-class cap removed", "report.go", "./eco-check/", "TestImportResolvedAtTheMount", "shown[r] <= findingCap", "shown[r] <= 100000"},
	{"shell: per-file byte bound removed", "shell.go", "./eco-check/", "TestOversizeFileIsReportedNotRead", "info.Size() > maxFileBytes", "info.Size() > (1 << 62)"},
	{"refs: citation target read with no regular-file test", "refs.go", "./eco-check/", "TestCitationTargetMustBeARegularFile", "if !shell.IsRegularFile(target) {", "if false {"},
	// Both directions off the bare-rule-ID scan's single pattern, which is why they share an anchor.
	// The second is aimed at the quiet case alone: it widens the separator and the phrase just far
	// enough to swallow the delimited citation that finding recommends writing, and leaves the three
	// cases that assert a finding green. A mutant that also broke those would let their failure stand
	// in for the quiet one's, which proves nothing about it.
	{"refs: bare rule-ID scan never fires", "refs.go", "./eco-check/", "TestBareRuleIDCitations", `[Cc]ore [Pp]rinciples? +#?[0-9]+`, `[Zz]ore [Pp]rinciples? +#?[0-9]+`},
	{"refs: bare rule-ID scan reports the form it recommends", "refs.go", "./eco-check/", "TestBareRuleIDCitations", `[Cc]ore [Pp]rinciples? +#?[0-9]+`, `[Cc]ore[ -][Pp]rinciples?[^0-9]*[0-9]+`},
	{"scripts: parse-error text left unsanitised", "scripts.go", "./eco-check/", "TestParseErrorsCarryNoControlByte", `"syntax: "+shell.Oneline(line)`, `"syntax: "+line`},
	{"mounts: resolved mount path left unsanitised", "mounts.go", "./eco-check/", "TestMountFindingCarriesNoControlByte", "shell.Oneline(flavorHave)", "flavorHave"},

	// The delimited-citation half. Undelimited is how a section citation stops resolving in silence,
	// so both directions are needed: the finding never firing, and it firing on the two forms the
	// finding itself recommends.
	{"citations: the undelimited form unreported", "refs.go", "./eco-check/", "TestDelimitedSectionCitations", "if !cited.isDelimited {", "if false {"},
	{"citations: the delimited forms reported as undelimited", "refs.go", "./eco-check/", "TestDelimitedSectionCitations", "isDelimited = section != \"\"", "isDelimited = false"},

	// The skill-directory scan. Each of the three defects makes a skill unreachable rather than
	// merely mis-linked, and each had exactly one case and no mutant behind it.
	{"skills: a directory carrying no SKILL.md unreported", "skills.go", "./eco-check/", "TestSkillDirectory", `if !shell.IsRegularFile(shell.Join(entry.path, "SKILL.md")) {`, "if false {"},
	{"skills: a name/dir mismatch unreported", "skills.go", "./eco-check/", "TestSkillDirectory", "if declared != shell.BaseName(shell.DirName(file)) {", "if false {"},
	{"skills: a SKILL.md with no description unreported", "skills.go", "./eco-check/", "TestSkillDirectory", `if shell.FrontmatterDescription(lines) == "" {`, "if false {"},

	// The test-position scan: eight shell mutants covered this and the port carried none over. Each
	// aims at one of the ways a script can hide from kk-reduce's Phase 6 — a header that declares
	// nothing, one naming a suite that is not there, and the four bounds that keep a crafted header
	// from being read as a declaration.
	//
	// `if len(named) > 8 {` matches twice, once for the report and once for the truncation, so the
	// report's anchor carries the line after it.
	//
	// Two guards in this scan have no mutant, both because nothing observes them. The truncation to
	// eight names is one: no case counts the findings a truncated list produces, and the *report* of
	// an over-long header is asserted separately, so the cut itself is invisible. The other is the
	// `isCleanBasename` filter on the suite map, and that one cannot be given a case at all — the
	// names it excludes are exactly the names `namedTestSuite` cannot extract from a header, so no
	// excluded key could ever have matched one. It is a shell defence against `find` output split on
	// newlines, and os.ReadDir does not split.
	{"test position: over-long header not reported", "scripts.go", "./eco-check/", "TestScriptTestPosition", "if len(named) > 8 {\n\t\t\tc.add(", "if false {\n\t\t\tc.add("},
	{"test position: header bound removed", "scripts.go", "./eco-check/", "TestScriptTestPosition", "if i >= 200 {", "if i >= 100000 {"},
	{"test position: header read past the comment block", "scripts.go", "./eco-check/", "TestScriptTestPosition", `if !strings.HasPrefix(line, "#") {`, "if false {"},
	{"test position: the -mutate.sh exemption removed", "scripts.go", "./eco-check/", "TestScriptTestPosition", `|| strings.HasSuffix(base, "-mutate.sh")`, "|| false"},
	{"test position: a named suite that is absent unreported", "scripts.go", "./eco-check/", "TestScriptTestPosition", "if !suites[suite] {", "if false {"},
	{"test position: a script declaring nothing unreported", "scripts.go", "./eco-check/", "TestScriptTestPosition", "if !anyMatch(header, untestedDeclared) {", "if false {"},
	{"test position: a bare untested: clears the check", "scripts.go", "./eco-check/", "TestScriptTestPosition", `untested:[[:space:]]*[^[:space:]]`, `untested:[[:space:]]*`},
	{"test position: a dash-led suite name goes unread", "scripts.go", "./eco-check/", "TestScriptTestPosition", `[A-Za-z0-9_.-]+-test\.sh`, `[A-Za-z0-9_.]+-test\.sh`},

	// The subcommand call-site scan's Go half. The shell-to-Go migration left this scan reading a
	// `case` dispatch that is no longer in any shell file, so it went quiet for all fourteen of
	// report.sh's subcommands instead of failing — a pass it did not earn. The first mutant below is
	// that exact regression.
	{"subcommands: the Go dispatch never consulted", "subcommands.go", "./eco-check/", "TestGoDispatchSubcommandCallSites", "want(base, c.toolSubcommands(base, lines))", "want(base, nil)"},
	{"subcommands: a missing tool source goes quiet", "subcommands.go", "./eco-check/", "TestADispatchThatCannotBeReadIsReported", `return nil, "no source directory at " + named`, `return nil, ""`},
	{"subcommands: a source with no dispatch goes quiet", "subcommands.go", "./eco-check/", "TestADispatchThatCannotBeReadIsReported", `return nil, "no switch under " + named + " refuses with a '" + shell.Oneline(marker) + "' line"`, `return nil, ""`},
	{"subcommands: any switch read as the dispatch", "subcommands.go", "./eco-check/", "TestGoDispatchSubcommandCallSites", "carries = carries || strings.Contains(line, marker)", "carries = true"},
	{"subcommands: the usage grammar read one line only", "subcommands.go", "./eco-check/", "TestGoDispatchSubcommandCallSites", "if closed || len(grammar) > 4096 {", "if closed || len(grammar) > 0 {"},
	{"subcommands: count bound removed", "subcommands.go", "./eco-check/", "TestSubcommandCountIsBounded", "if len(queries) >= subcommandCap {", "if len(queries) >= 100000 {"},
	{"subcommands: a name only the dispatch has goes unreported", "subcommands.go", "./eco-check/", "TestUsageAndDispatchAreHeldAgainstEachOther", "onlyIn(dispatched, documented)", "onlyIn(dispatched, dispatched)"},
	{"subcommands: a name only the usage has goes unreported", "subcommands.go", "./eco-check/", "TestUsageAndDispatchAreHeldAgainstEachOther", "onlyIn(documented, dispatched)", "onlyIn(documented, documented)"},

	// The rest live outside ecocheck/. A mutant names its file relative to that directory, and the
	// overlay reaches a dependency, or a sibling package, as readily as the package under test.
	// ecoroot holds the `@import` scan and the mount: they are facts about one checkout, not shell
	// primitives, so they sit with the root every path is built from.
	{"imports: name cut a fixed two bytes past the boundary", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", "token[at[0]+boundary+1:at[1]]", "token[at[0]+boundary*0+2:at[1]]"},
	{"imports: uncounted name left unsanitised", "../eco-root/imports.go", "./eco-check/", "TestUncountedNoteCarriesNoControlByte", "shell.CutBytes(shell.Oneline(name), 60)", "shell.CutBytes(name, 60)"},
	// The three caps on the naming half of that note, which shares its anchor with the mutant above:
	// a mutant is refused when *its own* anchor matches twice, and two mutants aiming at one line are
	// two separate questions about it.
	{"imports: uncounted names not capped in entries", "../eco-root/imports.go", "./eco-check/", "TestUncountedNamesAreCapped", "if len(shown) > 10 {", "if len(shown) > 100000 {"},
	{"imports: the uncounted list not capped in bytes", "../eco-root/imports.go", "./eco-check/", "TestUncountedNamesAreCapped", "shell.CutBytes(joined.String(), 200)", "joined.String()"},
	{"imports: one uncounted name not capped in bytes", "../eco-root/imports.go", "./eco-check/", "TestUncountedNamesAreCapped", "shell.CutBytes(shell.Oneline(name), 60)", "shell.Oneline(name)"},
	// The `@import` guard family: fifteen shell mutants, of which the port carried two. What the scan
	// reads an import out of, and what the mount will resolve one to, are the two halves — a budget
	// blind to a resolved import under-reports the tier it exists to measure, and a mount that
	// resolves a shape nothing legitimate produces reads a file the tree pointed it at.
	{"imports: a fenced mention counted as an import", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", "if shell.IsFenceDelimiter(line) {", "if false {"},
	{"imports: a backticked mention counted as an import", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", `backtickSpan.ReplaceAllString(line, " ")`, "line"},
	{"imports: the non-word boundary before the @ dropped", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", "`[^A-Za-z0-9_]@[~A-Za-z0-9._/-]+\\.[A-Za-z0-9]+`", "`@[~A-Za-z0-9._/-]+\\.[A-Za-z0-9]+`"},
	{"imports: the field's leading-space prefix dropped", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", `token := " " + field`, "token := field"},
	{"imports: resolution not gated on the installed checkout", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", `if !m.isInstalled || name == "" {`, "if false {"},
	{"imports: a traversal refused without a word", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", `return "", "a traversal, not a bare filename"`, `return "", ""`},
	{"imports: a subdirectory import resolved anyway", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", `case strings.Contains(name, "/"):`, "case false:"},
	{"imports: a name this carrier never declared resolved", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", "if !m.declared[name] {", "if false {"},
	{"imports: a symlink at the mount followed", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", "if shell.IsSymlink(mounted) {", "if false {"},
	{"imports: an unreadable file at the mount counted", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", "if !isReadable(mounted) {", "if false {"},
	{"imports: resolution attempt cap removed", "../eco-root/imports.go", "./eco-check/", "TestImportResolvedAtTheMount", "if attempts < 64 {", "if attempts < 100000 {"},

	// Carried over from the shell mutation harness for the stats side, against the ports of the two
	// scripts it mutated. Its first two entries were the same edit made twice, once per script,
	// because `contained_in_root` was two copies of one region; here it is one, so it is one mutant —
	// and it is ecostats' suite that holds both the case for the refusal and the case that the other
	// tool refuses it too.
	{"contained-in-root: readability test removed", "../eco-root/contained.go", "./eco-stats/", "", " || !isReadable(path) {", " {"},
	// `* 0` rather than a bare 0, or `words` goes unused and the mutant does not compile — which is a
	// `broken` verdict, and says nothing about the guard.
	{"ecocheck: budget words not counted", "budget.go", "./eco-stats/", "", "budgetWords += words", "budgetWords += words * 0"},
	// To the end of the line, or the anchor also matches the three `+= wordsInFile(…)` above it.
	{"stats: resolved import contributes nothing", "../eco-stats/budget.go", "./eco-stats/", "", "s.alwaysLoadedWords += words\n", "s.alwaysLoadedWords += 0\n"},
	{"stats: no newline collapse in the note", "../eco-stats/eco-stats.go", "./eco-stats/", "", "return ' '", "return r"},
	{"stats: no pipe escaping in the note", "../eco-stats/eco-stats.go", "./eco-stats/", "", "strings.ReplaceAll(note, \"|\", `\\|`)", "strings.ReplaceAll(note, \"|\", \"|\")"},
	{"stats: no note-length bar", "../eco-stats/eco-stats.go", "./eco-stats/", "", "words > noteWordCap", "words > 100000"},
	{"stats: import refusals unreported", "../eco-stats/budget.go", "./eco-stats/", "", `fmt.Fprintf(errOut, "stats.sh: import refused`, `fmt.Fprintf(io.Discard, "stats.sh: import refused`},
	// The name and the path both, because the path is built from the name: sanitising one and printing
	// the other through would leave the ESC byte on the line anyway.
	{"stats: Read-always target left unsanitised", "../eco-stats/budget.go", "./eco-stats/", "TestAMissingReadAlwaysTargetCannotReachTheTerminalRaw", "shell.Oneline(target), shell.Oneline(file))", "target, file)"},
	{"stats: ledger not taken out of prose", "../eco-stats/measure.go", "./eco-stats/", "", "s.prose -= s.ledgerWords", "s.prose -= 0"},
	{"stats: ledger figure unreported", "../eco-stats/report.go", "./eco-stats/", "", `fmt.Fprintf(out, "ledger:`, `fmt.Fprintf(io.Discard, "ledger:`},
	{"stats: mounted-outside unreported", "../eco-stats/report.go", "./eco-stats/", "", `fmt.Fprintf(out, "mounted outside:`, `fmt.Fprintf(io.Discard, "mounted outside:`},
	{"stats: mounted-outside gate removed", "../eco-stats/budget.go", "./eco-stats/", "", "if !s.root.IsInstalled() {", "if false {"},
	{"stats: in-tree mounts not excluded", "../eco-stats/budget.go", "./eco-stats/", "", "if s.root.HoldsSkillFile(file) {", "if false {"},
	{"stats: ledger symlink followed on write", "../eco-stats/ledger.go", "./eco-stats/", "", "if shell.IsSymlink(history) {", "if false {"},
	{"stats: fresh ledger loses the + legend", "../eco-stats/ledger.go", "./eco-stats/", "", "makes it a lower bound", "makes it a lower limit"},
	// The absolute the seed had lost entirely, so a fresh install carried no protection for the column
	// at all. Nothing but the seed-versus-live case notices: this text is written only where there is
	// no ledger yet.
	{"stats: fresh ledger loses the measurement absolute", "../eco-stats/ledger.go", "./eco-stats/", "", "never edited — however that edit is authorised", "never edited"},
	{"stats: fresh ledger loses its columns", "../eco-stats/ledger.go", "./eco-stats/", "", "| date | prose | scripts | always-loaded | skills | what ran |", "| date | prose | scripts | always-loaded | skills |"},

	// The root both tools resolve through. It was a copy per tool until this package took it, and
	// neither tool's suite reaches it: every fixture there names its root outright, so a candidate
	// dropped from the list would have gone unnoticed in both.
	{"ecoroot: the ./ai candidate dropped", "../eco-root/eco-root.go", "./eco-root/", "", `var candidates = []string{".", "./ai"}`, `var candidates = []string{"."}`},
	{"ecoroot: a root needs only one of the two directories", "../eco-root/eco-root.go", "./eco-root/", "", "&& shell.IsDir(shell.Join(dir, skillsDir))", ""},
}

// Every suite a mutant names, in first-named order — what the baseline has to be green over.
func suitesNamed() []string {
	var suites []string
	seen := map[string]bool{}
	for _, m := range mutants {
		if !seen[m.suite] {
			seen[m.suite] = true
			suites = append(suites, m.suite)
		}
	}
	return suites
}

// Where a mutant's anchor is, and how many times it matches there. Exactly once, or the mutant is
// ambiguous: a string matching twice edits a guard it was not aimed at, and one matching zero times
// is the stale anchor the shell harness kept producing. Preflight and the run itself both ask, and
// they ask here so they cannot come to different answers about one mutant.
func (m mutant) anchor(pkgDir string) (path, source string, count int, err error) {
	path = filepath.Join(pkgDir, m.file)
	body, err := os.ReadFile(path)
	if err != nil {
		return path, "", 0, err
	}
	return path, string(body), strings.Count(string(body), m.from), nil
}

// The top-level test names one suite holds, from `go test -list`. Subtests are not listed and are not
// wanted: a mutant names the test function, never the case's prose.
func testsIn(pkgDir, suite string) (map[string]bool, error) {
	cmd := exec.Command("go", "test", "-list", ".*", suite)
	cmd.Dir = filepath.Dir(pkgDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	names := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if name := strings.TrimSpace(line); strings.HasPrefix(name, "Test") {
			names[name] = true
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("it listed no tests at all")
	}
	return names, nil
}

// The overlay is how one file's content changes without the tree changing: `go build` and `go test`
// both read it, so nothing under the repo is written and a killed run leaves no debris behind.
func writeOverlay(dir, realPath, mutatedPath string) (string, error) {
	doc := struct {
		Replace map[string]string `json:"Replace"`
	}{Replace: map[string]string{realPath: mutatedPath}}
	body, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "overlay.json")
	return path, os.WriteFile(path, body, 0o644)
}

// One mutant: rewrite the file into a temp copy, point an overlay at it, and run the suite through it.
// Compilation failure is `broken`, not a kill — a mutant that cannot build says nothing about a guard.
func run(pkgDir string, m mutant, runFilter string) (verdict string, elapsed time.Duration) {
	at := time.Now()
	work, err := os.MkdirTemp("", "gomutate")
	if err != nil {
		return "invalid", time.Since(at)
	}
	defer os.RemoveAll(work)

	realPath, source, matches, err := m.anchor(pkgDir)
	if err != nil {
		return "invalid", time.Since(at)
	}
	if matches != 1 {
		return fmt.Sprintf("anchor x%d", matches), time.Since(at)
	}
	// Base, not the mutant's own relative path: a file named `../shell/x.go` would otherwise write
	// outside the temp dir. Each mutant has its own work dir, so two basenames cannot collide.
	mutated := filepath.Join(work, filepath.Base(m.file))
	if err := os.WriteFile(mutated, []byte(strings.Replace(source, m.from, m.to, 1)), 0o644); err != nil {
		return "invalid", time.Since(at)
	}
	overlay, err := writeOverlay(work, realPath, mutated)
	if err != nil {
		return "invalid", time.Since(at)
	}

	// The mutant's own test unless the caller overrode it: `-run` on the command line is for driving
	// one mutant by hand, and it has to be able to widen the filter as well as narrow it.
	filter := m.by
	if runFilter != "" {
		filter = runFilter
	}
	args := []string{"test", "-overlay=" + overlay, "-count=1", "-failfast"}
	if filter != "" {
		args = append(args, "-run", filter)
	}
	args = append(args, m.suite)
	cmd := exec.Command("go", args...)
	cmd.Dir = filepath.Dir(pkgDir)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return "KILLED NOTHING", time.Since(at)
	}
	if strings.Contains(string(out), "[build failed]") || strings.Contains(string(out), "cannot use") {
		return "broken", time.Since(at)
	}
	return "killed", time.Since(at)
}

func main() {
	jobs := flag.Int("jobs", 0, "mutants in flight at once (default: cores - 2)")
	preflightOnly := flag.Bool("preflight", false, "check every anchor matches exactly once, then stop")
	runFilter := flag.String("run", "", "pass through to `go test -run`")
	flag.Parse()

	if *jobs <= 0 {
		if *jobs = runtime.NumCPU() - 2; *jobs < 1 {
			*jobs = 1
		}
	}
	here, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gomutate: cannot locate myself")
		os.Exit(2)
	}
	pkgDir := filepath.Join(filepath.Dir(filepath.Dir(here)), "eco-check")
	if _, err := os.Stat(pkgDir); err != nil {
		fmt.Fprintf(os.Stderr, "gomutate: %s is not there — exit 2, nothing ran.\n", pkgDir)
		os.Exit(2)
	}

	started := time.Now()
	stale := 0
	// Asked once per suite rather than once per mutant: a `by` naming a test its suite no longer holds
	// would run as a filter matching nothing, which `go test` exits 0 on — a KILLED NOTHING verdict
	// loudly about the wrong thing. A stale test name is the same defect as a stale anchor, so it is
	// refused in the same place.
	held := map[string]map[string]bool{}
	for _, suite := range suitesNamed() {
		names, err := testsIn(pkgDir, suite)
		if err != nil {
			fmt.Printf("  cannot list     %s: %v\n", suite, err)
			stale++
			continue
		}
		held[suite] = names
	}
	for _, m := range mutants {
		_, _, matches, err := m.anchor(pkgDir)
		if err != nil {
			fmt.Printf("  missing file    %s (%s)\n", m.label, m.file)
			stale++
			continue
		}
		if matches != 1 {
			fmt.Printf("  anchor x%-6d %s\n", matches, m.label)
			stale++
		}
		if m.by != "" && held[m.suite] != nil && !held[m.suite][m.by] {
			fmt.Printf("  no such test    %s names %s, which %s does not hold\n", m.label, m.by, m.suite)
			stale++
		}
	}
	if stale > 0 {
		fmt.Printf("preflight: %d of %d anchors do not match exactly once — nothing was run\n", stale, len(mutants))
		os.Exit(1)
	}
	fmt.Printf("preflight: %d anchors, all matching exactly once (%.2fs)\n", len(mutants), time.Since(started).Seconds())
	if *preflightOnly {
		return
	}

	// The baseline first, over every suite a mutant names: each verdict below means "this edit turned
	// a green suite red", which says nothing if it was already red.
	base := exec.Command("go", append([]string{"test", "-count=1"}, suitesNamed()...)...)
	base.Dir = filepath.Dir(pkgDir)
	if out, err := base.CombinedOutput(); err != nil {
		fmt.Println("  BASELINE RED    a suite does not pass unmutated")
		fmt.Println(string(out))
		os.Exit(2)
	}

	fmt.Printf("%s — one guard removed at a time, %d at once\n", strings.Join(suitesNamed(), " "), *jobs)
	verdicts := make([]string, len(mutants))
	times := make([]time.Duration, len(mutants))
	sem := make(chan struct{}, *jobs)
	var wg sync.WaitGroup
	for i, m := range mutants {
		wg.Add(1)
		go func(i int, m mutant) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			verdicts[i], times[i] = run(pkgDir, m, *runFilter)
		}(i, m)
	}
	wg.Wait()

	bad := 0
	for i, m := range mutants {
		if verdicts[i] == "killed" {
			fmt.Printf("  killed          %s  (%.1fs)\n", m.label, times[i].Seconds())
		} else {
			fmt.Printf("  %-15s %s  (%.1fs)\n", verdicts[i], m.label, times[i].Seconds())
			bad++
		}
	}
	fmt.Printf("%d mutation(s), %d that proved nothing, %.1fs wall clock\n", len(mutants), bad, time.Since(started).Seconds())
	if bad > 0 {
		os.Exit(1)
	}
}
