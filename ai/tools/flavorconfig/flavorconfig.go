// Package flavorconfig reads the tracked defaults the flavor ships under `kk-flavor/configs/`.
//
// One shape for every tool with a tunable: `<key> <value>` a line, `#` comments, and the file read
// through the installed mount, so an arbitrary repository being judged does not set the bounds it is
// judged under (`~/.kk-flavor/standards/ecosystem.md` → **Conventions a new file joins**). The flavor's
// own checkout is the exception — there the mount is the working tree — and every value read here is
// a bounded number, never a path, a deletion target or a subprocess argument.
//
// Parsing only. What a value means, whether a missing key is fatal, and what a refusal costs the run
// are each caller's, because each states the consequence in its own vocabulary.
//
// A caller whose key names a path, a deletion target or a command needs more than this: the guards that
// stop another account steering such a value live in `eco-report`'s own reader, which is why the one
// value that reaches a RemoveAll does not come through here.
package flavorconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"kk-flavor/tools/shell"
)

// Path is where the tracked default named name lives. Empty when home is not absolute: there is then
// no mount for one to sit in, which the caller reads the same way as having none.
func Path(home, name string) string {
	if !filepath.IsAbs(home) {
		return ""
	}
	return filepath.Join(home, ".kk-flavor", "configs", name)
}

// Read returns the settings the file at path holds, or nil when there is no such file. allowed lists
// every key the caller understands.
//
// Absent is quiet and everything else refuses, because a default quietly restored is
// indistinguishable from the config working. A key outside allowed is refused rather than skipped for
// the same reason one level down: a line the human meant as a setting, silently ignored, is a value
// they believe they changed and did not.
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

// The link's target, or a stand-in when it cannot be read — the refusal names what it points at, and
// an unreadable link is still a refusal rather than a reason to say nothing.
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
