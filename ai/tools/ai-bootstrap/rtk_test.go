package aibootstrap_test

import (
	"os"
	"testing"

	"kk-flavor/tools/machine"
)

// rtk writes into the client's own configuration, so it is handed the client's native arguments. The
// whole list is asserted, because a flag dropped from the middle of one is the defect this has had:
// Claude's hook-only patching and Codex's profile init are different modes, not different spellings.
func TestClaudeRtkIsInitialisedWithItsHookOnlyArguments(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--agent=claude", "--owner", "--skip-brew", "--skip-tools", "--skip-mcp", "--skip-verify"), 0)

	if !f.machine.Ran("rtk", "init", "--agent", "claude", "--global", "--hook-only", "--auto-patch") {
		t.Errorf("rtk was not initialised the way Claude needs: %v", f.machine.Spelled("rtk"))
	}
}

// Codex's rtk init writes a whole profile, so it is pointed at a staging directory and only RTK.md is
// copied out. Run against the real profile it would also rewrite AGENTS.md — the file the owner tier
// has just made its own copy of.
func TestCodexRtkIsInitialisedInAStagingProfileAndOnlyItsDocumentIsKept(t *testing.T) {
	f := newFixture(t)
	f.machine.Answering("rtk", func(command machine.Command) int {
		staging := stagingProfile(t, command)
		writeInto(t, staging+"/RTK.md", "RTK usage\n")
		writeInto(t, staging+"/AGENTS.md", "rtk would have rewritten this\n")
		return 0
	})

	f.expectCode(f.run("--agent=codex", "--owner", "--skip-brew", "--skip-tools", "--skip-mcp", "--skip-verify"), 0)

	if !f.machine.Ran("rtk", "init", "--codex", "--global") {
		t.Errorf("rtk was not initialised the way Codex needs: %v", f.machine.Spelled("rtk"))
	}
	f.expectFileBody(f.codexHome+"/RTK.md", "RTK usage\n")
	f.expectFileBody(f.codexHome+"/AGENTS.md", ownerTemplate)
}

// A profile that already holds an RTK.md keeps it: that document is the human's own, and rtk would
// write over it.
func TestAnExistingCodexRtkDocumentIsKeptAndTheCliIsNotRun(t *testing.T) {
	f := newFixture(t)
	f.Write(f.codexHome+"/RTK.md", "Personal RTK instructions\n")

	f.expectCode(f.run("--agent=codex", "--owner", "--skip-brew", "--skip-tools", "--skip-mcp", "--skip-verify"), 0)

	f.expectFileBody(f.codexHome+"/RTK.md", "Personal RTK instructions\n")
	if f.machine.RanAny("rtk") {
		t.Errorf("rtk ran over a profile that already had its document: %v", f.machine.Spelled("rtk"))
	}
}

// A symlink at that path would have rtk writing into a file the profile never named. Refused before
// the CLI is reached, so nothing is written through it.
func TestASymlinkedCodexRtkDocumentIsRefusedBeforeTheCliRuns(t *testing.T) {
	f := newFixture(t)
	f.Symlink(f.home+"/elsewhere", f.codexHome+"/RTK.md")

	f.expectCode(f.run("--agent=codex", "--owner", "--skip-brew", "--skip-tools", "--skip-mcp", "--skip-verify"), 1)

	f.expectAbsent(f.home + "/elsewhere")
	if f.machine.RanAny("rtk") {
		t.Errorf("rtk ran against a symlinked document: %v", f.machine.Spelled("rtk"))
	}
}

func TestAFailedRtkInitFailsTheRun(t *testing.T) {
	f := newFixture(t)
	f.machine.Answering("rtk", func(machine.Command) int { return 1 })

	f.expectCode(f.run("--agent=claude", "--owner", "--skip-brew", "--skip-tools", "--skip-mcp", "--skip-verify"), 1)

	f.expectSaid("rtk init failed")
}

func TestAMachineWithoutRtkSaysSoRatherThanFailingSilently(t *testing.T) {
	f := newFixture(t)
	f.machine.without("rtk")

	f.expectCode(f.run("--agent=claude", "--owner", "--skip-brew", "--skip-tools", "--skip-mcp", "--skip-verify"), 1)

	f.expectSaid("rtk is not on PATH — install it or use --skip-rtk")
}

func TestADryRunNamesTheRtkInvocationAndRunsNothing(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--agent=claude", "--owner", "--dry-run", "--skip-brew", "--skip-tools", "--skip-mcp", "--skip-verify"), 0)

	f.expectSaid("would run rtk init --agent claude --global --hook-only --auto-patch")
	if f.machine.RanAny("rtk") {
		t.Errorf("a dry run invoked rtk: %v", f.machine.Spelled("rtk"))
	}
}

