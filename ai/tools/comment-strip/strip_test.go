package commentstrip

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gitrepo "configs/ai/tools/repo"
	"configs/ai/tools/repo/repotest"
)

// What a case hands a run that never passes --changed. Only that option asks the repository anything,
// and a run that asked anyway panics on this nil value. The case arranged no answers for it to read.
var noRepository gitrepo.Git

// One source file under a directory of its own, with the facts directory beside it. Every case here
// strips one path, so the fixture holds that path and that directory in place of respelling both.
type fixture struct {
	t     *testing.T
	dir   string
	path  string
	facts string
}

// What one strip exited with and printed.
type outcome struct {
	code   int
	stdout string
	stderr string
}

func newFixture(t *testing.T, name, source string) *fixture {
	t.Helper()
	f := &fixture{t: t, dir: t.TempDir()}
	f.path, f.facts = filepath.Join(f.dir, name), filepath.Join(f.dir, "facts")
	f.write(source)
	return f
}

func (f *fixture) write(source string) {
	f.t.Helper()
	if err := os.WriteFile(f.path, []byte(source), 0o644); err != nil {
		f.t.Fatalf("could not write %s: %v", f.path, err)
	}
}

// run strips the fixture's own file. The options a case adds go ahead of the path, which is the
// grammar the tool takes.
func (f *fixture) run(options ...string) outcome {
	f.t.Helper()
	var out, errOut strings.Builder
	args := append(append([]string{"--facts=" + f.facts}, options...), f.path)
	code := Strip("comment-strip.sh", args, f.dir, noRepository, &out, &errOut)
	return outcome{code: code, stdout: out.String(), stderr: errOut.String()}
}

// The cut helper wraps run and fails a run that cut no block, so each case inherits that check.
// Every assertion under a strip reads a file the run was supposed to have rewritten.
func (f *fixture) cut(options ...string) outcome {
	f.t.Helper()
	said := f.run(options...)
	if said.code != exitCut {
		f.t.Fatalf("exit %d, want %d: %s", said.code, exitCut, said.stderr)
	}
	return said
}

// body is the file as the strip left it.
func (f *fixture) body() string { return string(mustRead(f.t, f.path)) }

// fact is one facts file the run wrote, under the name a site line gives it.
func (f *fixture) fact(name string) string {
	return string(mustRead(f.t, filepath.Join(f.facts, name)))
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestStripRemovesEveryBlockAndRecordsItsSiteInTheStrippedFile(t *testing.T) {
	f := newFixture(t, "f.ts",
		"/** Lists things. */\nfunction a() {}\n\n// A note about b.\n// Its second line.\n\nfunction b() {}\n")
	said := f.cut()
	if want := "function a() {}\n\n\nfunction b() {}\n"; f.body() != want {
		t.Fatalf("stripped file:\n%q\nwant\n%q", f.body(), want)
	}
	// Sites are lines of the stripped file: a() is its line 1, b() its line 4.
	if want := f.path + ":1 1.facts\n" + f.path + ":4 2.facts\n"; said.stdout != want {
		t.Fatalf("sites:\n%s\nwant\n%s", said.stdout, want)
	}
	if want := f.path + ":4\n// A note about b.\n// Its second line.\n"; f.fact("2.facts") != want {
		t.Fatalf("2.facts:\n%q\nwant\n%q", f.fact("2.facts"), want)
	}
}

func TestStripKeepsAComentTheToolchainReads(t *testing.T) {
	f := newFixture(t, "f.ts", "// eslint-disable-next-line no-console\nconsole.log(1);\n// a note\nconst x = 1;\n")
	said := f.cut()
	if want := "// eslint-disable-next-line no-console\nconsole.log(1);\nconst x = 1;\n"; f.body() != want {
		t.Fatalf("stripped file:\n%q\nwant\n%q", f.body(), want)
	}
	if !strings.Contains(said.stderr, f.path+":1: a comment the toolchain reads, kept") {
		t.Fatalf("the kept directive is not named on stderr:\n%s", said.stderr)
	}
}

// A comment is prose to a reader and input to a check at the same time. voice-check's block limit
// asked for a header to be split, the split put a blank line through one, and two scripts stopped
// reaching their binary in silence. A writer restores only the lines the strip showed it.
func TestStripKeepsTheLinesThisRepositorysChecksRead(t *testing.T) {
	for _, c := range []struct {
		name   string
		source string
		want   string
	}{
		{"a usage line in the header",
			"#!/usr/bin/env bash\n# A note about the script.\n# usage: t.sh [--gate]\nset -eu\n# a note\ntrue\n",
			"#!/usr/bin/env bash\n# A note about the script.\n# usage: t.sh [--gate]\nset -eu\ntrue\n"},
		{"a test declaration in the header",
			"#!/usr/bin/env bash\n# untested: a fixture.\nset -eu\n# a note\ntrue\n",
			"#!/usr/bin/env bash\n# untested: a fixture.\nset -eu\ntrue\n"},
		{"a named suite in the header",
			"#!/usr/bin/env bash\n# Covered by t-test.sh beside it.\nset -eu\n# a note\ntrue\n",
			"#!/usr/bin/env bash\n# Covered by t-test.sh beside it.\nset -eu\ntrue\n"},
		// The markers and the text between them are one run of comment lines, so the whole region is the
		// block that holds a marker and the whole region stays. A note under the region is its own block
		// and goes, which is what the writer is there to replace.
		{"a shared-region marker anywhere",
			"#!/usr/bin/env bash\nset -eu\n# --- shared:tool-stub ---\n# The stub.\n# --- end shared:tool-stub ---\nstub\n# a note\ntrue\n",
			"#!/usr/bin/env bash\nset -eu\n# --- shared:tool-stub ---\n# The stub.\n# --- end shared:tool-stub ---\nstub\ntrue\n"},
		// The bound. A header scan ends at the first line of code, so a usage line under the code has
		// no reader, and the writer may rewrite it as prose.
		{"a usage line below the header is prose",
			"#!/usr/bin/env bash\nset -eu\n# usage: t.sh [--gate]\ntrue\n",
			"#!/usr/bin/env bash\nset -eu\ntrue\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t, "t.sh", c.source)
			f.run()
			if f.body() != c.want {
				t.Fatalf("stripped file:\n%q\nwant\n%q", f.body(), c.want)
			}
		})
	}
}

