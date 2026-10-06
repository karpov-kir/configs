package commentrun

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	commentstrip "configs/ai/tools/comment-strip"
	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

var (
	// A verdict names the file and a line, and may carry a note between the line and the bar. Run 14's
	// writers wrote `(landed on line 23)`, `[site :19]` and `(offered :64)` there, and one dropped the
	// path after its first verdict.
	reWrittenVerdict = regexp.MustCompile(`(?m)^\**Block \d+/\d+ (\S*?):(\d+)\s*([(\[][^)\]]*[)\]])?\s*\| OK`)
	reAnyVerdict     = regexp.MustCompile(`(?m)^\**Block \d+/\d+ .*\| OK.*$`)
	reLandedNote     = regexp.MustCompile(`landed on line (\d+)`)
	reFence          = regexp.MustCompile("(?s)```[A-Za-z]*\n(.*?)```")
	reRecordSlot     = regexp.MustCompile(`^\s*(?:-\s*)?(fact|bears_on|does):\s*(.*)$`)
	reCommentLine    = regexp.MustCompile(`^\s*(/\*|\*|//|#)`)
	reOpensBody      = regexp.MustCompile(`[{(\[]\s*$`)
)

// archiveWritten reads each writer's return, finds each block it wrote in the file as the file stands
// now, and archives it with the record the writer returned. A block holding only a summary archives
// with an empty record. The runner rebuilt this from the skill's prose in runs 12 and 13.
func archiveWritten(r *runner, opts options, returns []string) int {
	run, archive, runDir := opts.one("run"), opts.one("archive"), opts.one("run-dir")
	if runDir != "" {
		r.absolute(&runDir)
	}
	// A writer writes its return to the run directory, and a run of 162 sites fits no message.
	if len(returns) == 0 && runDir != "" {
		returns, _ = filepath.Glob(returnFile(runDir, "*"))
	}
	if run == "" || archive == "" || len(returns) == 0 {
		return r.refuse("%s", "archive-written takes --run=<run>, --archive=<dir> and the writers' returns, or --run-dir holding them")
	}
	// A writer read its rules from the mount. After a merge between the prompts and this stage, its blocks
	// carry a sum for rules it never read. The run's prompts recorded the sum beside the returns.
	dirs := map[string]bool{runDir: runDir != ""}
	for _, ret := range returns {
		dirs[filepath.Dir(ret)] = true
	}
	for dir, isRunDir := range dirs {
		if held := rulesHeld(dir); isRunDir && held != "" && held != commentstrip.RulesSum() {
			return r.refuse("the rules changed since this run's prompts: %s then, %s now; the writers wrote under the earlier rules",
				held, commentstrip.RulesSum())
		}
	}
	records, err := os.MkdirTemp("", "comment-run-records-")
	if err != nil {
		return r.refuse("cannot make a directory for the records: %v", err)
	}
	defer os.RemoveAll(records)
	archived, refused := 0, 0
	snapshot := map[string]bool{}
	for _, path := range returns {
		body, err := os.ReadFile(path)
		if err != nil {
			return r.refuse("cannot read %s", shell.Echoable(path))
		}
		text := string(body)
		placed := map[int]bool{}
		starts := reWrittenVerdict.FindAllStringSubmatchIndex(text, -1)
		for _, m := range starts {
			placed[m[0]] = true
		}
		// A verdict this stage cannot place goes back by writer and line, with the shape it wants, so a
		// runner never re-asks a writer by hand.
		// A file-level site places its block anywhere in the file, so a block written for it answers it
		// `OK`. Run 28's runner rewrote such an answer as `none` with `moved to`. That shape belongs to a
		// moved block, and it left the site's claims to a decline when the block had answered them.
		for _, m := range reFileLevelMove.FindAllStringSubmatch(text, -1) {
			refused++
			fmt.Fprintf(r.stdout, "%s:0 refused: %s\n", m[1], fileLevelShape)
		}
		for _, m := range reAnyVerdict.FindAllStringIndex(text, -1) {
			if !placed[m[0]] {
				refused++
				fmt.Fprintf(r.stdout, "%s:%d refused: the verdict %q names no <path>:<line>; the shape is %s\n",
					path, strings.Count(text[:m[0]], "\n")+1, shell.CutBytesMarked(text[m[0]:m[1]], 80), verdictShape)
			}
		}
		lastFile := ""
		for n, m := range starts {
			end := len(text)
			if n+1 < len(starts) {
				end = starts[n+1][0]
			}
			file, site := text[m[2]:m[3]], text[m[4]:m[5]]
			offeredSite := site
			if file == "" {
				file = lastFile
			}
			if file == "" {
				refused++
				fmt.Fprintf(r.stdout, "%s:%d refused: the verdict names no path and none comes before it; the shape is %s\n",
					path, strings.Count(text[:m[0]], "\n")+1, verdictShape)
				continue
			}
			lastFile = file
			// One entry archives one block. Run 22's loop writer moved an enum's note onto two members in one
			// entry, and the second note was archived nowhere.
			if fences := len(reFence.FindAllStringIndex(text[m[0]:end], -1)); fences > 1 {
				refused++
				fmt.Fprintf(r.stdout, "%s:%s refused: the entry fences %d blocks, and each block is its own entry; %s\n",
					file, site, fences, membersShape)
				continue
			}
			if m[6] >= 0 {
				if landed := reLandedNote.FindStringSubmatch(text[m[6]:m[7]]); landed != nil {
					site = landed[1]
				}
			}
			at, why := r.locate(file, site, text[m[0]:end])
			if why != "" {
				refused++
				fmt.Fprintf(r.stdout, "%s:%s refused: %s\n", file, site, why)
				continue
			}
			// Run 26's writer A answered a type's site as written and put the block on a constant, and the
			// offered site kept no record of its own.
			if decl, moved := r.movedFrom(runDir, path, file, offeredSite, at); moved {
				refused++
				fmt.Fprintf(r.stdout, "%s:%s refused: the block stands at :%d on `%s`, not on the declaration offered; %s\n",
					file, offeredSite, at, shell.CutBytesMarked(decl, 80), movedShape(file, at))
				continue
			}
			record := filepath.Join(records, fmt.Sprintf("%d-%d.record", n, archived))
			if err := os.WriteFile(record, []byte(recordOf(text[m[0]:end])), 0o644); err != nil {
				return r.refuse("cannot write %s", shell.Echoable(record))
			}
			var out, errOut strings.Builder
			if code := commentstrip.Strip("comment-strip.sh", []string{"--archive=" + archive, "--written=" + run,
				file, strconv.Itoa(at), record}, r.cwd, repo.Exec{}, &out, &errOut); code != 0 {
				refused++
				fmt.Fprintf(r.stdout, "%s:%d refused: %s\n", file, at, shell.Oneline(errOut.String()))
				continue
			}
			archived++
			fmt.Fprintf(r.stdout, "%s:%d archived\n", file, at)
			snapshot[file] = true
			// The block answers every claim its site was offered, so its record settles each of them.
			if err := r.settleOffered(runDir, path, archive, file, offeredSite, at); err != nil {
				refused++
				fmt.Fprintf(r.stdout, "%s:%d refused: the block is archived, and the claims it settles are not: %v\n", file, at, err)
			}
		}
	}
	// The file as the writers left it, which the carried stage reads the refactor lane's lines against.
	// The lane's verdict names a line of this tree, and its edit then moves or removes the block.
	if runDir != "" {
		var files []string
		for file := range snapshot {
			files = append(files, file)
		}
		sort.Strings(files)
		for _, file := range files {
			at, err := codeChanged(runDir, r.cwd, file)
			if err != nil {
				return r.refuse("cannot read %s as its writer received it: %v", shell.Echoable(file), err)
			}
			if at > 0 {
				refused++
				fmt.Fprintf(r.stdout, "%s:%d code changed: a writer's edit changed this code line, and a writer writes comment lines only\n", file, at)
			}
		}
		if err := copyFiles(r.cwd, files, writtenDir(runDir)); err != nil {
			return r.refuse("%v", err)
		}
	}
	// A site a writer answered `none` is recorded too, so the next run on unchanged code and rules does
	// not offer it again. Run 22 offered 82 sites run 20 had declined under the same rules.
	if runDir != "" {
		declined, cleared, err := decideFromReturns(runDir, archive, run, r.cwd, r.stderr)
		if err != nil {
			return r.refuse("%v", err)
		}
		fmt.Fprintf(r.stderr, "%s: %d declined record(s) recorded, %d decline(s) cleared\n", r.self, declined, cleared)
	}
	fmt.Fprintf(r.stderr, "%s: %d block(s) archived, %d refused\n", r.self, archived, refused)
	if refused > 0 {
		return exitFindings
	}
	return exitClean
}

