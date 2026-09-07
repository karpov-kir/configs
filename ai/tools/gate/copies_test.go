package gate

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A repository whose suites build fixtures the way this repo's do: copy repository files in, then run
// them. Every spelling the scan has to resolve is here — a tail rooted at the repository, a tail
// naming a sibling of the suite, and one that resolves nowhere but as a suffix.
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

	// Each file, the unit whose suite copies it in, and whether that unit's key must move when the file is
	// edited. ai/uncopied.sh is the control: no suite copies it, so a unit keyed on the tree wholesale
	// would move for it too and re-run work nobody's edit could have touched.
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
			t.Fatalf("discovery built no unit called %s, so the rows below are checking a unit that "+
				"does not exist", c.unit)
		}
	}

	for _, c := range copies {
		editFixture(t, filepath.Join(root, c.file))
		moved := keysOverTree(t, root)
		switch {
		case c.mustMove && moved[c.unit] == keys[c.unit]:
			t.Errorf("editing %s left %s's key where it was. That suite copies the file in and runs "+
				"the copy, so the gate now reports a cached pass over code that just changed",
				c.file, c.unit)
		case !c.mustMove && moved[c.unit] != keys[c.unit]:
			t.Errorf("editing %s moved %s's key, and no suite copies that file. The unit is keyed on "+
				"more of the tree than it reads, so the gate throws away good verdicts", c.file, c.unit)
		}
		keys = moved
	}
}

func TestACopyNamingAFileTheGateCannotResolveIsRefused(t *testing.T) {
	// One row per spelling a suite can write a copy in. `keyedOn` is what the unit must hash beyond its
	// own base inputs, and an empty one still asserts something: a row that only checked "not refused"
	// would go green on a pattern that had quietly stopped matching anything at all.
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
		{name: "a long flag carrying a value", line: `cp --preserve=all "$checkout/ai/bootstrap-test.sh" "$d/x"`,
			keyedOn: "ai/bootstrap-test.sh"},
		{name: "a braced variable", line: `cp "${checkout}/ai/bootstrap-test.sh" "$d/x"`,
			keyedOn: "ai/bootstrap-test.sh"},
		{name: "two copies on one line", line: `cp "$checkout/ai/bootstrap-test.sh" "$d/x" && cp "$script_dir/mcp-sync.sh" "$d/y"`,
			keyedOn: "ai/bootstrap-test.sh,ai/mcp-sync.sh"},
		{name: "a tail beside the suite", line: `cp "$script_dir/mcp-sync.sh" "$d/x"`,
			keyedOn: "ai/mcp-sync.sh"},
		{name: "a tail resolved by suffix", line: `cp "$skills/kk-reduce/scripts/stats.sh" "$d/x"`,
			keyedOn: "ai/skills/kk-reduce/scripts/stats.sh"},
		{name: "a copy whose basename is a variable", line: `cp "$skills/$skill/scripts/$script" "$d/x"`},
		{name: "a copy of a whole fixture tree", line: `cp -R "$root" "$d/x"`},
		{name: "a directory copy written as a path", line: `cp -R "$checkout/ai/skills" "$d/x"`,
			keyedOn: "ai/skills"},
		{name: "a destination under the fixture, never a source", line: `cp "$script" "$d/ai/mcp-sync.sh"`},
		{name: "a commented-out copy", line: `# cp "$checkout/ai/bootstrap-test.sh" "$d/x"`},
		{name: "a path inside the fixture", line: `cp "$fixture/config.json" "$other/config.json"`},
		{name: "a tail naming no repository file", line: `cp "$dir/ai/absent.sh" "$d/x"`,
			wantRefusal: true, names: "ai/absent.sh"},
		{name: "a tail resolving two ways at once", line: `cp "$either/ai/mcp.jsonc" "$d/x"`,
			keyedOn: "ai/mcp.jsonc"},
		{name: "a tail naming two repository files", line: `cp "$dir/scripts/stats.sh" "$d/x"`,
			keyedOn: "ai/other/scripts/stats.sh,ai/skills/kk-reduce/scripts/stats.sh"},
	}
	for _, form := range forms {
		t.Run(form.name, func(t *testing.T) {
			root := newCopyFixture(t)
			writeRepoFile(t, root, "ai/other/scripts/stats.sh", "#!/bin/sh\ntrue\n")
			writeRepoFile(t, root, "ai/extra-test.sh", "#!/bin/sh\n"+form.line+"\n")

			said := &strings.Builder{}
			g := &gate{root: root, env: Env{Root: root}, stamp: "test-digest", errOut: said}
			code := g.discoverShellSuites()

			if form.wantRefusal {
				if code == 0 {
					t.Fatalf("discovery accepted a copy it cannot resolve (%d units built); the unit "+
						"covering ai/extra-test.sh runs that file while hashing nothing of it",
						len(g.units))
				}
				if !strings.Contains(said.String(), form.names) {
					t.Errorf("the refusal does not name the path that caused it, so the reader cannot "+
						"tell which line to fix: %q", said.String())
				}
				return
			}
			if code != 0 {
				t.Fatalf("discovery refused a copy it can key on (exit %d): %s", code, said.String())
			}
			extra := extraInputs(t, g, "shell:ai/extra")
			want := []string(nil)
			if form.keyedOn != "" {
				want = strings.Split(form.keyedOn, ",")
			}
			if !slices.Equal(extra, want) {
				t.Errorf("the unit covering ai/extra-test.sh is keyed on %v beyond its own base inputs, "+
					"and the copy on that line names %v", extra, want)
			}
		})
	}
}

// Every shell unit hashes its own suite and the runner that reads its result. Dropping those two
// leaves what the scans added, which is all these cases are about.
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

// A copied path is the one unit input built from a file's text rather than from git's own listing, so
// it is the one that can carry pathspec magic into `buildManifest`'s `git ls-files` call. There,
// `:!ai/tools/gate` reads as an exclude: those files drop out of the manifest, and whatever the
// excluder chose passes from cache from then on. The fixture names a directory git reads as magic and
// copies a file out of it, the shortest route to that value becoming an input.
func TestACopiedPathHoldingPathspecMagicIsRefused(t *testing.T) {
	root := newCopyFixture(t)
	writeRepoFile(t, root, ":!ai/target.sh", "#!/bin/sh\ntrue\n")
	writeRepoFile(t, root, "ai/hostile-test.sh", "#!/bin/sh\ncp \"$x/:!ai/target.sh\" \"$d/y\"\n")

	said := &strings.Builder{}
	g := &gate{root: root, env: Env{Root: root}, stamp: "test-digest", errOut: said}

	if code := g.discoverShellSuites(); code == 0 {
		for _, u := range g.units {
			if u.id != "shell:ai/hostile" {
				continue
			}
			t.Fatalf("discovery accepted a copied path holding pathspec magic and keyed %s on %v; "+
				"those values reach git ls-files as pathspecs, where an exclude silently shrinks the "+
				"manifest", u.id, u.inputs)
		}
		t.Fatalf("discovery accepted the fixture but built no unit for ai/hostile-test.sh, so this " +
			"case checked nothing")
	}
	if !strings.Contains(said.String(), "copied path") {
		t.Errorf("the refusal never says it was a copied path, so it reads like any other gate "+
			"refusal: %q", said.String())
	}
}
