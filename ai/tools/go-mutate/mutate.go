package main

// How one mutant is run: the anchor it edits, the argv the suite is driven with, the overlay that
// swaps the file without touching the tree, the verdict a finished run means, and the preflight
// that refuses a mutant which cannot be run as written.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Every suite a mutant names, in first-named order — what the baseline has to be green over.
// Over the selected mutants, never the whole list: a scoped run that compiled and listed every suite
// would pay the cost the scope exists to avoid, and would go red over a suite no selected mutant
// touches.
func suitesNamed(list []mutant) []string {
	var suites []string
	seen := map[string]bool{}
	for _, m := range list {
		if !seen[m.suite] {
			seen[m.suite] = true
			suites = append(suites, m.suite)
		}
	}
	return suites
}

// The baseline run's argv after `go`.
//
// `-count=1` because `go test` reports `ok (cached)` over a package that fails. `-timeout 30m` because
// this runs whole suites, not one filtered test, and eco-report's alone runs past Go's 10m default on
// a loaded machine — overrunning it panics, and main reads that non-zero exit as a suite that does not
// pass unmutated, printing BASELINE RED for what was machine load.
//
// It is a function so main_test.go can pin both flags: each fails silently when absent, and nothing
// else in the harness would catch one going.
func baselineArgs(selected []mutant) []string {
	return append([]string{"test", "-count=1", "-timeout", suiteTimeout}, suitesNamed(selected)...)
}

