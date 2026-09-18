package commentstrip

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripRemovesEveryBlockAndRecordsItsSiteInTheStrippedFile(t *testing.T) {
	source := "/** Lists things. */\nfunction a() {}\n\n// A note about b.\n// Its second line.\n\nfunction b() {}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "f.ts")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	facts := filepath.Join(dir, "facts")
	var out, errOut strings.Builder
	code := Strip("comment-strip.sh", []string{"--facts=" + facts, path}, dir, &out, &errOut)
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
	if code := Strip("comment-strip.sh", []string{"--facts=" + filepath.Join(dir, "facts"), path}, dir, &out, &errOut); code != exitCut {
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
	if code := Strip("comment-strip.sh", []string{"--facts=" + filepath.Join(dir, "facts"), path}, dir, &out, &errOut); code != exitClean {
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
	if code := Strip("comment-strip.sh", []string{"--facts=" + facts, path}, dir, &out, &errOut); code != exitDidNotRun {
		t.Fatalf("exit %d, want %d", code, exitDidNotRun)
	}
	if got, _ := os.ReadFile(path); string(got) != "// x\nlet a;\n" {
		t.Fatalf("a refused strip changed the file: %q", got)
	}
}

// The tool takes one path and no kind: only a source file has comment blocks, so a second argument is
// a caller still passing the judge's grammar.
func TestStripRefusesASecondArgument(t *testing.T) {
	var out, errOut strings.Builder
	if code := Strip("comment-strip.sh", []string{"--facts=" + t.TempDir(), "comment", "x.md"}, t.TempDir(), &out, &errOut); code != exitDidNotRun {
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
	if code := Strip("comment-strip.sh", []string{"--facts=" + filepath.Join(repo, "facts"), "--changed", "f.go"}, repo, &out, &errOut); code != exitCut {
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