// The archive held every record for the file under one prefix, and each site read the lot. A writer
// at one site was handed another site's claims, under a marker saying they were made at this one. A
// run over a two-site file reported the whole file's history twice.
func TestASitesHistoryIsItsOwn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.go")
	archive := filepath.Join(dir, "archive")
	first := "package a\n\n// Note about alpha.\nfunc alpha() {}\n\n// Note about beta.\nfunc beta() {}\n"
	stripOnce(t, dir, path, archive, first, "facts1")
	second := "package a\n\n// Fresh alpha.\nfunc alpha() {}\n\n// Fresh beta.\nfunc beta() {}\n"
	facts := stripOnce(t, dir, path, archive, second, "facts2")
	if got := read(t, facts, "1.facts"); !strings.Contains(got, "Note about alpha") {
		t.Errorf("alpha lost its own history:\n%s", got)
	} else if strings.Contains(got, "Note about beta") {
		t.Errorf("alpha was handed beta's history as its own:\n%s", got)
	}
	if got := read(t, facts, "2.facts"); !strings.Contains(got, "Note about beta") {
		t.Errorf("beta lost its own history:\n%s", got)
	} else if strings.Contains(got, "Note about alpha") {
		t.Errorf("beta was handed alpha's history as its own:\n%s", got)
	}
}

// Sites are numbered in the stripped file. A block's own height leaves them where they are, and code
// over them moves them. The declaration carries a site's history across that move. A run matching on
// the line hands beta an empty history and offers its old claims as a site of their own.
func TestASiteKeepsItsHistoryWhenCodeAboveMovesIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "w.go")
	archive := filepath.Join(dir, "archive")
	stripOnce(t, dir, path, archive, "package d\n\n// Alpha note.\nfunc alpha() {}\n\n// Beta note, the original.\nfunc beta() {}\n", "facts1")
	moved := "package d\n\n// Alpha note.\nfunc alpha() {}\n\nfunc inserted() {\n\treturn\n}\n\n// Beta note, rewritten.\nfunc beta() {}\n"
	facts := stripOnce(t, dir, path, archive, moved, "facts2")
	if got := read(t, facts, "2.facts"); !strings.Contains(got, "Beta note, the original") {
		t.Errorf("beta's history did not follow its site:\n%s", got)
	}
	// The directory also holds identifiers.txt, so the count is of facts files alone.
	written, _ := filepath.Glob(filepath.Join(facts, "*.facts"))
	if len(written) != 2 {
		t.Errorf("%d facts file(s), want 2 — a third is beta's old claims offered as a site of their own", len(written))
	}
}

// A run that removed a block and left the site empty kept the claims in the archive. No unit found
// that site afterwards, so the strip exited clean and the claims sat unread. A site stands on its
// declaration, and a block standing there is no part of it.
func TestAnArchivedSiteWithNoBlockIsStillASite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "y.go")
	archive := filepath.Join(dir, "archive")
	stripOnce(t, dir, path, archive, "package b\n\n// A fact about the vendor export.\nfunc gamma() {}\n", "facts1")
	var out, errOut strings.Builder
	code := Strip("comment-strip.sh", []string{"--facts=" + filepath.Join(dir, "facts2"), "--archive=" + archive, path},
		dir, noRepository, &out, &errOut)
	if code != exitCut {
		t.Fatalf("exit %d, want %d — the archived site was not offered: %s", code, exitCut, errOut.String())
	}
	got := read(t, filepath.Join(dir, "facts2"), "1.facts")
	if !strings.Contains(got, "A fact about the vendor export") {
		t.Errorf("the archived claim did not reach the writer:\n%s", got)
	}
}

