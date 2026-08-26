// A parallel, early-exit mutation runner for the ecosystem checker.
//
//	usage: mutate -target <script> -suite <suite> [-jobs N] [-preflight]
//
// It replaces check-mutate.sh's serial loop and nothing else: the mutant list, the sandbox shape and
// the verdict vocabulary are that script's, read out of it rather than restated here.
//
// Three things make it fast, in the order they pay:
//   - Preflight. Every expression is checked for applying as a one-line edit before any suite runs, so
//     a stale anchor costs two seconds instead of surfacing at the end of a two-hour run.
//   - Early exit. A mutant is proved by one red case, so the suite is killed at the first `  FAIL`
//     rather than run to its trailer. Half the average suite, and far less on a mutant that kills wide.
//   - Parallelism. Mutants are independent — each gets its own sandbox and its own throwaway HOME.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

type mutant struct {
	label string
	expr  string
}

type result struct {
	label   string
	verdict string // applied | invalid | inert | broken | spread | truncated
	kills   int
	elapsed time.Duration
}

// The mutant list stays in check-mutate.sh, and bash parses it: those expressions carry nested quoting
// that only a shell reads the same way twice, so re-implementing the split here would be a second home
// for the list that drifts from the first.
func readMutants(harness string) ([]mutant, error) {
	src, err := os.ReadFile(harness)
	if err != nil {
		return nil, err
	}
	var calls []string
	for _, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(line, "run_mutant ") {
			calls = append(calls, line)
		}
	}
	if len(calls) == 0 {
		return nil, fmt.Errorf("no run_mutant lines in %s — nothing to run, which is not the same as nothing to prove", harness)
	}
	script := "run_mutant() { printf '%s\\000%s\\000' \"$1\" \"$2\"; }\n" + strings.Join(calls, "\n")
	out, err := exec.Command("bash", "-c", script).Output()
	if err != nil {
		return nil, fmt.Errorf("bash could not read the mutant list: %w", err)
	}
	fields := strings.Split(string(out), "\x00")
	var ms []mutant
	for i := 0; i+1 < len(fields); i += 2 {
		ms = append(ms, mutant{label: fields[i], expr: fields[i+1]})
	}
	if len(ms) != len(calls) {
		return nil, fmt.Errorf("read %d mutants from %d run_mutant lines — the list did not parse whole", len(ms), len(calls))
	}
	return ms, nil
}

// sed to a new file, never in place: the in-place flag spells differently on BSD and GNU, and the
// original has to survive for the comparison below.
func applyMutant(target, expr, dest string) (verdict string) {
	original, err := os.ReadFile(target)
	if err != nil {
		return "invalid"
	}
	out, err := exec.Command("sed", "--", expr, target).Output()
	if err != nil {
		return "invalid"
	}
	if string(out) == string(original) {
		return "inert"
	}
	if err := os.WriteFile(dest, out, 0o755); err != nil {
		return "invalid"
	}
	if exec.Command("bash", "-n", dest).Run() != nil {
		return "broken"
	}
	// Exactly one original line, or it removed a guard it was not aimed at.
	if changedLines(string(original), string(out)) != 1 {
		return "spread"
	}
	return "applied"
}

func changedLines(before, after string) int {
	b := strings.Split(before, "\n")
	a := strings.Split(after, "\n")
	present := map[string]int{}
	for _, l := range a {
		present[l]++
	}
	gone := 0
	for _, l := range b {
		if present[l] > 0 {
			present[l]--
		} else {
			gone++
		}
	}
	return gone
}

var failLine = regexp.MustCompile(`^  FAIL`)
var trailer = regexp.MustCompile(`^[0-9]+ passed, [0-9]+ failed$`)

// Killed at the first red case. One is all a mutant has to prove, and the suite's remaining cases
// cannot unprove it — so the trailer is only needed on the path where nothing went red.
func runSuite(mountDir, homeDir string) (kills int, sawTrailer bool) {
	cmd := exec.Command(filepath.Join(mountDir, "check-test.sh"))
	cmd.Env = append(os.Environ(), "HOME="+homeDir)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, false
	}
	cmd.Stderr = cmd.Stdout
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return 0, false
	}
	killGroup := func() {
		// Negative pid is the group. The suite spawns a checker per case, each spawning find and grep;
		// killing the leader alone orphans all of them, and early exit makes that every mutant.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 256*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if failLine.MatchString(line) {
			kills++
			killGroup()
			_ = cmd.Wait()
			return kills, true
		}
		if trailer.MatchString(line) {
			sawTrailer = true
		}
	}
	_ = cmd.Wait()
	return kills, sawTrailer
}

