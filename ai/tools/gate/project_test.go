package gate

import (
	"path/filepath"
	"testing"
)

func TestProjectSuitesTrackTheirInstallationInputs(t *testing.T) {
	root := newLibFixture(t)
	for name, body := range map[string]string{
		"ai/install-project.sh":           "#!/bin/sh\n. \"$repo/../lib/mount.sh\"\n",
		"ai/install-project-test.sh":      "#!/bin/sh\ntrue\n",
		"ai/project-skills.sh":            "#!/bin/sh\ntrue\n",
		"ai/project-skills-test.sh":       "#!/bin/sh\nbash \"$here/install-project.sh\"\n",
		"ai/project-dependencies.sh":      "#!/bin/sh\ntrue\n",
		"ai/project-dependencies-test.sh": "#!/bin/sh\ntrue\n",
		"ai/project-mcp.sh":               "#!/bin/sh\nnode \"$here/project-mcp.mjs\"\n",
		"ai/project-mcp-test.sh":          "#!/bin/sh\ntrue\n",
		"ai/project-mcp.mjs":              "console.log('project MCP');\n",
		"ai/mcp.jsonc":                    "{}\n",
		"ai/mcp-env.sh":                   "#!/bin/sh\ntrue\n",
	} {
		writeRepoFile(t, root, name, body)
	}
	units := []string{
		"shell:ai/install-project", "shell:ai/project-skills",
		"shell:ai/project-mcp", "shell:ai/project-dependencies",
	}
	keys := keysOverTree(t, root)
	for _, id := range units {
		if keys[id] == "" {
			t.Fatalf("discovery produced no %s unit", id)
		}
	}
	for _, scenario := range []struct {
		file                                 string
		installer, skills, mcp, dependencies bool
	}{
		{file: "ai/install-project.sh", installer: true, skills: true},
		{file: "ai/project-skills.sh", installer: true, skills: true},
		{file: "lib/mount.sh", installer: true, skills: true},
		{file: "ai/project-dependencies.sh", installer: true, skills: true, dependencies: true},
		{file: "ai/project-mcp.sh", installer: true, skills: true, mcp: true},
		{file: "ai/project-mcp.mjs", installer: true, skills: true, mcp: true},
		{file: "ai/mcp.jsonc", installer: true, skills: true, mcp: true},
		{file: "ai/mcp-env.sh", installer: true, skills: true, mcp: true},
		{file: "ai/bootstrap.sh"},
		{file: "lib/unsourced.sh"},
	} {
		editFixture(t, filepath.Join(root, scenario.file))
		moved := keysOverTree(t, root)
		for index, wantMove := range []bool{scenario.installer, scenario.skills, scenario.mcp, scenario.dependencies} {
			id := units[index]
			if gotMove := moved[id] != keys[id]; gotMove != wantMove {
				t.Errorf("editing %s changed %s cache key: %v, want %v", scenario.file, id, gotMove, wantMove)
			}
		}
		keys = moved
	}
}
