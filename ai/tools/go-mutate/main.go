// Mutation testing for the Go checker: break one guard at a time and require the suite to notice.
//
//	usage: gomutate [-jobs N] [-preflight] [-run <go test -run pattern>] [-file a.go,b.go] [-units]
//
// `-file` is what selects mutants. `-run` is not: it goes through to `go test -run` and narrows which
// cases each mutant is asked, so `-run <a mutant label>` names no test at all, and preflight refuses
// the run rather than starting it.
//
// A mutant costs one package compile and one in-process test run, never a checkout: `go build
// -overlay` swaps a file's content without copying the module or touching the tree, and `-failfast`
// stops at the first red case, which is all a mutant has to prove.
//
// The mutants live in mutants.go beside this file rather than beside the code they break: each names
// a file, a search string and its replacement, and preflight refuses any that no longer matches
// exactly once.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// How one mutant's run is reported, and whether it counts against the exit code. Four outcomes over
// two facts — what the suite did, and whether this mutant was declared unreachable — and the two that
// matter are the mismatches: an undeclared survivor is a finding, and a declared one that got killed
// is a stale declaration. Answered here so a case can reach every arm; reached only from main, it would
// take a full mutation run to observe one.
func outcomeOf(verdict string, isDeclared bool) (shown string, isBad bool) {
	switch {
	case verdict == "killed" && !isDeclared:
		return "killed", false
	case verdict == "killed":
		return "STALE CLAIM", true
	case verdict == "KILLED NOTHING" && isDeclared:
		return "unreachable", false
	}
	// A suite that never finished says nothing about the guard either way, so it is neither a finding
	// nor an excuse — the caller counts it apart and exits 2, which is the reading a watchdog kill has
	// always been given here.
	if verdict == "TIMED OUT" {
		return "TIMED OUT", false
	}
	// Everything else is a finding whatever was declared: `broken` says the mutant never built, and a
	// declaration is about a guard being unobservable, never about the edit failing to compile.
	return verdict, true
}

// The reason declared for a mutant, and whether one was declared at all.
func declaredUnreachable(label string) (string, bool) {
	for _, u := range unreachableMutants {
		if u.label == label {
			return u.why, true
		}
	}
	return "", false
}

// The mutants whose file one of `names` names. Empty selects everything: a scope nobody asked for
// is not a scope. A name matching no mutant is the caller's typo and is refused by the caller, never
// silently narrowed to nothing here — which is the one way a scoped run reports a pass over no work.
func selectByFile(list []mutant, names string) (selected []mutant, unmatched []string) {
	if strings.TrimSpace(names) == "" {
		return list, nil
	}
	held := map[string]bool{}
	for _, m := range list {
		held[m.file] = true
	}
	want := map[string]bool{}
	for _, name := range strings.Split(names, ",") {
		if name = strings.TrimSpace(name); name == "" {
			continue
		}
		if !held[name] {
			unmatched = append(unmatched, name)
			continue
		}
		want[name] = true
	}
	for _, m := range list {
		if want[m.file] {
			selected = append(selected, m)
		}
	}
	return selected, unmatched
}

// One line per mutated file: the file, the suites its mutants name, and how many there are. Built
// from the list rather than restated anywhere else — a second copy of this mapping is a second thing
// to go stale when a mutant moves. This is the whole of what a caller scopes on — `ai/gate.sh` builds
// one unit per line — so a file dropped here is a mutation unit that silently stops existing, which
// is the narrowing that harness exists to refuse.
func unitLines(list []mutant, pkgDir string) []string {
	var files []string
	suites := map[string][]string{}
	counts := map[string]int{}
	for _, m := range list {
		if counts[m.file] == 0 {
			files = append(files, m.file)
		}
		counts[m.file]++
		if !slices.Contains(suites[m.file], m.suite) {
			suites[m.file] = append(suites[m.file], m.suite)
		}
	}
	lines := make([]string, 0, len(files))
	for _, file := range files {
		// The resolved path of the mutated file is the fourth column, because the caller cannot derive
		// it: `file` is relative to the package this harness is pointed at, and that base lives here.
		// `ai/gate.sh` used to hardcode its own copy of it, which is the same mapping in two homes and
		// a rename away from a gate that resolves nothing.
		lines = append(lines, fmt.Sprintf("%s\t%s\t%d\t%s",
			file, strings.Join(suites[file], ","), counts[file], filepath.Join(pkgDir, file)))
	}
	return lines
}

// A block of suite output, set in from the verdict lines so the two do not read as one stream. Named
// apart from the `indent` in this module's test harnesses: that one ends every line with a newline,
// this one joins without a trailing one. Two contracts, so two names.
func indentBlock(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, line := range lines {
		lines[i] = "                  " + line
	}
	return strings.Join(lines, "\n")
}

