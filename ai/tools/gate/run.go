package gate

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (g *gate) printUnits() int {
	fmt.Fprintf(g.out, "%-34s %-9s %-8s %s\n", "UNIT", "KIND", "STATE", "INPUTS")
	for _, u := range g.units {
		key, _ := g.keyMaterial(u)
		state := "stale"
		if _, err := os.Stat(filepath.Join(g.cache, u.stem+"."+key)); err == nil {
			state = "fresh"
		}
		fmt.Fprintf(g.out, "%-34s %-9s %-8s %s\n", u.id, u.kind, state, strings.Join(u.inputs, " "))
	}
	return 0
}

func (g *gate) printWhy(want string) int {
	for _, u := range g.units {
		if u.id != want {
			continue
		}
		key, lines := g.keyMaterial(u)
		fmt.Fprintf(g.out, "%s  (%s)\n", u.id, u.kind)
		fmt.Fprintf(g.out, "  command: %s\n", u.cmd)
		fmt.Fprintf(g.out, "  key:     %s\n", key)
		fmt.Fprintln(g.out, "  inputs:")
		for _, line := range lines {
			fmt.Fprintf(g.out, "    %s\n", line)
		}
		return 0
	}
	return g.fail("no unit is called '%s' — run --units for the list", want)
}

// One unit's line in the run report. The column widths live here rather than at each of the eight
// states that print one. The duration rides in the detail rather than in a column of its own, because
// only some states have one.
func (g *gate) unitLine(state, id, detail string) {
	fmt.Fprintf(g.out, "  %-11s %-32s %s\n", state, id, detail)
}

