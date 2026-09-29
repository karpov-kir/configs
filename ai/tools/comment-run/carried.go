package commentrun

import (
	"fmt"
	"os"
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
	for _, p := range []*string{&runDir, &archive} {
		if !filepath.IsAbs(*p) {
			*p = filepath.Join(r.cwd, *p)
		}
	}
	held, err := readRun(filepath.Join(runDir, "run.txt"))
	if err != nil {
		return r.refuse("%s holds no run.txt: run seed first", shell.Echoable(runDir))
	}
	recorded, refused, stays := 0, 0, 0
	edited := map[string]bool{}
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
			snapshot, err := os.ReadFile(filepath.Join(runDir, "written", file))
			if err != nil {
				refused++
				fmt.Fprintf(r.stdout, "%s:%d refused: no copy of the file as the writers left it; run archive-written with --run-dir\n", file, at)
				continue
			}
			edits, err := laneCommentEdits(filepath.Join(runDir, "written", file), filepath.Join(held["top"], file))
			if err != nil {
				return r.refuse("%v", err)
			}
			if len(edits) > 0 {
				refused++
				if !edited[file] {
					edited[file] = true
					fmt.Fprintf(r.stdout, "%s refused: the lane wrote comment text, and a lane never edits a comment line; "+
						"run revert on the file and return `carried by` with the block deleted or `stays:`: %s\n", file,
						shell.CutBytesMarked(strings.Join(edits, " / "), 200))
				}
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
	fmt.Fprintf(r.stderr, "%s: %d carried block(s) recorded, %d refused, %d stays\n", r.self, recorded, refused, stays)
	if refused > 0 {
		return exitFindings
	}
	return exitClean
}
