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
	from  string
	to    string
}

// Every one of these aims at a guard with a case behind it. A mutation that kills nothing means the
// guard is unobserved — the finding the shell harness surfaced twice, both times correctly.
var mutants = []mutant{
	{"direction: cites bound removed", "direction.go", "counters.cites <= findingCap", "counters.cites <= 100000"},
	{"direction: names bound removed", "direction.go", "counters.names <= findingCap", "counters.names <= 100000"},
	{"direction: basename bound removed", "direction.go", "counters.basenames <= findingCap", "counters.basenames <= 100000"},
	{"direction: unchecked notice unbounded", "direction.go", "counters.ambiguous <= findingCap", "counters.ambiguous <= 100000"},
	{"report: per-class cap removed", "report.go", "shown[r] <= findingCap", "shown[r] <= 100000"},
	{"shell: per-file byte bound removed", "shell.go", "info.Size() > maxFileBytes", "info.Size() > (1 << 62)"},
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

	realPath := filepath.Join(pkgDir, m.file)
	source, err := os.ReadFile(realPath)
	if err != nil {
		return "invalid", time.Since(at)
	}
	// Exactly once, or the mutant is ambiguous: a string matching twice edits a guard it was not aimed
	// at, and one matching zero times is the stale anchor the shell harness kept producing.
	if n := strings.Count(string(source), m.from); n != 1 {
		return fmt.Sprintf("anchor x%d", n), time.Since(at)
	}
	mutated := filepath.Join(work, m.file)
	if err := os.WriteFile(mutated, []byte(strings.Replace(string(source), m.from, m.to, 1)), 0o644); err != nil {
		return "invalid", time.Since(at)
	}
	overlay, err := writeOverlay(work, realPath, mutated)
	if err != nil {
		return "invalid", time.Since(at)
	}

	args := []string{"test", "-overlay=" + overlay, "-count=1", "-failfast"}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, "./ecocheck/")
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
	pkgDir := filepath.Join(filepath.Dir(filepath.Dir(here)), "ecocheck")
	if _, err := os.Stat(pkgDir); err != nil {
		fmt.Fprintf(os.Stderr, "gomutate: %s is not there — exit 2, nothing ran.\n", pkgDir)
		os.Exit(2)
	}

	started := time.Now()
	stale := 0
	for _, m := range mutants {
		source, err := os.ReadFile(filepath.Join(pkgDir, m.file))
		if err != nil {
			fmt.Printf("  missing file    %s (%s)\n", m.label, m.file)
			stale++
			continue
		}
		if n := strings.Count(string(source), m.from); n != 1 {
			fmt.Printf("  anchor x%-6d %s\n", n, m.label)
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

	// The baseline first: every verdict below means "this edit turned a green suite red", which says
	// nothing if it was already red.
	base := exec.Command("go", "test", "-count=1", "./ecocheck/")
	base.Dir = filepath.Dir(pkgDir)
	if out, err := base.CombinedOutput(); err != nil {
		fmt.Println("  BASELINE RED    the suite does not pass unmutated")
		fmt.Println(string(out))
		os.Exit(2)
	}

	fmt.Printf("ecocheck — one guard removed at a time, %d at once\n", *jobs)
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
