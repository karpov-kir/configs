package aibootstrap_test

import "testing"

// Codex discovers skills under ~/.agents/skills and instructions in the profile CODEX_HOME names, so
// an install for it must write neither of Claude's two paths.
func TestCodexInstallsIntoItsOwnDiscoveryDirectoryAndProfile(t *testing.T) {
	f := newFixture(t)
	f.codexHome = f.home + "/codex-profile"

	f.expectCode(f.install("--agent=codex"), 0)

	f.expectLinkTo(f.skillsMount("codex")+"/kk-build", f.repo+"/kk-flavor/skills/kk-build")
	f.expectFileContains(f.codexHome+"/AGENTS.md", "kk-flavor:begin")
	f.expectAbsent(f.home + "/.claude")
}

// A nonempty AGENTS.override.md shadows AGENTS.md, so a run that wrote the instructions anyway would
// leave the tree installed and nothing loading it. Refused before anything is written.
func TestCodexRefusesAShadowedInstructionFileBeforeWritingAnything(t *testing.T) {
	f := newFixture(t)
	f.Write(f.codexHome+"/AGENTS.override.md", "custom override\n")

	f.expectCode(f.install("--agent=codex"), 1)

	f.expectSaid("AGENTS.override.md")
	f.expectAbsent(f.home + "/.kk-flavor")
}

// Codex kept its skills under CODEX_HOME before the shared discovery directory existed, and machines
// set up then still hold those links. The old one goes only once its replacement is in place, so an
// interrupted migration leaves Codex with a working mount rather than none.
func TestCodexDropsALegacyMountOnceItsReplacementIsThere(t *testing.T) {
	f := newFixture(t)
	f.Symlink(f.repo+"/kk-flavor/skills/kk-build", f.codexHome+"/skills/kk-build")

	f.expectCode(f.install("--agent=codex"), 0)

	f.expectLinkTo(f.skillsMount("codex")+"/kk-build", f.repo+"/kk-flavor/skills/kk-build")
	f.expectAbsent(f.codexHome + "/skills/kk-build")
}

// A link the old README's loop made reads back with a trailing slash, because `$d` came from a `*/`
// glob. It names the same skill, so it migrates like any other — and compared raw it would not.
func TestCodexMigratesALegacyLinkEndingInASlash(t *testing.T) {
	f := newFixture(t)
	f.Symlink(f.repo+"/kk-flavor/skills/kk-build/", f.codexHome+"/skills/kk-build")

	f.expectCode(f.install("--agent=codex"), 0)

	f.expectAbsent(f.codexHome + "/skills/kk-build")
}

// A real directory at the replacement's path is not this run's mount. Dropping the old link against it
// would leave Codex with a directory nobody wrote and no skill behind it.
func TestCodexKeepsALegacyMountWhoseDestinationIsOccupied(t *testing.T) {
	f := newFixture(t)
	f.Symlink(f.repo+"/kk-flavor/skills/kk-build", f.codexHome+"/skills/kk-build")
	f.MkdirAll(f.skillsMount("codex") + "/kk-build")

	f.expectCode(f.install("--agent=codex"), 1)

	f.expectSymlink(f.codexHome + "/skills/kk-build")
}

// CODEX_HOME is routinely an alias of the directory it names. A run that read its own destination as a
// second, older mount directory would migrate every skill out of the directory it had just mounted
// them into.
func TestAProfileAliasOfTheDiscoveryDirectoryIsNotASecondMountDirectory(t *testing.T) {
	f := newFixture(t)
	f.MkdirAll(f.home + "/.agents")
	f.Symlink(f.home+"/.agents", f.home+"/profile")
	f.codexHome = f.home + "/profile"

	f.expectCode(f.install("--agent=codex"), 0)

	f.expectLinkTo(f.skillsMount("codex")+"/kk-build", f.repo+"/kk-flavor/skills/kk-build")
}

// The two clients share the bucket and the source tree and are installed and removed independently.
// The bucket stays while the other client still has mounts from this checkout — removing it would
// leave that client's whole set resolving through a link that is gone.
func TestUninstallingOneClientKeepsTheBucketTheOtherStillNeeds(t *testing.T) {
	f := newFixture(t)
	f.expectCode(f.install("--agent=claude"), 0)
	f.expectCode(f.install("--agent=codex"), 0)

	f.expectCode(f.install("--agent=codex", "--uninstall"), 0)

	f.expectSaid("another client still has skill mounts")
	f.expectLinkTo(f.home+"/.kk-flavor", f.repo+"/kk-flavor")
	f.expectLinkTo(f.skillsMount("claude")+"/kk-build", f.repo+"/kk-flavor/skills/kk-build")
	f.expectAbsent(f.skillsMount("codex") + "/kk-build")
}

// And the last client out takes it. A profile alias must not be counted as another client here either:
// counted, the bucket would be kept for a client that is itself being removed.
func TestTheLastClientOutRemovesTheBucket(t *testing.T) {
	f := newFixture(t)
	f.MkdirAll(f.home + "/.agents")
	f.Symlink(f.home+"/.agents", f.home+"/profile")
	f.codexHome = f.home + "/profile"
	f.expectCode(f.install("--agent=codex"), 0)

	f.expectCode(f.install("--agent=codex", "--uninstall"), 0)

	f.expectAbsent(f.home + "/.kk-flavor")
}
