package aibootstrap_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"configs/ai/tools/flavor"
)

// The owner tier gets a copy of ai/owner-instructions.md rather than a region inside their own file,
// so the whole file is this repository's and a later edit of it is the owner's.
func TestTheOwnerTierInstallsAnIndependentCopy(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude", "--owner"), 0)

	f.expectNotSymlink(f.home + "/.claude/CLAUDE.md")
	f.expectFileBody(f.home+"/.claude/CLAUDE.md", ownerTemplate)
	// The receipt, which is what a later run compares against when the template has moved on: without
	// it an upgrade would read as a file the owner edited, and refuse forever.
	f.expectFileBody(f.home+"/.claude/CLAUDE.md.kk-flavor-installed", ownerTemplate)
}

// The owner's own writing is never replaced. This is the refusal the whole tier rests on: the copy is
// destructive in a way a region never is.
func TestTheOwnerTierRefusesToOverwriteAFileItDidNotWrite(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/.claude/CLAUDE.md", "My custom instructions\n")

	f.expectCode(f.install("--agent=claude", "--owner"), 1)

	f.expectSaid("contains personal instructions")
	f.expectFileBody(f.home+"/.claude/CLAUDE.md", "My custom instructions\n")
}

// An edit made after the copy landed is the owner's too. The receipt is what tells that apart from a
// template this repository has upgraded since.
func TestAnEditAfterTheCopyLandedIsRefusedOnTheNextRun(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=claude", "--owner"), 0)
	appendTo(t, f.home+"/.claude/CLAUDE.md", "\nLocal addition.\n")

	f.expectCode(f.install("--agent=claude", "--owner"), 1)

	f.expectFileContains(f.home+"/.claude/CLAUDE.md", "Local addition.")
}

// An upgraded template is not an edit, and the receipt is the only thing that can tell the two apart.
// Without it every machine already installed would refuse the moment the template changed.
func TestAnUpgradedTemplateReplacesTheCopyAndBacksTheOldOneUp(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=claude", "--owner"), 0)
	rewrite(t, f.repo+"/owner-instructions.md", ownerTemplate+"\nA new paragraph.\n")

	f.expectCode(f.install("--agent=claude", "--owner"), 0)

	f.expectFileContains(f.home+"/.claude/CLAUDE.md", "A new paragraph.")
	f.expectSaid("  backup   ")
	if backups := backupsOf(t, f.home+"/.claude", "CLAUDE.md.backup."); len(backups) != 1 {
		t.Errorf("the upgrade left %d backup(s) beside the copy, wanted 1: %v", len(backups), backups)
	}
}

// The first owner install mounted this repository's own file. That symlink is this repository's, so it
// becomes a copy rather than a refusal — and the copy replaces the link instead of being written
// through it, which would have edited the checkout.
func TestALegacyOwnerSymlinkBecomesAnIndependentCopy(t *testing.T) {
	f := newFixture(t)
	f.Write(f.repo+"/CLAUDE.md", "the checkout's own instructions\n")
	f.Symlink(f.repo+"/CLAUDE.md", f.home+"/.claude/CLAUDE.md")

	f.expectCode(f.install("--agent=claude", "--owner"), 0)

	f.expectNotSymlink(f.home + "/.claude/CLAUDE.md")
	f.expectFileBody(f.home+"/.claude/CLAUDE.md", ownerTemplate)
	f.expectFileBody(f.repo+"/CLAUDE.md", "the checkout's own instructions\n")
}

// A Codex machine set up before the owner tier existed holds the generated region, with or without the
// RTK note that sat under it. Both are this repository's writing, so both migrate.
func TestAGeneratedCodexInstructionFileMigratesToTheOwnerCopy(t *testing.T) {
	f := newFixture(t)
	f.Write(f.codexHome+"/AGENTS.md",
		flavor.RegionOpen+"\n"+flavor.RegionBody+"\n"+flavor.RegionClose+"\n")

	f.expectCode(f.install("--agent=codex", "--owner"), 0)

	f.expectFileBody(f.codexHome+"/AGENTS.md", ownerTemplate)
	if backups := backupsOf(t, f.codexHome, "AGENTS.md.backup."); len(backups) != 1 {
		t.Errorf("the migration left %d backup(s), wanted 1: %v", len(backups), backups)
	}
}

func TestAGeneratedCodexFileCarryingTheOldRtkNoteMigratesToo(t *testing.T) {
	f := newFixture(t)
	f.Write(f.codexHome+"/AGENTS.md",
		flavor.RegionOpen+"\n"+flavor.RegionBody+"\n"+flavor.RegionClose+"\n\n"+
			"@"+f.codexHome+"/RTK.md\n\n"+
			flavor.LegacyRtkRegionOpen+"\n"+
			"Read `"+f.codexHome+"/RTK.md` for RTK usage. For unsupported commands or exact output, "+
			"use `rtk proxy <command>`.\n"+
			flavor.LegacyRtkRegionClose+"\n")

	f.expectCode(f.install("--agent=codex", "--owner"), 0)

	f.expectFileBody(f.codexHome+"/AGENTS.md", ownerTemplate)
}

// A dry run over the migration must write nothing: someone checks with --dry-run and it is the run
// that replaced their instructions.
func TestADryRunOverTheOwnerMigrationLeavesTheFileAlone(t *testing.T) {
	f := newFixture(t)
	generated := flavor.RegionOpen + "\n" + flavor.RegionBody + "\n" + flavor.RegionClose + "\n"
	f.Write(f.codexHome+"/AGENTS.md", generated)

	f.expectCode(f.install("--agent=codex", "--owner", "--dry-run"), 0)

	f.expectFileBody(f.codexHome+"/AGENTS.md", generated)
	f.expectSaid("would install")
}

