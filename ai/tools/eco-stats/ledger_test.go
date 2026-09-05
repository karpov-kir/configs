package ecostats_test

// What `--append` writes: the note it will take, the row it forges nothing extra into, and the header
// a ledger that does not exist yet is opened with.

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	ecostats "kk-flavor/tools/eco-stats"
)

// The ledger a case starts from when it needs one that already has its columns.
const ledgerColumns = "| date | prose | scripts | always-loaded | skills | what ran |\n|---|---|---|---|---|---|\n"

func TestAnOverLongNoteIsRefusedRatherThanAppended(t *testing.T) {
	t.Run("a 60-word note appends nothing, a short one appends one row", func(t *testing.T) {
		f := newRoot(t)
		ledger := f.newLedger("| date |\n")
		f.write(f.root+"/CLAUDE.md", "one two\n")
		f.run("--append", wordsCount(60), f.root)
		afterLong := rowsIn(t, ledger)
		f.run("--append", "short enough to keep", f.root)
		afterShort := rowsIn(t, ledger)
		if afterLong != 1 || afterShort != 2 {
			t.Errorf("rows after the long note: %d (want 1)\nrows after the short one: %d (want 2)",
				afterLong, afterShort)
		}
	})
}

func TestTheNoteCannotForgeALedgerRow(t *testing.T) {
	t.Run("a note carrying a newline and a pipe still writes exactly one 6-column row", func(t *testing.T) {
		f := newRoot(t)
		ledger := f.newLedger(ledgerColumns)
		f.write(f.root+"/CLAUDE.md", "one two\n")
		before := rowsIn(t, ledger)
		f.run("--append", "first line\nsecond line | with a pipe", f.root)
		appended := rowsIn(t, ledger) - before
		lines := strings.Split(strings.TrimSuffix(readFile(t, ledger), "\n"), "\n")
		row := lines[len(lines)-1]
		// Escaped pipes come out before the count: `\|` separates no column, so counting raw `|` would
		// read the guard working as the guard failing.
		columns := strings.Count(strings.ReplaceAll(row, `\|`, ""), "|")
		if appended != 1 || columns != 7 {
			t.Errorf("rows appended: %d (want 1)\nunescaped pipes: %d (want 7)\n%s", appended, columns, row)
		}
	})
}

func TestTheNoteCannotCarryAControlByteIntoTheLedger(t *testing.T) {
	// The note is written by whoever ran the skill, committed, and read back by a later pass. `\x1b[2K`
	// erases the line it lands on, so an ESC left in it edits the terminal of everyone who later reads
	// the ledger — the byte TestAMissingReadAlwaysTargetCannotReachTheTerminalRaw bars from a message,
	// barred here from the record.
	t.Run("an ESC in the note reaches neither the ledger nor the terminal", func(t *testing.T) {
		f := newRoot(t)
		ledger := f.newLedger(ledgerColumns)
		f.write(f.root+"/CLAUDE.md", "one two\n")
		before := rowsIn(t, ledger)
		stdout, stderr, status := f.run("--append", "ran a pass\x1b[2K and stopped", f.root)
		written := readFile(t, ledger)
		appended := rowsIn(t, ledger) - before

		// The row has to have landed: a run that appended nothing carries no ESC either, and would
		// pass a byte check while saying nothing about sanitising.
		if appended != 1 || strings.Contains(written+stdout+stderr, "\x1b") {
			t.Errorf("status: %d\nrows appended: %d (want 1)\n%s", status, appended,
				indent(strings.ReplaceAll(written+stdout+stderr, "\x1b", "<ESC>")))
		}
	})
}

func TestAMissingLedgerIsOpenedWithAHeaderAReaderCanUse(t *testing.T) {
	// The only case that leaves stats.md out of the fixture, so it is the only one that reaches the
	// header written when the ledger does not exist. Every other --append case creates the file first
	// and never sees that block. The `+` legend is asserted because it is the one thing in the header
	// a reader cannot reconstruct from the columns, and the header is where a fresh ledger states it.
	t.Run("creates the ledger with the column header, the + legend, and one row under them", func(t *testing.T) {
		f := newRoot(t)
		f.installStats()
		f.write(f.root+"/CLAUDE.md", "one two\n")
		fresh := f.root + "/skills/kk-reduce/stats.md"
		f.run("--append", "opening row", f.root)
		content := readFile(t, fresh)
		rows := rowsIn(t, fresh)
		if rows != 3 ||
			!strings.Contains(content, "| date | prose | scripts | always-loaded | skills | what ran |") ||
			!strings.Contains(content, "lower bound") {
			t.Errorf("lines starting '|': %d (want 3 — header, rule, row)\n%s", rows, indent(content))
		}
	})
}