// One line per mutant, and the two counts the exit code is decided on. Apart from main for the same
// reason outcomeOf is: reached only from there, an arm of it would take a full mutation run to see.
func report(selected []mutant, results []result) (bad, declared, unmeasured int) {
	for i, m := range selected {
		why, isDeclared := declaredUnreachable(m.label)
		shown, isBad := outcomeOf(results[i].verdict, isDeclared)
		fmt.Printf("  %-15s %s  (%.1fs)\n", shown, m.label, results[i].elapsed.Seconds())
		switch {
		case isBad:
			bad++
			// The guard became observable, so the declaration is now a false statement about this
			// suite — and left standing it would go on excusing this mutant's next survival. Said here
			// because nothing else will ever report it.
			if shown == "STALE CLAIM" {
				fmt.Printf("                  a case reddens it, so it is no longer unreachable — drop the declaration, which says: %s\n", why)
			}
		case shown == "TIMED OUT":
			unmeasured++
			// The only place the suite's own words reach the log. Without them a reader auditing the
			// run for manufactured kills can only infer from durations.
			if results[i].evidence != "" {
				fmt.Println(indentBlock(results[i].evidence))
			}
		case shown == "unreachable":
			declared++
		}
	}
	return bad, declared, unmeasured
}

// Every refusal this command makes: the one prefix a reader greps for, and the exit status that says
// nothing ran. Five sites spelled both by hand, and one of them had dropped the "nothing ran" half.
func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "gomutate: "+format+" — exit 2, nothing ran.\n", a...)
	os.Exit(2)
}

func main() {
	jobs := flag.Int("jobs", 0, "mutants in flight at once (default: cores - 2)")
	preflightOnly := flag.Bool("preflight", false, "check every anchor matches exactly once, then stop — never that a mutant still dies")
	runFilter := flag.String("run", "", "pass through to `go test -run`")
	fileFilter := flag.String("file", "", "run only the mutants whose file is one of these (comma-separated)")
	listUnits := flag.Bool("units", false, "print one line per mutated file — file, suites, mutant count — and stop")
	flag.Parse()

	if *jobs <= 0 {
		if *jobs = runtime.NumCPU() - 2; *jobs < 1 {
			*jobs = 1
		}
	}
	here, err := os.Executable()
	if err != nil {
		die("cannot locate myself")
	}
	pkgDir := filepath.Join(filepath.Dir(filepath.Dir(here)), "eco-check")
	if _, err := os.Stat(pkgDir); err != nil {
		die("%s is not there", pkgDir)
	}

	// After pkgDir, not before it: the listing's fourth column is the resolved path of each mutated
	// file, and resolving it is the whole reason a caller does not have to know this base itself.
	if *listUnits {
		for _, line := range unitLines(mutants, pkgDir) {
			fmt.Println(line)
		}
		return
	}

	// The scope, resolved before anything is compiled. A name matching no mutant exits 2 rather than
	// narrowing the run: a caller that misspells a file would otherwise get a green over zero mutants,
	// which is the one verdict this harness must never produce.
	selected, unmatched := selectByFile(mutants, *fileFilter)
	if len(unmatched) > 0 {
		die("no mutant names %s", strings.Join(unmatched, ", "))
	}
	if len(selected) == 0 {
		die("the selection is empty")
	}
	scope := "every mutant"
	if len(selected) != len(mutants) {
		scope = fmt.Sprintf("%d of %d mutant(s), scoped to %s — the rest were NOT run", len(selected), len(mutants), *fileFilter)
	}

	started := time.Now()
	refused, stale, unrunnableFilter := preflight(pkgDir, selected, *runFilter)
	if unrunnableFilter != "" {
		die("%s", unrunnableFilter)
	}
	if len(refused) > 0 || stale > 0 {
		fmt.Printf("preflight: %d of %d mutant(s) do not resolve, %d suite(s) could not be listed — nothing was run\n",
			len(refused), len(selected), stale)
		os.Exit(1)
	}
	fmt.Printf("preflight: %d anchors, all matching exactly once (%.2fs); %s\n", len(selected), time.Since(started).Seconds(), scope)
	if *preflightOnly {
		return
	}

	// The baseline first, over every suite a mutant names: each verdict below means "this edit turned
	// a green suite red", which says nothing if it was already red.
	base := exec.Command("go", baselineArgs(selected)...)
	base.Dir = filepath.Dir(pkgDir)
	if out, err := base.CombinedOutput(); err != nil {
		fmt.Println("  BASELINE RED    a suite does not pass unmutated")
		fmt.Println(string(out))
		os.Exit(2)
	}

	fmt.Printf("%s — one guard removed at a time, %d at once\n", strings.Join(suitesNamed(selected), " "), *jobs)
	bad, declared, unmeasured := report(selected, runAll(pkgDir, selected, *jobs, *runFilter))
	// The declared count prints whether or not it is zero, the way a suite's skipped field does: its
	// presence is the information, and a run that silently stopped counting them would look identical
	// to a run with none.
	//
	// The out-of-scope count sits in that same line rather than in a note above it: a scoped run whose
	// last line reads like a whole one is exactly the narrowing this harness is not allowed to hide.
	fmt.Printf("%d mutation(s) run, %d not run (out of scope), %d that proved nothing, %d that never measured, %d declared unreachable, %.1fs wall clock\n",
		len(selected), len(mutants)-len(selected), bad, unmeasured, declared, time.Since(started).Seconds())
	// A finding about the code outranks one about the machine. Exit 2 alone means nothing was found
	// wrong and something never ran, which a caller may never read as a pass — `ai/gate.sh` maps it to
	// NO MEASURE for exactly that.
	if bad > 0 {
		os.Exit(1)
	}
	if unmeasured > 0 {
		fmt.Printf("%d mutant(s) never measured — their suite ran past %s, so nothing is known about the guards they name.\n", unmeasured, suiteTimeout)
		os.Exit(2)
	}
}
