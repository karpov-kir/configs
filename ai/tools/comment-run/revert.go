package commentrun

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	commentstrip "configs/ai/tools/comment-strip"
	"configs/ai/tools/shell"
)

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

// revert restores each file to the copy archive-written saved as the writers left it, and withdraws the
// carried records this run made for it. Run 18's lane changed code logic to carry a fact, and a ruling
// reverted the change by hand. Its carried record stood, and runs 14 and 16 had no stage for either.
func revert(r *runner, opts options, paths []string) int {
	run, runDir, archive := opts.one("run"), opts.one("run-dir"), opts.one("archive")
	if run == "" || runDir == "" || archive == "" || len(paths) == 0 {
		return r.refuse("%s", "revert takes --run=<run>, --run-dir=<dir>, --archive=<dir> and the files to restore")
	}
	for _, p := range []*string{&runDir, &archive} {
		if !filepath.IsAbs(*p) {
			*p = filepath.Join(r.cwd, *p)
		}
	}
	held, err := readRun(filepath.Join(runDir, "run.txt"))
	if err != nil {
		return r.refuse("%s holds no run.txt: run seed first", shell.Echoable(runDir))
	}
	for _, given := range paths {
		// A path comes from a lane's return, and one leaving the tree would be copied over a file outside it.
		// Every spelling of one file reaches the same archive key.
		path := filepath.Clean(given)
		if !filepath.IsLocal(path) {
			return r.refuse("%s is no path inside the tree", shell.Echoable(path))
		}
		snapshot, err := os.ReadFile(filepath.Join(runDir, "written", path))
		if err != nil {
			return r.refuse("no copy of %s as the writers left it; run archive-written with --run-dir", shell.Echoable(path))
		}
		if err := os.WriteFile(filepath.Join(held["top"], path), snapshot, 0o644); err != nil {
			return r.refuse("cannot restore %s", shell.Echoable(path))
		}
		withdrawn, err := commentstrip.Withdraw(archive, run, path)
		if err != nil {
			return r.refuse("cannot withdraw the carried records of %s: %v", shell.Echoable(path), err)
		}
		fmt.Fprintf(r.stdout, "%s restored as the writers left it, %d carried record(s) withdrawn\n", path, withdrawn)
	}
	return exitClean
}