func (g *gate) runUnits(selected mode, started time.Time) int {
	name := map[mode]string{modeFast: "fast", modeFull: "full", modeMutants: "mutants"}[selected]
	fmt.Fprintf(g.out, "%d unit(s): %s path\n\n", len(g.units), name)

	ran, fresh, deferred, failed, unmeasured, empty := 0, 0, 0, 0, 0, 0
	var deferredIDs []string

	for _, u := range g.units {
		key, lines := g.keyMaterial(u)
		// An input set that resolves to no file is a rename or a typo quietly narrowing the gate. It is
		// the one state this treats as worse than a failure, because it looks exactly like a pass.
		if len(lines) == 0 {
			g.unitLine("NO INPUTS", u.id, "declared: "+strings.Join(u.inputs, " "))
			empty++
			continue
		}
		record := filepath.Join(g.cache, u.stem+"."+key)
		inputsFile := filepath.Join(g.cache, u.stem+".inputs")

		if selected != modeFull {
			if _, err := os.Stat(record); err == nil {
				// A fresh hit says these exact inputs are green, so it can also repair the sidecar the
				// forcing reads. Without this, one failed run deletes the sidecar and every later run
				// over-forces.
				if _, err := os.Stat(inputsFile); err != nil {
					os.WriteFile(inputsFile, []byte(renderLines(lines)), 0o644)
				}
				g.unitLine("fresh", u.id, key[:12]+" — inputs unchanged since it last passed")
				fresh++
				continue
			}
		}
		if u.kind == "mutation" && selected == modeFast {
			g.unitLine("DEFERRED", u.id, "inputs moved — not run on the fast path")
			deferred++
			deferredIDs = append(deferredIDs, u.id)
			continue
		}
		if u.kind == "check" && selected == modeMutants {
			g.unitLine("not asked", u.id, "--mutants settles the mutation units only")
			continue
		}

		at := time.Now()
		output, note, status := g.execute(u)
		took := int(time.Since(at).Round(time.Second).Seconds())
		if note != "" {
			for _, line := range strings.Split(strings.TrimRight(note, "\n"), "\n") {
				fmt.Fprintf(g.out, "             %s\n", line)
			}
		}
		ran++
		if status == 0 {
			// Written only for a verdict this run actually observed, and the inputs it was observed
			// over are written beside it — that second file is what changedSinceGreen reads.
			os.WriteFile(record, nil, 0o644)
			os.WriteFile(inputsFile, []byte(renderLines(lines)), 0o644)
			g.unitLine("ran ok", u.id, fmt.Sprintf("%ds", took))
			continue
		}
		// Neither a record nor a pass, whichever way it went: a verdict recorded before someone broke
		// the code would answer for the broken tree the moment they reverted an unrelated file.
		os.Remove(record)
		os.Remove(inputsFile)
		switch status {
		case 2:
			// This repo's "it did not run" — a mutation suite the watchdog killed on a loaded machine, a
			// fixture that could not be built. Held apart from a failure: calling it one names the code
			// for something the machine did.
			g.unitLine("NO MEASURE", u.id, fmt.Sprintf("%ds  it exited 2 — it did not run, so nothing is known", took))
			g.tail(output, 10, "                  ")
			unmeasured++
		case 3:
			// run-tests.sh's "ran, and refuses its own result" — the checkout moved underneath it. Read
			// as a failure it would name the code for something a neighbouring agent did.
			g.unitLine("REFUSED", u.id, fmt.Sprintf("%ds  it exited 3 — the checkout moved while it ran, so it refuses its own result", took))
			g.tail(output, 10, "                  ")
			unmeasured++
		default:
			g.unitLine("FAILED", u.id, fmt.Sprintf("%ds", took))
			g.tail(output, 40, "                  ")
			failed++
		}
	}

	if len(deferredIDs) > 0 {
		fmt.Fprintln(g.out, "\nDEFERRED — these have inputs that moved, and the fast path did not run them:")
		for _, id := range deferredIDs {
			fmt.Fprintf(g.out, "    %s\n", id)
		}
		fmt.Fprintln(g.out, "  Mutation is a statement about whether the suites can fail, not about this change, and on this")
		fmt.Fprintln(g.out, "  machine it costs minutes per script. CI runs the full sweep on every push. To settle them here:")
		fmt.Fprintln(g.out, "      ai/gate.sh --mutants")
	}

	fmt.Fprintf(g.out, "\n%d unit(s): %d ran, %d fresh from cache, %d deferred, %d failed, %d that never measured, %d with no inputs, %ds wall clock\n",
		len(g.units), ran, fresh, deferred, failed, unmeasured, empty, int(time.Since(started).Round(time.Second).Seconds()))

	// A unit that resolved to nothing outranks everything: the gate does not know what it did not look
	// at, so it may not call the run clean, and it may not call it a failure of the code either.
	if empty > 0 {
		fmt.Fprintf(g.errOut, "%d unit(s) resolved to no input file — the gate narrowed itself and cannot report on them. Exit 2, and this is not a pass.\n", empty)
		return 2
	}
	if ran == 0 && fresh == 0 {
		fmt.Fprintln(g.errOut, "nothing was measured and nothing was answered from cache — exit 2, and this is not a pass.")
		return 2
	}
	// A finding about the code outranks one about the machine. Exit 2 alone means nothing was found
	// wrong and something never ran, which a caller may never read as a pass.
	if failed > 0 {
		return 1
	}
	if unmeasured > 0 {
		fmt.Fprintf(g.errOut, "%d unit(s) exited 2 without measuring — nothing is known about them, and this is not a pass.\n", unmeasured)
		return 2
	}
	return 0
}

// One unit's command. The two built-in checks run in process; everything else goes to a shell, because
// the commands are written as shell and several of them cd.
//
// A unit's own output is held back and shown only if it fails, so a unit that needs to explain a cost
// while it is still passing returns a note instead. Without it, gotest can spend two extra minutes
// forcing packages and the run says only "ran ok".
func (g *gate) execute(u unit) (output string, note string, status int) {
	switch u.cmd {
	case "@gofmt":
		return g.runGofmt()
	case "@gotest":
		return g.runGotest()
	}
	cmd := exec.Command("sh", "-c", u.cmd)
	cmd.Dir = g.root
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	return buf.String(), "", exitCode(err)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode()
	}
	return 127
}

