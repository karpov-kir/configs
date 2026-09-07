package gate

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A repository whose suites build their fixtures the way this one's do: by copying repository files in
// and running them. The three spellings the scan has to resolve are all here — a repository-rooted
// tail, a tail naming a sibling of the suite, and one that resolves nowhere but as a suffix.
func newCopyFixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "r")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("making the fixture root: %v", err)
	}
	newGitRepo(t, root)
	for name, body := range map[string]string{
		"lib/test-harness.sh":                  "#!/bin/sh\ntrue\n",
		"ai/run-tests.sh":                      "#!/bin/sh\ntrue\n",
		"ai/bootstrap-test.sh":                 "#!/bin/sh\ntrue\n",
		"ai/mcp-sync.sh":                       "#!/bin/sh\ntrue\n",
		"ai/mcp.jsonc":                         "{}\n",
		"ai/uncopied.sh":                       "#!/bin/sh\ntrue\n",
		"ai/skills/kk-reduce/scripts/stats.sh": "#!/bin/sh\ntrue\n",
		"env/bootstrap-test.sh":                "#!/bin/sh\ncp \"$checkout/ai/bootstrap-test.sh\" \"$halfway/ai/bootstrap-test.sh\"\n",
		"ai/mcp-sync-test.sh":                  "#!/bin/sh\ncp \"$script_dir/mcp.jsonc\" \"$dir/mcp.jsonc\"\n",
		"ai/deep-test.sh":                      "#!/bin/sh\ncp \"$skills/kk-reduce/scripts/stats.sh\" \"$dir/stats.sh\"\n",
	} {
		writeRepoFile(t, root, name, body)
	}
	return root
}

// writeFixture with the parent directories made, so a case can add a file the fixture has no folder
// for yet.
func writeRepoFile(t *testing.T, root, name, body string) {
	t.Helper()
	full := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("making the directory for %s: %v", name, err)
	}
	writeFixture(t, full, body)
}

func TestEditingACopiedRepositoryFileMovesTheCopyingUnitsKey(t *testing.T) {
	root := newCopyFixture(t)

	// Each file, the unit whose suite copies it in, and whether that unit's key must move when it is
	// edited. ai/uncopied.sh is the control: no suite copies it, and a unit keyed on the tree wholesale
	// would move for it too, which retires verdicts nobody had to.
	copies := []struct {
		file     string
		unit     string
		mustMove bool
	}{
		{file: "ai/bootstrap-test.sh", unit: "shell:env/bootstrap", mustMove: true},
		{file: "ai/mcp.jsonc", unit: "shell:ai/mcp-sync", mustMove: true},
		{file: "ai/skills/kk-reduce/scripts/stats.sh", unit: "shell:ai/deep", mustMove: true},
		{file: "ai/uncopied.sh", unit: "shell:ai/mcp-sync", mustMove: false},
	}

	keys := keysOverTree(t, root)
	for _, c := range copies {
		if keys[c.unit] == "" {
			t.Fatalf("discovery produced no unit called %s, so every assertion below is about a unit "+
				"that does not exist", c.unit)
		}
	}

	for _, c := range copies {
		editFixture(t, filepath.Join(root, c.file))
		moved := keysOverTree(t, root)
		switch {
		case c.mustMove && moved[c.unit] == keys[c.unit]:
			t.Errorf("editing %s left %s's key where it was. That suite copies the file into its "+
				"fixture and runs the copy, so the gate answers a cached pass over code that just "+
				"changed", c.file, c.unit)
		case !c.mustMove && moved[c.unit] != keys[c.unit]:
			t.Errorf("editing %s moved %s's key, and no suite copies that file. The unit is keyed on "+
				"more of the tree than it reads, which retires good verdicts", c.file, c.unit)
		}
		keys = moved
	}
}

