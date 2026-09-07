package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newLibFixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "r")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("making the fixture root: %v", err)
	}
	newGitRepo(t, root)
	for name, body := range map[string]string{
		"lib/mount.sh":        "#!/bin/sh\ntrue\n",
		"lib/test-harness.sh": "#!/bin/sh\ntrue\n",
		"lib/unsourced.sh":    "#!/bin/sh\ntrue\n",
		"ai/run-tests.sh":     "#!/bin/sh\ntrue\n",
		"ai/bootstrap.sh":     "#!/bin/sh\n. \"$repo/../lib/mount.sh\"\n",
		"ai/bootstrap-test.sh": "#!/bin/sh\n. \"$checkout/lib/test-harness.sh\" ||\n" +
			"  { printf 'no harness\\n' >&2; exit 2; }\n",
		"env/bootstrap.sh":      "#!/bin/sh\n. \"$repo/../lib/mount.sh\"\n",
		"env/bootstrap-test.sh": "#!/bin/sh\n. \"$checkout/lib/test-harness.sh\"\n",
	} {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("making the directory for %s: %v", name, err)
		}
		writeFixture(t, full, body)
	}
	return root
}

func keysOverTree(t *testing.T, root string) map[string]string {
	t.Helper()
	said := &strings.Builder{}
	g := &gate{root: root, env: Env{Root: root}, stamp: "test-digest", errOut: said}
	if code := g.discoverShellSuites(); code != 0 {
		t.Fatalf("discovery exited %d: %s", code, said.String())
	}
	if code := g.buildManifest(); code != 0 {
		t.Fatalf("the manifest exited %d: %s", code, said.String())
	}
	keys := map[string]string{}
	for _, u := range g.units {
		key, _ := g.keyMaterial(u)
		keys[u.id] = key
	}
	return keys
}

func editFixture(t *testing.T, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	writeFixture(t, path, string(body)+"\n# edited\n")
}

func TestEditingASourcedLibraryMovesTheBootstrapUnitsKeys(t *testing.T) {
	root := newLibFixture(t)

	bootstrapUnits := []string{"shell:ai/bootstrap", "shell:env/bootstrap"}
	keys := keysOverTree(t, root)
	for _, id := range bootstrapUnits {
		if keys[id] == "" {
			t.Fatalf("discovery produced no unit called %s, so every assertion below is about a unit "+
				"that does not exist", id)
		}
	}

	for _, lib := range []string{"lib/mount.sh", "lib/test-harness.sh", "lib/unsourced.sh"} {
		isSourced := lib != "lib/unsourced.sh"
		editFixture(t, filepath.Join(root, lib))
		moved := keysOverTree(t, root)
		for _, id := range bootstrapUnits {
			switch {
			case isSourced && moved[id] == keys[id]:
				t.Errorf("editing %s left %s's key where it was. The suite or the script it covers "+
					"sources that file, so the gate answers a cached pass over code that just changed",
					lib, id)
			case !isSourced && moved[id] != keys[id]:
				t.Errorf("editing %s moved %s's key, and nothing sources that file. The unit is keyed "+
					"on lib/ wholesale, which retires good verdicts the way keying on all of "+
					"kk-flavor once did", lib, id)
			}
		}
		keys = moved
	}
}

func TestASuiteSourcingALibraryInAnUnreadableFormIsRefused(t *testing.T) {
	forms := []struct {
		name        string
		line        string
		wantRefusal bool
	}{
		{name: "the form the scan reads", line: `. "$checkout/lib/other.sh"`, wantRefusal: false},
		{name: "source instead of dot", line: `source "$checkout/lib/other.sh"`, wantRefusal: true},
		{name: "an unquoted path", line: `. $checkout/lib/other.sh`, wantRefusal: true},
	}
	for _, form := range forms {
		t.Run(form.name, func(t *testing.T) {
			root := newLibFixture(t)
			writeFixture(t, filepath.Join(root, "lib", "other.sh"), "#!/bin/sh\ntrue\n")
			writeFixture(t, filepath.Join(root, "ai", "extra-test.sh"), "#!/bin/sh\n"+form.line+"\n")

			said := &strings.Builder{}
			g := &gate{root: root, env: Env{Root: root}, stamp: "test-digest", errOut: said}
			code := g.discoverShellSuites()

			if !form.wantRefusal {
				if code != 0 {
					t.Fatalf("discovery refused a source line it does read (exit %d): %s",
						code, said.String())
				}
				return
			}
			if code == 0 {
				t.Fatalf("discovery accepted a library it cannot key on; it built %d unit(s), and the "+
					"one covering ai/extra-test.sh runs that library while hashing nothing of it",
					len(g.units))
			}
			if !strings.Contains(said.String(), "lib/other.sh") {
				t.Errorf("the refusal does not name the library that caused it, so nobody can act on "+
					"it: %q", said.String())
			}
		})
	}
}
