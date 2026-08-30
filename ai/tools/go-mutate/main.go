// Mutation testing for the Go checker: break one guard at a time and require the suite to notice.
//
//	usage: gomutate [-jobs N] [-preflight] [-run <substring>]
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

// How one mutant's run is reported, and whether it counts against the exit code. Four outcomes over
// two facts — what the suite did, and whether this mutant was declared unreachable — and the pair that
// matters is the diagonal: an undeclared survivor is a finding, and a declared one that got killed is
// a stale declaration. Answered here so a case can reach every arm; reached only from main, it would
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
// edits nothing. Preflight and the run itself both ask, and they ask here so they cannot come to
// different answers about one mutant.
func (m mutant) anchor(pkgDir string) (path, source string, count int, err error) {
	path = filepath.Join(pkgDir, m.file)
	body, err := os.ReadFile(path)
	if err != nil {
		return path, "", 0, err
	}
	return path, string(body), strings.Count(string(body), m.from), nil
}

// One mutant that cannot be run as written, and every way it is wrong at once. A renamed guard and a
// renamed test travel together, so a mutant is regularly wrong twice — and the summary counts mutants,
// never complaints, or it claims more broken anchors than the list has entries.
type staleMutant struct {
	label   string
	reasons []string
}

// Every mutant preflight refuses, one entry each. A stale anchor and a test name no suite holds are
// the same defect — both make a mutant report on something other than the guard it names — so both are
// gathered here and the caller decides how to print them.
func staleMutants(list []mutant, pkgDir string, held map[string]map[string]bool) []staleMutant {
	var stale []staleMutant
	for _, m := range list {
		var reasons []string
		switch _, _, matches, err := m.anchor(pkgDir); {
		case err != nil:
			reasons = append(reasons, "no file at "+m.file)
		case matches != 1:
			reasons = append(reasons, fmt.Sprintf("anchor matches %d time(s), want exactly 1", matches))
		}
		if m.by != "" && held[m.suite] != nil && !held[m.suite][m.by] {
			reasons = append(reasons, "names "+m.by+", which "+m.suite+" does not hold")
		}
		if len(reasons) > 0 {
			stale = append(stale, staleMutant{label: m.label, reasons: reasons})
		}
	}
	return stale
}

// What one suite run says about the guard the mutant removed. The three answers print in the same
// column, so telling them apart is the whole of what this harness reports: a mutant that never built
// proves nothing, and read as a kill it manufactures exactly the finding the harness exists to
// produce. `KILLED NOTHING` is the guard being unobserved, which is a finding about the suite.
func verdictOf(suiteFailed bool, output string) string {
	if !suiteFailed {
		return "KILLED NOTHING"
	}
	if strings.Contains(output, "[build failed]") || strings.Contains(output, "cannot use") {
		return "broken"
	}
	return "killed"
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
// Compilation failure is `broken`, not a kill: a mutant that cannot build says nothing about a guard.
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
	return verdictOf(err != nil, string(out)), time.Since(at)
}

// Everything that has to resolve before a single mutant runs, each refusal named as it is found. It
// returns the mutants that cannot be run as written and how many suites could not be listed — the
// second counted apart, because no mutant is at fault for one.
func preflight(pkgDir string) (refused []staleMutant, unlistable int) {
	// Asked once per suite rather than once per mutant. A `by` naming a test its suite no longer holds
	// runs as a filter matching nothing, and `go test` exits 0 on that — so the mutant comes back
	// KILLED NOTHING, loud about the wrong thing. A stale test name is the same defect as a stale
	// anchor, so it is refused in the same place.
	held := map[string]map[string]bool{}
	for _, suite := range suitesNamed() {
		names, err := testsIn(pkgDir, suite)
		if err != nil {
			fmt.Printf("  cannot list     %s: %v\n", suite, err)
			unlistable++
			continue
		}
		held[suite] = names
	}
	// A declaration is an identifier into the list, so the list has to be able to answer it: a label
	// carried twice makes "which mutant is declared" unanswerable, and a declaration naming nothing is
	// a claim about a mutant that is gone. Both are refused here rather than found as odd output later.
	carried := map[string]int{}
	for _, m := range mutants {
		carried[m.label]++
	}
	for label, count := range carried {
		if count > 1 {
			fmt.Printf("  stale           %s: %d mutants share this label, so a declaration cannot name one\n", label, count)
			unlistable++
		}
	}
	for _, u := range unreachableMutants {
		switch {
		case carried[u.label] == 0:
			fmt.Printf("  stale           %q is declared unreachable, and no mutant carries that label\n", u.label)
			unlistable++
		case strings.TrimSpace(u.why) == "":
			fmt.Printf("  stale           %q is declared unreachable with no reason given\n", u.label)
			unlistable++
		}
	}
	refused = staleMutants(mutants, pkgDir, held)
	for _, s := range refused {
		for _, reason := range s.reasons {
			fmt.Printf("  stale           %s: %s\n", s.label, reason)
		}
	}
	return refused, unlistable
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
	refused, stale := preflight(pkgDir)
	if len(refused) > 0 || stale > 0 {
		fmt.Printf("preflight: %d of %d mutant(s) do not resolve, %d suite(s) could not be listed — nothing was run\n",
			len(refused), len(mutants), stale)
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

	bad, declared := 0, 0
	for i, m := range mutants {
		why, isDeclared := declaredUnreachable(m.label)
		shown, isBad := outcomeOf(verdicts[i], isDeclared)
		fmt.Printf("  %-15s %s  (%.1fs)\n", shown, m.label, times[i].Seconds())
		switch {
		case isBad:
			bad++
			// The guard became observable, so the declaration is now a false statement about this
			// suite — and left standing it would go on excusing this mutant's next survival. Said here
			// because nothing else will ever report it.
			if shown == "STALE CLAIM" {
				fmt.Printf("                  a case reddens it, so it is no longer unreachable — drop the declaration, which says: %s\n", why)
			}
		case shown == "unreachable":
			declared++
		}
	}
	// The declared count prints whether or not it is zero, the way a suite's skipped field does: its
	// presence is the information, and a run that silently stopped counting them would look identical
	// to a run with none.
	fmt.Printf("%d mutation(s), %d that proved nothing, %d declared unreachable, %.1fs wall clock\n",
		len(mutants), bad, declared, time.Since(started).Seconds())
	if bad > 0 {
		os.Exit(1)
	}
}
