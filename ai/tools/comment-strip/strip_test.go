package commentstrip

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gitrepo "configs/ai/tools/repo"
)

// What a case hands a run that never passes --changed. Only that option asks the repository
// anything, so a run that started to would panic here rather than read an answer the case never
// arranged.
var noRepository gitrepo.Git

func TestStripRemovesEveryBlockAndRecordsItsSiteInTheStrippedFile(t *testing.T) {
	source := "/** Lists things. */\nfunction a() {}\n\n// A note about b.\n// Its second line.\n\nfunction b() {}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "f.ts")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	facts := filepath.Join(dir, "facts")
	var out, errOut strings.Builder
	code := Strip("comment-strip.sh", []string{"--facts=" + facts, path}, dir, noRepository, &out, &errOut)
	if code != exitCut {
		t.Fatalf("exit %d, want %d: %s", code, exitCut, errOut.String())
	}
	got, _ := os.ReadFile(path)
	if want := "function a() {}\n\n\nfunction b() {}\n"; string(got) != want {
		t.Fatalf("stripped file:\n%q\nwant\n%q", got, want)
	}
	// Sites are lines of the stripped file: a() is its line 1, b() its line 4.
	if want := path + ":1 1.facts\n" + path + ":4 2.facts\n"; out.String() != want {
		t.Fatalf("sites:\n%s\nwant\n%s", out.String(), want)
	}
	second, _ := os.ReadFile(filepath.Join(facts, "2.facts"))
	if want := path + ":4\n// A note about b.\n// Its second line.\n"; string(second) != want {
		t.Fatalf("2.facts:\n%q\nwant\n%q", second, want)
	}
}

func TestStripKeepsAComentTheToolchainReads(t *testing.T) {
	source := "// eslint-disable-next-line no-console\nconsole.log(1);\n// a note\nconst x = 1;\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "f.ts")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	if code := Strip("comment-strip.sh", []string{"--facts=" + filepath.Join(dir, "facts"), path}, dir, noRepository, &out, &errOut); code != exitCut {
		t.Fatalf("exit %d, want %d: %s", code, exitCut, errOut.String())
	}
	got, _ := os.ReadFile(path)
	if want := "// eslint-disable-next-line no-console\nconsole.log(1);\nconst x = 1;\n"; string(got) != want {
		t.Fatalf("stripped file:\n%q\nwant\n%q", got, want)
	}
	if !strings.Contains(errOut.String(), path+":1: a comment the toolchain reads, kept") {
		t.Fatalf("the kept directive is not named on stderr:\n%s", errOut.String())
	}
}

func TestStripWithNothingToRemoveLeavesTheFileAndExitsClean(t *testing.T) {
	source := "function a() {}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "f.ts")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	if code := Strip("comment-strip.sh", []string{"--facts=" + filepath.Join(dir, "facts"), path}, dir, noRepository, &out, &errOut); code != exitClean {
		t.Fatalf("exit %d, want %d: %s", code, exitClean, errOut.String())
	}
	got, _ := os.ReadFile(path)
	if string(got) != source || out.Len() != 0 {
		t.Fatalf("file or output changed: %q / %q", got, out.String())
	}
}

