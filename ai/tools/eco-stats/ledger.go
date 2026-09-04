package ecostats

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kk-flavor/tools/shell"
)

// stats.md owns the rules below — kk-reduce's SKILL.md says so, and its reader arrives at the file,
// not at the skill — so a fresh one has to carry them or it begins life with none of the protection
// the ledger exists to have. The shell version's copy had already drifted out of all three: no
// `, start`, no campaign-cut-versus-drift, no never-edited absolute.
//
// This and the live stats.md are a .md/source pair, which no drift check covers — the shared-region
// scan reads `*.sh` — so a case in stats_test.go compares the two directly. It runs only where there
// is no ledger yet, which is never the tree that would show it had drifted.
const ledgerSeed = "# Ecosystem size\n" +
	"\n" +
	"Appended by `kk-reduce` alone, via `~/.claude/skills/kk-reduce/scripts/stats.sh --append <note>`: one row before a\n" +
	"campaign, whose note ends `, start`, and one after. **A delta across that pair is the campaign's own\n" +
	"cut, not drift** — drift is measured from a closing row forward.\n" +
	"\n" +
	"`kk-reduce`'s own SKILL.md defines what each row's note carries. **A column is a measurement and is\n" +
	"never edited — however that edit is authorised**\n" +
	"(`~/.kk-flavor/standards/skill-protocol.md` → **Caller**): every delta is read off the rows below it,\n" +
	"so one corrected figure silently restates every campaign since.\n" +
	"\n" +
	"**A `+` on a row's always-loaded figure makes it a lower bound**: `stats.sh` named an `@import` it\n" +
	"could not resolve and left it uncounted. Read the delta between two marked rows as \"at least this\n" +
	"much\". From a marked row to an unmarked one, part of the rise is `stats.sh` resolving more rather\n" +
	"than the tree growing. The unmarked row's note says how much.\n" +
	"\n" +
	"| date | prose | scripts | always-loaded | skills | what ran |\n" +
	"|---|---|---|---|---|---|\n"

// Every write below is guarded: unguarded, an unwritable ledger still prints "appended to …" and
// exits 0, and the next pass reads a row that never landed as what happened.
func (s *stats) appendRow(self, note string, out, errOut io.Writer) int {
	// The row states how much of its always-loaded figure came from imports this run resolved, or a
	// reader comparing two rows cannot tell a tier that grew from one the tool merely started seeing.
	// Appended after the sanitising in noteFrom, and safe there: fixed text and a digit string forge
	// no row.
	if s.importResolvedWords != 0 {
		note += fmt.Sprintf(" [of the always-loaded figure, %d words are imports this run resolved]", s.importResolvedWords)
	}

	historyDir, ok := ownDirectory(self)
	if !ok {
		fmt.Fprintln(errOut, "stats.sh: could not resolve kk-reduce's own directory — the row was NOT appended.")
		return 2
	}
	history := shell.Join(historyDir, "stats.md")

	// Refused as a symlink on write, the way Root.Contains refuses one on read: following one
	// appends the row to whatever it points at, and a dangling one creates that file outright.
	if shell.IsSymlink(history) {
		fmt.Fprintf(errOut, "stats.sh: %s is a symlink — exit 2, no row was appended.\n", shell.Oneline(history))
		return 2
	}
	if !shell.IsRegularFile(history) {
		if err := createLedger(history); err != nil {
			fmt.Fprintf(errOut, "stats.sh: could not create %s — the row was NOT appended.\n", history)
			return 2
		}
	}
	row := fmt.Sprintf("| %s | %d | %d | %d%s | %d | %s |\n",
		time.Now().Format("2006-01-02"), s.prose, s.scripts, s.alwaysLoaded(), s.budgetMark(), s.skills, note)
	if err := appendTo(history, row); err != nil {
		fmt.Fprintf(errOut, "stats.sh: could not append to %s — the row was NOT recorded; the ledger still shows the previous pass.\n", history)
		return 2
	}
	fmt.Fprintf(out, "appended to %s\n", history)
	return 0
}

// `cd "$(dirname "$0")/.." && pwd` — the ledger belongs to kk-reduce and this program runs from the
// scripts/ directory inside it. Resolved lexically, the way the shell's logical `cd` resolved it, so
// a path reached through a symlinked directory keeps the name it was invoked by.
//
// The stub execs `-a "$0"`, so the self name is whatever the caller typed, and guessing wrong
// doesn't fail. `createLedger` creates a ledger under whatever directory came back. So a name that
// doesn't place the program is refused: no slash means PATH found it from a working directory that
// says nothing about where it lives, and a trailing slash names a directory rather than a file.
//
// `./stats.sh` does place it, and has to keep resolving. Don't re-guard on what `DirName` returned;
// it answers `.` for that one too. The test has to be the shape of the name.
func ownDirectory(self string) (string, bool) {
	if !strings.Contains(self, "/") || strings.HasSuffix(self, "/") {
		return "", false
	}
	dir, err := filepath.Abs(shell.DirName(self) + "/..")
	if err != nil || !shell.IsDir(dir) {
		return "", false
	}
	return dir, true
}

// O_EXCL rather than the shell's truncating `>`: between the symlink test above and this line the
// path can become one, and O_EXCL refuses exactly what that test would have caught. It also fails
// fast on a FIFO left at the path, which `>` would open and block on forever.
func createLedger(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(ledgerSeed); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func appendTo(path, row string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(row); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