// One mutant run's argv after `go`. Every verdict this harness prints comes from this run, so
// main_test.go pins these flags too. `-count=1` because a cached `ok` makes each mutant read KILLED
// NOTHING, a finding manufactured rather than missed; `-failfast` because the first red case has
// already answered what the mutant was asked.
//
// The filter is the mutant's own test unless the caller overrode it: `-run` on the command line is for
// driving one mutant by hand, so it has to widen the filter as well as narrow it. A mutant naming no
// test gets no flag rather than an empty one: `-run ""` is an empty pattern that matches every test,
// so the two runs are the same suite, and the argv a failure prints should say which was meant.
func mutantArgs(m mutant, overlay, runFilter string) []string {
	filter := m.by
	if runFilter != "" {
		filter = runFilter
	}
	args := []string{"test", "-overlay=" + overlay, "-count=1", "-failfast", "-timeout", suiteTimeout}
	if filter != "" {
		args = append(args, "-run", filter)
	}
	return append(args, m.suite)
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

// How long a suite gets before the run is read as not having happened. Explicit, because `go test`
// otherwise applies its own 10m default and eco-report has been measured at 603s — over the line on a
// loaded machine. Generous on purpose: this bounds a hang, it does not police a slow suite.
const suiteTimeout = "20m"

// What one suite run says about the guard the mutant removed. The four answers print in the same
// column, so telling them apart is the whole of what this harness reports: a mutant that never built
// proves nothing, and read as a kill it manufactures exactly the finding the harness exists to
// produce. `KILLED NOTHING` is the guard being unobserved, which is a finding about the suite.
func verdictOf(suiteFailed bool, output string) string {
	if !suiteFailed {
		return "KILLED NOTHING"
	}
	// Before the kill arm, and this is the whole reason it exists. A timed-out suite exits non-zero
	// with a panic, which is neither a build failure nor a case going red, so it used to fall through
	// and be reported as `killed` — the mutant credited with a guard it never observed. That is the
	// finding this harness exists to produce, manufactured by the harness itself, and load is what
	// makes it happen.
	if strings.Contains(output, "test timed out") {
		return "TIMED OUT"
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

// What one mutant's run came back with. One value, not a slice per field for the caller to keep
// index-aligned by hand. Evidence means nothing except beside its own verdict, and separate slices
// sit one edit away from pinning a timeout's output on the mutant next to it.
type result struct {
	verdict string
	elapsed time.Duration
	// The suite's own output, kept for a timeout and nothing else: `test timed out` shows up only in
	// this output, so a timeout verdict cannot be checked without it. Empty otherwise.
	evidence string
}

// One mutant: rewrite the file into a temp copy, point an overlay at it, and run the suite through it.
// Compilation failure is `broken`, not a kill: a mutant that cannot build says nothing about a guard.
func run(pkgDir string, m mutant, runFilter string) result {
	at := time.Now()
	work, err := os.MkdirTemp("", "gomutate")
	if err != nil {
		return result{verdict: "invalid", elapsed: time.Since(at)}
	}
	defer os.RemoveAll(work)

	realPath, source, matches, err := m.anchor(pkgDir)
	if err != nil {
		return result{verdict: "invalid", elapsed: time.Since(at)}
	}
	if matches != 1 {
		return result{verdict: fmt.Sprintf("anchor x%d", matches), elapsed: time.Since(at)}
	}
	// Base, not the mutant's own relative path: a file named `../shell/x.go` would otherwise write
	// outside the temp dir. Each mutant has its own work dir, so two basenames cannot collide.
	mutated := filepath.Join(work, filepath.Base(m.file))
	if err := os.WriteFile(mutated, []byte(strings.Replace(source, m.from, m.to, 1)), 0o644); err != nil {
		return result{verdict: "invalid", elapsed: time.Since(at)}
	}
	overlay, err := writeOverlay(work, realPath, mutated)
	if err != nil {
		return result{verdict: "invalid", elapsed: time.Since(at)}
	}

	cmd := exec.Command("go", mutantArgs(m, overlay, runFilter)...)
	cmd.Dir = filepath.Dir(pkgDir)
	out, err := cmd.CombinedOutput()
	verdict, evidence := verdictWithEvidence(err != nil, string(out))
	return result{verdict: verdict, elapsed: time.Since(at), evidence: evidence}
}

// What a finished suite run means, and what of its output has to survive. Kept out of run(), which
// spawns a real `go test`: reaching this decision through run() would mean making a suite genuinely
// time out, so the capture went unexercised and deleting it still left the package green.
func verdictWithEvidence(suiteFailed bool, output string) (verdict, evidence string) {
	verdict = verdictOf(suiteFailed, output)
	if verdict == "TIMED OUT" {
		evidence = output
	}
	return verdict, evidence
}

// Why the suites cannot answer this `-run`, or empty when they can. `go test -run` takes one regexp
// per `/`-separated level and exits 0 when it selects nothing, so a filter naming a test that is not
// there would turn every mutant into KILLED NOTHING — a wall of findings the harness manufactured.
//
// Only the top level is checked: below it are subtests, which `go test -list` does not print and no
// mutant names.
func unmatchedRunFilter(runFilter string, held map[string]map[string]bool) string {
	if runFilter == "" {
		return ""
	}
	top, _, _ := strings.Cut(runFilter, "/")
	pattern, err := regexp.Compile(top)
	if err != nil {
		return fmt.Sprintf("-run %q does not compile as a regexp: %v", runFilter, err)
	}
	for _, names := range held {
		for name := range names {
			if pattern.MatchString(name) {
				return ""
			}
		}
	}
	return fmt.Sprintf("-run %q matches no test in the suites these mutants name", runFilter)
}

// Everything that has to resolve before a single mutant runs, each refusal named as it is found. It
// returns the mutants that cannot be run as written, how many suites could not be listed — counted
// apart, because no mutant is at fault for one — and why a `-run` given on the command line cannot be
// answered by those suites.
func preflight(pkgDir string, list []mutant, runFilter string) (refused []staleMutant, unlistable int, unrunnableFilter string) {
	// Asked once per suite rather than once per mutant. A `by` naming a test its suite no longer holds
	// runs as a filter matching nothing, and `go test` exits 0 on that — so the mutant comes back
	// KILLED NOTHING, loud about the wrong thing. A stale test name is the same defect as a stale
	// anchor, so it is refused in the same place.
	held := map[string]map[string]bool{}
	for _, suite := range suitesNamed(list) {
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
	refused = staleMutants(list, pkgDir, held)
	for _, s := range refused {
		for _, reason := range s.reasons {
			fmt.Printf("  stale           %s: %s\n", s.label, reason)
		}
	}
	return refused, unlistable, unmatchedRunFilter(runFilter, held)
}

// Every selected mutant, `jobs` of them in flight at once. The results come back in the list's own
// order rather than completion order, so the report reads the same however the machine scheduled it.
func runAll(pkgDir string, selected []mutant, jobs int, runFilter string) []result {
	results := make([]result, len(selected))
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	for i, m := range selected {
		wg.Add(1)
		go func(i int, m mutant) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = run(pkgDir, m, runFilter)
		}(i, m)
	}
	wg.Wait()
	return results
}
