package commentstrip

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	readerjudge "configs/ai/tools/reader-judge"
	"configs/ai/tools/shell"
	voicecheck "configs/ai/tools/voice-check"
)

// A block the lane wrote stands byte for byte in the next run while its record holds. Every run
// stripped every site and wrote each block again. Run 12 reworded about fifty blocks run 11 had
// written correctly, with no change of fact or tie, and a reviewer read those rewordings as a diff.

// The lane archives each block it wrote with its record, `--written=<run>`. The next full strip keeps
// the block where its bytes, its rules and the declaration and body under it are unchanged. Its record
// id must carry no contradiction, and its record check must pass. The strip decides this itself, so a
// kept block costs no call.

const writtenOption = "--written="

// KeptLine ends the stderr line naming a block the strip kept. A run counts its kept blocks by it.
const KeptLine = " wrote it, since its record holds"

// written is one block the lane wrote, as the archive keeps it.
type written struct {
	Run   string `json:"run"`
	Rules string `json:"rules"`
	Decl  string `json:"decl"`
	Span  string `json:"span"`
	Block string `json:"block"`
	// Record is the note's three slot lines, as the writer returned them.
	Record string `json:"record"`
}

// writtenName is the file under the archive holding the blocks the lane wrote in one source file.
func writtenName(archive, path string) string {
	return filepath.Join(archive, strings.TrimSuffix(archiveName(path, 0), "@0.facts")+".written")
}

// rulePaths are the rules a block is written under, below the flavor root. A block written under
// other rules is written again.
var rulePaths = []string{"standards/comments.md", "workers/comment-writer.md"}

// movedRules maps the sum of rules that moved without a word changing to the sum they stand under now.
// The Comments section left code-style.md for comments.md byte for byte, and the brief changed only in
// its two citations of it. The old sum held all of code-style.md, so an edit to Naming reopened every
// block.
var movedRules = map[string]string{"1562bee4f02f": "8add386518c3"}

// RulesSum is the sum of the rules standing now, or "" where they cannot be read. The prompts stage
// records it, so a run's writers and its archive hold one set of rules.
func RulesSum() string { return rulesSum() }

// sameRules says whether a block written under the rules summed as written stands under the rules
// summed as now.
func sameRules(written, now string) bool {
	return now != "" && (written == now || movedRules[written] == now)
}

// rulesSum is a hash of the rules standing now, or "" where they cannot be read. `~/.kk-flavor` is
// where the brief itself names them.
func rulesSum() string {
	home, _ := os.LookupEnv("HOME")
	sum := sha256.New()
	for _, path := range rulePaths {
		body, err := os.ReadFile(filepath.Join(home, ".kk-flavor", path))
		if err != nil {
			return ""
		}
		sum.Write(body)
	}
	return hex.EncodeToString(sum.Sum(nil))[:12]
}

// declarationSpan is the declaration at line `at` and the body under it: the lines indented past the
// declaration, and a closing bracket at its own indent. A one-line declaration is its line.
func declarationSpan(lines []string, at int) []string {
	if at < 1 || at > len(lines) {
		return nil
	}
	indent := func(line string) int { return len(line) - len(strings.TrimLeft(line, " \t")) }
	depth := indent(lines[at-1])
	span := []string{lines[at-1]}
	for next := at + 1; next <= len(lines); next++ {
		line := lines[next-1]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || indent(line) > depth {
			span = append(span, line)
			continue
		}
		if indent(line) == depth && strings.ContainsAny(trimmed[:1], "}])") {
			span = append(span, line)
		}
		break
	}
	for len(span) > 1 && strings.TrimSpace(span[len(span)-1]) == "" {
		span = span[:len(span)-1]
	}
	return span
}

func spanSum(span []string) string {
	sum := sha256.Sum256([]byte(strings.Join(span, "\n")))
	return hex.EncodeToString(sum[:])[:12]
}

// commentLines is every line a comment block in the file covers.
func commentLines(lines []string) map[int]bool {
	comment := map[int]bool{}
	for _, u := range readerjudge.CommentBlocks(lines) {
		for at := u.Line; at < u.Line+u.Span; at++ {
			comment[at] = true
		}
	}
	return comment
}

// codeSum is the span's hash with its comment lines left out. Run 13's loop rewrote a row note inside
// an object literal, and the block over the literal reopened although no code under it changed.
func codeSum(lines []string, at int) string {
	comment := commentLines(lines)
	var code []string
	for n, line := range declarationSpan(lines, at) {
		if !comment[at+n] {
			code = append(code, line)
		}
	}
	return spanSum(code)
}

