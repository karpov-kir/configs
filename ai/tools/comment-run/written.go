package commentrun

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	commentstrip "configs/ai/tools/comment-strip"
	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

var (
	reWrittenVerdict = regexp.MustCompile(`(?m)^Block \d+/\d+ (\S+?):(\d+) \| OK`)
	reFence          = regexp.MustCompile("(?s)```[A-Za-z]*\n(.*?)```")
	reRecordSlot     = regexp.MustCompile(`^\s*(?:-\s*)?(fact|bears_on|does):\s*(.*)$`)
	reCommentLine    = regexp.MustCompile(`^\s*(/\*|\*|//|#)`)
)

// archiveWritten reads each writer's return, finds each block it wrote in the file as the file stands
// now, and archives it with the record the writer returned. A block holding only a summary archives
// with an empty record. The runner rebuilt this from the skill's prose in runs 12 and 13.
func archiveWritten(r *runner, opts options, returns []string) int {
	run, archive := opts.one("run"), opts.one("archive")
	if run == "" || archive == "" || len(returns) == 0 {
		return r.refuse("%s", "archive-written takes --run=<run>, --archive=<dir> and the writers' returns")
	}
	records, err := os.MkdirTemp("", "comment-run-records-")
	if err != nil {
		return r.refuse("cannot make a directory for the records: %v", err)
	}
	defer os.RemoveAll(records)
	archived, refused := 0, 0
	for _, path := range returns {
		body, err := os.ReadFile(path)
		if err != nil {
			return r.refuse("cannot read %s", shell.Echoable(path))
		}
		text := string(body)
		starts := reWrittenVerdict.FindAllStringSubmatchIndex(text, -1)
		for n, m := range starts {
			end := len(text)
			if n+1 < len(starts) {
				end = starts[n+1][0]
			}
			file, site := text[m[2]:m[3]], text[m[4]:m[5]]
			at, why := r.locate(file, site, text[m[0]:end])
			if why != "" {
				refused++
				fmt.Fprintf(r.stdout, "%s:%s refused: %s\n", file, site, why)
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
		}
	}
	fmt.Fprintf(r.stderr, "%s: %d block(s) archived, %d refused\n", r.self, archived, refused)
	if refused > 0 {
		return exitFindings
	}
	return exitClean
}

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
	var block []string
	for _, line := range strings.Split(fence[1], "\n") {
		if strings.TrimSpace(line) == "" && len(block) == 0 {
			continue
		}
		if !reCommentLine.MatchString(line) {
			break
		}
		block = append(block, strings.TrimSpace(line))
	}
	if len(block) == 0 {
		return 0, "the fenced block holds no comment line"
	}
	read := file
	if !filepath.IsAbs(read) {
		read = filepath.Join(r.cwd, read)
	}
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
