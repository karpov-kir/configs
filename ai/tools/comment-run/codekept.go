package commentrun

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// dispatchedDir is where a run keeps each file as its writers received it: the seed's strip, the tree
// the refactor lane left, then each loop round's strip. archive-written reads the writers' code
// against it, and the loop's archive-written reads every return again after the lane changed code.
func dispatchedDir(runDir string) string {
	return filepath.Join(runDir, "dispatched")
}

// reEditableLine is a line a writer's edit may change: a comment line or a blank one.
var reEditableLine = regexp.MustCompile(`^\s*(?://|/\*|\*|$)`)

// codeChanged is the first line of the file, as the tree numbers it, whose code differs from the file
// as its writer received it, or 0. A writer writes comment lines only. Run 26's writers changed the
// spacing of seven code lines under their blocks, and prettier caught it after the run. A run
// directory older than the dispatched copy reads the seed's strip.
func codeChanged(runDir, top, file string) (int, error) {
	received, err := os.ReadFile(filepath.Join(dispatchedDir(runDir), file))
	if os.IsNotExist(err) {
		received, err = os.ReadFile(filepath.Join(runDir, "post-strip", file))
	}
	if err != nil {
		return 0, err
	}
	left, err := os.ReadFile(filepath.Join(top, file))
	if err != nil {
		return 0, err
	}
	before, after := codeLines(string(received)), codeLines(string(left))
	for i := range after {
		if i >= len(before) || before[i].text != after[i].text {
			return after[i].at, nil
		}
	}
	if len(before) > len(after) {
		return len(strings.Split(string(left), "\n")), nil
	}
	return 0, nil
}

// codeLine is a line of code and its line number.
type codeLine struct {
	at   int
	text string
}

// codeLines is the text's lines of code, each with its whitespace as it stands.
func codeLines(text string) []codeLine {
	var out []codeLine
	for i, line := range strings.Split(text, "\n") {
		if !reEditableLine.MatchString(line) {
			out = append(out, codeLine{i + 1, line})
		}
	}
	return out
}

// refreshDispatched copies each file the dispatched copy holds from the tree again.
func refreshDispatched(runDir, top string) error {
	var files []string
	root := dispatchedDir(runDir)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		files = append(files, rel)
		return err
	})
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return copyFiles(top, files, root)
}
