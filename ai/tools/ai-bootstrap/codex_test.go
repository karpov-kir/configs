package aibootstrap_test

import "testing"

// Codex discovers skills under ~/.agents/skills and instructions in the profile CODEX_HOME names, so
// an install for it must write neither of Claude's two paths.
func TestCodexInstallsIntoItsOwnDiscoveryDirectoryAndProfile(t *testing.T) {
	f := newFixture(t)
	f.codexHome = f.home + "/codex-profile"

	f.ExpectCode(f.install("--agent=codex"), 0)

	f.ExpectLinkTo(f.skillsMount("codex")+"/kk-build", f.repo+"/kk-flavor/skills/kk-build")
	f.ExpectFileBody(f.codexHome+"/AGENTS.md", ownerTemplate)
	f.ExpectAbsent(f.home + "/.claude")
}

// A nonempty AGENTS.override.md shadows AGENTS.md. A run that wrote the instructions anyway would
// leave the tree installed with no client loading it, so the refusal comes before the first write.
func TestCodexRefusesAShadowedInstructionFileBeforeWritingAnything(t *testing.T) {
	f := newFixture(t)
	f.Write(f.codexHome+"/AGENTS.override.md", "custom override\n")

	f.ExpectCode(f.install("--agent=codex"), 1)

	f.ExpectSaid("AGENTS.override.md")
	f.ExpectAbsent(f.home + "/.kk-flavor")
}

func TestCodexInstallsWhenItsProfileAliasesTheDiscoveryRoot(t *testing.T) {
	f := newFixture(t)
	f.MkdirAll(f.home + "/.agents")
	f.Symlink(f.home+"/.agents", f.home+"/profile")
	f.codexHome = f.home + "/profile"

	f.ExpectCode(f.install("--agent=codex"), 0)

	f.ExpectLinkTo(f.skillsMount("codex")+"/kk-build", f.repo+"/kk-flavor/skills/kk-build")
}

// The two clients share the bucket and the source tree and are installed and removed independently.
// The bucket stays while the other client still has mounts from this checkout. A removal there would
// leave that client's whole set resolving through a link that is gone.
func TestUninstallingOneClientKeepsTheBucketTheOtherStillNeeds(t *testing.T) {
	f := newFixture(t)
	f.ExpectCode(f.install("--agent=claude"), 0)
	f.ExpectCode(f.install("--agent=codex"), 0)

	f.ExpectCode(f.install("--agent=codex", "--uninstall"), 0)

	f.ExpectSaid("another client still has skill mounts")
	f.ExpectLinkTo(f.home+"/.kk-flavor", f.repo+"/kk-flavor")
	f.ExpectLinkTo(f.skillsMount("claude")+"/kk-build", f.repo+"/kk-flavor/skills/kk-build")
	f.ExpectAbsent(f.skillsMount("codex") + "/kk-build")
}

// Removing the last client also removes the shared bucket.
func TestTheLastClientOutRemovesTheBucket(t *testing.T) {
	f := newFixture(t)
	f.MkdirAll(f.home + "/.agents")
	f.Symlink(f.home+"/.agents", f.home+"/profile")
	f.codexHome = f.home + "/profile"
	f.ExpectCode(f.install("--agent=codex"), 0)

	f.ExpectCode(f.install("--agent=codex", "--uninstall"), 0)

	f.ExpectAbsent(f.home + "/.kk-flavor")
}
