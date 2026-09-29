package voicecheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

func TestTieOpeningsReadTheConnectorAndItsSubject(t *testing.T) {
	for _, tc := range []struct{ text, want string }{
		{"The ledger rounds late, so this function reads both fields.", "so this function"},
		{"The ledger rounds late. This check therefore tells the two apart.", "therefore this check"},
		{"The ledger rounds late, therefore this lookup reads both.", "therefore this lookup"},
		{"The ledger rounds late, which is why a deferred posting goes first.", "which is why a deferred"},
		{"This branch keeps a status from 100 up, because a fetch reports 0.", "because a fetch"},
	} {
		got := tieOpenings(tc.text)
		if !got[tc.want] || len(got) != 1 {
			t.Errorf("%q opens %v, want %q", tc.text, got, tc.want)
		}
	}
}

// Two blocks of a file already tie with "so this function", and the third is the writer's to vary.
func TestATieOpeningInTwoOtherBlocksIsNamed(t *testing.T) {
	file := shell.SplitLines(`// The ledger rounds late, so this function reads both fields.
function a() {}
// The book drops a period, so this function asks twice.
function b() {}
// The scheme is new. This check therefore tells the two apart.
function c() {}
`)
	third := []string{"The export predates the field, so this function reads the type."}
	if got := checksOf(TieFindings("-", third, file)); strings.Join(got, ",") != checkTieRepeated {
		t.Fatalf("a third so this function reports %v", got)
	}
	varied := []string{"The export predates the field, which is why this lookup reads the type."}
	if got := TieFindings("-", varied, file); len(got) != 0 {
		t.Fatalf("a varied tie reports %v", got)
	}
	// The block the file already holds is the one being checked again, and it is not its own neighbour.
	again := []string{"The ledger rounds late, so this function reads both fields."}
	if got := TieFindings("-", again, file); len(got) != 0 {
		t.Fatalf("a block counted against itself reports %v", got)
	}
}

// The bound reads the file only where the writer names it. The keep criteria run RecordFindings alone,
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