func (g *gate) runGofmt() (string, string, int) {
	// Both streams, because a parse error goes to stderr and exits 2 with an empty stdout — captured
	// one-sided, that unit prints FAILED and not one word about which file will not parse.
	out, err := runIn(filepath.Join(g.root, "ai", "tools"), "gofmt", "-l", ".")
	if err != nil {
		return out, "", 1
	}
	if strings.TrimSpace(out) == "" {
		return "", "", 0
	}
	return "gofmt would rewrite:\n" + out, "", 1
}

// Go's own cache is content-keyed over the module and is exactly right there: a warm `go test ./...`
// is 2.4s against 185s cold, and it does not cache failures. What it cannot see is the files outside
// the module that the fixtures copy in, so those packages are forced with -count=1 whenever one moved.
// goSuiteTimeout is the bound every `go test` here carries, and it has one home so the two workflows
// can be held to the same number. `go test`'s default is 10 minutes and the eco-report package alone
// can run past that on a loaded machine; an overrun prints a goroutine dump, which reads as a deadlock
// in os/exec rather than as a slow pass, and has twice been reported here as a red gate that was not
// one. `workflows_test.go` reads this line, so keep the assignment at column zero and on one line.
const goSuiteTimeout = "30m"

func (g *gate) runGotest() (string, string, int) {
	var groups []string
	if g.changedSinceGreen([]string{extFlavor}) {
		groups = append(groups, "eco-report")
	}
	if g.changedSinceGreen(extQualify) {
		groups = append(groups, "eco-report")
	}
	if g.changedSinceGreen([]string{extReduce}) {
		groups = append(groups, "eco-stats")
	}
	if g.changedSinceGreen([]string{extWorkflows}) {
		groups = append(groups, ".")
	}
	tools := filepath.Join(g.root, "ai", "tools")
	if len(groups) == 0 {
		out, err := runIn(tools, "go", "test", "-timeout", goSuiteTimeout, "./...")
		return out, "", exitCode(err)
	}
	module, err := runIn(tools, "go", "list", "-m")
	if err != nil {
		return module, "", 1
	}
	module = strings.TrimSpace(module)
	forced := map[string]bool{}
	for _, dir := range groups {
		if dir == "." {
			forced[module] = true
		} else {
			forced[module+"/"+dir] = true
		}
	}
	listed, err := runIn(tools, "go", "list", "./...")
	if err != nil {
		return listed, "", 1
	}
	var rest []string
	for _, pkg := range strings.Fields(listed) {
		if !forced[pkg] {
			rest = append(rest, pkg)
		}
	}
	forcedList := make([]string, 0, len(forced))
	for pkg := range forced {
		forcedList = append(forcedList, pkg)
	}
	sort.Strings(forcedList)
	note := fmt.Sprintf("forcing %d package(s) with -count=1: an input outside the module moved, and the Go cache cannot see those", len(forcedList))

	var out strings.Builder
	status := 0
	if len(rest) > 0 {
		// The forced packages come OUT of the cached run rather than being run in both. Left in, the
		// whole of eco-report is measured twice.
		body, err := runIn(tools, "go", append([]string{"test", "-timeout", goSuiteTimeout}, rest...)...)
		out.WriteString(body)
		if err != nil {
			status = 1
		}
	}
	body, err := runIn(tools, "go", append([]string{"test", "-timeout", goSuiteTimeout, "-count=1"}, forcedList...)...)
	out.WriteString(body)
	if err != nil {
		status = 1
	}
	return out.String(), note, status
}

func runIn(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	return buf.String(), err
}

func (g *gate) tail(output string, n int, indent string) {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for _, line := range lines {
		fmt.Fprintf(g.out, "%s%s\n", indent, line)
	}
}
