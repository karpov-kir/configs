package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const noInputsRefusal = "not one input path resolved to a file — nothing ran"

// What a unit's recorded input lines are filed under, its stem before it. One home, because gate.go's
// sweep recognises a leaked temp by this same spelling — spelt twice, a rename on one side leaves the
// sweep matching nothing and reporting exactly the silence it reports over a clean store.
const sidecarSuffix = ".inputs"

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
	//
	// Literal pathspecs, because one of these patterns comes out of the text of a shell script while the
	// rest are plain paths. Leave the magic on and `:!ai/tools/gate` arrives as an EXCLUDE: those files
	// drop out of the manifest, the no-input refusal stays quiet because the other patterns still match,
	// and every unit keyed under that directory answers from cache forever.
	args := append([]string{"ls-files", "-z", "--cached", "--others", "--exclude-standard", "--"}, patterns...)
	out, err := g.captureLiteralPathspecs(args...)
	if err != nil && out == "" {
		return g.fail("%s", noInputsRefusal)
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
		return g.fail("%s", noInputsRefusal)
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
			return g.fail("could not read the declared input %s, so some file's changes would stop invalidating its unit — nothing ran", path)
		}
		g.manifest = append(g.manifest, manifestLine{hash: hash, path: path})
	}
	if len(g.manifest) == 0 {
		return g.fail("%s", noInputsRefusal)
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

func isGoTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.go")
}

func (g *gate) keyMaterial(u unit) (key string, lines []manifestLine) {
	lines = linesUnder(g.manifest, u.inputs)
	if u.blindToGoTests {
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
//
// No `<stem>.inputs` sidecar carries a key, and this is the only one anything reads back. The store
// is the clone's (gate.go, at g.cache), so every worktree of a checkout reads and writes this one
// path. Sound for the reason sharing the records is sound: the body is a set of `hash  path` lines,
// so one written by another worktree compares equal only where the bytes really are equal. A worktree
// holding different content wrote a body that differs, this answers true, and the caller over-runs —
// the safe direction.
//
// Sound only over a COMPLETE body, which is why writeSidecar publishes by rename. A record that lost
// its tail is a well-formed record of a smaller set, and nothing in the bytes says which it is: a
// green recorded over {a, b} and cut back to {a} compares equal to a live {a} whose b was deleted, so
// this answers false over content that moved. TestASidecarMissingALineCannotBeToldFromAMatch holds
// that, and is what to read before trusting a body from anywhere but writeSidecar.
func (g *gate) changedSinceGreen(paths []string) bool {
	recorded := filepath.Join(g.cache, "gotest"+sidecarSuffix)
	body, err := os.ReadFile(recorded)
	if err != nil {
		// Nothing recorded means nothing to compare against, so every group counts as moved: it
		// over-runs, and never skips.
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

// A unit's input sidecar, published whole. Written beside its path and renamed onto it, never written
// onto it: the store is shared by every worktree of the clone, so the reader racing this is another
// gate run rather than this one, and a write onto the path truncates it to nothing first. That window
// alone makes a peer over-force packages it did not need to; a peer catching the write further along
// gets a shorter record it has no way to refuse, and then skips forcing a package whose inputs moved.
// Rename is one step, so there is no part-done state to catch.
//
// Silent, because a sidecar that does not appear costs the next run some forcing and nothing else —
// there is no verdict here for a caller to decide.
func writeSidecar(path, body string) {
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return
	}
	_, err = temp.WriteString(body)
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(temp.Name(), 0o644)
	}
	if err == nil {
		err = os.Rename(temp.Name(), path)
	}
	if err != nil {
		os.Remove(temp.Name())
	}
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