// declarationUnder is the first code line after the block, the line the block sits on. A file header
// and the block under it both sit on that line.
func declarationUnder(lines []string, u readerjudge.Unit) int {
	comment := commentLines(lines)
	next := u.Line + u.Span
	for next <= len(lines) && (strings.TrimSpace(lines[next-1]) == "" || comment[next]) {
		next++
	}
	return next
}

func blockText(lines []string, u readerjudge.Unit) string {
	var out strings.Builder
	for at := u.Line; at < u.Line+u.Span && at <= len(lines); at++ {
		out.WriteString(lines[at-1])
		out.WriteByte('\n')
	}
	return out.String()
}

func readWritten(archive, path string) []written {
	body, err := os.ReadFile(writtenName(archive, path))
	if err != nil {
		return nil
	}
	var held []written
	if json.Unmarshal(body, &held) != nil {
		return nil
	}
	return held
}

// contradictedIDs is every record id code review found false in one source file.
func contradictedIDs(archive, path string) map[string]bool {
	ids := map[string]bool{}
	body, err := os.ReadFile(contradictedName(archive, path))
	if err != nil {
		return ids
	}
	for _, line := range strings.Split(string(body), "\n") {
		if claim, _, found := strings.Cut(line, "\t"); found && recordIDShape.MatchString(claim) {
			ids[claim] = true
		}
	}
	return ids
}

// keeper says which of a file's blocks stand as the lane wrote them. It reads the archive once.
type keeper struct {
	held          []written
	rules         string
	contradiction map[string]bool
}

func newKeeper(archive, path string) keeper {
	return keeper{held: readWritten(archive, path), rules: rulesSum(), contradiction: contradictedIDs(archive, path)}
}

// keeps is the run that wrote the block, where the block stands as that run wrote it, or "".
func (k keeper) keeps(file string, lines []string, u readerjudge.Unit) string {
	run, _ := k.verdict(file, lines, u)
	return run
}

// verdict is the run that wrote the block where it stands, or why it reopens.
func (k keeper) verdict(file string, lines []string, u readerjudge.Unit) (run, reopened string) {
	if k.rules == "" {
		return "", "the rules under ~/.kk-flavor could not be read"
	}
	block := blockText(lines, u)
	at := declarationUnder(lines, u)
	if at > len(lines) {
		return "", "the block sits on no code"
	}
	span := declarationSpan(lines, at)
	// An entry written before run 14 hashed the span with its comment lines, and it still matches.
	code, whole := codeSum(lines, at), spanSum(span)
	reopened = "no archived entry holds these bytes"
	for _, w := range k.held {
		why := ""
		switch {
		case w.Block != block:
			continue
		case !sameRules(w.Rules, k.rules):
			why = "the rules changed since " + w.Run
		case w.Decl != strings.TrimSpace(lines[at-1]):
			why = "the declaration under it changed"
		case w.Span != code && w.Span != whole:
			why = "the code under it changed"
		case k.contradiction[recordID(block)]:
			why = "code review contradicted its record"
		case strings.TrimSpace(w.Record) != "":
			// A block holding only a summary carries an empty record, since a record belongs to a note.
			input := append(append(shell.SplitLines(w.Record), "---"), shell.SplitLines(block)...)
			var checks []string
			for _, f := range voicecheck.RecordFindings(file, append(input, span...)) {
				checks = append(checks, f.Check)
			}
			if len(checks) > 0 {
				why = "the record check reports " + strings.Join(checks, ", ")
			}
		}
		if why == "" {
			return w.Run, ""
		}
		reopened = why
	}
	return "", reopened
}

// KeepVerdict is what the next full strip does with one block: keep it as Run wrote it, or reopen it
// for Reopened.
type KeepVerdict struct {
	Line     int
	Run      string
	Reopened string
}

// KeepVerdicts reads every block in one file against the archive, as a full strip would, and only reads. A run's dry keep test prints them, so a block that will be written again names its reason.
func KeepVerdicts(archive, path string, lines []string) []KeepVerdict {
	k := newKeeper(archive, path)
	var out []KeepVerdict
	for _, u := range readerjudge.CommentBlocks(lines) {
		if holdsDirective(lines, u) {
			continue
		}
		run, why := k.verdict(path, lines, u)
		out = append(out, KeepVerdict{Line: u.Line, Run: run, Reopened: why})
	}
	return out
}

