package aibootstrap_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kk-flavor/tools/flavor"
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
	f.write(f.home+"/.claude/CLAUDE.md", "My custom instructions\n")

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
	f.write(f.repo+"/CLAUDE.md", "the checkout's own instructions\n")
	f.symlink(f.repo+"/CLAUDE.md", f.home+"/.claude/CLAUDE.md")

	f.expectCode(f.install("--agent=claude", "--owner"), 0)

	f.expectNotSymlink(f.home + "/.claude/CLAUDE.md")
	f.expectFileBody(f.home+"/.claude/CLAUDE.md", ownerTemplate)
	f.expectFileBody(f.repo+"/CLAUDE.md", "the checkout's own instructions\n")
}

// A Codex machine set up before the owner tier existed holds the generated region, with or without the
// RTK note that sat under it. Both are this repository's writing, so both migrate.
func TestAGeneratedCodexInstructionFileMigratesToTheOwnerCopy(t *testing.T) {
	f := newFixture(t)
	f.write(f.codexHome+"/AGENTS.md",
		flavor.RegionOpen+"\n"+flavor.RegionBody+"\n"+flavor.RegionClose+"\n")

	f.expectCode(f.install("--agent=codex", "--owner"), 0)

	f.expectFileBody(f.codexHome+"/AGENTS.md", ownerTemplate)
	if backups := backupsOf(t, f.codexHome, "AGENTS.md.backup."); len(backups) != 1 {
		t.Errorf("the migration left %d backup(s), wanted 1: %v", len(backups), backups)
	}
}

func TestAGeneratedCodexFileCarryingTheOldRtkNoteMigratesToo(t *testing.T) {
	f := newFixture(t)
	f.write(f.codexHome+"/AGENTS.md",
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
	f.write(f.codexHome+"/AGENTS.md", generated)

	f.expectCode(f.install("--agent=codex", "--owner", "--dry-run"), 0)

	f.expectFileBody(f.codexHome+"/AGENTS.md", generated)
	f.expectSaid("would install")
}

// The owner's memory store is created empty and never written over: it is the one file in this install
// whose whole content is the owner's.
func TestTheOwnerMemoryStoreIsCreatedOnceAndThenLeftAlone(t *testing.T) {
	f := newFixture(t)
	memory := f.home + "/Document/AI/MEMORY.md"

	f.expectCode(f.install("--agent=claude", "--owner"), 0)
	f.expectFileBody(memory, "# Memory\n")
	rewrite(t, memory, "# Memory\n\nKeep this entry.\n")

	f.expectCode(f.install("--agent=claude", "--owner"), 0)

	f.expectFileBody(memory, "# Memory\n\nKeep this entry.\n")
}

func TestAnOrdinaryInstallCreatesNoOwnerMemory(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude"), 0)

	f.expectAbsent(f.home + "/Document")
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
	f.expectFileBody(f.home+"/Document/AI/MEMORY.md", "# Memory\n")
}

func TestTheOwnerUninstallPreservesAModifiedFile(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=claude", "--owner"), 0)
	appendTo(t, f.home+"/.claude/CLAUDE.md", "\nLocal addition.\n")

	f.expectCode(f.install("--agent=claude", "--owner", "--uninstall"), 1)

	f.expectSaid("was modified — owner instructions were preserved")
	f.expectFileContains(f.home+"/.claude/CLAUDE.md", "Local addition.")
}

// The template the owner tier copies and the region every other tier is given say the same thing and
// cannot be derived from one another — generating three lines would cost a generator and a gate unit
// to keep it honest. This is what catches the wording drifting apart.
//
// Compared as the body's lines, not as a whole file: the owner's copy sits under a heading and beside
// prose the fenced copy has no business carrying.
func TestTheShippedOwnerTemplateCarriesEveryLineOfTheRegionBody(t *testing.T) {
	const shipped = "../../owner-instructions.md"
	body, err := os.ReadFile(shipped)
	if err != nil {
		t.Fatalf("reading %s, which is the other half of this comparison: %v", shipped, err)
	}
	lines := strings.Split(flavor.RegionBody, "\n")
	if len(lines) < 2 {
		t.Fatalf("the region body is %d line(s), so this comparison would assert almost nothing", len(lines))
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.Contains(string(body), line) {
			t.Errorf("%s does not carry %q, which every other tier is given — the two wordings have drifted "+
				"apart, and an owner and a colleague are now reading different instructions", shipped, line)
		}
	}
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
