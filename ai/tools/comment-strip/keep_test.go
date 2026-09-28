package commentstrip

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"configs/ai/tools/shell"
)

// rulesHome is a home directory holding the rules a block is written under.
func rulesHome(t *testing.T, style string) string {
	t.Helper()
	home := t.TempDir()
	for _, path := range rulePaths {
		full := filepath.Join(home, ".kk-flavor", path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(style+path), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	return home
}

const keptSource = "import { keys } from './keys';\n\n" +
	"// A ledger build answers `canPost` for its own scheme alone, so this function asks it once per scheme.\n" +
	"export function claimFor(scheme: string): boolean {\n" +
	"  return keys.canPost(scheme);\n" +
	"}\n"

const keptRecord = "fact: a ledger build answers canPost for its own scheme alone\n" +
	"bears_on: claimFor\n" +
	"does: returns keys.canPost(scheme)\n"

// archiveWritten archives the fixture's block as run11 wrote it.
func archiveWritten(t *testing.T, f *fixture, archive string) {
	t.Helper()
	record := filepath.Join(f.dir, "record.txt")
	if err := os.WriteFile(record, []byte(keptRecord), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	if code := Strip("comment-strip.sh", []string{"--archive=" + archive, "--written=run11", f.path, "4", record},
		f.dir, noRepository, &out, &errOut); code != exitClean {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
}

// A block the lane wrote stands byte for byte in the next run while its record holds, and the strip
// offers no site for it. Run 12 reworded about fifty blocks run 11 had written correctly.
func TestABlockWhoseRecordHoldsStandsAsWritten(t *testing.T) {
	rulesHome(t, "rules one ")
	f := newFixture(t, "f.ts", keptSource)
	archive := filepath.Join(f.dir, "archive")
	archiveWritten(t, f, archive)
	said := f.run("--archive=" + archive)
	if said.code != exitClean || f.body() != keptSource || !strings.Contains(said.stderr, "kept as run11 wrote it") {
		t.Fatalf("exit %d, stderr %s, file:\n%s", said.code, said.stderr, f.body())
	}
}

// A changed rule, a changed body, a contradiction and a review sending the block back each reopen it.
func TestAKeptBlockReopensOnARuleABodyAContradictionOrAReview(t *testing.T) {
	cases := map[string]func(t *testing.T, f *fixture, archive string) []string{
		"a changed rule": func(t *testing.T, f *fixture, archive string) []string {
			rulesHome(t, "rules two ")
			return nil
		},
		"a changed body": func(t *testing.T, f *fixture, archive string) []string {
			f.write(strings.Replace(keptSource, "keys.canPost(scheme)", "keys.canPost(scheme) === true", 1))
			return nil
		},
		"a contradiction": func(t *testing.T, f *fixture, archive string) []string {
			var out, errOut strings.Builder
			block := "// A ledger build answers `canPost` for its own scheme alone, so this function asks it once per scheme.\n"
			if code := Strip("comment-strip.sh", []string{"--archive=" + archive, "--contradict=run12", f.path,
				recordID(block), "canPost answers for every scheme"}, f.dir, noRepository, &out, &errOut); code != exitClean {
				t.Fatalf("exit %d: %s", code, errOut.String())
			}
			return nil
		},
		"a review": func(t *testing.T, f *fixture, archive string) []string {
			return []string{"--lines=3"}
		},
	}
	for name, reopen := range cases {
		t.Run(name, func(t *testing.T) {
			rulesHome(t, "rules one ")
			f := newFixture(t, "f.ts", keptSource)
			archive := filepath.Join(f.dir, "archive")
			archiveWritten(t, f, archive)
			extra := reopen(t, f, archive)
			said := f.run(append([]string{"--archive=" + archive}, extra...)...)
			if said.code != exitCut || strings.Contains(f.body(), "// A ledger build") {
				t.Fatalf("the block stood: exit %d, stderr %s", said.code, said.stderr)
			}
		})
	}
}

// A kept block's older claims stay unread. The strip would offer them at the declaration the kept
// block stands on, and a writer would write a second block there.
func TestAKeptBlockLeavesItsOlderRecordUnoffered(t *testing.T) {
	rulesHome(t, "rules one ")
	f := newFixture(t, "f.ts", strings.Replace(keptSource, "// A ledger build", "// An older block. A ledger build", 1))
	archive := filepath.Join(f.dir, "archive")
	f.cut("--archive=" + archive)
	// The writer wrote the block run 11 archived.
	f.write(keptSource)
	if err := os.RemoveAll(f.facts); err != nil {
		t.Fatal(err)
	}
	archiveWritten(t, f, archive)
	if said := f.run("--archive=" + archive); said.code != exitClean || said.stdout != "" {
		t.Fatalf("exit %d, sites %q: %s", said.code, said.stdout, said.stderr)
	}
}

// A file header and the block under it sit on the same declaration. A key without the block let one
// entry replace the other, and the next run wrote the replaced block again.
func TestAHeaderAndTheBlockUnderItBothStand(t *testing.T) {
	rulesHome(t, "rules one ")
	source := "// A ledger answers for one scheme at a time, so this function asks one scheme.\n\n" + strings.TrimPrefix(keptSource, "import { keys } from './keys';\n\n")
	source = "import { keys } from './keys';\n" + source
	f := newFixture(t, "f.ts", source)
	archive := filepath.Join(f.dir, "archive")
	record := filepath.Join(f.dir, "record.txt")
	if err := os.WriteFile(record, []byte(keptRecord), 0o644); err != nil {
		t.Fatal(err)
	}
	// The header is named by its own first line, and the block under it by its declaration.
	for _, line := range []string{"2", "5"} {
		var out, errOut strings.Builder
		if code := Strip("comment-strip.sh", []string{"--archive=" + archive, "--written=run13", f.path, line, record},
			f.dir, noRepository, &out, &errOut); code != exitClean {
			t.Fatalf("line %s: exit %d: %s", line, code, errOut.String())
		}
	}
	said := f.run("--archive=" + archive)
	if said.code != exitClean || strings.Count(said.stderr, "kept as run13") != 2 {
		t.Fatalf("exit %d: %s", said.code, said.stderr)
	}
}

// writeRecord archives the block on the declaration at `line` with this record, as run13 wrote it.
func writeRecord(t *testing.T, f *fixture, archive, line, record string) {
	t.Helper()
	path := filepath.Join(f.dir, "record.txt")
	if err := os.WriteFile(path, []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	if code := Strip("comment-strip.sh", []string{"--archive=" + archive, "--written=run13", f.path, line, path},
		f.dir, noRepository, &out, &errOut); code != exitClean {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
}

// A block holding only a summary carries an empty record, since a record belongs to a note. Run 13 had
// four such blocks, and the check wanted slots they could not have, so every run rewrote them.
func TestASummaryAloneIsKeptWithoutARecord(t *testing.T) {
	rulesHome(t, "rules one ")
	source := "// Lists the postings of a closed book, newest first.\nexport function listPostings(book: LedgerBook): Posting[] {\n  return book.postings.slice().reverse();\n}\n"
	f := newFixture(t, "f.ts", source)
	archive := filepath.Join(f.dir, "archive")
	writeRecord(t, f, archive, "2", "")
	if said := f.run("--archive=" + archive); said.code != exitClean || f.body() != source {
		t.Fatalf("exit %d: %s", said.code, said.stderr)
	}
}

const literalSource = "// A ledger export names each scheme in its own casing, so this constant keeps the casing.\n" +
	"export const schemeTags = {\n" +
	"  // An accrual row predates the casing rule.\n" +
	"  accrual: 'Accrual',\n" +
	"  deferred: 'Deferred',\n" +
	"};\n"

const literalRecord = "fact: a ledger export names each scheme in its own casing\nbears_on: schemeTags\ndoes: none\n"

// A note inside a body that a later run rewrote leaves the block over the body standing. Run 13's
// loop rewrote a row note in an object literal, and the block over the literal reopened.
func TestARewrittenNoteInsideABodyLeavesTheBlockOverItStanding(t *testing.T) {
	rulesHome(t, "rules one ")
	f := newFixture(t, "f.ts", literalSource)
	archive := filepath.Join(f.dir, "archive")
	writeRecord(t, f, archive, "2", literalRecord)
	f.write(strings.Replace(literalSource, "An accrual row predates the casing rule.", "The accrual row predates the rule on casing.", 1))
	said := f.run("--archive=" + archive)
	if !strings.Contains(said.stderr, ":1: kept as run13") {
		t.Fatalf("the block over the literal reopened: %s", said.stderr)
	}
}

// An entry run 13 wrote hashed the span with its comment lines, and it is still read.
func TestAnEntryHashedWithItsCommentLinesStillStands(t *testing.T) {
	rulesHome(t, "rules one ")
	f := newFixture(t, "f.ts", literalSource)
	archive := filepath.Join(f.dir, "archive")
	writeRecord(t, f, archive, "2", literalRecord)
	held := readWritten(archive, f.path)
	lines := shell.SplitLines(literalSource)
	held[0].Span = spanSum(declarationSpan(lines, 2))
	body, _ := json.Marshal(held)
	if err := os.WriteFile(writtenName(archive, f.path), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if said := f.run("--archive=" + archive); !strings.Contains(said.stderr, ":1: kept as run13") {
		t.Fatalf("an entry hashed the older way reopened: %s", said.stderr)
	}
}

const hostSource = "// A ledger keeps one clearing host per book, so `clearingHost` holds one name.\n" +
	"export interface BookHosts {\n" +
	"  clearingHost: string;\n" +
	"}\n"

const hostRecord = "fact: a ledger keeps one clearing host per book\nbears_on: clearingHost\ndoes: none\n"

func renameIn(t *testing.T, f *fixture, archive string) outcome {
	t.Helper()
	var out, errOut strings.Builder
	code := Strip("comment-strip.sh", []string{"--rename=clearingHost=settlementHost", "--archive=" + archive, f.path},
		f.dir, noRepository, &out, &errOut)
	if code == exitDidNotRun {
		t.Fatalf("the rename did not run: %s", errOut.String())
	}
	return outcome{code: code, stdout: out.String(), stderr: errOut.String()}
}

// A rename reaches the blocks the archive keeps. Run 13 renamed a field, the archived record still
// named the old one, and two blocks would be written again on unchanged code.
func TestARenameReachesTheArchivedBlock(t *testing.T) {
	for name, commentRenamed := range map[string]bool{"with the comment": false, "after the comment": true} {
		t.Run(name, func(t *testing.T) {
			rulesHome(t, "rules one ")
			f := newFixture(t, "f.ts", hostSource)
			archive := filepath.Join(f.dir, "archive")
			writeRecord(t, f, archive, "2", hostRecord)
			// The refactor lane renames the field in the code. Run 14 met the archive after run 13 had
			// renamed the comment too.
			renamed := strings.Replace(hostSource, "  clearingHost: string;", "  settlementHost: string;", 1)
			if commentRenamed {
				renamed = strings.ReplaceAll(renamed, "clearingHost", "settlementHost")
			}
			f.write(renamed)
			renameIn(t, f, archive)
			if said := f.run("--archive=" + archive); said.code != exitClean || strings.Contains(f.body(), "clearingHost") {
				t.Fatalf("exit %d: %s\n%s", said.code, said.stderr, f.body())
			}
		})
	}
}

// A rename blesses no other change to the code under a block.
func TestARenameLeavesAnotherCodeChangeReopening(t *testing.T) {
	rulesHome(t, "rules one ")
	f := newFixture(t, "f.ts", hostSource)
	archive := filepath.Join(f.dir, "archive")
	writeRecord(t, f, archive, "2", hostRecord)
	f.write(strings.Replace(hostSource, "  clearingHost: string;", "  settlementHost: string;\n  backupHost?: string;", 1))
	renameIn(t, f, archive)
	if said := f.run("--archive=" + archive); said.code != exitCut {
		t.Fatalf("a block over a second code change stood: exit %d: %s", said.code, said.stderr)
	}
}

// The dry keep test reads each block against the archive and names why one reopens. It only reads.
func TestKeepVerdictsNameWhyABlockReopens(t *testing.T) {
	rulesHome(t, "rules one ")
	f := newFixture(t, "f.ts", keptSource)
	archive := filepath.Join(f.dir, "archive")
	archiveWritten(t, f, archive)
	lines := shell.SplitLines(keptSource)
	if got := KeepVerdicts(archive, f.path, lines); len(got) != 1 || got[0].Run != "run11" {
		t.Fatalf("got %+v, want the block kept as run11 wrote it", got)
	}
	changed := shell.SplitLines(strings.Replace(keptSource, "keys.canPost(scheme)", "keys.canPost(scheme) === true", 1))
	if got := KeepVerdicts(archive, f.path, changed); len(got) != 1 || got[0].Reopened != "the code under it changed" {
		t.Fatalf("got %+v, want the changed code named", got)
	}
	if f.body() != keptSource {
		t.Fatal("the dry test changed the file")
	}
}
