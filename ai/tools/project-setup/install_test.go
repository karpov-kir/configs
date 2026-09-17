package projectsetup_test

import (
	"slices"
	"testing"
)

// The whole of an ordinary install, driven end to end: the skills, both instruction files, the ignore
// rules and the registry entry.
func TestAFreshProjectGetsTheSkillsTheFilesAndTheRegistryEntry(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude"), 0)

	// Through the shared bucket, never straight into this checkout: a project holds the one path every
	// install of this flavor has, so the checkout can move without every project's mounts dangling.
	for _, name := range publicSkills {
		f.expectLinkTo(f.skillsMount("claude")+"/"+name, f.home+"/.kk-flavor/skills/"+name)
	}
	f.expectLinkTo(f.home+"/.kk-flavor", f.repo+"/kk-flavor")
	f.expectSaid("maintainer-only")

	f.expectFileContains(f.project+"/AGENTS.md", "kk-flavor:begin")
	f.expectFileContains(f.project+"/CLAUDE.md", "@AGENTS.md")
	f.expectFileContains(f.project+"/CLAUDE.md", "How this project works.")
	f.expectFileContains(f.project+"/.gitignore", ".claude/skills/kk-*")
	f.expectFileContains(f.project+"/.gitignore", "node_modules/")
	f.expectFileContains(f.home+"/.config/kk-flavor/installs", f.project)
}

// Safe to re-run is the property that makes this usable on a project someone is working in, and the
// only evidence for it is a second run over the first run's output.
func TestASecondRunRewritesNothingAndRecordsTheProjectOnce(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=claude"), 0)
	instructions := f.read(f.project + "/AGENTS.md")
	ignore := f.read(f.project + "/.gitignore")

	f.expectCode(f.install("--agent=claude"), 0)

	f.expectFileBody(f.project+"/AGENTS.md", instructions)
	f.expectFileBody(f.project+"/.gitignore", ignore)
	f.expectFileBody(f.home+"/.config/kk-flavor/installs", f.project+"\n")
}

func TestAProjectWithNeitherFileGetsBothCreated(t *testing.T) {
	f := newFixture(t)
	f.removeAll(f.project + "/CLAUDE.md")
	f.removeAll(f.project + "/.gitignore")

	f.expectCode(f.install("--agent=claude"), 0)

	f.expectFileContains(f.project+"/AGENTS.md", "kk-flavor:begin")
	f.expectFileContains(f.project+"/CLAUDE.md", "@AGENTS.md")
	f.expectFileContains(f.project+"/.gitignore", ".claude/skills/idsd-*")
}

// Reported rather than appended to: that rule covers this install's mounts and the project's own
// client settings alike, so what to do about it is a decision for the human whose repository it is.
func TestAProjectAlreadyIgnoringTheAgentDirectoryIsReportedNotAppendedTo(t *testing.T) {
	f := newFixture(t)
	f.write(f.project+"/.gitignore", "node_modules/\n.claude/\n")

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("already ignores .claude/ wholesale")
	f.expectSaid("line 2")
	f.expectFileLacks(f.project+"/.gitignore", "kk-flavor:begin")
}

// A dangling symlink answers "not there", and a write follows it — so without the check the creating
// branch rewrites whatever the link names, anywhere this run's user can write. A repository is not
// trusted input.
func TestASymlinkedIgnoreFileIsRefusedRatherThanWrittenThrough(t *testing.T) {
	f := newFixture(t)
	f.removeAll(f.project + "/.gitignore")
	f.write(f.base+"/outside.txt", "do not touch\n")
	f.symlink(f.base+"/outside.txt", f.project+"/.gitignore")

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("is a symlink")
	f.expectFileBody(f.base+"/outside.txt", "do not touch\n")
}

func TestADanglingInstructionSymlinkDoesNotCreateTheFileItNames(t *testing.T) {
	f := newFixture(t)
	f.removeAll(f.project + "/CLAUDE.md")
	f.symlink(f.base+"/never-created.txt", f.project+"/CLAUDE.md")

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectAbsent(f.base + "/never-created.txt")
}

// Half a fence means something edited inside the region or truncated the file, so the span a write
// would rewrite is no longer the span that was written. Refused before the mounts, because by the time
// the write refused the project would be half installed.
func TestAnIncompleteRegionIsRefusedBeforeAnythingIsMounted(t *testing.T) {
	f := newFixture(t)
	f.write(f.project+"/CLAUDE.md", "# project\n\n<!-- kk-flavor:begin -->\nhalf a region\n")

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("holds an incomplete kk-flavor region")
	f.expectAbsent(f.skillsMount("claude"))
}

// The flag has to write nothing, or it is worse than not having it: someone checks with --dry-run and
// it is the run that changed their project.
func TestADryRunMountsNothingWritesNothingAndRecordsNothing(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude", "--dry-run"), 0)

	f.expectSaid("would link")
	f.expectAbsent(f.project + "/.claude")
	f.expectFileLacks(f.project+"/CLAUDE.md", "kk-flavor:begin")
	f.expectAbsent(f.home + "/.config/kk-flavor/installs")
	f.expectAbsent(f.home + "/.kk-flavor")
}

// The dry run's preview has to name what a real run would write, and the bucket is what makes that
// non-obvious: the sources are rewritten between the survey and the mount.
func TestADryRunPreviewsTheBucketTheSkillsWouldReachThrough(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude", "--dry-run"), 0)

	f.expectSaid(f.home + "/.kk-flavor/skills/kk-build")
}

