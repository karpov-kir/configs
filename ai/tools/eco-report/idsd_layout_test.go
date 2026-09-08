package ecoreport_test

import (
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestLayoutMigrationPreservesArtifactsAndHumanFiles(t *testing.T) {
	f := newRepo(t)
	root := f.scratch()
	files := map[string]string{
		"charter.md":                  "# Charter\n",
		"roadmap.md":                  "# Roadmap\n",
		"decisions.md":                "## Promotion candidates\n\n## Decisions\n\n1x | 2026-09-08 | saved decision\n",
		"constraints.md":              "1x | 2026-09-08 | old constraint\n",
		"notes/data.txt":              "reference bytes\n",
		"intents/001-one/intent.md":   "intent bytes\n",
		"intents/001-one/language.md": "term bytes\n",
		"archive/002-two/intent.md":   "archived intent bytes\n",
		"archive/002-two/diagram.svg": "diagram bytes\n",
	}
	for name, body := range files {
		f.write(root+"/"+name, body)
	}
	f.runReport("layout", "migrate", "--dry-run")
	f.record("dry-run describes the migration", f.status == 0 && strings.Contains(f.out, "for-agents"), f.evidence())
	for name, body := range files {
		f.record("dry-run preserves "+name, f.read(root+"/"+name) == body, f.evidence())
	}
	f.record("dry-run creates no agent folder", !f.exists(root+"/for-agents"), f.evidence())
	f.runReport("layout", "migrate", "--apply")
	f.record("apply succeeds", f.status == 0, f.evidence())
	destinations := map[string]string{
		"charter.md": "charter.md", "roadmap.md": "roadmap.md",
		"decisions.md": "for-agents/decisions.md", "constraints.md": "for-agents/supporting/constraints.md",
		"notes/data.txt":              "for-agents/supporting/notes/data.txt",
		"intents/001-one/intent.md":   "intents/001-one/intent.md",
		"intents/001-one/language.md": "intents/001-one/for-agents/language.md",
		"archive/002-two/intent.md":   "archive/002-two/intent.md",
		"archive/002-two/diagram.svg": "archive/002-two/for-agents/supporting/diagram.svg",
	}
	for source, destination := range destinations {
		f.record("preserves "+source+" at "+destination, f.read(root+"/"+destination) == files[source], f.evidence())
	}
	f.runReport("layout", "migrate", "--apply")
	f.record("a repeated migration is harmless", f.status == 0, f.evidence())
}

func TestLayoutMigrationRefusesOpenReportsAndCollisions(t *testing.T) {
	for _, report := range []string{"qualify-report.md", "for-agents/qualify-report.md"} {
		f := newRepo(t)
		path := f.scratch() + "/intents/001-live/" + report
		f.write(path, "report obligations\n")
		f.runReport("layout", "migrate", "--apply")
		f.record("migration refuses open "+report, f.status != 0 && strings.Contains(f.out, "001-live"), f.evidence())
		f.record("the open report survives", f.read(path) == "report obligations\n", f.evidence())
	}
	f := newRepo(t)
	f.write(f.scratch()+"/decisions.md", "old\n")
	f.write(f.scratch()+"/for-agents/decisions.md", "new\n")
	f.runReport("layout", "migrate", "--apply")
	f.record("a collision refuses before any move", f.status != 0 && strings.Contains(f.out, "collision") && f.read(f.scratch()+"/decisions.md") == "old\n", f.evidence())
}

func TestLayoutCheckNamesPollutionAndLegacyInitCannotForkAReport(t *testing.T) {
	f := newRepo(t)
	f.write(f.scratch()+"/intents/001-live/qualify-report.md", "old obligations\n")
	f.runReport("init", "001-live")
	f.record("init refuses the legacy report", f.status != 0 && strings.Contains(f.out, "layout"), f.evidence())
	f.record("no second report was created", !f.exists(f.scratch()+"/intents/001-live/for-agents/qualify-report.md"), f.evidence())
	f.runReport("layout", "check")
	f.record("layout check identifies the misplaced report", f.status != 0 && strings.Contains(f.out, "qualify-report.md"), f.evidence())
}

func TestLayoutMigrationRefusesLinksAndHeldMergeSlots(t *testing.T) {
	f := newRepo(t)
	f.write(f.scratch()+"/decisions.md", "saved\n")
	f.takeMergeSlot("002-active")
	f.runReport("layout", "migrate", "--apply")
	f.record("an active merge blocks migration", f.status != 0 && strings.Contains(f.out, "merge slot"), f.evidence())
	linked := newRepo(t)
	linked.mkdirAll(linked.scratch())
	linked.symlink(linked.base+"/outside", linked.scratch()+"/for-agents")
	linked.runReport("layout", "migrate", "--apply")
	linked.record("a directory link blocks migration", linked.status != 0 && strings.Contains(linked.out, "symlink"), linked.evidence())
}

func TestLayoutMigrationSectionsLegacyDecisionsWithoutDroppingEntries(t *testing.T) {
	f := newRepo(t)
	root := f.scratch()
	body := "# Decisions\n\nPreserved preamble.\n\n3x | 2026-08-01 | old choice\n"
	f.write(root+"/decisions.md", body)
	f.runReport("layout", "migrate", "--apply")
	f.record("decision migration succeeds", f.status == 0, f.evidence())
	migrated := f.read(root + "/for-agents/decisions.md")
	f.record("the record has both sections and retains its old entry", strings.Contains(migrated, "## Promotion candidates") && strings.Contains(migrated, "## Decisions") && strings.Contains(migrated, "3x | 2026-08-01 | old choice") && strings.Contains(migrated, "Preserved preamble."), migrated)
	f.runReport("record", "append", "project-decisions", "a later choice")
	f.record("a migrated decision record accepts new entries", f.status == 0, f.evidence())
}

func TestLayoutCheckKeepsLegacyConstraintsVisibleUntilCurated(t *testing.T) {
	f := newRepo(t)
	f.write(f.scratch()+"/for-agents/supporting/constraints.md", "old human constraint\n")
	f.runReport("layout", "check")
	f.record("preserved constraints need explicit human curation", f.status == 1 && strings.Contains(f.out, "constraints.md"), f.evidence())
}

func TestNewReportsStayInAgentFoldersAndSupportingSurvivesDiscard(t *testing.T) {
	f := newShip(t, "001-new")
	f.record("the report is under for-agents", f.isFile(f.shipDir("001-new")+"/for-agents/qualify-report.md") && !f.exists(f.shipDir("001-new")+"/qualify-report.md"), f.evidence())
	f.write(f.scratch()+"/for-agents/supporting/reference.txt", "keep me\n")
	f.runReport("discard", "001-new")
	f.record("project supporting artifacts survive discard", f.status == 0 && f.read(f.scratch()+"/for-agents/supporting/reference.txt") == "keep me\n", f.evidence())
}

func TestLayoutCheckRejectsArchivedReportsAndMissingIntentFiles(t *testing.T) {
	for _, name := range []string{"archive/001-built/for-agents/qualify-report.md", "intents/002-missing/for-agents/language.md"} {
		f := newRepo(t)
		f.write(f.scratch()+"/"+name, "unexpected\n")
		f.runReport("layout", "check")
		f.record("layout rejects "+name, f.status == 1, f.evidence())
	}
	f := newShip(t, "review: standalone")
	f.runReport("layout", "check")
	f.record("a standalone review needs no intent or charter", f.status == 0, f.evidence())
}

func TestDiscardPreservesUnexpectedRootArtifacts(t *testing.T) {
	f := newShip(t, "001-going")
	f.write(f.scratch()+"/evidence.json", "evidence bytes\n")
	f.runReport("discard", "001-going")
	f.record("unexpected root evidence is retained", f.read(f.scratch()+"/evidence.json") == "evidence bytes\n", f.evidence())
}

func TestLayoutCheckRejectsSpecialFilesAndSanitizesPaths(t *testing.T) {
	f := newRepo(t)
	path := f.scratch() + "/for-agents/decisions.md"
	f.mkdirAll(f.scratch() + "/for-agents")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	f.runReport("layout", "check")
	f.record("a legal name cannot hide a nonregular file", f.status == 1 && strings.Contains(f.out, "nonregular"), f.evidence())
	weird := newRepo(t)
	weird.write(weird.scratch()+"/note\x1b[2J.md", "evidence\n")
	weird.runReport("layout", "check")
	weird.record("paths cannot drive the terminal", weird.status == 1 && !strings.Contains(weird.out, "\x1b"), weird.evidence())
}

func TestReportWritesCannotFollowAnAgentDirectoryLink(t *testing.T) {
	f := newRepo(t)
	outside := f.base + "/outside"
	f.mkdirAll(outside)
	f.mkdirAll(f.shipDir("001-linked"))
	f.symlink(outside, f.shipDir("001-linked")+"/for-agents")
	f.runReport("init", "001-linked")
	f.record("init rejects a linked agent directory", f.status != 0 && strings.Contains(f.out, "symlink") && !f.exists(outside+"/qualify-report.md"), f.evidence())
}

func TestLayoutMigrationUpdatesOnlyOwnedIgnoreRules(t *testing.T) {
	f := newRepo(t)
	f.write(f.treeIdsd()+"/charter.md", "# Charter\n")
	f.mustGit("add", ".idsd/charter.md")
	old := "# owner comment\n*.secret\n.idsd/intents/*/qualify-report.md\n"
	f.write(f.repo+"/.gitignore", old)
	f.runReport("layout", "migrate", "--dry-run")
	f.record("dry-run lists ignore changes without writing", f.status == 0 && strings.Contains(f.out, ".gitignore") && f.read(f.repo+"/.gitignore") == old, f.evidence())
	f.runReport("layout", "migrate", "--apply")
	f.record("ignore migration succeeds", f.status == 0, f.evidence())
	updated := f.read(f.repo + "/.gitignore")
	f.record("unrelated lines survive", strings.HasPrefix(updated, "# owner comment\n*.secret\n"), updated)
	f.record("the obsolete report pattern is replaced", !strings.Contains(updated, ".idsd/intents/*/qualify-report.md"), updated)
	f.runReport("check-ignore")
	f.record("the migrated ignore rules protect every new scratch file", f.status == 0, f.evidence())
	f.runReport("init", "001-new")
	f.record("a new report opens after migration", f.status == 0, f.evidence())
}

func TestLayoutMigrationRefusesLinkedIgnoreFilesBeforeMovingArtifacts(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink"} {
		f := newRepo(t)
		f.write(f.treeIdsd()+"/charter.md", "# Charter\n")
		f.write(f.treeIdsd()+"/language.md", "term bytes\n")
		f.mustGit("add", ".idsd/charter.md")
		outside := f.base + "/ignore-source"
		f.write(outside, "*.secret\n")
		if kind == "symlink" {
			f.symlink(outside, f.repo+"/.gitignore")
		} else if err := os.Link(outside, f.repo+"/.gitignore"); err != nil {
			t.Fatal(err)
		}
		f.runReport("layout", "migrate", "--apply")
		f.record("migration rejects "+kind+" ignore file before moves", f.status != 0 && f.read(f.treeIdsd()+"/language.md") == "term bytes\n" && f.read(outside) == "*.secret\n", f.evidence())
	}
}

func TestLayoutMigrationKeepsScratchIgnoredAfterANegation(t *testing.T) {
	f := newRepo(t)
	f.write(f.treeIdsd()+"/charter.md", "# Charter\n")
	f.mustGit("add", ".idsd/charter.md")
	negation := "!.idsd/intents/**/qualify-report.md"
	f.write(f.repo+"/.gitignore", ".idsd/intents/*/qualify-report.md\n"+negation+"\n")
	f.runReport("layout", "migrate", "--apply")
	f.record("migration succeeds", f.status == 0, f.evidence())
	migrated := f.read(f.repo + "/.gitignore")
	f.record("the unrelated negation is retained", strings.Contains(migrated, negation+"\n"), migrated)
	f.runReport("check-ignore")
	f.record("Git confirms all migrated scratch paths are ignored", f.status == 0, f.evidence())
	f.runReport("init", "001-after-negation")
	f.record("init can open a protected report after migration", f.status == 0, f.evidence())
}
