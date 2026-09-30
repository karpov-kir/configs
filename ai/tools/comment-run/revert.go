package commentrun

import (
	"fmt"
	"os"
	"path/filepath"

	commentstrip "configs/ai/tools/comment-strip"
	"configs/ai/tools/shell"
)

// revert restores each file to the copy archive-written saved as the writers left it, and withdraws the
// carried records this run made for it. Run 18's lane changed code logic to carry a fact, and a ruling
// reverted the change by hand. Its carried record stood, and runs 14 and 16 had no stage for either.
func revert(r *runner, opts options, paths []string) int {
	run, runDir, archive := opts.one("run"), opts.one("run-dir"), opts.one("archive")
	if run == "" || runDir == "" || archive == "" || len(paths) == 0 {
		return r.refuse("%s", "revert takes --run=<run>, --run-dir=<dir>, --archive=<dir> and the files to restore")
	}
	r.absolute(&runDir, &archive)
	held, err := readRun(runDir)
	if err != nil {
		return r.refuse("%v", err)
	}
	for _, given := range paths {
		// A path comes from a lane's return, and one leaving the tree would be copied over a file outside it.
		// Every spelling of one file reaches the same archive key.
		path := filepath.Clean(given)
		if !filepath.IsLocal(path) {
			return r.refuse("%s is no path inside the tree", shell.Echoable(path))
		}
		snapshot, err := os.ReadFile(filepath.Join(writtenDir(runDir), path))
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
