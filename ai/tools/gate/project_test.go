package gate

import (
	"path/filepath"
	"testing"
)

func TestAShellSuiteIsKeyedOnItsScriptAndEveryLibraryThatScriptSources(t *testing.T) {
	root := newLibFixture(t)
	for name, body := range map[string]string{
		"ai/install-project.sh":      "#!/bin/sh\n. \"$repo/../lib/mount.sh\"\n",
		"ai/install-project-test.sh": "#!/bin/sh\ntrue\n",
	} {
		writeRepoFile(t, root, name, body)
	}
	// One unit, and a fixture rather than the tree: the project installer is Go now, so the repository
	// itself has no such suite. What this holds is the general rule every shell suite still depends on —
	// a unit is keyed on the script it covers and on every library that script sources.
	units := []string{"shell:ai/install-project"}
	keys := keysOverTree(t, root)
	for _, id := range units {
		if keys[id] == "" {
			t.Fatalf("discovery produced no %s unit", id)
		}
	}
	for _, scenario := range []struct {
		file      string
		wantMoved bool
	}{
		{file: "ai/install-project.sh", wantMoved: true},
		{file: "lib/mount.sh", wantMoved: true},
		{file: "ai/bootstrap.sh"},
		{file: "lib/unsourced.sh"},
	} {
		editFixture(t, filepath.Join(root, scenario.file))
		moved := keysOverTree(t, root)
		for _, id := range units {
			if gotMove := moved[id] != keys[id]; gotMove != scenario.wantMoved {
				t.Errorf("editing %s changed %s cache key: %v, want %v", scenario.file, id, gotMove, scenario.wantMoved)
			}
		}
		keys = moved
	}
}
