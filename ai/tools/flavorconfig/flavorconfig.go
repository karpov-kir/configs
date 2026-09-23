// Package flavorconfig reads the tracked defaults the flavor ships under `kk-flavor/configs/`.
//
// One shape for every tool with a tunable: `<key> <value>` a line, `#` comments. The file comes from
// the checkout the running binary was built in, so a config is exactly as trusted as the code reading
// it, and an arbitrary repository being judged does not set the bounds it is judged under
// (`~/.kk-flavor/standards/ecosystem.md` → "Conventions a new file joins").
//
// This package parses. What a value means and what a refusal costs the run are each caller's.
//
// Every value read here is a bounded number; a key naming a path, a deletion target or a command
// needs the guards in `eco-report`'s own reader instead.
package flavorconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"configs/ai/tools/shell"
)

// Path returns where the tracked default named name lives. Empty when there is nowhere for one to sit,
// which the caller reads the same way as having none.
func Path(home, name string) string {
	exe, err := os.Executable()
	if err != nil {
		exe = ""
	}
	dir := dirFor(exe, home)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, name)
}

// dirFor is Path's decision, with the executable passed in so a case can name one.
//
// A binary the resolver builds or downloads sits at `<checkout>/ai/tools/bin/<tool>`, and that
// checkout's configs are the ones this reads. Two things follow. A worktree tests its own configs
// rather than whatever the installed mount has checked out, and CI, which has no mount, reads the
// configs it built from. A host repository still cannot choose them: a skill runs a script through
// `~/.kk-flavor` wherever the working directory holds code the human did not write, and that stub
// resolves to the installed checkout's binary.
//
// A binary anywhere else — a `go test` build, a `go run` — falls back to the mount under home. Empty
// when home is not absolute: there is then no mount for one to sit in.
func dirFor(exe, home string) string {
	if exe != "" {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		bin := filepath.Dir(exe)
		if filepath.Base(bin) == "bin" && filepath.Base(filepath.Dir(bin)) == "tools" {
			beside := filepath.Join(filepath.Dir(filepath.Dir(bin)), "kk-flavor", "configs")
			if info, err := os.Stat(beside); err == nil && info.IsDir() {
				return beside
			}
		}
	}
	if !filepath.IsAbs(home) {
		return ""
	}
	return filepath.Join(home, ".kk-flavor", "configs")
}

// Read returns the settings the file at path holds, or nil when there is no such file. allowed lists
// every key the caller understands.
//
// Everything but an absent file refuses: a default quietly restored is indistinguishable from the
// config working, and a key outside allowed is a setting the human believes they changed and did not.
func Read(path string, allowed []string) (map[string]string, error) {
	// `IsSymlink` as well, so a dangling link refuses instead of reading as absent — an existence test
	// alone cannot see one.
	if path == "" || (!shell.PathExists(path) && !shell.IsSymlink(path)) {
		return nil, nil
	}
	// Before IsRegularFile, which FOLLOWS the link and so passes a link to a good file: whoever can
	// repoint it chooses what this tool reads on the next run. Only the final component is tested, so
	// the `~/.kk-flavor` mount being a symlink is not what this catches.
	if shell.IsSymlink(path) {
		return nil, fmt.Errorf("%s is a symlink -> %s, and whoever can repoint it chooses what this reads; replace it with a regular file",
			path, shell.Oneline(readLink(path)))
	}
	if !shell.IsRegularFile(path) {
		return nil, fmt.Errorf("%s is not a readable regular file", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read %s (%v)", path, err)
	}
	settings := map[string]string{}
	for _, line := range shell.SplitLines(string(raw)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// One-lined AND cut, these files being written by hand and their content reaching a terminal: a
		// line holding no newline is as long as the file, and echoing it whole buries the refusal it
		// belongs to.
		fields := shell.SplitFields(trimmed)
		if len(fields) != 2 || !slices.Contains(allowed, fields[0]) {
			return nil, fmt.Errorf("%s has a line this does not understand: %s — the supported lines are %s",
				path, echoable(trimmed), supported(allowed))
		}
		if _, repeated := settings[fields[0]]; repeated {
			return nil, fmt.Errorf("%s sets %s more than once — which one wins is not this tool's guess to make", path, fields[0])
		}
		settings[fields[0]] = fields[1]
	}
	return settings, nil
}

// Returns the link's target, or a stand-in when it cannot be read.
func readLink(path string) string {
	target, err := os.Readlink(path)
	if err != nil {
		return "(unreadable)"
	}
	return target
}

func echoable(line string) string {
	return shell.CutBytesMarked(shell.Oneline(line), 80)
}

func supported(allowed []string) string {
	forms := make([]string, 0, len(allowed))
	for _, key := range allowed {
		forms = append(forms, "`"+key+" <value>`")
	}
	return strings.Join(forms, ", ")
}
