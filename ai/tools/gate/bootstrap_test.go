package gate

import (
	"path/filepath"
	"testing"
)

func TestOwnerBootstrapSuitesTrackTheFilesTheyRunAndRead(t *testing.T) {
	root := newLibFixture(t)
	for name, body := range map[string]string{
		"ai/rtk-bootstrap-test.sh": "#!/bin/sh\nbash \"$here/bootstrap.sh\" --agent=codex --owner\n",
		"ai/bootstrap-owner.sh":    "#!/bin/sh\nbash \"$here/bootstrap.sh\" --owner \"$@\"\n",
		"ai/owner-instructions.md": "# Owner instructions\n",
	} {
		writeRepoFile(t, root, name, body)
	}
	units := []string{"shell:ai/bootstrap", "shell:ai/rtk-bootstrap"}
	keys := keysOverTree(t, root)
	for _, id := range units {
		if keys[id] == "" {
			t.Fatalf("discovery produced no %s unit", id)
		}
	}
	for _, file := range []string{
		"ai/bootstrap.sh", "ai/owner-instructions.md", "lib/mount.sh",
		"ai/bootstrap-owner.sh", "lib/unsourced.sh",
	} {
		editFixture(t, filepath.Join(root, file))
		moved := keysOverTree(t, root)
		for _, id := range units {
			wantMove := file != "lib/unsourced.sh" &&
				(file != "ai/bootstrap-owner.sh" || id == "shell:ai/bootstrap")
			if gotMove := moved[id] != keys[id]; gotMove != wantMove {
				t.Errorf("editing %s changed %s cache key: %v, want %v", file, id, gotMove, wantMove)
			}
		}
		keys = moved
	}
}
