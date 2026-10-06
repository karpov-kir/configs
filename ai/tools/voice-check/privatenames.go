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
// commits, tests and body, and every check passed them. The owner keeps a list of names that stay on
// the machine, outside every repository, and this scan refuses any text holding one. A finding names
// the entry by its line in the list and the place, so the report holds none of the words.

const checkPrivateName = "private-name"

// privateNamesEnv names a list in place of the owner's, for a test or a second list.
const privateNamesEnv = "KK_PRIVATE_NAMES"

// PrivateNamesPath is where the owner's list lives. It sits under the user's config directory, outside
// every repository, so a commit has it out of reach.
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
const PrivateNamesHeader = `# Private names kept out of public repositories, one per line: a whole word in any case, or
# "re:" and a case-sensitive regular expression, as "re:(?i)acme\w*" for every identifier built on
# a word. Lines opening on "#" are comments. The register scan, the gate and the configs hooks read
# this file, and a match is reported by the entry's line here.
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
	if os.IsNotExist(err) && os.Getenv(privateNamesEnv) == "" {
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

// privateNameText is a finding's text: the entry's line in the list, with none of its words.
func privateNameText(line int) string {
	return fmt.Sprintf("the entry on line %d of the private-name list", line)
}

// emptyTree is git's empty tree. A range from it reads a branch from its first commit.
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// redactedPath stands in for a path that holds a private name, in every report that prints a path.
const redactedPath = "<a path holding a private name>"

// redact is the path as a report may print it.
func (names privateNames) redact(path string) string {
	if len(names.hits(path)) > 0 {
		return redactedPath
	}
	return path
}

// privateScan is the `--private-names` mode. The gate and the pre-push hook hand it a revision range,
// and it reads each path, each added line and each commit message there. `--message=<file>` reads a PR
// body whole. `--commit-message=<file>` reads a commit message as git keeps it, with git's `#` lines
// dropped, as the commit-msg hook receives it.
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
		return out.refuseArguments(fmt.Errorf("--private-names takes one revision range, --message=<file> or --commit-message=<file> — the scan did NOT run"))
	}
	var found []string
	message, whole := strings.CutPrefix(args[0], "--message=")
	commit, isCommit := strings.CutPrefix(args[0], "--commit-message=")
	switch {
	case whole || isCommit:
		file := message
		if isCommit {
			file = commit
		}
		body, err := os.ReadFile(file)
		if err != nil {
			return out.refuseArguments(fmt.Errorf("cannot read %s — the scan did NOT run", file))
		}
		for n, line := range strings.Split(string(body), "\n") {
			if isCommit && strings.HasPrefix(line, "# ") && strings.Contains(line, ">8") {
				break
			}
			if isCommit && strings.HasPrefix(line, "#") {
				continue
			}
			for _, entry := range names.hits(line) {
				found = append(found, fmt.Sprintf("%s:%d: %s: %s", names.redact(file), n+1, checkPrivateName, privateNameText(entry)))
			}
		}
	default:
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

// privateRange reads the paths and lines a revision range adds and the messages of its commits. A hunk
// header counts the hunk's lines, and each of those lines is content. An added line reading `++ x`
// shows in the diff as `+++ x`, the shape of a file header.
func privateRange(cwd, revisions string, names privateNames) ([]string, []string, error) {
	diff, err := exec.Command("git", "-C", cwd, "diff", "--no-color", "--unified=0", revisions).Output()
	if err != nil {
		return nil, nil, fmt.Errorf("git cannot diff %s", revisions)
	}
	var lines []string
	file, at, oldLeft, newLeft := "", 0, 0, 0
	for _, line := range strings.Split(string(diff), "\n") {
		if oldLeft > 0 || newLeft > 0 {
			switch {
			case strings.HasPrefix(line, "+"):
				for _, entry := range names.hits(line[1:]) {
					lines = append(lines, fmt.Sprintf("%s:%d: %s: %s", names.redact(file), at, checkPrivateName, privateNameText(entry)))
				}
				at++
				newLeft--
			case strings.HasPrefix(line, "-"):
				oldLeft--
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "diff --git "):
			if _, after, found := strings.Cut(line, " b/"); found {
				file = after
				for _, entry := range names.hits(file) {
					lines = append(lines, fmt.Sprintf("%s: %s: the path holds %s", redactedPath, checkPrivateName, privateNameText(entry)))
				}
			}
		case strings.HasPrefix(line, "@@ "):
			if m := reHunkCounts.FindStringSubmatch(line); m != nil {
				oldLeft, newLeft = count(m[1]), count(m[3])
				at, _ = strconv.Atoi(m[2])
			}
		}
	}
	// A bare revision is the work not yet committed, and its diff is all there is to read.
	if !strings.Contains(revisions, "..") {
		return lines, nil, nil
	}
	commits := strings.Replace(revisions, "...", "..", 1)
	// A range from the empty tree reads the branch from its first commit, which `A..B` would leave out.
	if base, head, found := strings.Cut(commits, ".."); found && base == emptyTree {
		commits = head
	}
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

// reHunkCounts is a hunk header's old count, new start and new count. A count left out is one.
var reHunkCounts = regexp.MustCompile(`^@@ -\d+(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// count reads a hunk count. git leaves a count of one out.
func count(text string) int {
	if text == "" {
		return 1
	}
	n, _ := strconv.Atoi(text)
	return n
}