// The owner's memory store is created empty and never written over: it is the one file in this install
// whose whole content is the owner's.
func TestTheOwnerMemoryStoreIsCreatedOnceAndThenLeftAlone(t *testing.T) {
	f := newFixture(t)
	memory := f.home + "/Documents/AI/MEMORY.md"

	f.expectCode(f.install("--agent=claude", "--owner"), 0)
	f.expectFileBody(memory, "# Memory\n")
	rewrite(t, memory, "# Memory\n\nKeep this entry.\n")

	f.expectCode(f.install("--agent=claude", "--owner"), 0)

	f.expectFileBody(memory, "# Memory\n\nKeep this entry.\n")
}

func TestAnOrdinaryInstallCreatesNoOwnerMemory(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude"), 0)

	f.expectAbsent(f.home + "/Documents")
}

// Both clients get the same file, so an owner running one after the other is not reading two different
// sets of instructions depending on which client they opened.
func TestBothClientsGetTheSameOwnerCopy(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude", "--owner"), 0)
	f.expectCode(f.install("--agent=codex", "--owner"), 0)

	f.expectFileBody(f.home+"/.claude/CLAUDE.md", ownerTemplate)
	f.expectFileBody(f.codexHome+"/AGENTS.md", ownerTemplate)
}

// Uninstall takes back what this put there, and only that. By the time the owner has edited the file
// it is no longer that, so it is preserved and the run says why.
func TestTheOwnerUninstallRemovesItsOwnCopyAndRefusesAnEditedOne(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=claude", "--owner"), 0)

	f.expectCode(f.install("--agent=claude", "--owner", "--uninstall"), 0)

	f.expectAbsent(f.home + "/.claude/CLAUDE.md")
	f.expectAbsent(f.home + "/.claude/CLAUDE.md.kk-flavor-installed")
	// The memory store stays: it is the owner's own writing, and nothing here records whether they
	// still want it.
	f.expectFileBody(f.home+"/Documents/AI/MEMORY.md", "# Memory\n")
}

func TestTheOwnerUninstallPreservesAModifiedFile(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=claude", "--owner"), 0)
	appendTo(t, f.home+"/.claude/CLAUDE.md", "\nLocal addition.\n")

	f.expectCode(f.install("--agent=claude", "--owner", "--uninstall"), 1)

	f.expectSaid("was modified — owner instructions were preserved")
	f.expectFileContains(f.home+"/.claude/CLAUDE.md", "Local addition.")
}

// --- fixture helpers a case needs and the harness does not ------------------------------------------

func appendTo(t *testing.T, path, text string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the fixture could not read %s: %v", path, err)
	}
	rewrite(t, path, string(body)+text)
}

func rewrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("the fixture could not write %s: %v", path, err)
	}
}

func backupsOf(t *testing.T, directory, prefix string) []string {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil
	}
	var found []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			found = append(found, filepath.Join(directory, entry.Name()))
		}
	}
	return found
}

// The spelling was `Document` for months, and correcting the destination alone would leave every
// entry already written at a path no session reads. Silent, too: the run would report a memory file
// created, and the file it created would be empty.
func TestOwnerMemoryWrittenAtTheOldPathIsMoved(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/Document/AI/MEMORY.md", "# Memory\n\nAn entry written before the move.\n")

	f.expectCode(f.install("--agent=claude", "--owner"), 0)

	f.expectFileBody(f.home+"/Documents/AI/MEMORY.md", "# Memory\n\nAn entry written before the move.\n")
	f.expectAbsent(f.home + "/Document/AI/MEMORY.md")
	// The directories it left behind go too, but only because nothing else is in them.
	f.expectAbsent(f.home + "/Document")
	f.expectSaid("moved")
}

// A directory the human put something else in is theirs, whatever this run emptied beside it.
func TestTheOldMemoryDirectoryIsKeptWhenItHoldsAnythingElse(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/Document/AI/MEMORY.md", "# Memory\n")
	f.Write(f.home+"/Document/AI/notes.md", "mine\n")

	f.expectCode(f.install("--agent=claude", "--owner"), 0)

	f.expectFileBody(f.home+"/Document/AI/notes.md", "mine\n")
}

// Two stores and no way to tell which holds what. Merging them is the human's call: this cannot read
// either, and cannot know which entry is the newer one.
func TestTwoOwnerMemoryStoresRefuseRatherThanPickOne(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/Document/AI/MEMORY.md", "# Memory\n\nThe old one.\n")
	f.Write(f.home+"/Documents/AI/MEMORY.md", "# Memory\n\nThe new one.\n")

	f.expectCode(f.install("--agent=claude", "--owner"), 1)

	f.expectFileBody(f.home+"/Document/AI/MEMORY.md", "# Memory\n\nThe old one.\n")
	f.expectFileBody(f.home+"/Documents/AI/MEMORY.md", "# Memory\n\nThe new one.\n")
	f.expectSaid("exists at both")
}

// A dry run moves nothing and says so once. Saying it would move the file AND that it would create one
// at the destination is two lines that cannot both hold, about the one file this exists to protect.
func TestADryRunSaysItWouldMoveAndCreatesNothing(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/Document/AI/MEMORY.md", "# Memory\n\nStill here afterwards.\n")

	f.install("--agent=claude", "--owner", "--dry-run")

	f.expectFileBody(f.home+"/Document/AI/MEMORY.md", "# Memory\n\nStill here afterwards.\n")
	f.expectAbsent(f.home + "/Documents/AI/MEMORY.md")
	f.expectSaid("would move")
	f.expectNotSaid("would create " + f.home + "/Documents/AI/MEMORY.md")
}
