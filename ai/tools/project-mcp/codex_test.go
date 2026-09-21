package projectmcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Codex's file is TOML, which this tool does not parse. It owns one fenced region and reads the rest
// only well enough to know it is not being asked to redefine something.

// Every case in this file is about what the tool does with a file it cannot fully understand. The
// answer is always to say so. A wrong guess silently detaches a server the human still has in their
// config.

func TestAnEditedManagedRegionIsKeptRatherThanRemoved(t *testing.T) {
	t.Parallel()
	p := newProject(t, codexAgent)
	if outcome := p.run(); outcome.code != exitDone {
		t.Fatalf("the install exited %d\n%s", outcome.code, outcome.stderr)
	}
	edited := strings.Replace(p.read(), `command = "sh"`, `command = "mine"`, 1)
	p.writeConfigFile(edited)

	if outcome := p.run("--uninstall"); outcome.code == exitDone {
		t.Fatal("a region the human edited was removed — this tool can only take back what it wrote, and " +
			"an edited region is no longer that")
	}
	if after := p.read(); after != edited {
		t.Errorf("the refusal changed the file anyway\n  before: %q\n   after: %q", edited, after)
	}
}

// A managed region with the project's own tables after it stays where it is. A file reassembled
// around the region moves it to the end: the same servers, a different file, and a diff in the
// project's history on every reinstall.
func TestAReinstallLeavesARegionThatIsNotAtTheEndOfTheFileWhereItIs(t *testing.T) {
	t.Parallel()
	p := newProject(t, codexAgent)
	if outcome := p.run(); outcome.code != exitDone {
		t.Fatalf("the install exited %d\n%s", outcome.code, outcome.stderr)
	}
	p.writeConfigFile(p.read() + "[other_tool]\nsetting = 1\n")
	before := p.read()

	if outcome := p.run(); outcome.code != exitDone {
		t.Fatalf("the reinstall exited %d\n%s", outcome.code, outcome.stderr)
	}
	if after := p.read(); after != before {
		t.Errorf("the reinstall moved the region past the project's own tables\n  before: %q\n   after: %q",
			before, after)
	}
}

// A setting under the closing fence belongs to the table that precedes it, a managed server. The
// region's removal silently moves that setting onto whatever table comes next.
func TestUninstallRefusesATrailingFieldThatBelongsToAManagedServer(t *testing.T) {
	t.Parallel()
	p := newProject(t, codexAgent)
	if outcome := p.run(); outcome.code != exitDone {
		t.Fatalf("the install exited %d\n%s", outcome.code, outcome.stderr)
	}
	p.writeConfigFile(p.read() + "startup_timeout_sec = 60\n")
	before := p.read()

	if outcome := p.run("--uninstall"); outcome.code == exitDone {
		t.Fatal("a setting belonging to a managed server was re-homed on whatever table followed it")
	}
	if after := p.read(); after != before {
		t.Errorf("the refusal changed the file anyway\n  before: %q\n   after: %q", before, after)
	}
}

// Codex's config sits one directory down, so this tool may have to create `.codex/`. A link there
// points that write at another project's configuration, or at anything else the link names.
func TestADirectoryLinkWhereCodexsConfigBelongsIsRefused(t *testing.T) {
	t.Parallel()
	p := newProject(t, codexAgent)
	outside := filepath.Join(p.root, "outside-directory")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("building the fixture: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(p.dir, ".codex")); err != nil {
		t.Fatalf("linking .codex: %v", err)
	}

	if outcome := p.run(); outcome.code == exitDone {
		t.Fatal("the link was followed, so the write landed outside the project")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatalf("reading what the link named: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("the directory the link named was written into: %d entries", len(entries))
	}
}

// What the merge does with a file it cannot read confidently. Every arm refuses, and each refusal has
// to name its own reason: they are the lines a human reads before deciding to merge by hand.
func TestAFileThisToolCannotReadConfidentlyIsLeftForAnExplicitMerge(t *testing.T) {
	t.Parallel()
	servers := []projectServer{{Name: "playwright", Args: []string{"-c", launcher, launcherName, "x"}}}
	region, err := codexRegion(servers)
	if err != nil {
		t.Fatalf("building the region: %v", err)
	}
	for _, scenario := range []struct {
		name string
		text string
		want string
	}{
		{name: "a multiline TOML string, which this tool cannot scan", want: "multiline",
			text: "note = '''\n[mcp_servers.playwright]\n'''\n"},
		{name: "an opening fence with no closing one", want: "incomplete or repeated",
			text: regionOpen + "\n"},
		{name: "the region written twice", want: "incomplete or repeated",
			text: region + region},
		{name: "a fence sharing its line with something else", want: "whole lines",
			text: "x = 1 " + regionOpen + "\n" + regionClose + "\n"},
		{name: "an escaped table name this tool cannot recognise", want: "escaped TOML",
			text: "[mcp_servers.\\u0070laywright]\ncommand = \"mine\"\n"},
		{name: "a managed server defined outside the region", want: "conflicts or uses ambiguous syntax",
			text: "[mcp_servers.playwright]\ncommand = \"mine\"\n"},
		{name: "a table spelling this tool cannot recognise", want: "conflicts or uses ambiguous syntax",
			text: "[\"mcp_servers\".other]\ncommand = \"mine\"\n"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			_, err := mergeCodex(scenario.text, servers, false)
			if err == nil {
				t.Fatalf("%s was merged into rather than refused, so a server the human still has in their "+
					"config is silently detached", scenario.name)
			}
			if !strings.Contains(err.Error(), scenario.want) {
				t.Errorf("the refusal reads %q, which does not name %q — a human deciding whether to merge "+
					"by hand has only this line", err, scenario.want)
			}
		})
	}
}

// The other side of TestAFileThisToolCannotReadConfidentlyIsLeftForAnExplicitMerge. A server this
// tool does not manage is none of its business, and the install goes ahead around it.
func TestAnUnmanagedServerOutsideTheRegionIsLeftAlone(t *testing.T) {
	t.Parallel()
	p := newProject(t, codexAgent)
	p.writeConfigFile("[mcp_servers.other]\ncommand = \"keep\"\n")

	if outcome := p.run(); outcome.code != exitDone {
		t.Fatalf("exit %d, want 0 — an unrelated server is not a conflict\n%s", outcome.code, outcome.stderr)
	}
	text := p.read()
	if !strings.Contains(text, `command = "keep"`) {
		t.Errorf("the project's own server was dropped\n%s", text)
	}
	if !strings.Contains(text, regionOpen) {
		t.Errorf("the managed region was not added\n%s", text)
	}
}
