package aibootstrap_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The install puts a whole copy of ai/owner-instructions.md in place, so the file is this
// repository's and a later edit of it is the owner's.
func TestTheInstallPutsAnIndependentOwnerCopyInPlace(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.install("--agent=claude"), 0)

	f.ExpectNotSymlink(f.home + "/.claude/CLAUDE.md")
	f.ExpectFileBody(f.home+"/.claude/CLAUDE.md", ownerTemplate)
	// The receipt, which is what a later run compares against when the template has moved on. A run
	// lacking one reads an upgrade as a file the owner edited, and refuses forever.
	f.ExpectFileBody(f.home+"/.claude/CLAUDE.md.kk-flavor-installed", ownerTemplate)
}

// The owner's own writing is never replaced. This is the refusal the copy rests on, because a
// whole-file replacement is destructive.
func TestTheInstallRefusesToOverwriteAFileItDidNotWrite(t *testing.T) {
	f := newFixture(t)
	f.Write(f.home+"/.claude/CLAUDE.md", "My custom instructions\n")

	f.ExpectCode(f.install("--agent=claude"), 1)

	f.ExpectSaid("contains personal instructions")
	f.ExpectFileBody(f.home+"/.claude/CLAUDE.md", "My custom instructions\n")
}

// An edit made after the copy landed is the owner's too. The receipt is what tells that apart from a
// template this repository has upgraded since.
func TestAnEditAfterTheCopyLandedIsRefusedOnTheNextRun(t *testing.T) {
	f := newFixture(t)
	f.ExpectCode(f.install("--agent=claude"), 0)
	f.appendTo(f.home+"/.claude/CLAUDE.md", "\nLocal addition.\n")

	f.ExpectCode(f.install("--agent=claude"), 1)

	f.ExpectFileContains(f.home+"/.claude/CLAUDE.md", "Local addition.")
}

// An upgraded template is not an edit, and the receipt is the only thing that can tell the two apart.
// A machine lacking one would refuse the moment the template changed.
func TestAnUpgradedTemplateReplacesTheCopyAndBacksTheOldOneUp(t *testing.T) {
	f := newFixture(t)
	f.ExpectCode(f.install("--agent=claude"), 0)
	f.Write(f.repo+"/owner-instructions.md", ownerTemplate+"\nA new paragraph.\n")

	f.ExpectCode(f.install("--agent=claude"), 0)

	f.ExpectFileContains(f.home+"/.claude/CLAUDE.md", "A new paragraph.")
	f.ExpectSaid("  backup   ")
	if backups := backupsOf(t, f.home+"/.claude", "CLAUDE.md.backup."); len(backups) != 1 {
		t.Errorf("the upgrade left %d backup(s) beside the copy, wanted 1: %v", len(backups), backups)
	}
}

// The first owner install mounted this repository's own file. That symlink is this repository's, so
// the run turns it into a copy. The copy replaces the link, because a write through the link would
// have edited the checkout.
func TestALegacyOwnerSymlinkBecomesAnIndependentCopy(t *testing.T) {
	f := newFixture(t)
	f.Write(f.repo+"/CLAUDE.md", "the checkout's own instructions\n")
	f.Symlink(f.repo+"/CLAUDE.md", f.home+"/.claude/CLAUDE.md")

	f.ExpectCode(f.install("--agent=claude"), 0)

	f.ExpectNotSymlink(f.home + "/.claude/CLAUDE.md")
	f.ExpectFileBody(f.home+"/.claude/CLAUDE.md", ownerTemplate)
	f.ExpectFileBody(f.repo+"/CLAUDE.md", "the checkout's own instructions\n")
}

// A dry run over the migration that wrote anything would be worse than having no flag at all. Someone
// checks with it, and the check is the run that replaced their instructions.
func TestADryRunOverTheOwnerMigrationLeavesTheFileAlone(t *testing.T) {
	f := newFixture(t)
	f.Write(f.repo+"/CLAUDE.md", "the checkout's own instructions\n")
	f.Symlink(f.repo+"/CLAUDE.md", f.home+"/.claude/CLAUDE.md")

	f.ExpectCode(f.install("--agent=claude", "--dry-run"), 0)

	f.ExpectSymlink(f.home + "/.claude/CLAUDE.md")
	f.ExpectSaid("would install")
}

// The owner's memory store is created empty and never written over. Its whole content is the owner's,
// and this is the only file in the install of which that is true.
func TestTheOwnerMemoryStoreIsCreatedOnceAndThenLeftAlone(t *testing.T) {
	f := newFixture(t)
	memory := f.home + "/Documents/AI/MEMORY.md"

	f.ExpectCode(f.install("--agent=claude"), 0)
	f.ExpectFileBody(memory, "# Memory\n")
	f.Write(memory, "# Memory\n\nKeep this entry.\n")

	f.ExpectCode(f.install("--agent=claude"), 0)

	f.ExpectFileBody(memory, "# Memory\n\nKeep this entry.\n")
}

// Both clients get the same file. An owner running one after the other reads one set of instructions,
// whichever client they opened.
func TestBothClientsGetTheSameOwnerCopy(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.install("--agent=claude"), 0)
	f.ExpectCode(f.install("--agent=codex"), 0)

	f.ExpectFileBody(f.home+"/.claude/CLAUDE.md", ownerTemplate)
	f.ExpectFileBody(f.codexHome+"/AGENTS.md", ownerTemplate)
}

// Uninstall takes back what this put there, and only that. By the time the owner has edited the file
// it is no longer that, so it is preserved and the run says why.
func TestTheOwnerUninstallRemovesItsOwnCopyAndRefusesAnEditedOne(t *testing.T) {
	f := newFixture(t)
	f.ExpectCode(f.install("--agent=claude"), 0)

	f.ExpectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.ExpectAbsent(f.home + "/.claude/CLAUDE.md")
	f.ExpectAbsent(f.home + "/.claude/CLAUDE.md.kk-flavor-installed")
	// The memory store stays: it is the owner's own writing, and no record here says whether they
	// still want it.
	f.ExpectFileBody(f.home+"/Documents/AI/MEMORY.md", "# Memory\n")
}

func TestTheOwnerUninstallPreservesAModifiedFile(t *testing.T) {
	f := newFixture(t)
	f.ExpectCode(f.install("--agent=claude"), 0)
	f.appendTo(f.home+"/.claude/CLAUDE.md", "\nLocal addition.\n")

	f.ExpectCode(f.install("--agent=claude", "--uninstall"), 1)

	f.ExpectSaid("was modified — owner instructions were preserved")
	f.ExpectFileContains(f.home+"/.claude/CLAUDE.md", "Local addition.")
}

// --- fixture helpers a case needs and the harness does not ------------------------------------------

func (f *fixture) appendTo(path, text string) {
	f.t.Helper()
	f.Write(path, f.Read(path)+text)
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