// The survey exists because the mount sources are rewritten to the bucket before anything is linked:
// rewritten, the second-checkout guard has no checkout to recognise, and a project still mounted from
// somebody else's clone would be repointed without a word.
func TestAProjectMountedFromAnotherCheckoutIsRefusedWithItsOwnScope(t *testing.T) {
	f := newFixture(t)
	stranger := f.base + "/stranger/ai"
	f.newCheckout(stranger)
	f.symlink(stranger+"/kk-flavor/skills/kk-build", f.skillsMount("claude")+"/kk-build")

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid(stranger)
	// This project's skills, not the machine's configuration: a refusal that overstated what was at
	// stake would teach people to ignore it.
	f.expectSaid(f.project + "'s skills")
	f.expectLinkTo(f.skillsMount("claude")+"/kk-build", stranger+"/kk-flavor/skills/kk-build")
}

func TestRelocateMovesAProjectsMountsOntoThisCheckout(t *testing.T) {
	f := newFixture(t)
	stranger := f.base + "/stranger/ai"
	f.newCheckout(stranger)
	f.symlink(stranger+"/kk-flavor/skills/kk-build", f.skillsMount("claude")+"/kk-build")

	f.expectCode(f.install("--agent=claude", "--relocate"), 0)

	f.expectLinkTo(f.skillsMount("claude")+"/kk-build", f.home+"/.kk-flavor/skills/kk-build")
}

// A skill renamed or deleted upstream leaves a mount nothing in the table names any more, so only the
// run that would have written it can notice. Scoped to what this checkout wrote: another checkout's
// link is not this run's to drop.
func TestAMountWhoseSkillIsGoneIsSweptAndAStrangersIsNot(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=claude"), 0)
	f.symlink(f.home+"/.kk-flavor/skills/kk-was-renamed", f.skillsMount("claude")+"/kk-was-renamed")
	f.symlink(f.base+"/another-checkout/kk-flavor/skills/kk-stranger", f.skillsMount("claude")+"/kk-stranger")

	f.expectCode(f.install("--agent=claude"), 0)

	f.expectAbsent(f.skillsMount("claude") + "/kk-was-renamed")
	f.expectSymlink(f.skillsMount("claude") + "/kk-stranger")
	f.expectSymlink(f.skillsMount("claude") + "/kk-edit")
}

func TestADryRunOverARetiredMountRemovesNothing(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=claude"), 0)
	f.symlink(f.home+"/.kk-flavor/skills/kk-was-renamed", f.skillsMount("claude")+"/kk-was-renamed")

	f.expectCode(f.install("--agent=claude", "--dry-run"), 0)

	f.expectSymlink(f.skillsMount("claude") + "/kk-was-renamed")
}

// Maintainer-only skills are for maintaining this instruction tree and do nothing for a project that
// merely uses it.
func TestTheDefaultTierLeavesTheMarkedSkillsOutOfAProject(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude"), 0)

	if mounted := f.mounted(f.skillsMount("claude")); !slices.Equal(mounted, publicSkills) {
		t.Errorf("the project holds %v, wanted %v", mounted, publicSkills)
	}
}

// The tier a project was installed with is written down nowhere the uninstall reads, so an uninstall
// that re-applied the filter would build its removal table for the tier being asked for NOW —
// --maintainer in and plain out would leave exactly the marked skills mounted, and the registry entry
// goes too, so nothing on the machine would ever name them again.
func TestAPlainUninstallRemovesWhatMaintainerInstalled(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=claude", "--maintainer"), 0)

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

	if mounted := f.mounted(f.skillsMount("claude")); len(mounted) > 0 {
		t.Errorf("the uninstall left %v mounted", mounted)
	}
}

func TestUninstallTakesItsOwnRegionsAndLeavesTheProjectsWriting(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=claude"), 0)

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.expectFileLacks(f.project+"/CLAUDE.md", "kk-flavor:begin")
	f.expectFileContains(f.project+"/CLAUDE.md", "How this project works.")
	f.expectFileLacks(f.project+"/.gitignore", "kk-flavor:begin")
	f.expectFileContains(f.project+"/.gitignore", "node_modules/")
	f.expectFileLacks(f.home+"/.config/kk-flavor/installs", f.project)
	// The bucket is the machine's, not this project's: another project may still be mounting through it.
	f.expectSymlink(f.home + "/.kk-flavor")
	f.expectSaid("Shared " + f.home + "/.kk-flavor was kept")
}

func TestUninstallingTwiceIsNotAnError(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=claude"), 0)
	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)

	f.expectCode(f.install("--agent=claude", "--uninstall"), 0)
}

// The MCP tool is a whole tool of its own; what this installer decides is what to do with the code it
// answers. A refusal there has to stop the install rather than be reported and passed over.
func TestARefusedMcpConfigurationStopsTheInstall(t *testing.T) {
	f := newFixture(t)
	f.mcp.code = 1

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("project MCP configuration needs attention")
	f.expectAbsent(f.skillsMount("claude"))
}

func TestTheMcpToolIsAskedAboutTheClientAndModeThisRunIsIn(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=codex", "--dry-run"), 0)

	if want := "codex install dry-run " + f.project; !slices.Contains(f.mcp.calls, want) {
		t.Errorf("the MCP tool was asked %v, wanted %q", f.mcp.calls, want)
	}
}
