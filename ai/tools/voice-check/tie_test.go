package voicecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"configs/ai/tools/repo/repotest"
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

// writeThirdTieFixture writes a source file and a record, and returns their directory and both paths.
// The file's two blocks tie with "so this function", and the record's block ties that way a third time.
func writeThirdTieFixture(t *testing.T) (string, string, string) {
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
	return dir, source, record
}

// The bound reads the file only where the writer names it. A kept block is read only by RecordFindings,
// so a kept block is never reopened by a neighbour written after it.
func TestTheFileBoundRunsOnlyWithFile(t *testing.T) {
	dir, source, record := writeThirdTieFixture(t)
	run := func(args ...string) (int, string) {
		var out, errOut strings.Builder
		code := Run("voice-check.sh", args, dir, repotest.New(dir), baseConfig(), &out, &errOut)
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

// The summary tally names the file bound and the record checks, which sit outside the corpus list.
func TestTheTallyNamesAFileBoundFinding(t *testing.T) {
	dir, source, record := writeThirdTieFixture(t)
	var out, errOut strings.Builder
	Run("voice-check.sh", []string{"--profile=comment", "--source", "--record", "--file=" + source, record}, dir, repotest.New(dir), baseConfig(), &out, &errOut)
	if !strings.Contains(errOut.String(), checkTieRepeated+" 2") {
		t.Fatalf("the tally leaves the file bound out:\n%s", errOut.String())
	}
}

// A writer checks a file's blocks in one call, with a line reading `===` between parts. Each part is read and named
// by its place, and the file bound counts the parts before it. Run 18's writers made a call per block.
func TestOneCallChecksEveryBlockOfAFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "ledger.ts")
	if err := os.WriteFile(source, []byte("export function a() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	part := func(name string) string {
		return "fact: a ledger answers late\nbears_on: " + name + "\ndoes: reads the rate\n---\n" +
			"// A ledger of kind " + strings.ToLower(name[4:]) + " answers late, so this function reads the rate.\nfunction " + name + "(row) {\n  return row.rate;\n}\n"
	}
	input := filepath.Join(dir, "blocks.txt")
	if err := os.WriteFile(input, []byte(part("readA")+"===\n"+part("readB")+"===\n"+part("readC")), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	Run("voice-check.sh", []string{"--profile=comment", "--source", "--record", "--file=" + source, input}, dir,
		repotest.New(dir), baseConfig(), &out, &errOut)
	text := out.String()
	if strings.Contains(text, "#1:") || strings.Contains(text, "#2:") || !strings.Contains(text, "#3:1: "+checkTieRepeated) {
		t.Fatalf("the third part alone should repeat the tie:\n%s%s", text, errOut.String())
	}
}

// The bound scales with the file: max(2, a quarter of its blocks). Each of run 26's six refusals and run
// 20's two is rebuilt as a file of its size, with two other blocks opening the tie alike. The four in
// files of ten and eighteen clear, and the four in files of five to seven still fire.
func TestTheTieBoundScalesWithTheFile(t *testing.T) {
	for _, tc := range []struct {
		run     string
		blocks  int
		opening string
		fires   bool
	}{
		{"run26", 5, "so", true}, {"run26", 6, "therefore", true},
		{"run26", 18, "so", false}, {"run26", 18, "this check", false},
		{"run26", 10, "so", false}, {"run26", 10, "therefore", false},
		{"run20", 5, "therefore", true}, {"run20", 7, "therefore", true},
	} {
		tie := func(i int) string {
			if tc.opening == "this check" {
				return fmt.Sprintf("// A ledger closes book %d, and this check reads it.", i)
			}
			return fmt.Sprintf("// A ledger closes book %d, %s the entry reads it.", i, tc.opening)
		}
		var file []string
		for i := 1; i < tc.blocks; i++ {
			note := fmt.Sprintf("// A ledger opens book %d.", i)
			if i <= 2 {
				note = tie(i)
			}
			file = append(file, note, fmt.Sprintf("export const BOOK_%d = %d;", i, i), "")
		}
		got := TieFindings("f.ts", []string{strings.TrimPrefix(tie(99), "// ")}, file)
		if fired := len(got) > 0; fired != tc.fires {
			t.Errorf("%s: %q in a file of %d blocks fires %v, want %v", tc.run, tc.opening, tc.blocks, fired, tc.fires)
		}
	}
}