// writtenDir is where archive-written saves each file as the writers left it.
func writtenDir(runDir string) string {
	return filepath.Join(runDir, "written")
}

// verdictShape is the verdict line this stage reads, with its example.
// membersShape is the return for a note moved onto the members it tells apart: the site offered gets no
// block, and each member is an entry of its own. The members check's finding says it in these words.
const membersShape = "where a note moves onto the members it tells apart, the offered site returns `none` with the line " +
	"`moved to its members`, and each member is an entry of its own, with its own line, block and record"

// movedShape is the return for a block moved to the declaration its claim is about. The offered site
// returns `none` with a line naming where the block went, and the new site is an entry of its own.
func movedShape(file string, at int) string {
	return fmt.Sprintf("return the offered site as `none` with the line `moved to %s:%d`, and give :%d an entry of its own, "+
		"with its own line, block and record", file, at, at)
}

const verdictShape = "`Block N/M <path>:<offered line> | OK`, as in `Block 2/16 src/ledger.ts:64 | OK`"

// verdictSentence asks a writer for that shape in its prompt. A sentence in the brief changes the rules
// sum, and every block a run wrote would reopen, so the prompt carries it.
const verdictSentence = "Every verdict line holds the file and the site's line as offered, and only those, even where the block landed elsewhere: " + verdictShape + ". Each entry holds one block: " + membersShape + ". " + fileLevelShape + "."