func TestACopyNamingAFileTheGateCannotResolveIsRefused(t *testing.T) {
	// One row per spelling a suite can write a copy in. `keyedOn` is what the unit must hash beyond its
	// own base inputs — empty where the copy names no file, which is a claim about the pattern's reach
	// rather than an absent expectation: a row that only asserted "not refused" would pass on a
	// pattern that had stopped matching anything at all.
	forms := []struct {
		name        string
		line        string
		keyedOn     string
		wantRefusal bool
		names       string
	}{
		{name: "a repository-rooted tail", line: `cp "$checkout/ai/bootstrap-test.sh" "$d/x"`,
			keyedOn: "ai/bootstrap-test.sh"},
		{name: "a long flag before the tail", line: `cp --preserve "$checkout/ai/bootstrap-test.sh" "$d/x"`,
			keyedOn: "ai/bootstrap-test.sh"},
		{name: "a braced variable", line: `cp "${checkout}/ai/bootstrap-test.sh" "$d/x"`,
			keyedOn: "ai/bootstrap-test.sh"},
		{name: "a tail beside the suite", line: `cp "$script_dir/mcp-sync.sh" "$d/x"`,
			keyedOn: "ai/mcp-sync.sh"},
		{name: "a tail resolved by suffix", line: `cp "$skills/kk-reduce/scripts/stats.sh" "$d/x"`,
			keyedOn: "ai/skills/kk-reduce/scripts/stats.sh"},
		{name: "a copy whose basename is a variable", line: `cp "$skills/$skill/scripts/$script" "$d/x"`},
		{name: "a copy of a whole fixture tree", line: `cp -R "$root" "$d/x"`},
		{name: "a destination under the fixture, never a source", line: `cp "$script" "$d/ai/mcp-sync.sh"`},
		{name: "a tail naming no repository file", line: `cp "$dir/absent.sh" "$d/x"`,
			wantRefusal: true, names: "absent.sh"},
		{name: "a tail naming two repository files", line: `cp "$dir/scripts/stats.sh" "$d/x"`,
			wantRefusal: true, names: "scripts/stats.sh"},
	}
	for _, form := range forms {
		t.Run(form.name, func(t *testing.T) {
			root := newCopyFixture(t)
			// The second stats.sh, which is what makes the suffix rule ambiguous for the case above.
			writeRepoFile(t, root, "ai/other/scripts/stats.sh", "#!/bin/sh\ntrue\n")
			writeRepoFile(t, root, "ai/extra-test.sh", "#!/bin/sh\n"+form.line+"\n")

			said := &strings.Builder{}
			g := &gate{root: root, env: Env{Root: root}, stamp: "test-digest", errOut: said}
			code := g.discoverShellSuites()

			if form.wantRefusal {
				if code == 0 {
					t.Fatalf("discovery accepted a copy it cannot resolve; it built %d unit(s), and the "+
						"one covering ai/extra-test.sh runs that file while hashing nothing of it",
						len(g.units))
				}
				if !strings.Contains(said.String(), form.names) {
					t.Errorf("the refusal does not name the path that caused it, so nobody can act on "+
						"it: %q", said.String())
				}
				return
			}
			if code != 0 {
				t.Fatalf("discovery refused a copy it can key on (exit %d): %s", code, said.String())
			}
			extra := extraInputs(t, g, "shell:ai/extra")
			want := []string(nil)
			if form.keyedOn != "" {
				want = []string{form.keyedOn}
			}
			if !slices.Equal(extra, want) {
				t.Errorf("the unit covering ai/extra-test.sh is keyed on %v beyond its own base inputs, "+
					"and the copy on that line names %v", extra, want)
			}
		})
	}
}

// What a unit hashes beyond what every shell unit hashes — its own suite and the runner that reads its
// result. Whatever is left came from the scans, which is what these cases are about.
func extraInputs(t *testing.T, g *gate, id string) []string {
	t.Helper()
	base := map[string]bool{strings.TrimPrefix(id, "shell:") + "-test.sh": true, "ai/run-tests.sh": true}
	for _, u := range g.units {
		if u.id != id {
			continue
		}
		var extra []string
		for _, in := range u.inputs {
			if !base[in] {
				extra = append(extra, in)
			}
		}
		return extra
	}
	t.Fatalf("discovery produced no unit called %s, so this case checked nothing", id)
	return nil
}

func TestACopyOfAnIgnoredBuildArtifactIsNeitherKeyedNorRefused(t *testing.T) {
	root := newCopyFixture(t)
	writeRepoFile(t, root, "ai/tools/.gitignore", "bin/\n")
	// ai/tools/tool-stub-test.sh copies ai/tools/bin/eco-stats in; the fixture names something else,
	// because every real tool name is a marker that keys a suite on the whole tool tree and would hide
	// the one input this case is about.
	writeRepoFile(t, root, "ai/tools/artifact-test.sh", "#!/bin/sh\ncp \"$here/bin/stub-tool\" \"$d/x\"\n")

	said := &strings.Builder{}
	g := &gate{root: root, env: Env{Root: root}, stamp: "test-digest", errOut: said}
	if code := g.discoverShellSuites(); code != 0 {
		t.Fatalf("discovery refused a copy of a build artifact (exit %d). Nothing in the repository "+
			"holds that path, and its source tree is what a unit reading it is keyed on: %s",
			code, said.String())
	}
	if extra := extraInputs(t, g, "shell:ai/tools/artifact"); len(extra) != 0 {
		t.Errorf("the unit is keyed on %v, and git tracks none of it, so its hash comes from whatever "+
			"the last build left behind", extra)
	}
}
