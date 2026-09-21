package projectsetup_test

import "testing"

// Codex discovers skills under .agents/skills and loads the shared AGENTS.md. The Claude file stays a
// file of its own carrying an import, so a project's two clients read one set of instructions.
func TestACodexInstallUsesItsOwnDirectoryAndTheSharedInstructions(t *testing.T) {
	f := newFixture(t)
	f.Write(f.project+"/AGENTS.md", "Project instructions.\n")

	f.expectCode(f.install("--agent=codex"), 0)

	f.expectLinkTo(f.skillsMount("codex")+"/kk-build", f.home+"/.kk-flavor/skills/kk-build")
	f.expectFileContains(f.project+"/AGENTS.md", "kk-flavor:begin")
	f.expectFileContains(f.project+"/AGENTS.md", "Project instructions.")
	f.expectFileContains(f.project+"/CLAUDE.md", "@AGENTS.md")
	f.expectFileContains(f.project+"/CLAUDE.md", "How this project works.")
}

// A nonempty AGENTS.override.md shadows AGENTS.md. A run that wrote the instructions anyway leaves
// the project installed with no client loading them.
func TestCodexRefusesAShadowedProjectInstructionFile(t *testing.T) {
	f := newFixture(t)
	f.Write(f.project+"/AGENTS.override.md", "Project override instructions.\n")

	f.expectCode(f.install("--agent=codex"), 1)

	f.expectSaid(f.project + "/AGENTS.override.md")
	f.expectAbsent(f.project + "/AGENTS.md")
}

func TestCodexRefusesItsOwnBroadIgnoreRule(t *testing.T) {
	f := newFixture(t)
	f.Write(f.project+"/.gitignore", "node_modules/\n.agents/\n")

	f.expectCode(f.install("--agent=codex"), 1)

	f.expectSaid("already ignores .agents/ wholesale")
}

// The two clients have a skills directory and an ignore region each. That is what lets one be removed
// while the other stays. One shared region would make the first uninstall unhide the second client's
// mounts.
func TestUninstallingOneClientLeavesTheOthersMountsRulesAndRegistryEntry(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=codex"), 0)
	f.expectCode(f.install("--agent=claude"), 0)

	f.expectCode(f.install("--agent=codex", "--uninstall"), 0)

	f.expectAbsent(f.skillsMount("codex") + "/kk-build")
	f.expectLinkTo(f.skillsMount("claude")+"/kk-build", f.home+"/.kk-flavor/skills/kk-build")
	f.expectFileContains(f.project+"/.gitignore", ".claude/skills/kk-*")
	f.expectFileLacks(f.project+"/.gitignore", ".agents/skills/kk-*")
	// The instructions and the import are shared, so they stay for as long as a client loads them.
	f.expectFileContains(f.project+"/AGENTS.md", "kk-flavor:begin")
	f.expectFileContains(f.project+"/CLAUDE.md", "@AGENTS.md")
	f.expectSaid("another client still uses them")
	f.expectFileContains(f.home+"/.config/kk-flavor/installs", f.project)
}

// And the last client out takes the shared files and the registry entry with it.
func TestTheLastClientUninstalledTakesTheSharedInstructionsAndTheEntry(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=codex"), 0)
	f.expectCode(f.install("--agent=claude"), 0)
	f.expectCode(f.install("--agent=codex", "--uninstall"), 0)

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.expectFileLacks(f.project+"/AGENTS.md", "kk-flavor:begin")
	f.expectFileLacks(f.project+"/CLAUDE.md", "@AGENTS.md")
	f.expectFileLacks(f.home+"/.config/kk-flavor/installs", f.project)
}

// A project already carrying the region in CLAUDE.md is what the first version of this installer left
// behind. The region moves to the shared file and CLAUDE.md keeps an import, so the two clients stop
// reading two copies.
func TestALegacyClaudeOnlyRegionMigratesToTheSharedFile(t *testing.T) {
	f := newFixture(t)
	f.Write(f.project+"/AGENTS.md", "# Shared standards\n\nKeep this shared rule.\n")
	f.Write(f.project+"/CLAUDE.md", "# project\n\nHow this project works.\n\n"+
		"<!-- kk-flavor:begin -->\n### KK Flavor\n\n"+
		"Read `~/.kk-flavor/inject.md` now and follow it — applies to all work, skill-invoked or ad-hoc.\n"+
		"<!-- kk-flavor:end -->\n")

	f.expectCode(f.install("--agent=claude"), 0)

	f.expectFileContains(f.project+"/CLAUDE.md", "@AGENTS.md")
	f.expectFileLacks(f.project+"/CLAUDE.md", "inject.md")
	f.expectFileContains(f.project+"/CLAUDE.md", "How this project works.")
	f.expectFileContains(f.project+"/AGENTS.md", "Keep this shared rule.")
	f.expectFileContains(f.project+"/AGENTS.md", "inject.md")
}

// A symlinked shared file is refused before either file is written. The checks on the project's own
// files all run ahead of the first mount, so a run cannot leave a project half installed.
func TestASymlinkedSharedFileIsRefusedBeforeEitherFileIsWritten(t *testing.T) {
	f := newFixture(t)
	f.Symlink(f.base+"/untouched-shared-target", f.project+"/AGENTS.md")

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectAbsent(f.base + "/untouched-shared-target")
	f.expectFileLacks(f.project+"/CLAUDE.md", "@AGENTS.md")
	f.expectAbsent(f.skillsMount("claude"))
}

// The import names the shared file, and an import written after a failed shared write points a client
// at a file that is missing. Only one path reaches that: a shared file this run has to create and
// cannot. Every other failure is caught by projectFilesWritable, before the first mount.
func TestAnUnwritableSharedFileLeavesNoImportBehind(t *testing.T) {
	f := newFixture(t)
	f.RemoveAll(f.project + "/CLAUDE.md")
	f.expectCode(f.install("--agent=claude"), 0)
	// Both files back out, and the project closed to new ones. A later run then has the mounts already
	// and reaches the instruction step, which is the only path this case can take.
	f.RemoveAll(f.project + "/AGENTS.md")
	f.RemoveAll(f.project + "/CLAUDE.md")
	f.closeToNewFiles(f.project)

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("could not create " + f.project + "/AGENTS.md")
	// And the import was never attempted. The run's own output answers this, since the tree cannot: the
	// same directory refuses both files. A run that went for the import anyway leaves the same tree
	// behind and a second refusal the reader should never see.
	f.expectNotSaid("could not create " + f.project + "/CLAUDE.md")
}