// write archives the block standing on the declaration at `line` with the record the writer returned
// for it. The run after this one keeps that block while its record holds.
func write(archive, run string, args []string, cwd string, refuse func(string, ...any) int) int {
	if archive == "" || run == "" || len(args) != 3 {
		return refuse("%s", "--written=<run> takes the path, the declaration's line and the record's file")
	}
	path, recordPath := args[0], args[2]
	var at int
	if _, err := fmt.Sscanf(args[1], "%d", &at); err != nil || at < 1 {
		return refuse("%q is not a line", shell.Echoable(args[1]))
	}
	for _, p := range []*string{&archive, &recordPath} {
		if !filepath.IsAbs(*p) {
			*p = filepath.Join(cwd, *p)
		}
	}
	readPath := path
	if !filepath.IsAbs(readPath) {
		readPath = filepath.Join(cwd, readPath)
	}
	raw, err := os.ReadFile(readPath)
	if err != nil {
		return refuse("cannot read %s", shell.Echoable(path))
	}
	record, err := os.ReadFile(recordPath)
	if err != nil {
		return refuse("cannot read %s", shell.Echoable(args[2]))
	}
	rules := rulesSum()
	if rules == "" {
		return refuse("%s", "cannot read the rules under ~/.kk-flavor, and a block keeps the rules it was written under")
	}
	lines := shell.SplitLines(string(raw))
	// A declaration's line names the last block over it. A file header shares that declaration with the
	// block under it, so the header is named by its own first line.
	var chosen []readerjudge.Unit
	for _, u := range readerjudge.CommentBlocks(lines) {
		switch {
		case u.Line == at:
			chosen = []readerjudge.Unit{u}
		case declarationUnder(lines, u) == at && (len(chosen) == 0 || chosen[0].Line != at):
			chosen = []readerjudge.Unit{u}
		}
	}
	for _, u := range chosen {
		at := declarationUnder(lines, u)
		if at > len(lines) {
			return refuse("the block on line %d of %s sits on no code", u.Line, shell.Echoable(path))
		}
		entry := written{Run: run, Rules: rules, Decl: strings.TrimSpace(lines[at-1]),
			Span: codeSum(lines, at), Block: blockText(lines, u), Record: strings.TrimSpace(string(record))}
		held := readWritten(archive, path)
		kept := held[:0]
		// An entry goes only where the same block stands on the same declaration and body again. A file
		// header and the block under it share one declaration, and a key without the block let one entry
		// replace the other. An entry for wording since replaced matches no block standing.
		for _, w := range held {
			if w.Decl != entry.Decl || w.Span != entry.Span || w.Block != entry.Block {
				kept = append(kept, w)
			}
		}
		body, err := json.MarshalIndent(append(kept, entry), "", " ")
		if err != nil {
			return refuse("%s", err.Error())
		}
		if err := os.MkdirAll(archive, 0o755); err != nil {
			return refuse("cannot create the archive at %s", shell.Echoable(archive))
		}
		if err := os.WriteFile(writtenName(archive, path), append(body, '\n'), 0o644); err != nil {
			return refuse("cannot write %s", shell.Echoable(writtenName(archive, path)))
		}
		return exitClean
	}
	return refuse("no comment block sits on line %d of %s", at, shell.Echoable(path))
}

// renameWritten renames the identifier in the blocks the archive keeps for one file, which is `lines`
// after the rename. The span's hash is replaced where undoing the rename in the span gives the archived
// hash. Any other change to the code under a block still reopens it.
func renameWritten(archive, path string, lines []string, forward, back func(string) string) (int, error) {
	held := readWritten(archive, path)
	renamed := 0
	for i, w := range held {
		next := w
		next.Block, next.Record, next.Decl = forward(w.Block), forward(w.Record), forward(w.Decl)
		for _, u := range readerjudge.CommentBlocks(lines) {
			at := declarationUnder(lines, u)
			if blockText(lines, u) != next.Block || at > len(lines) || strings.TrimSpace(lines[at-1]) != next.Decl {
				continue
			}
			comment := commentLines(lines)
			var code, whole []string
			for n, line := range declarationSpan(lines, at) {
				whole = append(whole, back(line))
				if !comment[at+n] {
					code = append(code, back(line))
				}
			}
			if spanSum(code) == w.Span || spanSum(whole) == w.Span {
				next.Span = codeSum(lines, at)
			}
			break
		}
		if next != w {
			held[i] = next
			renamed++
		}
	}
	if renamed == 0 {
		return 0, nil
	}
	body, err := json.MarshalIndent(held, "", " ")
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(writtenName(archive, path), append(body, '\n'), 0o644); err != nil {
		return 0, fmt.Errorf("cannot write %s", shell.Echoable(writtenName(archive, path)))
	}
	return renamed, nil
}
