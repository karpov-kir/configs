package voicecheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

func TestTieOpeningsCountTheConnectorAndTheSubjectApart(t *testing.T) {
	for _, tc := range []struct {
		text string
		want []string
	}{
		{"The ledger rounds late, so this function reads both fields.", []string{"so", "this function"}},
		{"The ledger rounds late. This check therefore tells the two apart.", []string{"therefore", "this check"}},
		{"The ledger rounds late, which is why a deferred posting goes first.", []string{"which is why"}},
		{"This branch keeps a status from 100 up, because a fetch reports 0.", []string{"because", "this branch"}},
	} {
		got := tieOpenings(tc.text)
		for _, w := range tc.want {
			if !got[w] {
				t.Errorf("%q opens %v, want %q", tc.text, got, w)
			}
		}
		if len(got) != len(tc.want) {
			t.Errorf("%q opens %v, want %v", tc.text, got, tc.want)
		}
	}
}

// Three blocks of one file tying with the same connector break the bound, whatever subject each takes.
func TestAConnectorWithVariedSubjectsStillCounts(t *testing.T) {
	file := shell.SplitLines(`// The ledger rounds late, so this check reads both fields.
function a() {}
// The book drops a period, so a posting is asked twice.
function b() {}
`)
	if got := checksOf(TieFindings("-", []string{"The export predates the field, so this lookup reads the type."}, file)); strings.Join(got, ",") != checkTieRepeated {
		t.Fatalf("a third so reports %v", got)
	}
}

// Two blocks of a file already tie with "so this function", and the third is the writer's to vary: its
// connector and its subject are each named.
func TestATieOpeningInTwoOtherBlocksIsNamed(t *testing.T) {
	file := shell.SplitLines(`// The ledger rounds late, so this function reads both fields.
function a() {}
// The book drops a period, so this function asks twice.
function b() {}
// The scheme is new. This check therefore tells the two apart.
function c() {}
`)
	third := []string{"The export predates the field, so this function reads the type."}
	if got := checksOf(TieFindings("-", third, file)); strings.Join(got, ",") != checkTieRepeated+","+checkTieRepeated {
		t.Fatalf("a third so this function reports %v", got)
	}
	varied := []string{"The export predates the field, which is why this lookup reads the type."}
	if got := TieFindings("-", varied, file); len(got) != 0 {
		t.Fatalf("a varied tie reports %v", got)
	}
	// A block the file already holds is being checked again, and it is not its own neighbour.
	again := []string{"The ledger rounds late, so this function reads both fields."}
	if got := TieFindings("-", again, file); len(got) != 0 {
		t.Fatalf("a block counted against itself reports %v", got)
	}
}

// The bound reads the file only where the writer names it. The keep criteria run only RecordFindings,
// so a kept block is never reopened by a neighbour written after it.
func TestTheFileBoundRunsOnlyWithFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "ledger.ts")
	if err := os.WriteFile(source, []byte("// A ledger rounds late, so this function reads both.\nfunction a() {}\n"+
		"// A book drops a period, so this function asks twice.\nfunction b() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(dir, "record.txt")
	if err := os.WriteFile(record, []byte("fact: an export predates the field\nbears_on: readType\ndoes: reads the type\n---\n"+
		"// An export predates the field, so this function reads the type.\nfunction readType(row) {\n  return row.type;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (int, string) {
		var out, errOut strings.Builder
		code := Run("voice-check.sh", args, dir, repo.Exec{}, baseConfig(), &out, &errOut)
		return code, out.String() + errOut.String()
	}
	if code, text := run("--profile=comment", "--source", "--record", "--file="+source, record); code != 1 || !strings.Contains(text, checkTieRepeated) {
		t.Fatalf("with --file the third tie exits %d:\n%s", code, text)
	}
	if _, text := run("--profile=comment", "--source", "--record", record); strings.Contains(text, checkTieRepeated) {
		t.Fatalf("without --file the bound ran:\n%s", text)
	}
	if code, _ := run("--profile=comment", "--source", "--file="+source, record); code != 2 {
		t.Fatalf("--file without --record exits %d, want 2", code)
	}
}