// stripOnce writes `body` to `path`, strips it into a fresh facts directory beside it, and returns
// that directory.
func stripOnce(t *testing.T, dir, path, archive, body, facts string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	into := filepath.Join(dir, facts)
	if code := Strip("comment-strip.sh", []string{"--facts=" + into, "--archive=" + archive, path},
		dir, noRepository, &out, &errOut); code != exitCut {
		t.Fatalf("exit %d, want %d: %s", code, exitCut, errOut.String())
	}
	return into
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(body)
}

func TestStripWithNothingToRemoveLeavesTheFileAndExitsClean(t *testing.T) {
	source := "function a() {}\n"
	f := newFixture(t, "f.ts", source)
	said := f.run()
	if said.code != exitClean {
		t.Fatalf("exit %d, want %d: %s", said.code, exitClean, said.stderr)
	}
	if f.body() != source || said.stdout != "" {
		t.Fatalf("file or output changed: %q / %q", f.body(), said.stdout)
	}
}

func TestStripRefusesAFactsDirectoryThatIsNotEmpty(t *testing.T) {
	source := "// x\nlet a;\n"
	f := newFixture(t, "f.ts", source)
	if err := os.MkdirAll(f.facts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.facts, "1.facts"), []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if said := f.run(); said.code != exitDidNotRun {
		t.Fatalf("exit %d, want %d", said.code, exitDidNotRun)
	}
	if f.body() != source {
		t.Fatalf("a refused strip changed the file: %q", f.body())
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

// --changed keeps the blocks the diff did not touch, so the diff is what this case states. What git
// prints for a change is repo.Exec's subject, and repo/exec_test.go holds it against a real git. A
// diff derived here would be this fixture's own, and the strip would be measured against that.
func TestStripChangedRemovesOnlyTheBlocksTheDiffTouched(t *testing.T) {
	f := newFixture(t, "f.go", "// human one\nfunc a() {}\n// agent two\nfunc b() {}\n")
	git := repotest.New(f.dir)
	// The change added lines 3 and 4, and the block on line 1 was there before it.
	git.Diff("diff --git a/f.go b/f.go\n--- a/f.go\n+++ b/f.go\n@@ -1,2 +1,4 @@\n" +
		" // human one\n func a() {}\n+// agent two\n+func b() {}\n")

	var out, errOut strings.Builder
	code := Strip("comment-strip.sh", []string{"--facts=" + f.facts, "--changed", "f.go"}, f.dir, git, &out, &errOut)
	if code != exitCut {
		t.Fatalf("exit %d, want %d: %s", code, exitCut, errOut.String())
	}
	if want := "// human one\nfunc a() {}\nfunc b() {}\n"; f.body() != want {
		t.Fatalf("stripped file:\n%q\nwant\n%q", f.body(), want)
	}
	if want := "f.go:3 1.facts\n"; out.String() != want {
		t.Fatalf("sites:\n%s\nwant\n%s", out.String(), want)
	}
}

// A file header sits above a blank line, and that blank goes when the header goes. A blank left on
// top makes line 1 empty in the file the writer opens. The formatter drops it at the gate, putting a
// line the change never wrote into the change set. Five files in the #3194 run carried one.
func TestStripNumbersSitesUnderAHeaderItTrimmed(t *testing.T) {
	f := newFixture(t, "f.ts",
		"// A header nobody reads.\n\nexport function a() {}\n\n// A note about b.\nexport function b() {}\n")
	said := f.cut()
	if want := "export function a() {}\n\nexport function b() {}\n"; f.body() != want {
		t.Fatalf("stripped file:\n%q\nwant\n%q", f.body(), want)
	}
	// Every line under the trim moves up. A site the writer opens on names a declaration, so a site
	// one line high names the declaration before the block's own. Two files in the #3194 run reported
	// sites that were each one high for this reason.
	// a() is line 1 of the stripped file and b() is line 3.
	if want := f.path + ":1 1.facts\n" + f.path + ":3 2.facts\n"; said.stdout != want {
		t.Errorf("sites:\n%swant\n%s", said.stdout, want)
	}
	if want := f.path + ":3\n// A note about b.\n"; f.fact("2.facts") != want {
		t.Errorf("2.facts:\n%q\nwant\n%q", f.fact("2.facts"), want)
	}
}

// The control for TestStripNumbersSitesUnderAHeaderItTrimmed: the same file with a blank line of its
// own on top. Only blankness the strip created goes, or the strip is reformatting a file it was asked
// to read. The file's own first line stays, and every site keeps the number the blocks alone gave it.
func TestStripLeavesSitesWhereItTrimmedNothing(t *testing.T) {
	f := newFixture(t, "f.ts",
		"\n// A header nobody reads.\n\nexport function a() {}\n\n// A note about b.\nexport function b() {}\n")
	said := f.cut()
	if want := "\n\nexport function a() {}\n\nexport function b() {}\n"; f.body() != want {
		t.Fatalf("stripped file:\n%q\nwant\n%q", f.body(), want)
	}
	if want := f.path + ":3 1.facts\n" + f.path + ":5 2.facts\n"; said.stdout != want {
		t.Errorf("sites:\n%swant\n%s", said.stdout, want)
	}
}

// The shift counts the lines the trim took, so a header over two blank lines moves its site by two.
func TestStripCountsEveryLineTheTrimTook(t *testing.T) {
	f := newFixture(t, "f.ts", "// A header nobody reads.\n\n\nexport function a() {}\n")
	said := f.cut()
	if want := "export function a() {}\n"; f.body() != want {
		t.Fatalf("stripped file:\n%q\nwant\n%q", f.body(), want)
	}
	if want := f.path + ":1 1.facts\n"; said.stdout != want {
		t.Errorf("sites:\n%swant\n%s", said.stdout, want)
	}
}

// The writer's audit classifies a noun as the code's own word by looking it up in this file. The
// brief named the file before any code wrote it. A writer following the audit then classified every
// noun as none of the three and rewrote until it declined the site.
func TestStripWritesTheIdentifierWordsBesideTheFacts(t *testing.T) {
	f := newFixture(t, "f.ts",
		"// A note about the entry type.\nexport function readEntryType(book: Element): string {\n  return '';\n}\n")
	f.cut()
	body := f.fact("identifiers.txt")
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

// A fact a writer dropped on one run is gone from every later run of the same change set. The strip
// reads the block as it stands, and the dropped text lives in an earlier run's facts alone. With an
// archive, question 3 sees every claim ever made at the site.
func TestTheFactsFileCarriesWhatEarlierRunsClaimed(t *testing.T) {
	f := newFixture(t, "f.ts", "// The vendor's export predates the closing profile.\nexport const ROWS = [1];\n")
	archive := "--archive=" + filepath.Join(f.dir, "archive")
	// One strip of a rewritten file. The facts directory goes first, because a run refuses one that
	// is not empty, and the archive is what carries the earlier run across.
	restrip := func(source string) string {
		t.Helper()
		if err := os.RemoveAll(f.facts); err != nil {
			t.Fatal(err)
		}
		f.write(source)
		f.cut(archive)
		return f.fact("1.facts")
	}

	// A site with no history reads as it always did.
	f.cut(archive)
	first := f.fact("1.facts")
	if strings.Contains(first, earlierMarker) {
		t.Errorf("a site with no history carried an earlier-run marker:\n%s", first)
	}
	if !strings.Contains(first, "vendor's export") {
		t.Errorf("the standing block is missing:\n%s", first)
	}

	// The writer drops that claim and writes a different one. The next strip carries both.
	second := restrip("// The estate cannot fetch the newer export.\nexport const ROWS = [1];\n")
	if !strings.Contains(second, "estate cannot fetch") {
		t.Errorf("the standing block is missing:\n%s", second)
	}
	if !strings.Contains(second, "vendor's export") {
		t.Errorf("the claim an earlier run recorded is gone, which is the defect:\n%s", second)
	}
	if !strings.Contains(second, earlierMarker) {
		t.Errorf("the earlier claim is unmarked:\n%s", second)
	}

	// A third run sees both earlier claims.
	third := restrip("// A third claim.\nexport const ROWS = [1];\n")
	for _, want := range []string{"A third claim", "estate cannot fetch", "vendor's export"} {
		if !strings.Contains(third, want) {
			t.Errorf("two earlier runs' claims are not all here, %q is missing:\n%s", want, third)
		}
	}

	// A block byte-identical to one already held is not handed over twice.
	again := restrip("// A third claim.\nexport const ROWS = [1];\n")
	if n := strings.Count(again, "A third claim"); n != 1 {
		t.Errorf("the standing block appears %d times, and a repeat of it is the same claim:\n%s", n, again)
	}
}

// An archive a caller leaves unnamed leaves the facts file as it was. A lane wanting no history pays
// for none.
func TestAStripWithNoArchiveCarriesOnlyTheStandingBlock(t *testing.T) {
	f := newFixture(t, "f.ts", "// One claim.\nexport const ROWS = [1];\n")
	f.cut()
	if body := f.fact("1.facts"); strings.Contains(body, earlierMarker) {
		t.Errorf("a run with no archive carried an earlier-run marker:\n%s", body)
	}
}
