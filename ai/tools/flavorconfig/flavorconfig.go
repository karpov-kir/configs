// Package flavorconfig reads the tracked defaults the flavor ships under `kk-flavor/configs/`, one
// `<key> <value>` a line with `#` comments. The file comes from the checkout the running binary was
// built in. A config is then as trusted as the code reading it, and a repository being judged cannot
// set its own bounds (`~/.kk-flavor/standards/ecosystem.md`).
//
// This package parses. What a value means and what a refusal costs the run are each caller's. Every
// value read here is a bounded number. A key naming a path, a deletion target or a command needs the
// guards in `eco-report`'s own reader.
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

// dirFor is Path's decision, with the executable passed in so a case can name one. A worktree tests
// its own configs this way, and CI reads the configs it built from.
func dirFor(exe, home string) string {
	if dir := configsBesideBinary(exe); dir != "" {
		return dir
	}
	return configsInMount(home)
}

// configsBesideBinary answers for a binary at `<checkout>/ai/tools/bin/<tool>`, where the resolver
// puts one. A host repository cannot choose it. A skill runs a script through `~/.kk-flavor` wherever
// the working directory holds code the human did not write. That stub runs the installed binary.
func configsBesideBinary(exe string) string {
	if exe == "" {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	bin := filepath.Dir(exe)
	if filepath.Base(bin) != "bin" || filepath.Base(filepath.Dir(bin)) != "tools" {
		return ""
	}
	beside := filepath.Join(filepath.Dir(filepath.Dir(bin)), "kk-flavor", "configs")
	if info, err := os.Stat(beside); err != nil || !info.IsDir() {
		return ""
	}
	return beside
}

// configsInMount answers for any other binary, such as a `go test` build. A relative home has no
// mount to read.
func configsInMount(home string) string {
	if !filepath.IsAbs(home) {
		return ""
	}
	return filepath.Join(home, ".kk-flavor", "configs")
}

// Read returns the settings the file at path holds, or nil when there is no such file. allowed lists
// every key the caller understands.
//
// Everything but an absent file refuses. A default quietly restored looks the same as the config
// working. A key outside allowed is a setting the human believes they changed.
func Read(path string, allowed []string) (map[string]string, error) {
	// `IsSymlink` as well, so a dangling link refuses. An existence test by itself reads one as absent.
	if path == "" || (!shell.PathExists(path) && !shell.IsSymlink(path)) {
		return nil, nil
	}
	// Ahead of IsRegularFile, which follows the link and passes a link to a good file. Whoever can
	// repoint the link chooses what this tool reads next run. Only the final component is tested, so the
	// `~/.kk-flavor` mount being a symlink is fine.
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
		fields := shell.SplitFields(trimmed)
		if len(fields) != 2 || !slices.Contains(allowed, fields[0]) {
			return nil, fmt.Errorf("%s has a line this does not understand: %s — the supported lines are %s",
				path, shell.Echoable(trimmed), supported(allowed))
		}
		if _, repeated := settings[fields[0]]; repeated {
			return nil, fmt.Errorf("%s sets %s more than once — which one wins is not this tool's guess to make", path, fields[0])
		}
		settings[fields[0]] = fields[1]
	}
	return settings, nil
}

func readLink(path string) string {
	target, err := os.Readlink(path)
	if err != nil {
		return "(unreadable)"
	}
	return target
}

func supported(allowed []string) string {
	forms := make([]string, 0, len(allowed))
	for _, key := range allowed {
		forms = append(forms, "`"+key+" <value>`")
	}
	return strings.Join(forms, ", ")
}
