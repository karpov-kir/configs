package commentrun

import (
	"fmt"
	"os"

	commentstrip "configs/ai/tools/comment-strip"
	"configs/ai/tools/shell"
)

// keepTest reads every block in the named files against the archive, as the next full strip would,
// and prints whether it stands and why it reopens. It only reads the files and the archive. Run 13
// took a copy of the tree and the archive by hand to learn that 16 of 73 blocks would be rewritten.
func keepTest(r *runner, opts options, paths []string) int {
	archive := opts.one("archive")
	if archive == "" || len(paths) == 0 {
		return r.refuse("%s", "keep-test takes --archive=<dir> and the paths to read")
	}
	r.absolute(&archive)
	root := r.cwd
	if top, err := git(r.cwd, "rev-parse", "--show-toplevel"); err == nil {
		root = top
	}
	kept, total := 0, 0
	for _, path := range paths {
		read := path
		r.absolute(&read)
		raw, err := os.ReadFile(read)
		if err != nil {
			return r.refuse("cannot read %s", shell.Echoable(path))
		}
		lines := shell.SplitLines(string(raw))
		for _, c := range commentstrip.CarriedVerdicts(archive, path, root, read, lines) {
			total++
			kept++
			fmt.Fprintf(r.stdout, "%s: %s carried by %s as %s left it\n", path, c.Decl, c.Carrier, c.Run)
		}
		for _, v := range commentstrip.KeepVerdicts(archive, path, lines) {
			total++
			if v.Run != "" {
				kept++
				fmt.Fprintf(r.stdout, "%s:%d kept as %s wrote it\n", path, v.Line, v.Run)
				continue
			}
			fmt.Fprintf(r.stdout, "%s:%d rewrite: %s\n", path, v.Line, v.Reopened)
		}
	}
	fmt.Fprintf(r.stderr, "%s: %d of %d block(s) kept, %d to rewrite\n", r.self, kept, total, total-kept)
	if kept < total {
		return exitFindings
	}
	return exitClean
}