// fileLevelShape is the answer to a file-level site, `:0`, whose claim a block now carries.
const fileLevelShape = "a file-level site, `:0`, whose claim a block you wrote carries is answered `OK` with that block, wherever in the file it landed, and never `none` with `moved to`"

// reFileLevelMove is a file-level site answered `none` with a `moved to` line in its entry.
var reFileLevelMove = regexp.MustCompile(`(?m)^\**Block \d+/\d+ (\S+):0\b[^\n]*\|\s*none\b[^\n]*\n(?:(?:[^B*\n][^\n]*)?\n)*?moved to `)

// recordOf is the record lines a verdict's segment carries, or an empty string for a summary.
func recordOf(segment string) string {
	var slots []string
	for _, line := range strings.Split(segment, "\n") {
		if m := reRecordSlot.FindStringSubmatch(line); m != nil {
			slots = append(slots, m[1]+": "+strings.TrimSpace(m[2]))
		}
	}
	if len(slots) == 0 {
		return ""
	}
	return strings.Join(slots, "\n") + "\n"
}

// locate is the first line of the block a verdict fenced, in the file as it stands, nearest the site the
// verdict names, or why no such block stands.
func (r *runner) locate(file, site, segment string) (int, string) {
	fence := reFence.FindStringSubmatch(segment)
	if fence == nil {
		return 0, "the verdict fences no block"
	}
	// A writer may fence a block together with the code line that precedes it, as run 26's loop writer
	// did for two summary-only blocks. The block is the fence's first run of comment lines. A code line
	// before it that opens a body puts the run inside that body, and the fence is refused.
	var block []string
	for _, line := range strings.Split(fence[1], "\n") {
		if !reCommentLine.MatchString(line) {
			if len(block) == 0 {
				if reOpensBody.MatchString(line) {
					return 0, "the fenced block holds no comment line before the code"
				}
				continue
			}
			break
		}
		block = append(block, strings.TrimSpace(line))
	}
	if len(block) == 0 {
		return 0, "the fenced block holds no comment line"
	}
	read := file
	r.absolute(&read)
	raw, err := os.ReadFile(read)
	if err != nil {
		return 0, "cannot read " + shell.Echoable(file)
	}
	lines := shell.SplitLines(string(raw))
	want, _ := strconv.Atoi(site)
	best := 0
	for at := 1; at+len(block)-1 <= len(lines); at++ {
		match := true
		for i, line := range block {
			if strings.TrimSpace(lines[at-1+i]) != line {
				match = false
				break
			}
		}
		if match && (best == 0 || abs(at-want) < abs(best-want)) {
			best = at
		}
	}
	if best == 0 {
		return 0, "the block stands nowhere in the file"
	}
	return best, ""
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