// The suite refuses to run anywhere but $HOME/.claude/skills/kk-ecosystem/scripts, because it executes
// the checker beside it and that gate is what keeps a branch's copy from being the code that runs. So
// the sandbox *is* an installation: a throwaway HOME whose mount path holds the mutant and a copy of
// the suite, and nothing else.
func newSandbox(suite string) (root, mount string, err error) {
	root, err = os.MkdirTemp("", "mutate")
	if err != nil {
		return "", "", err
	}
	mount = filepath.Join(root, "home", ".claude", "skills", "kk-ecosystem", "scripts")
	if err := os.MkdirAll(mount, 0o755); err != nil {
		return root, "", err
	}
	body, err := os.ReadFile(suite)
	if err != nil {
		return root, "", err
	}
	return root, mount, os.WriteFile(filepath.Join(mount, filepath.Base(suite)), body, 0o755)
}

// Every path this tool touches is executed, one way or another: the harness is joined into a `bash -c`
// script, the suite is run directly, and the target is copied into a sandbox the suite then executes.
// So all three must be the installed copies, never a branch's. Without this a reviewed branch commits
// its own `check-mutate.sh` carrying `run_mutant "$(…)" …` and running the gate from that worktree
// executes it — while preflight reports every expression applying cleanly. This is the gate the shell
// harness states as "where it runs from decides whose code executes"; it is not optional here either.
func refuseUncontrolledPath(label, path string) string {
	home := os.Getenv("HOME")
	if home == "" {
		fmt.Fprintln(os.Stderr, "mutate: no $HOME, so the installed mount cannot be identified — exit 2, nothing ran.")
		os.Exit(2)
	}
	// Both sides canonicalised, never the mount path compared literally: each skill under the mount is
	// a symlink into the checkout, so an installed script's real path *is* the repo path. Comparing the
	// unresolved mount would refuse every legitimate run, and comparing the unresolved input would admit
	// any copy. The shell harness settles it the same way, with `cd -P` on both.
	installed := map[string]bool{}
	entries, err := os.ReadDir(filepath.Join(home, ".claude", "skills"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "mutate: %s is not a directory — exit 2, nothing ran.\n", filepath.Join(home, ".claude", "skills"))
		os.Exit(2)
	}
	for _, e := range entries {
		real, err := filepath.EvalSymlinks(filepath.Join(home, ".claude", "skills", e.Name(), "scripts"))
		if err == nil {
			installed[real] = true
		}
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mutate: %s (%s) does not resolve — exit 2, nothing ran.\n", label, path)
		os.Exit(2)
	}
	if !installed[filepath.Dir(real)] {
		fmt.Fprintf(os.Stderr, "mutate: %s resolves to %s, which is no installed skill's scripts directory — exit 2, nothing ran.\n", label, real)
		fmt.Fprintln(os.Stderr, "mutate: this tool executes all three of its inputs, so each must be the installed copy and not a branch's.")
		os.Exit(2)
	}
	return real
}

