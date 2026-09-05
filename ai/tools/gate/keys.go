package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Every input file of every unit, hashed once. One pass over the union rather than one per unit: the
// units overlap heavily, and hashing ai/tools once instead of forty times is the difference between a
// key computation that is free and one that is the slowest thing here.
//
// In process, because on a machine that charges for every exec, keying the gate would otherwise cost
// more than several of the units it is deciding about.
func (g *gate) buildManifest() int {
	declared := map[string]bool{}
	var patterns []string
	for _, u := range g.units {
		for _, in := range u.inputs {
			if !declared[in] {
				declared[in] = true
				patterns = append(patterns, in)
			}
		}
	}
	sort.Strings(patterns)

	// --cached and --others: a new file that is not yet added still changes what the suites see, and a
	// gate that keys only on tracked files reports a cached pass over a test someone just wrote.
	args := append([]string{"ls-files", "-z", "--cached", "--others", "--exclude-standard", "--"}, patterns...)
	out, err := g.capture("git", args...)
	if err != nil && out == "" {
		return g.fail("not one input path resolved to a file — nothing ran")
	}
	seen := map[string]bool{}
	var paths []string
	for _, path := range strings.Split(out, "\x00") {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		return g.fail("not one input path resolved to a file — nothing ran")
	}
	sort.Strings(paths)

	// Existing files only, since ls-files still lists one that was deleted but not yet staged. Skipping
	// it is what MOVES that unit's key rather than what hides the deletion: a key is built from the
	// manifest lines alone, so one line fewer is a different key and the unit runs.
	for _, path := range paths {
		full := filepath.Join(g.root, path)
		info, err := os.Stat(full)
		if err != nil || info.IsDir() {
			continue
		}
		hash, err := hashFile(full)
		if err != nil {
			// A file that could not be read used to vanish from the manifest silently, which takes it
			// out of every key that declared it — its edits then stop invalidating anything, and the
			// gate narrows itself exactly as its header swears it will not.
			return g.fail("could not read the declared input %s, so some file's changes would stop invalidating its unit — nothing ran", path)
		}
		g.manifest = append(g.manifest, manifestLine{hash: hash, path: path})
	}
	if len(g.manifest) == 0 {
		return g.fail("not one input path resolved to a file — nothing ran")
	}
	return 0
}

// The manifest lines under one of these paths, sorted so a key does not move with the order the
// listing happened to answer in. A path matches a line when it IS that line's path or is a directory
// prefix of it.
func linesUnder(manifest []manifestLine, paths []string) []manifestLine {
	want := make([]string, 0, len(paths))
	for _, p := range paths {
		want = append(want, strings.TrimSuffix(p, "/"))
	}
	var out []manifestLine
	for _, line := range manifest {
		for _, w := range want {
			if line.path == w || strings.HasPrefix(line.path, w+"/") {
				out = append(out, line)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// A Go test file, which `go build` never compiles into a binary.
//
// Matched on the path rather than on the package, because that is the whole of what the exclusion
// claims: the file is one the toolchain leaves out of every binary. `_test.go` is the compiler's own
// spelling of that, so this cannot drift away from what `go build` does.
func isGoTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.go")
}

// One unit's key material: three header lines, then the sorted `<hash>  <path>` line per input file.
//
// A unit that reaches Go only through a compiled binary drops the module's test files here rather
// than at discovery, so the narrowing is in one place and `--why` prints exactly what the key was
// built from.
func (g *gate) keyMaterial(u unit) (key string, lines []manifestLine) {
	lines = linesUnder(g.manifest, u.inputs)
	if u.viaCompiledBinary {
		kept := lines[:0:0]
		for _, line := range lines {
			if !isGoTestFile(line.path) {
				kept = append(kept, line)
			}
		}
		lines = kept
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n%s\n", u.id, u.cmd, g.stamp)
	b.WriteString(renderLines(lines))
	return hashString(b.String()), lines
}

// Whether one path set's contents differ from what the last green `gotest` was keyed on. Read from
// that unit's own recorded input lines, so it answers about the same bytes the verdict was recorded
// over.
func (g *gate) changedSinceGreen(paths []string) bool {
	recorded := filepath.Join(g.cache, "gotest.inputs")
	body, err := os.ReadFile(recorded)
	if err != nil {
		// Nothing recorded means nothing to compare against, so every group counts as moved.
		// Conservative in the only safe direction: it over-runs, it never skips.
		return true
	}
	var was []manifestLine
	for _, line := range strings.Split(string(body), "\n") {
		hash, path, ok := strings.Cut(line, "  ")
		if !ok {
			continue
		}
		was = append(was, manifestLine{hash: hash, path: path})
	}
	return digestOf(linesUnder(g.manifest, paths)) != digestOf(linesUnder(was, paths))
}

func digestOf(lines []manifestLine) string {
	return hashString(renderLines(lines))
}

func renderLines(lines []manifestLine) string {
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(line.String())
		b.WriteByte('\n')
	}
	return b.String()
}