func TestStripRefusesAFactsDirectoryThatIsNotEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.ts")
	if err := os.WriteFile(path, []byte("// x\nlet a;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	facts := filepath.Join(dir, "facts")
	if err := os.MkdirAll(facts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(facts, "1.facts"), []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	if code := Strip("comment-strip.sh", []string{"--facts=" + facts, path}, dir, noRepository, &out, &errOut); code != exitDidNotRun {
		t.Fatalf("exit %d, want %d", code, exitDidNotRun)
	}
	if got, _ := os.ReadFile(path); string(got) != "// x\nlet a;\n" {
		t.Fatalf("a refused strip changed the file: %q", got)
	}
}

// The tool takes one path. Only a source file has comment blocks, so a second argument is a caller
// still passing the judge's grammar.
func TestStripRefusesASecondArgument(t *testing.T) {
	var out, errOut strings.Builder
	if code := Strip("comment-strip.sh", []string{"--facts=" + t.TempDir(), "comment", "x.md"}, t.TempDir(), noRepository, &out, &errOut); code != exitDidNotRun {
		t.Fatalf("exit %d, want %d", code, exitDidNotRun)
	}
}

func TestStripChangedRemovesOnlyTheBlocksTheDiffTouched(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	committed := "// human one\nfunc a() {}\n"
	if err := os.WriteFile(filepath.Join(repo, "f.go"), []byte(committed), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "f.go")
	git("commit", "-qm", "base")
	if err := os.WriteFile(filepath.Join(repo, "f.go"), []byte(committed+"// agent two\nfunc b() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	if code := Strip("comment-strip.sh", []string{"--facts=" + filepath.Join(repo, "facts"), "--changed", "f.go"}, repo, gitrepo.Exec{}, &out, &errOut); code != exitCut {
		t.Fatalf("exit %d, want %d: %s", code, exitCut, errOut.String())
	}
	got, _ := os.ReadFile(filepath.Join(repo, "f.go"))
	if want := "// human one\nfunc a() {}\nfunc b() {}\n"; string(got) != want {
		t.Fatalf("stripped file:\n%q\nwant\n%q", got, want)
	}
	if want := "f.go:3 1.facts\n"; out.String() != want {
		t.Fatalf("sites:\n%s\nwant\n%s", out.String(), want)
	}
}

// A file header sits above a blank line. The header goes and that blank becomes line 1. The writer
// then opens a file whose first line is empty, and the formatter drops it at the gate, putting a line
// the change never wrote into the change set. Five files in the #3194 run carried one.
func TestStripLeavesNoBlankLineWhereAFileHeaderWas(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.ts")
	if err := os.WriteFile(path, []byte("// A header nobody reads.\n\nexport function a() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	if code := Strip("comment-strip.sh", []string{"--facts=" + filepath.Join(dir, "facts"), path}, dir, noRepository, &out, &errOut); code != exitCut {
		t.Fatalf("exit %d — %s", code, errOut.String())
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), "export function a() {}\n"; got != want {
		t.Errorf("the strip left %q, and the blank above the declaration was the header's", got)
	}
}

// A file that opened on a blank line keeps it. Only blankness the strip created goes, or the strip is
// reformatting a file it was asked to read.
func TestStripKeepsABlankLineItDidNotCreate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.ts")
	if err := os.WriteFile(path, []byte("\n// A header nobody reads.\nexport function a() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	Strip("comment-strip.sh", []string{"--facts=" + filepath.Join(dir, "facts"), path}, dir, noRepository, &out, &errOut)
	body, _ := os.ReadFile(path)
	if got, want := string(body), "\nexport function a() {}\n"; got != want {
		t.Errorf("the strip left %q, and the file's own first line was blank", got)
	}
}

// The blank the header left goes, and every line under it moves up one. A site the writer opens on
// names a declaration, so a site one line high names the declaration before the block's own. Two
// files in the #3194 run reported sites that were each one high for this reason.
func TestStripNumbersSitesUnderAHeaderItTrimmed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.ts")
	source := "// A header nobody reads.\n\nexport function a() {}\n\n// A note about b.\nexport function b() {}\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	facts := filepath.Join(dir, "facts")
	var out, errOut strings.Builder
	if code := Strip("comment-strip.sh", []string{"--facts=" + facts, path}, dir, noRepository, &out, &errOut); code != exitCut {
		t.Fatalf("exit %d — %s", code, errOut.String())
	}
	if got, want := string(mustRead(t, path)), "export function a() {}\n\nexport function b() {}\n"; got != want {
		t.Fatalf("stripped file:\n%q\nwant\n%q", got, want)
	}
	// a() is line 1 of the stripped file and b() is line 3.
	if want := path + ":1 1.facts\n" + path + ":3 2.facts\n"; out.String() != want {
		t.Errorf("sites:\n%swant\n%s", out.String(), want)
	}
	if got, want := string(mustRead(t, filepath.Join(facts, "2.facts"))), path+":3\n// A note about b.\n"; got != want {
		t.Errorf("2.facts:\n%q\nwant\n%q", got, want)
	}
}

// The control for TestStripNumbersSitesUnderAHeaderItTrimmed: the same file with a blank line of
// its own on top. The strip leaves that blank, so each site keeps the number the blocks alone gave
// it.
func TestStripLeavesSitesWhereItTrimmedNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.ts")
	source := "\n// A header nobody reads.\n\nexport function a() {}\n\n// A note about b.\nexport function b() {}\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	if code := Strip("comment-strip.sh", []string{"--facts=" + filepath.Join(dir, "facts"), path}, dir, noRepository, &out, &errOut); code != exitCut {
		t.Fatalf("exit %d — %s", code, errOut.String())
	}
	if got, want := string(mustRead(t, path)), "\n\nexport function a() {}\n\nexport function b() {}\n"; got != want {
		t.Fatalf("stripped file:\n%q\nwant\n%q", got, want)
	}
	if want := path + ":3 1.facts\n" + path + ":5 2.facts\n"; out.String() != want {
		t.Errorf("sites:\n%swant\n%s", out.String(), want)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// The shift counts the lines the trim took, so a header over two blank lines moves its site by two.
func TestStripCountsEveryLineTheTrimTook(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.ts")
	if err := os.WriteFile(path, []byte("// A header nobody reads.\n\n\nexport function a() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	if code := Strip("comment-strip.sh", []string{"--facts=" + filepath.Join(dir, "facts"), path}, dir, noRepository, &out, &errOut); code != exitCut {
		t.Fatalf("exit %d — %s", code, errOut.String())
	}
	if got, want := string(mustRead(t, path)), "export function a() {}\n"; got != want {
		t.Fatalf("stripped file:\n%q\nwant\n%q", got, want)
	}
	if want := path + ":1 1.facts\n"; out.String() != want {
		t.Errorf("sites:\n%swant\n%s", out.String(), want)
	}
}

// The writer's audit classifies a noun as the code's own word by looking it up in this file. The
// brief named the file before any code wrote it. A writer following the audit then classified every
// noun as none of the three and rewrote until it declined the site.
func TestStripWritesTheIdentifierWordsBesideTheFacts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.ts")
	source := "// A note about the entry type.\nexport function readEntryType(book: Element): string {\n  return '';\n}\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	facts := filepath.Join(dir, "facts")
	var out, errOut strings.Builder
	if code := Strip("comment-strip.sh", []string{"--facts=" + facts, path}, dir, noRepository, &out, &errOut); code != exitCut {
		t.Fatalf("exit %d — %s", code, errOut.String())
	}
	body := string(mustRead(t, filepath.Join(facts, "identifiers.txt")))
	words := strings.Fields(body)
	held := map[string]bool{}
	for _, word := range words {
		held[word] = true
	}
	// The whole identifier and each of its humps, because a reader meets `readEntryType` in prose as
	// "entry type" and the audit compares the parts.
	for _, want := range []string{"readentrytype", "read", "entry", "type", "book", "element"} {
		if !held[want] {
			t.Errorf("identifiers.txt holds no %q, and the file spells it:\n%s", want, body)
		}
	}
	if held["note"] {
		t.Errorf("identifiers.txt holds a word from the comment, and the comment is what the writer replaces")
	}
	for i := 1; i < len(words); i++ {
		if words[i-1] >= words[i] {
			t.Fatalf("identifiers.txt is not sorted and deduplicated at %q, %q", words[i-1], words[i])
		}
	}
}
