package aibootstrap_test

import (
	"os"
	"testing"

	"configs/ai/tools/machine"
)

// rtk writes into the client's own configuration, so it is handed the client's native arguments. The
// whole list is asserted, because a flag dropped from the middle of one is the defect this has had.
// Claude patches a global hook and Codex initialises a profile, which are different modes.
func TestClaudeRtkIsInitialisedWithItsHookOnlyArguments(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.run("--agent=claude", "--owner", "--skip-brew", "--skip-tools", "--skip-mcp", "--skip-verify", "--skip-models"), 0)

	if !f.machine.Ran("rtk", "init", "--agent", "claude", "--global", "--hook-only", "--auto-patch") {
		t.Errorf("rtk was not initialised the way Claude needs: %v", f.machine.Spelled("rtk"))
	}
}

// Codex's rtk init writes a whole profile, so it is pointed at a staging directory and only RTK.md is
// copied out. A run against the real profile would also rewrite AGENTS.md, which the owner tier has
// just made its own copy of.
func TestCodexRtkIsInitialisedInAStagingProfileAndOnlyItsDocumentIsKept(t *testing.T) {
	f := newFixture(t)
	f.machine.Answering("rtk", func(command machine.Command) int {
		staging := stagingProfile(t, command)
		writeInto(t, staging+"/RTK.md", "RTK usage\n")
		writeInto(t, staging+"/AGENTS.md", "rtk would have rewritten this\n")
		return 0
	})

	f.ExpectCode(f.run("--agent=codex", "--owner", "--skip-brew", "--skip-tools", "--skip-mcp", "--skip-verify", "--skip-models"), 0)

	if !f.machine.Ran("rtk", "init", "--codex", "--global") {
		t.Errorf("rtk was not initialised the way Codex needs: %v", f.machine.Spelled("rtk"))
	}
	f.ExpectFileBody(f.codexHome+"/RTK.md", "RTK usage\n")
	f.ExpectFileBody(f.codexHome+"/AGENTS.md", ownerTemplate)
}

// A profile that already holds an RTK.md keeps it: that document is the human's own, and rtk would
// write over it.
func TestAnExistingCodexRtkDocumentIsKeptAndTheCliIsNotRun(t *testing.T) {
	f := newFixture(t)
	f.Write(f.codexHome+"/RTK.md", "Personal RTK instructions\n")

	f.ExpectCode(f.run("--agent=codex", "--owner", "--skip-brew", "--skip-tools", "--skip-mcp", "--skip-verify", "--skip-models"), 0)

	f.ExpectFileBody(f.codexHome+"/RTK.md", "Personal RTK instructions\n")
	if f.machine.RanAny("rtk") {
		t.Errorf("rtk ran over a profile that already had its document: %v", f.machine.Spelled("rtk"))
	}
}

// A symlink at that path would have rtk writing into a file the profile never named. The refusal
// comes before the CLI is reached, so no byte goes through the link.
func TestASymlinkedCodexRtkDocumentIsRefusedBeforeTheCliRuns(t *testing.T) {
	f := newFixture(t)
	f.Symlink(f.home+"/elsewhere", f.codexHome+"/RTK.md")

	f.ExpectCode(f.run("--agent=codex", "--owner", "--skip-brew", "--skip-tools", "--skip-mcp", "--skip-verify", "--skip-models"), 1)

	f.ExpectAbsent(f.home + "/elsewhere")
	if f.machine.RanAny("rtk") {
		t.Errorf("rtk ran against a symlinked document: %v", f.machine.Spelled("rtk"))
	}
}

func TestAFailedRtkInitFailsTheRun(t *testing.T) {
	f := newFixture(t)
	f.machine.Answering("rtk", func(machine.Command) int { return 1 })

	f.ExpectCode(f.run("--agent=claude", "--owner", "--skip-brew", "--skip-tools", "--skip-mcp", "--skip-verify", "--skip-models"), 1)

	f.ExpectSaid("rtk init failed")
}

func TestAMachineWithoutRtkSaysSoRatherThanFailingSilently(t *testing.T) {
	f := newFixture(t)
	f.machine.Without("rtk")

	f.ExpectCode(f.run("--agent=claude", "--owner", "--skip-brew", "--skip-tools", "--skip-mcp", "--skip-verify", "--skip-models"), 1)

	f.ExpectSaid("rtk is not on PATH — install it or use --skip-rtk")
}

