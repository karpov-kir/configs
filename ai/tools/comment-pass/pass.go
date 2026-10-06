package commentpass

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const usage = "usage: comment-pass.sh --base=<rev> [--notes=<file>] [--list | --dry-run] [--page=<file>] [--model=<name>] [<path>...]"

// sourceExtensions are the files whose comment syntax the block finder reads.
var sourceExtensions = map[string]bool{".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true,
	".cjs": true, ".go": true, ".py": true, ".sh": true, ".java": true, ".kt": true, ".swift": true, ".rs": true}

// Run is the pass over a change: each changed source file in one call. Exit 0: every file passed.
// Exit 1: a file's reply or gate failed. Exit 2: the pass did not run.
func Run(self string, args []string, cwd string, lookup func(string) (string, bool), call func(model string) Caller,
	stdout, stderr io.Writer) int {
	refuse := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "%s: "+format+" — the pass did NOT run\n%s\n", append([]any{self}, append(a, usage)...)...)
		return 2
	}
	var base, notesFile, pageFile, model string
	list, dry := false, false
	var paths []string
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--base="):
			base = strings.TrimPrefix(arg, "--base=")
		case strings.HasPrefix(arg, "--notes="):
			notesFile = strings.TrimPrefix(arg, "--notes=")
		case strings.HasPrefix(arg, "--page="):
			pageFile = strings.TrimPrefix(arg, "--page=")
		case strings.HasPrefix(arg, "--model="):
			model = strings.TrimPrefix(arg, "--model=")
		case arg == "--list":
			list = true
		case arg == "--dry-run":
			dry = true
		case strings.HasPrefix(arg, "--"):
			return refuse("%q is no option", arg)
		default:
			paths = append(paths, arg)
		}
	}
	if base == "" {
		return refuse("%s", "it takes --base=<rev>, the change's base")
	}
	home, _ := lookup("HOME")
	if pageFile == "" {
		pageFile = filepath.Join(home, ".kk-flavor", "standards", "comment-page.md")
	}
	page, err := os.ReadFile(pageFile)
	if err != nil && !list {
		return refuse("cannot read the page at %s", pageFile)
	}
	page = []byte(withoutLayer(string(page)))
	notes, err := readNotes(notesFile)
	if err != nil {
		return refuse("%v", err)
	}
	top, err := gitOut(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return refuse("%s is no git checkout", cwd)
	}
	if len(paths) == 0 {
		names, err := gitOut(top, "diff", "--name-only", "--diff-filter=AMR", "-M", base)
		if err != nil {
			return refuse("git cannot list the files changed since %s", base)
		}
		paths = strings.Fields(names)
	}
	stateHome, ok := lookup("XDG_STATE_HOME")
	if !ok || stateHome == "" {
		stateHome = filepath.Join(home, ".local", "state")
	}
	repoKey := repositoryKey(top)
	width := printWidth(top)
	changedByFile, err := changedLines(top, base)
	if err != nil {
		return refuse("%v", err)
	}
	failed := 0
	sort.Strings(paths)
	for _, path := range paths {
		if !sourceExtensions[filepath.Ext(path)] {
			continue
		}
		body, err := os.ReadFile(filepath.Join(top, path))
		if err != nil {
			continue
		}
		// A file with CRLF line ends is read without them and written with them again.
		crlf := strings.Contains(string(body), "\r\n")
		text := strings.ReplaceAll(string(body), "\r\n", "\n")
		lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
		m := findMaterial(path, lines, changedByFile[path])
		if len(m.candidates) == 0 && len(m.places) == 0 {
			continue
		}
		prompt := userPrompt(path, lines, m, notes[path])
		if list {
			fmt.Fprintf(stdout, "%s: %d comment(s), %d place(s)\n", path, len(m.candidates), len(m.places))
			for _, c := range m.candidates {
				fmt.Fprintf(stdout, "  %s lines %d-%d, on line %d\n", c.id, c.first, c.last, c.decl)
			}
			for _, p := range m.places {
				fmt.Fprintf(stdout, "  %s line %d\n", p.id, p.line)
			}
			continue
		}
		key := inputKey(string(page), prompt, model)
		store := cachePath(stateHome, repoKey, path)
		reply, source := cachedReply(store, key), "kept"
		if reply == "" {
			if reply, err = call(model)(string(page), prompt); err != nil {
				failed++
				fmt.Fprintf(stdout, "%s: the call failed: %v\n", path, err)
				continue
			}
			source = "called"
		}
		if dry {
			fmt.Fprintf(stdout, "=== %s (%s)\n--- system\n%s\n--- user\n%s\n--- reply\n%s\n", path, source, page, prompt, reply)
		}
		decisions, err := parseReply(reply, m)
		if err != nil {
			failed++
			fmt.Fprintf(stdout, "%s: the reply does not parse: %v\n%s\n", path, err, reply)
			continue
		}
		if found := gateFindings(m, decisions, widthFor(path, width)); len(found) > 0 {
			failed++
			fmt.Fprintf(stdout, "%s: the gate refused the file, which is left as it was:\n%s\n", path, strings.Join(found, "\n"))
			continue
		}
		// A reply is kept only once it parsed and passed the gate, so a refused one is asked again.
		if source == "called" {
			if err := keepReply(store, key, reply); err != nil {
				fmt.Fprintf(stderr, "%s: cannot keep the reply for %s: %v\n", self, path, err)
			}
		}
		written := 0
		for _, d := range decisions {
			if d.verb != "keep" && d.verb != "skip" {
				written++
			}
		}
		fmt.Fprintf(stdout, "%s: %d decision(s), %d edit(s), reply %s\n", path, len(decisions), written, source)
		if dry || written == 0 {
			continue
		}
		out := strings.Join(applyDecisions(lines, m, decisions), "\n") + "\n"
		if crlf {
			out = strings.ReplaceAll(out, "\n", "\r\n")
		}
		if err := os.WriteFile(filepath.Join(top, path), []byte(out), 0o644); err != nil {
			return refuse("cannot write %s", path)
		}
	}
	if failed > 0 {
		return 1
	}
	return 0
}

// readNotes reads the reviewer's notes, one `<path>:<line> <what is wrong>` per line, by path.
func readNotes(file string) (map[string][]string, error) {
	out := map[string][]string{}
	if file == "" {
		return out, nil
	}
	body, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("cannot read the notes at %s", file)
	}
	for n, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		at, _, found := strings.Cut(line, " ")
		path, _, colon := strings.Cut(at, ":")
		if !found || !colon {
			return nil, fmt.Errorf("line %d of the notes is no `<path>:<line> <what is wrong>`", n+1)
		}
		out[path] = append(out[path], line)
	}
	return out, nil
}

// repositoryKey names the repository the cache keeps replies for. Every worktree shares the origin's
// URL, and a checkout with no origin falls back to its path.
func repositoryKey(top string) string {
	name, err := gitOut(top, "remote", "get-url", "origin")
	if err != nil || name == "" {
		name = top
	}
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:6])
}

func gitOut(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

// withoutLayer takes off the `**Layer:**` line every standard opens on. The model has no use for it.
func withoutLayer(page string) string {
	if first, rest, found := strings.Cut(page, "\n"); found && strings.HasPrefix(first, "**Layer:**") {
		return strings.TrimLeft(rest, "\n")
	}
	return page
}
