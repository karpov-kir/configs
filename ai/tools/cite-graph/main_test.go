package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A path or a heading reaches the report exactly as the tree spelled it: an ESC there erases the rows
// above it on the terminal reading the report, and a path carrying a newline forges a row of its own —
// the stderr messages have always been sanitised and the report was not.
func TestNamesPrintedInTheReportAreSanitised(t *testing.T) {
	got := printable([]string{"Dead\x1b[2K\rCLEAN", "ev\nil.md", "plain.md"})
	want := []string{"Dead [2K CLEAN", "ev il.md", "plain.md"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("printable[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// Every figure the report prints comes out of one aggregation over the edges, and each one of them
// means something different: a door that enters two sections is the debt, a citer holding the file
// whole is not a door at all, and a section only a precision citer names is entered. Miscount any of
// those and the report still prints a plausible table.
func TestTheReportCountsDoorsDeepDoorsPrecisionCitersAndUnenteredSections(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/proto.md", "# P\n\n## Caller\n\n## Queue\n\n## Nobody\n")
	write(t, root, "deep.md", "See `std/proto.md` → **Caller** and → **Queue**.\n")
	write(t, root, "shallow.md", "See `std/proto.md` → **Caller**.\n")
	write(t, root, "whole.md", "You run under `std/proto.md`. See `std/proto.md` → **Queue**.\n")

	defined, edges, _ := graph(t, root)
	var out, errOut bytes.Buffer
	report(&out, &errOut, defined, edges, map[string]bool{})
	got := out.String()

	if want := "4 file(s), 3 citation edge(s)"; !strings.Contains(got, want) {
		t.Errorf("report is missing %q:\n%s", want, got)
	}
	// Two doors, one of them entering both sections; the citer that also names the file bare is
	// precision, so it is neither a door nor a door section.
	if want := "2 door section(s) /  2 door(s), 1 of them deep,  1 precision citer(s)"; !strings.Contains(got, want) {
		t.Errorf("report is missing %q:\n%s", want, got)
	}
	// `Queue` is entered only by the precision citer and must not be reported unentered; `Nobody` is
	// entered by nothing at all and must be.
	if want := "  shared  std/proto.md                       Nobody\n"; !strings.Contains(got, want) {
		t.Errorf("report is missing %q:\n%s", want, got)
	}
	if want := "\ndepth 1, widest door surface 2 section(s), 1 unentered section(s) of which 1 are in the shared layer\n"; !strings.HasSuffix(got, want) {
		t.Errorf("report does not end with %q:\n%s", want, got)
	}
	if errOut.Len() != 0 {
		t.Errorf("an honest four-file tree wrote to stderr: %q", errOut.String())
	}
}

// The whole point of the count. A scan that missed part of the tree measured a different tree, and it
// prints at full confidence in exactly the shape a measurement of this one takes: over a copy of `ai/`
// with two directories shut, the report went from 49 files and 100 edges to 33 and 13, said the shared
// layer held no unentered section — because the shared layer was what it could not open — and exited
// 0. Both `cite-graph.sh` and CI read the exit code.
func TestAPartialScanRefusesRatherThanReporting(t *testing.T) {
	root := t.TempDir()
	away := t.TempDir()
	write(t, away, "std/proto.md", "# P\n\n## Caller\n")
	write(t, root, "caller.md", "see `std/proto.md` → **Caller**\n")
	if err := os.Symlink(filepath.Join(away, "std"), filepath.Join(root, "std")); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}

	code, out, errOut := runOver(t, root)
	if code != 2 {
		t.Errorf("exit %d over a tree that was not read whole, want 2", code)
	}
	if strings.Contains(out, "DEPTH") || strings.Contains(out, "file(s)") {
		t.Errorf("a partial scan printed a report:\n%s", out)
	}
	if !strings.Contains(errOut, "were NOT read") {
		t.Errorf("stderr %q does not say the tree was not read whole", errOut)
	}
}

// The other side of it: a tree read whole reports and exits 0. Without this the refusal above is
// satisfied by a tool that refuses everything, which measures nothing and passes.
func TestATreeReadWholeReportsAndExitsZero(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/proto.md", "# P\n\n## Caller\n")
	write(t, root, "caller.md", "see `std/proto.md` → **Caller**\n")

	code, out, errOut := runOver(t, root)
	if code != 0 {
		t.Fatalf("exit %d over a tree read whole, want 0; stderr was %q", code, errOut)
	}
	if !strings.Contains(out, "2 file(s), 1 citation edge(s)") {
		t.Errorf("report does not measure the fixture:\n%s", out)
	}
}

// A root holding no markdown is a measurement that did not run, never the 0 a flat tree exits — the
// distinction three skills read this exit code for.
func TestARootHoldingNoMarkdownRefuses(t *testing.T) {
	root := t.TempDir()
	write(t, root, "notes.txt", "nothing to read here\n")

	code, out, errOut := runOver(t, root)
	if code != 2 {
		t.Errorf("exit %d over a root holding no markdown, want 2", code)
	}
	if out != "" {
		t.Errorf("a refusal printed a report:\n%s", out)
	}
	if !strings.Contains(errOut, "read nothing under") {
		t.Errorf("stderr %q does not say it read nothing", errOut)
	}
}

func TestNoRootIsARefusalWithTheUsageGrammar(t *testing.T) {
	code, _, errOut := runOver(t)
	if code != 2 || !strings.Contains(errOut, "usage: cite-graph <root>") {
		t.Errorf("exit %d, stderr %q — want 2 and the usage grammar", code, errOut)
	}
}

// A file the router loads is entered whole on its trigger, so its unentered sections are not a
// finding — and the count in the last line has to agree with the list above it.
func TestARouterLoadedFileContributesNoUnenteredSection(t *testing.T) {
	root := t.TempDir()
	write(t, root, "std/proto.md", "# P\n\n## Nobody\n")
	write(t, root, "caller.md", "nothing here\n")

	defined, edges, _ := graph(t, root)
	var out, errOut bytes.Buffer
	report(&out, &errOut, defined, edges, map[string]bool{"std/proto.md": true})
	got := out.String()

	if strings.Contains(got, "shared  std/proto.md") {
		t.Errorf("a router-loaded file was listed as unentered:\n%s", got)
	}
	if want := "1 unentered section(s) of which 0 are in the shared layer\n"; !strings.HasSuffix(got, want) {
		t.Errorf("report does not end with %q:\n%s", want, got)
	}
}