func TestADryRunNamesTheRtkInvocationAndRunsNothing(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.run("--agent=claude", "--owner", "--dry-run", "--skip-brew", "--skip-tools", "--skip-mcp", "--skip-verify", "--skip-models"), 0)

	f.ExpectSaid("would run rtk init --agent claude --global --hook-only --auto-patch")
	if f.machine.RanAny("rtk") {
		t.Errorf("a dry run invoked rtk: %v", f.machine.Spelled("rtk"))
	}
}

func TestSkipRtkSaysSoAndRunsNothing(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.install("--agent=claude", "--owner"), 0)

	f.ExpectSaid("skipped  RTK initialization")
	if f.machine.RanAny("rtk") {
		t.Errorf("--skip-rtk invoked rtk: %v", f.machine.Spelled("rtk"))
	}
}

// rtk is personal tooling, and the instruction tree works without it. A colleague's machine keeps
// what it has, since this repository wrote no leftover there. rtk is never initialised on it either.
func TestADefaultTierLeavesRtkAloneAndSaysWhose(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/.claude/RTK.md", "not ours to remove\n")

	f.ExpectCode(f.run("--agent=claude", "--skip-brew", "--skip-tools", "--skip-mcp", "--skip-verify", "--skip-models"), 0)

	f.ExpectSaid("rtk is the owner tier's")
	f.ExpectFileBody(f.home+"/.claude/RTK.md", "not ours to remove\n")
	if f.machine.RanAny("rtk") {
		t.Errorf("a default tier invoked rtk: %v", f.machine.Spelled("rtk"))
	}
}

// ~/.claude/RTK.md is a copy an earlier bootstrap left behind, and what the next `rtk init -g` puts
// back. Every machine already set up from this repository is holding one.
func TestTheLeftoverClaudeRtkDocumentIsRemoved(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/.claude/RTK.md", "the copy an earlier bootstrap left here\n")

	f.ExpectCode(f.install("--agent=claude", "--owner"), 0)

	f.ExpectSaid("removed  " + f.home + "/.claude/RTK.md")
	f.ExpectAbsent(f.home + "/.claude/RTK.md")
}

// A symlink there is what a machine set up from an older README by hand holds. The link goes, and
// what it points at stays, because a removal that followed the link would take a file this never
// wrote.
func TestASymlinkAtTheLeftoverPathGoesWithoutFollowingIt(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/.claude/pointed-at.md", "the file the link named\n")
	f.Symlink(f.home+"/.claude/pointed-at.md", f.home+"/.claude/RTK.md")

	f.ExpectCode(f.install("--agent=claude", "--owner"), 0)

	f.ExpectAbsent(f.home + "/.claude/RTK.md")
	f.ExpectFileBody(f.home+"/.claude/pointed-at.md", "the file the link named\n")
}

// This only ever wrote a file there, so a directory holds something else, and removing it is the data
// loss this whole installer exists to avoid.
func TestADirectoryAtTheLeftoverPathIsRefusedAndItsContentsSurvive(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/.claude/RTK.md/notes.md", "somebody else put this here\n")

	f.ExpectCode(f.install("--agent=claude", "--owner"), 1)

	f.ExpectSaid("is a directory, and this script only ever wrote a file")
	f.ExpectFileBody(f.home+"/.claude/RTK.md/notes.md", "somebody else put this here\n")
}

func TestADryRunOverTheLeftoverLeavesItAlone(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/.claude/RTK.md", "still here afterwards\n")

	f.ExpectCode(f.install("--agent=claude", "--owner", "--dry-run"), 0)

	f.ExpectSaid("would remove the leftover " + f.home + "/.claude/RTK.md")
	f.ExpectFileBody(f.home+"/.claude/RTK.md", "still here afterwards\n")
}

// Codex keeps its own RTK.md in the profile, so Claude's cleanup does not apply. A silent step reads
// exactly like one that was never reached.
func TestCodexSaysTheClaudeCleanupDoesNotApply(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/.claude/RTK.md", "Claude's own\n")

	f.ExpectCode(f.install("--agent=codex", "--owner"), 0)

	f.ExpectSaid("Claude RTK cleanup does not apply to Codex")
	f.ExpectFileBody(f.home+"/.claude/RTK.md", "Claude's own\n")
}

// --- fixture helpers for the rtk cases ---------------------------------------------------------------

// Where rtk was pointed. The value comes out of the environment the run handed it, because the
// staging directory being a directory of this run's own is the property the case is about.
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