func TestTheLedgerIsNotWrittenThroughASymlink(t *testing.T) {
	t.Run("refuses to append through a symlinked ledger", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "one two\n")
		f.installStats()
		f.write(f.base+"/decoy-target.md", "untouched\n")
		f.symlink(f.base+"/decoy-target.md", f.root+"/skills/kk-reduce/stats.md")
		f.run("--append", "should not land", f.root)
		if decoy := readFile(t, f.base+"/decoy-target.md"); decoy != "untouched\n" {
			t.Errorf("the decoy was written: %s", decoy)
		}
	})

	// The seed and the live ledger are a .md/source pair, which the shared-region scan cannot cover —
	// it reads `*.sh` only. They had already drifted: the seed carried none of the three rules the
	// real file owns, so a fresh install started with no protection for the column. Nothing but this
	// case notices, because the seed path runs only when there is no ledger — never on the tree that
	// would show it.
	t.Run("the seeded ledger says what the live one says", func(t *testing.T) {
		f := newRoot(t)
		f.installStats()
		f.run("--append", "seed, start", f.root)
		seeded := ledgerProse(readFile(t, f.root+"/skills/kk-reduce/stats.md"))
		live, err := os.ReadFile(liveLedger)
		if err != nil {
			t.Fatalf("the live ledger is what this case compares against: %v", err)
		}
		if seeded == "" || seeded != ledgerProse(string(live)) {
			t.Errorf("seeded:\n%slive:\n%s", indent(seeded), indent(ledgerProse(string(live))))
		}
	})
}

// argv[0] is where this program learns its own location, and the stub passes on whatever path the
// caller typed. `dirname` answers `.` for `./stats.sh` as readily as for a bare `stats.sh`, so the
// guard cannot be narrowed to `DirName`'s answer. The case that appends from `./stats.sh` is what
// holds that line.
func TestASelfNameThatDoesNotPlaceTheProgramAppendsNothing(t *testing.T) {
	// With the guard removed, a bare name resolves its ledger to `<cwd>/../stats.md` and *creates* it.
	// A row that ran from this package's own directory would put it at `ai/tools/stats.md`, a whole
	// seeded ledger written into the checkout. The Chdir into scratch is what keeps it out of there.
	// `scripts/` and `/` carry a slash, so a guard that only looks for one admits them both. `/` is
	// the worst: its ledger is `//stats.md`, at the filesystem root for any run with the privilege to
	// create it.
	for _, c := range []struct{ name, self string }{
		{"a bare name, found on PATH", "stats.sh"},
		{"all directory and no program", "scripts/"},
		{"the filesystem root", "/"},
	} {
		t.Run("refuses a self name that is "+c.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			f := newRoot(t)
			f.write(f.root+"/CLAUDE.md", "one two\n")

			var out, errOut bytes.Buffer
			status := ecostats.Run(c.self, []string{"--append", "should not land", f.root}, &out, &errOut)

			if status != 2 || !strings.Contains(errOut.String(), "could not resolve") {
				t.Errorf("status: %d (want 2)\n%s", status, indent(out.String()+errOut.String()))
			}
		})
	}

	// The boundary: this name does place the program, so it appends. Refusing it would take the
	// ledger away from anyone who runs the stub from the directory it lives in.
	t.Run("a self name relative to the working directory still appends its row", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "one two\n")
		ledger := f.newLedger(ledgerColumns)
		before := rowsIn(t, ledger)
		t.Chdir(f.root + "/skills/kk-reduce/scripts")

		var out, errOut bytes.Buffer
		if status := ecostats.Run("./stats.sh", []string{"--append", "lands", f.root}, &out, &errOut); status != 0 {
			t.Fatalf("status: %d\n%s", status, indent(out.String()+errOut.String()))
		}
		if rowsIn(t, ledger)-before != 1 {
			t.Errorf("rows appended: %d (want 1)", rowsIn(t, ledger)-before)
		}
	})

	// The other side: a self name that does say where the program lives still appends. Without it the
	// refusal above is satisfied by a tool that never writes a row at all.
	t.Run("a self name carrying a directory still appends its row", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "one two\n")
		ledger := f.newLedger(ledgerColumns)
		before := rowsIn(t, ledger)
		if _, stderr, status := f.run("--append", "lands", f.root); status != 0 {
			t.Fatalf("status: %d\n%s", status, indent(stderr))
		}
		if rowsIn(t, ledger)-before != 1 {
			t.Errorf("rows appended: %d (want 1)", rowsIn(t, ledger)-before)
		}
	})
}

// The tree's own ledger, from the package directory `go test` runs in.
const liveLedger = "../../skills/kk-reduce/stats.md"

// `awk 'BEGIN { for (i = 1; i <= n; i++) printf "word%d ", i }'`.
func wordsCount(n int) string {
	var note strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&note, "word%d ", i)
	}
	return note.String()
}
