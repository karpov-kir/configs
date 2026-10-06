package commentrun

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	commentstrip "configs/ai/tools/comment-strip"
	"configs/ai/tools/shell"
)

var (
	reCarriedVerdict = regexp.MustCompile(`(?m)^\**Comment \d+/\d+ (\S+?):(\d+) \| carried by (.+)$`)
	reStaysVerdict   = regexp.MustCompile(`(?m)^\**Comment \d+/\d+ \S+ \| stays:`)
)

// carriedStage records each block the refactor lane carried into code. The next strip then offers its
// site no more while the carrier stands. The lane's verdict names a line of the tree the writers left,
// which archive-written saved under the run directory. A `stays:` verdict leaves the block standing,
// and the strip reads that block as it reads any other.
func carriedStage(r *runner, opts options, returns []string) int {
	run, runDir, archive := opts.one("run"), opts.one("run-dir"), opts.one("archive")
	if run == "" || runDir == "" || archive == "" || len(returns) == 0 {
		return r.refuse("%s", "carried takes --run=<run>, --run-dir=<dir>, --archive=<dir> and the refactor lane's returns")
	}
	r.absolute(&runDir, &archive)
	held, err := readRun(runDir)
	if err != nil {
		return r.refuse("%v", err)
	}
	recorded, refused, stays := 0, 0, 0
	laneEdits := map[string][]string{}
	for _, path := range returns {
		body, err := os.ReadFile(path)
		if err != nil {
			return r.refuse("cannot read %s", shell.Echoable(path))
		}
		stays += len(reStaysVerdict.FindAllString(string(body), -1))
		for _, m := range reCarriedVerdict.FindAllStringSubmatch(string(body), -1) {
			file, carrier := filepath.Clean(m[1]), strings.TrimSpace(m[3])
			at, _ := strconv.Atoi(m[2])
			if !filepath.IsLocal(file) {
				refused++
				fmt.Fprintf(r.stdout, "%s:%d refused: no path inside the tree\n", shell.Echoable(file), at)
				continue
			}
			written := filepath.Join(writtenDir(runDir), file)
			snapshot, err := os.ReadFile(written)
			if err != nil {
				refused++
				fmt.Fprintf(r.stdout, "%s:%d refused: no copy of the file as the writers left it; run archive-written with --run-dir\n", file, at)
				continue
			}
			edits, diffed := laneEdits[file]
			if !diffed {
				if edits, err = laneCommentEdits(written, filepath.Join(held["top"], file)); err != nil {
					return r.refuse("%v", err)
				}
				laneEdits[file] = edits
				if len(edits) > 0 {
					fmt.Fprintf(r.stdout, "%s refused: the lane wrote comment text, and a lane never edits a comment line; "+
						"run revert on the file and return `carried by` with the block deleted or `stays:`: %s\n", file,
						shell.CutBytesMarked(strings.Join(edits, " / "), 200))
				}
			}
			if len(edits) > 0 {
				refused++
				continue
			}
			if err := commentstrip.Carry(archive, run, file, shell.SplitLines(string(snapshot)), at, carrier); err != nil {
				refused++
				fmt.Fprintf(r.stdout, "%s:%d refused: %v\n", file, at, err)
				continue
			}
			recorded++
			fmt.Fprintf(r.stdout, "%s:%d carried by %s\n", file, at, carrier)
		}
	}
	// The lane's own decisions are recorded, and an edit to a declaration an earlier lane settled is
	// refused while the code under it held.
	for file, ruled := range settleLane(runDir, archive, run, held["top"], returns) {
		for _, edit := range ruled {
			refused++
			fmt.Fprintf(r.stdout, "%s refused: %s; revert the edit, or leave the declaration as it stood\n", file, edit)
		}
	}
	// The lane changed code, and a later writer receives the tree it left.
	if err := refreshDispatched(runDir, held["top"]); err != nil {
		return r.refuse("%v", err)
	}
	fmt.Fprintf(r.stderr, "%s: %d carried block(s) recorded, %d refused, %d stays\n", r.self, recorded, refused, stays)
	if refused > 0 {
		return exitFindings
	}
	return exitClean
}

// laneCommentEdits is each comment line the refactor lane added or changed in a file, against the copy
// archive-written saved as the writers left it. A lane never edits a comment line: run 18's lane
// shortened a note under a `carried by` verdict, and the note failed voice and the keep test.
func laneCommentEdits(snapshot, current string) ([]string, error) {
	out, err := exec.Command("git", "diff", "--no-index", "--unified=0", "--", snapshot, current).Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			return nil, fmt.Errorf("cannot compare %s with the writers' copy: %v", shell.Echoable(current), err)
		}
	}
	var edits []string
	for _, line := range shell.SplitLines(string(out)) {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") && reCommentLine.MatchString(line[1:]) {
			edits = append(edits, strings.TrimSpace(line[1:]))
		}
	}
	return edits, nil
}

// reStaysAt is a `stays:` verdict with its file and the line it names in the tree the writers left.
var reStaysAt = regexp.MustCompile(`(?m)^\**Comment \d+/\d+ (\S+):(\d+) \| stays:`)

// settleLane records the lane's decisions on each file of the change set and returns, by file, each
// settled declaration the lane edited. The file as the lane received it is the writers' copy where
// archive-written saved one, and the dispatched copy otherwise. The writers write comments only, so the
// code under each declaration is the same in both.
func settleLane(runDir, archive, run, top string, returns []string) map[string][]string {
	stays := map[string][]int{}
	for _, path := range returns {
		body, _ := os.ReadFile(path)
		for _, m := range reStaysAt.FindAllStringSubmatch(string(body), -1) {
			at, _ := strconv.Atoi(m[2])
			stays[filepath.Clean(m[1])] = append(stays[filepath.Clean(m[1])], at)
		}
	}
	out := map[string][]string{}
	// A run directory older than the dispatched copy holds only the writers' copy, as run 26's does.
	root := dispatchedDir(runDir)
	if _, err := os.Stat(root); err != nil {
		root = writtenDir(runDir)
	}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		file, _ := filepath.Rel(root, path)
		before, err := os.ReadFile(filepath.Join(writtenDir(runDir), file))
		if err != nil {
			before, _ = os.ReadFile(path)
		}
		after, err := os.ReadFile(filepath.Join(top, file))
		if err != nil {
			return nil
		}
		edits, err := commentstrip.Settle(archive, run, file, shell.SplitLines(string(before)), shell.SplitLines(string(after)), stays[file])
		if err == nil && len(edits) > 0 {
			out[file] = edits
		}
		return nil
	})
	return out
}
