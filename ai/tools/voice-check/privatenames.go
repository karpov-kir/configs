package voicecheck

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// A public repository was handed a private codebase's file names and identifiers in a pull request's
// commits, tests and body, and no check read them. The owner keeps a list of names that must never
// leave the machine, outside every repository, and this scan refuses any text holding one. A finding
// names the entry by its line in the list and the place, and never the text, so it leaks nothing.

const checkPrivateName = "private-name"

// privateNamesEnv names a list in place of the owner's, for a test or a second list.
const privateNamesEnv = "KK_PRIVATE_NAMES"

// PrivateNamesPath is where the owner's list lives: under the user's config directory and never inside
// a repository, so no commit can carry it.
func PrivateNamesPath(lookup func(string) (string, bool)) string {
	if path, ok := lookup(privateNamesEnv); ok && path != "" {
		return path
	}
	config, ok := lookup("XDG_CONFIG_HOME")
	if !ok || config == "" {
		home, _ := lookup("HOME")
		config = filepath.Join(home, ".config")
	}
	return filepath.Join(config, "kk-flavor", "private-names.txt")
}

// PrivateNamesHeader opens a new list. The list starts with no entries, and its owner fills it.
const PrivateNamesHeader = `# Names that must never reach a public repository: file names, identifiers, product and package
# names of private work. One entry per line. A plain entry matches as a whole word, in any case.
# An entry opening on "re:" is a regular expression. Lines opening on "#" are comments.
#
# The register scan, the gate and the configs checkout's commit-msg and pre-push hooks read this
# file. A match is reported by the entry's line here and the place it was found, never by its text.
`

// privateName is one entry of the owner's list, compiled, with the line it stands on.
type privateName struct {
	line int
	re   *regexp.Regexp
}

// privateNames is the owner's list in the order its entries stand.
type privateNames []privateName

// loadPrivateNames reads the list at path. A missing list is an empty one. An entry that does not
// compile is refused, since a list that silently drops an entry guards less than its owner thinks.
func loadPrivateNames(path string) (privateNames, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read the private-name list at %s: %v", path, err)
	}
	defer file.Close()
	var out privateNames
	lines := bufio.NewScanner(file)
	n := 0
	for lines.Scan() {
		n++
		entry := strings.TrimSpace(lines.Text())
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		pattern := `(?i)(?:^|[^\pL\pN_])` + regexp.QuoteMeta(entry) + `(?:[^\pL\pN_]|$)`
		if expr, found := strings.CutPrefix(entry, "re:"); found {
			pattern = expr
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("line %d of the private-name list at %s does not compile, so nothing was scanned", n, path)
		}
		out = append(out, privateName{n, re})
	}
	return out, lines.Err()
}

// hits is the line of each entry the text holds, once per entry.
func (names privateNames) hits(text string) []int {
	var out []int
	for _, name := range names {
		if name.re.MatchString(text) {
			out = append(out, name.line)
		}
	}
	return out
}

// privateNameText is a finding's text: the entry's line in the list and nothing of its words.
func privateNameText(line int) string {
	return fmt.Sprintf("the entry on line %d of the private-name list", line)
}

// privateScan is the `--private-names` mode. Given a revision range it reads every line the range adds,
// in every file, and every commit message in it: the gate and the pre-push hook call it so. Given
// `--message=<file>` it reads that file whole: the commit-msg hook and a PR body before it is sent.
func privateScan(out console, args []string, cwd string, cfg Config) int {
	names, err := loadPrivateNames(cfg.PrivateNames)
	if err != nil {
		return out.refuseArguments(err)
	}
	if len(names) == 0 {
		fmt.Fprintf(out.stderr, "%s: private names: no entry in %s, so nothing was matched\n", out.self, cfg.PrivateNames)
		return 0
	}
	if len(args) != 1 {
		return out.refuseArguments(fmt.Errorf("--private-names takes one revision range or --message=<file> — the scan did NOT run"))
	}
	var found []string
	if file, ok := strings.CutPrefix(args[0], "--message="); ok {
		body, err := os.ReadFile(file)
		if err != nil {
			return out.refuseArguments(fmt.Errorf("cannot read %s — the scan did NOT run", file))
		}
		for n, line := range strings.Split(string(body), "\n") {
			for _, entry := range names.hits(line) {
				found = append(found, fmt.Sprintf("%s:%d: %s: %s", file, n+1, checkPrivateName, privateNameText(entry)))
			}
		}
	} else {
		lines, messages, err := privateRange(cwd, args[0], names)
		if err != nil {
			return out.refuseArguments(fmt.Errorf("%v — the scan did NOT run", err))
		}
		found = append(lines, messages...)
	}
	for _, line := range found {
		fmt.Fprintln(out.stdout, line)
	}
	fmt.Fprintf(out.stderr, "%s: private names: %d finding(s)\n", out.self, len(found))
	if len(found) > 0 {
		return 1
	}
	return 0
}

// privateRange reads the lines a revision range adds and the messages of its commits.
func privateRange(cwd, revisions string, names privateNames) ([]string, []string, error) {
	diff, err := exec.Command("git", "-C", cwd, "diff", "--no-color", "--unified=0", revisions).Output()
	if err != nil {
		return nil, nil, fmt.Errorf("git cannot diff %s", revisions)
	}
	var lines []string
	file, at := "", 0
	for _, line := range strings.Split(string(diff), "\n") {
		switch {
		case strings.HasPrefix(line, "+++ "):
			file = strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/")
		case strings.HasPrefix(line, "@@ "):
			if m := reHunkStart.FindStringSubmatch(line); m != nil {
				at, _ = strconv.Atoi(m[1])
			}
		case strings.HasPrefix(line, "+"):
			for _, entry := range names.hits(line[1:]) {
				lines = append(lines, fmt.Sprintf("%s:%d: %s: %s", file, at, checkPrivateName, privateNameText(entry)))
			}
			at++
		}
	}
	// A bare revision is the work not yet committed, and it has no message of its own.
	if !strings.Contains(revisions, "..") {
		return lines, nil, nil
	}
	commits := strings.Replace(revisions, "...", "..", 1)
	log, err := exec.Command("git", "-C", cwd, "log", "--format=%H%x00%B%x00", commits).Output()
	if err != nil {
		return nil, nil, fmt.Errorf("git cannot list the commits of %s", commits)
	}
	var messages []string
	parts := strings.Split(string(log), "\x00")
	for n := 0; n+1 < len(parts); n += 2 {
		sha := strings.TrimSpace(parts[n])
		for _, entry := range names.hits(parts[n+1]) {
			messages = append(messages, fmt.Sprintf("commit %.12s: %s: %s", sha, checkPrivateName, privateNameText(entry)))
		}
	}
	return lines, messages, nil
}

var reHunkStart = regexp.MustCompile(`^@@ -\S+ \+(\d+)`)