func main() {
	target := flag.String("target", "", "the script to mutate")
	suite := flag.String("suite", "", "the suite that proves each mutation")
	harness := flag.String("harness", "", "the script holding the run_mutant list (defaults beside the target)")
	jobs := flag.Int("jobs", 0, "mutants in flight at once (default: cores - 2)")
	preflightOnly := flag.Bool("preflight", false, "check every expression applies, then stop")
	flag.Parse()

	if *target == "" || *suite == "" {
		fmt.Fprintln(os.Stderr, "mutate: -target and -suite are both required")
		os.Exit(2)
	}
	if *harness == "" {
		*harness = filepath.Join(filepath.Dir(*target), strings.TrimSuffix(filepath.Base(*target), ".sh")+"-mutate.sh")
	}
	*target = refuseUncontrolledPath("-target", *target)
	*suite = refuseUncontrolledPath("-suite", *suite)
	*harness = refuseUncontrolledPath("-harness", *harness)
	if *jobs <= 0 {
		*jobs = runtime.NumCPU() - 2
		if *jobs < 1 {
			*jobs = 1
		}
	}

	mutants, err := readMutants(*harness)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mutate:", err)
		os.Exit(2)
	}

	// Preflight, before any suite runs. An expression that no longer matches proves nothing, and the
	// whole point of doing it here is that finding out costs seconds rather than the full run.
	started := time.Now()
	stale := 0
	for _, m := range mutants {
		tmp, err := os.CreateTemp("", "preflight")
		if err != nil {
			fmt.Fprintln(os.Stderr, "mutate: no temp file for preflight")
			os.Exit(2)
		}
		v := applyMutant(*target, m.expr, tmp.Name())
		tmp.Close()
		os.Remove(tmp.Name())
		if v != "applied" {
			fmt.Printf("  %-14s %s\n", v, m.label)
			stale++
		}
	}
	if stale > 0 {
		fmt.Printf("preflight: %d of %d expression(s) do not apply as a one-line edit — nothing was run\n", stale, len(mutants))
		os.Exit(1)
	}
	fmt.Printf("preflight: %d expression(s), all apply as a one-line edit (%.1fs)\n", len(mutants), time.Since(started).Seconds())
	if *preflightOnly {
		return
	}

	// The baseline, before any mutation: every verdict below means "this edit turned a green case red",
	// which says nothing if the case was already red. Then the sandbox on an unmutated copy, or a
	// sandbox that fails on its own credits every mutant with a kill it did not earn.
	if out, err := exec.Command(*suite).CombinedOutput(); err != nil {
		fmt.Println("  BASELINE RED    the suite does not pass where it is installed")
		fmt.Println(indent(lastLines(string(out), 12)))
		os.Exit(2)
	}
	sbRoot, sbMount, err := newSandbox(*suite)
	if err == nil {
		body, _ := os.ReadFile(*target)
		err = os.WriteFile(filepath.Join(sbMount, filepath.Base(*target)), body, 0o755)
	}
	if err != nil {
		fmt.Println("  SANDBOX BROKEN  could not build a sandbox from an unmutated copy")
		os.Exit(2)
	}
	if kills, sawTrailer := runSuite(sbMount, filepath.Join(sbRoot, "home")); kills > 0 || !sawTrailer {
		fmt.Println("  SANDBOX RED     the suite is not green on an unmutated copy — every mutant below would credit itself with failures it did not cause")
		os.RemoveAll(sbRoot)
		os.Exit(2)
	}
	os.RemoveAll(sbRoot)

	fmt.Printf("%s — one guard removed at a time, %d at once\n", filepath.Base(*target), *jobs)

	results := make([]result, len(mutants))
	sem := make(chan struct{}, *jobs)
	var wg sync.WaitGroup
	for i, m := range mutants {
		wg.Add(1)
		go func(i int, m mutant) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			at := time.Now()
			r := result{label: m.label}
			root, mount, err := newSandbox(*suite)
			if err != nil {
				r.verdict = "invalid"
				results[i] = r
				os.RemoveAll(root)
				return
			}
			defer os.RemoveAll(root)
			r.verdict = applyMutant(*target, m.expr, filepath.Join(mount, filepath.Base(*target)))
			if r.verdict == "applied" {
				kills, sawTrailer := runSuite(mount, filepath.Join(root, "home"))
				// A mutant that drove the suite to exit part-way leaves no trailer, and its silence is
				// not a verdict on the guard.
				if kills == 0 && !sawTrailer {
					r.verdict = "truncated"
				}
				r.kills = kills
			}
			r.elapsed = time.Since(at)
			results[i] = r
		}(i, m)
	}
	wg.Wait()

	bad := 0
	for _, r := range results {
		switch {
		case r.verdict != "applied":
			fmt.Printf("  %-14s %s\n", r.verdict, r.label)
			bad++
		case r.kills == 0:
			fmt.Printf("  KILLED NOTHING  %s\n", r.label)
			bad++
		default:
			fmt.Printf("  killed %-9d %s  (%.1fs)\n", r.kills, r.label, r.elapsed.Seconds())
		}
	}
	slow := append([]result(nil), results...)
	sort.Slice(slow, func(a, b int) bool { return slow[a].elapsed > slow[b].elapsed })
	if len(slow) > 0 {
		fmt.Printf("slowest: %s at %.1fs\n", slow[0].label, slow[0].elapsed.Seconds())
	}
	fmt.Printf("%d mutation(s), %d that proved nothing, %.1fs wall clock\n", len(results), bad, time.Since(started).Seconds())
	if bad > 0 {
		os.Exit(1)
	}
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func indent(s string) string {
	return "      " + strings.ReplaceAll(s, "\n", "\n      ")
}