func TestSkipRtkSaysSoAndRunsNothing(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude", "--owner"), 0)

	f.expectSaid("skipped  RTK initialization")
	if f.machine.RanAny("rtk") {
		t.Errorf("--skip-rtk invoked rtk: %v", f.machine.Spelled("rtk"))
	}
}

// rtk is personal tooling rather than something the instruction tree needs, so a colleague's machine
// is never told about a leftover this repository never wrote there, and never has rtk initialised.
func TestADefaultTierLeavesRtkAloneAndSaysWhose(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/.claude/RTK.md", "not ours to remove\n")

	f.expectCode(f.run("--agent=claude", "--skip-brew", "--skip-tools", "--skip-mcp", "--skip-verify"), 0)

	f.expectSaid("rtk is the owner tier's")
	f.expectFileBody(f.home+"/.claude/RTK.md", "not ours to remove\n")
	if f.machine.RanAny("rtk") {
		t.Errorf("a default tier invoked rtk: %v", f.machine.Spelled("rtk"))
	}
}

// ~/.claude/RTK.md is a copy an earlier bootstrap left behind, and what the next `rtk init -g` puts
// back. Every machine already set up from this repository is holding one.
func TestTheLeftoverClaudeRtkDocumentIsRemoved(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/.claude/RTK.md", "the copy an earlier bootstrap left here\n")

	f.expectCode(f.install("--agent=claude", "--owner"), 0)

	f.expectSaid("removed  " + f.home + "/.claude/RTK.md")
	f.expectAbsent(f.home + "/.claude/RTK.md")
}

// A symlink there is what a machine set up from an older README by hand holds. The link goes; what it
// points at must not, because a removal that followed it would take a file this never wrote.
func TestASymlinkAtTheLeftoverPathGoesWithoutFollowingIt(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/.claude/pointed-at.md", "the file the link named\n")
	f.Symlink(f.home+"/.claude/pointed-at.md", f.home+"/.claude/RTK.md")

	f.expectCode(f.install("--agent=claude", "--owner"), 0)

	f.expectAbsent(f.home + "/.claude/RTK.md")
	f.expectFileBody(f.home+"/.claude/pointed-at.md", "the file the link named\n")
}

// A directory there is not a shape this ever wrote, so it holds something else and removing it would
// be the data loss this whole installer promises not to be.
func TestADirectoryAtTheLeftoverPathIsRefusedAndItsContentsSurvive(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/.claude/RTK.md/notes.md", "somebody else put this here\n")

	f.expectCode(f.install("--agent=claude", "--owner"), 1)

	f.expectSaid("is a directory, and this script only ever wrote a file")
	f.expectFileBody(f.home+"/.claude/RTK.md/notes.md", "somebody else put this here\n")
}

func TestADryRunOverTheLeftoverLeavesItAlone(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/.claude/RTK.md", "still here afterwards\n")

	f.expectCode(f.install("--agent=claude", "--owner", "--dry-run"), 0)

	f.expectSaid("would remove the leftover " + f.home + "/.claude/RTK.md")
	f.expectFileBody(f.home+"/.claude/RTK.md", "still here afterwards\n")
}

// Codex keeps its own RTK.md in the profile, so Claude's cleanup does not apply — and a step that
// printed nothing reads exactly like one that was never reached.
func TestCodexSaysTheClaudeCleanupDoesNotApply(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/.claude/RTK.md", "Claude's own\n")

	f.expectCode(f.install("--agent=codex", "--owner"), 0)

	f.expectSaid("Claude RTK cleanup does not apply to Codex")
	f.expectFileBody(f.home+"/.claude/RTK.md", "Claude's own\n")
}

// --- fixture helpers for the rtk cases ---------------------------------------------------------------

// Where rtk was pointed, read out of the environment the run handed it. Read rather than assumed,
// because the staging directory being a directory of this run's own is the property the case is about.
func stagingProfile(t *testing.T, command machine.Command) string {
	t.Helper()
	for _, entry := range command.Env {
		if len(entry) > len("CODEX_HOME=") && entry[:len("CODEX_HOME=")] == "CODEX_HOME=" {
			return entry[len("CODEX_HOME="):]
		}
	}
	t.Fatalf("rtk was run with no CODEX_HOME, so it would have written into the real profile: %v", command)
	return ""
}

func writeInto(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("the fake rtk could not write %s: %v", path, err)
	}
}
