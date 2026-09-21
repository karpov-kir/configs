// Cases for install.sh end to end: what a caller sees when a release cannot be downloaded, cannot be
// named, cannot be trusted, or installs.
//
// A faked `gh` answers what this script asked for. What a real `gh release download` returns is left
// alone, for the reason install.sh's header gives, so no case here claims anything about the network.
// Its argv is faked, because the repository and tag this asks for are the script's own decision.
//
// Exit 0 is every tool installed, 2 a refusal, and 3 the repository having cut no release. On 3 the
// repository had no release, and the run itself was sound. The third is separate because a caller
// cannot otherwise tell it from a download that failed, and the two send a reader to different places.
package reach

import (
	"configs/ai/tools/runtest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The tools the fixture release carries. Two here, where the repository ships twenty-two. Every case
// turns on the loop that installs every tool or none. The contents of the list are beside the point,
// and `ai/tools/shipped_test.go` holds the real list against the packages that back it.
var fixtureTools = []string{"alpha", "bravo"}

// What this machine's uname pair resolves to, as a literal. The mapping from uname to asset name is
// install.sh's, and every row of it is held in install_decisions_test.go.
var thisSuffix = runtime.GOOS + "-" + runtime.GOARCH

// The workflow identity the attestation is pinned to. The literal upstream path, and never one built
// from the remote. This is the trust anchor, so it stays put whatever origin a checkout happens to
// have. A fork cutting its own release is refused here and builds from source through resolve.sh.
const signerWorkflow = "--signer-workflow karpov-kir/configs/.github/workflows/release-tools.yml"

// `gh` resolves a release against the current directory's repository, with no `--repo` to tell it
// otherwise. This script runs from wherever the caller stands, and `../bootstrap.sh` invokes it without
// changing directory. Inside someone else's checkout that means downloading their assets and the
// SHA256SUMS they wrote to match, after which resolve.sh prefers those binaries for every stub exec.
func TestTheDownloadIsPinnedToTheOriginOfTheCheckoutInstallLivesIn(t *testing.T) {
	t.Parallel()
	sandbox := runtest.Sandbox(t)
	checkout := newCheckout(t, sandbox, "https://github.com/unreachable/target.git")
	refused, log := install(t, checkout, nil)

	runtest.ExpectRefusal(t, refused, "could not download")
	// The control this case's assertion needs: a gh that never ran leaves an empty log. A search over an
	// empty log finds no match, which reads exactly like a search that found the wrong thing.
	if len(log) == 0 {
		t.Fatalf("gh was never invoked, so its argv is not a measurement\n%v", refused)
	}
	if !calledWith(log, "--repo unreachable/target") {
		t.Errorf("the download was not pinned to the checkout's own origin, so gh resolves the release "+
			"against whatever repository the caller was standing in\n%s", strings.Join(log, "\n"))
	}
	// This gh fails the release listing too, so the repository's release state is unreadable. That is a
	// different fact from having no release, and the two must be reported apart. The third state, a
	// listing that does hold a release, is install_decisions_test.go's `releases_state` rows, and the
	// install that exits 0 below runs on it.
	if refused.Said("has cut no release") {
		t.Errorf("a listing gh could not answer was reported as a repository with no release, which tells "+
			"an offline machine there is nothing to download\n%v", refused)
	}
}

// A repository that has never cut a release and a download that failed are different facts with
// different consequences, and gh reports both by failing the download. The release listing is what
// separates them.
func TestARepositoryThatHasCutNoReleaseExitsThreeRatherThanTwo(t *testing.T) {
	t.Parallel()
	sandbox := runtest.Sandbox(t)
	checkout := newCheckout(t, sandbox, "https://github.com/no-release/target.git")
	refused, _ := install(t, checkout, nil)

	if refused.Code != 3 {
		t.Errorf("wanted exit 3, the one outcome that is neither an install nor a refusal\n%v", refused)
	}
	for _, wording := range []string{"has cut no release", "from source on first use", "Go toolchain"} {
		if !refused.Said(wording) {
			t.Errorf("the message does not say %q, so a reader is not told what happens instead or what it "+
				"needs\n%v", wording, refused)
		}
	}
	if refused.Said("could not download") {
		t.Errorf("a repository with no release was reported as a failed download, which sends a reader to "+
			"this machine's network rather than to the repository's releases\n%v", refused)
	}
	expectNothingInstalled(t, checkout)
}

// A tag reaches `gh release download` as its first positional argument, where a leading dash makes it
// an option instead: `--repo=someone/else` redirects the whole download. This case drives install.sh
// end to end, because what matters is that the refusal happens before the download.
func TestATagThatIsReallyAnOptionIsRefusedBeforeGhIsReached(t *testing.T) {
	t.Parallel()
	sandbox := runtest.Sandbox(t)
	checkout := newCheckout(t, sandbox, "https://github.com/pinned/target.git")
	refused, log := install(t, checkout, []string{"--repo=evil/pwn"})

	runtest.ExpectRefusal(t, refused, "is not a release tag")
	if len(log) != 0 {
		t.Errorf("gh ran anyway:\n%s", strings.Join(log, "\n"))
	}
}

// A repository install.sh cannot name is a refusal. gh would have guessed one, and guessing is the
// whole defect, so a checkout that cannot answer stops there. A fall back would restore the behaviour
// this pinning replaced.
func TestACheckoutWithNoOriginRemoteRefusesRatherThanLettingGhGuess(t *testing.T) {
	t.Parallel()
	sandbox := runtest.Sandbox(t)
	checkout := newCheckout(t, sandbox, "")
	refused, log := install(t, checkout, nil)

	runtest.ExpectRefusal(t, refused, "not a checkout with an 'origin' remote")
	if len(log) != 0 {
		t.Errorf("gh ran anyway:\n%s", strings.Join(log, "\n"))
	}
}

// The refusal a machine without gh gets. It carries the way out, which is the whole reason either a
// Go toolchain or a release is enough.
func TestAMachineWithoutGhIsToldTheWayOutThatNeedsNoGh(t *testing.T) {
	t.Parallel()
	sandbox := runtest.Sandbox(t)
	checkout := newCheckout(t, sandbox, "https://github.com/pinned/target.git")
	// A PATH holding only what install.sh needs to reach its gh check. Without `dirname` it dies at
	// self-resolution, which exits 2 as well, and the assertion is on the wording for that reason.
	refused := runtest.Launch(t, newLaunch(t, filepath.Join(checkout, "ai", "tools", "install.sh"),
		newPathDir(t, sandbox, "no-gh", "bash", "dirname")))

	runtest.ExpectRefusal(t, refused, "gh is not installed")
	// One line carrying both halves. `resolve.sh` is in the refusal for an unsupported platform too. An
	// assertion on it alone passes on that one and claims a way out this refusal may not carry.
	if !strings.Contains(refusalLine(refused, "gh is not installed"), "resolve.sh") {
		t.Errorf("the gh refusal does not name the way that needs no gh, so a machine without it is told "+
			"only what failed\n%v", refused)
	}
}

// The provenance gate, and the only run here that installs anything. Every case before it stops at
// the download. This is the only run in which the attestation check passes.

// The hash and the attestation answer different questions. The hash proves the asset matches the
// SHA256SUMS shipped beside it, and whoever publishes a release publishes both, so an attacker's
// assets verify against the attacker's own list. The attestation is signed against the release
// workflow's identity, and it alone says the binary came from here.
func TestAReleaseWhoseAssetsAllCarryAnAttestationInstallsWithTheirStamps(t *testing.T) {
	t.Parallel()
	sandbox := runtest.Sandbox(t)
	checkout := newCheckout(t, sandbox, "https://github.com/pinned/target.git")
	installed, log := install(t, checkout, nil, "GH_FAKE_DOWNLOAD=serve")

	if installed.Code != 0 {
		t.Fatalf("the install refused a release whose assets all carry an attestation\n%v", installed)
	}
	bin := filepath.Join(checkout, "ai", "tools", "bin")
	for _, name := range fixtureTools {
		info, err := os.Stat(filepath.Join(bin, name))
		if err != nil || info.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s did not land in bin/ executable (%v)", name, err)
		}
		// The stamp the release recorded, beside the binary it belongs to. A release install missing it has
		// no stamp to hold its binaries against, and resolve.sh reports every one of them as unknown for as
		// long as it is installed.
		if stamp := strings.TrimSpace(runtest.ReadFile(t, filepath.Join(bin, name+".stamp"))); stamp != "stamp-for-"+name {
			t.Errorf("%s was installed with the stamp %q rather than the one the release recorded", name, stamp)
		}
	}

	// One check per asset on the run that passed, each pinned to the same repository the download was. A
	// refusal says little about a check that was never reached. The match is on the whole argv, because
	// `--repo` alone leaves the signer unpinned: every workflow in the repository that can request an
	// id-token signs attestations gh would accept.

	// STAMPS is counted with the binaries. It decides, on every run afterwards, whether a binary is
	// reported as built from the source beside it. An unverified STAMPS is then a stale binary no human is
	// ever warned about.
	checked := 0
	for _, called := range log {
		if strings.HasPrefix(called, "attestation verify ") &&
			strings.Contains(called, "--repo pinned/target") && strings.HasSuffix(called, signerWorkflow) {
			checked++
		}
	}
	if want := len(fixtureTools) + 1; checked != want {
		t.Errorf("%d of the %d assets were checked for provenance against the release workflow's own "+
			"identity\n%s", checked, want, strings.Join(log, "\n"))
	}
}

// Every way a release is refused after it has been downloaded. Every tool or none, in each. A
// half-installed set is worse than none, because resolve.sh prefers whatever binary it finds.
func TestAReleaseThatCannotBeTrustedInstallsNothingAtAll(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		// What the fake release does differently, on top of serving the download.
		fake []string
		// The wording only this cause produces, then what the message has to carry beyond it.
		says      string
		alsoSays  string
		neverSays string
	}{
		{
			// One asset without an attestation, and the whole install stops.
			name:     "one asset carries no provenance attestation",
			fake:     []string{"GH_FAKE_UNATTESTED=bravo-" + thisSuffix},
			says:     "bravo-" + thisSuffix + " carries no provenance",
			alsoSays: "stub-gh: no attestation matching",
			// The hash passed on that run, so the refusal came from provenance and never from a broken fixture.
			neverSays: "does not match its recorded hash",
		},
		{
			// A release cut before source stamps existed. install.sh refuses it, because an unstamped binary
			// is one resolve.sh reports as unknown on every run, and a warning that never goes away is one
			// that stops being read. The needle is this refusal's own wording, and the per-tool loop's wording
			// fires for any asset that failed to arrive.
			name:     "the release carries no STAMPS at all",
			fake:     []string{"GH_FAKE_NO_STAMPS=1"},
			says:     "has no STAMPS asset",
			alsoSays: "take a later one",
		},
		{
			// One tool missing from a STAMPS that is otherwise whole. install.sh refuses it for the same
			// reason: a single unstamped tool is exactly what a reader would never think to check.
			name: "STAMPS omits one of the tools",
			fake: []string{"GH_FAKE_UNSTAMPED=bravo"},
			says: "records no source stamp for bravo",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			sandbox := runtest.Sandbox(t)
			checkout := newCheckout(t, sandbox, "https://github.com/pinned/target.git")
			refused, _ := install(t, checkout, nil, append([]string{"GH_FAKE_DOWNLOAD=serve"}, scenario.fake...)...)

			runtest.ExpectRefusal(t, refused, scenario.says)
			if scenario.alsoSays != "" && !refused.Said(scenario.alsoSays) {
				// gh's own reason is carried into the refusal. The asset sits in a staging directory the EXIT
				// trap deletes, so running the command again reports a missing file. gh's reason is the only
				// thing telling an unattested binary from an offline machine.
				t.Errorf("the refusal does not carry %q\n%v", scenario.alsoSays, refused)
			}
			if scenario.neverSays != "" && refused.Said(scenario.neverSays) {
				t.Errorf("the refusal says %q, so something other than the check this case names refused "+
					"it\n%v", scenario.neverSays, refused)
			}
			expectNothingInstalled(t, checkout)
		})
	}
}

// One run of install.sh in a fixture checkout, with the fake gh on PATH, and every line of argv that
// fake was called with.
func install(t *testing.T, checkout string, arguments []string, fake ...string) (runtest.Run, []string) {
	t.Helper()
	sandbox := filepath.Dir(checkout)
	log := filepath.Join(checkout, "gh-argv")
	command := newLaunch(t, filepath.Join(checkout, "ai", "tools", "install.sh"),
		newGhPath(t, sandbox)+":"+os.Getenv("PATH"), arguments...)
	command.Env = append(command.Env,
		"GH_FAKE_LOG="+log,
		"GH_FAKE_TOOLS="+strings.Join(fixtureTools, " "),
		"GH_FAKE_SUFFIX="+thisSuffix)
	command.Env = append(command.Env, fake...)
	ran := runtest.Launch(t, command)

	body, err := os.ReadFile(log)
	if err != nil {
		return ran, nil
	}
	return ran, strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
}

// A refusal installs every tool or none, so each case has to show bin/ empty. This helper is named,
// because a bare directory listing in a case reads as a listing and hides the claim the case is making.
func expectNothingInstalled(t *testing.T, checkout string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(checkout, "ai", "tools", "bin"))
	if err != nil {
		return
	}
	if len(entries) != 0 {
		var landed []string
		for _, entry := range entries {
			landed = append(landed, entry.Name())
		}
		t.Errorf("bin/ holds %v after a refusal. A half-installed set is worse than none: resolve.sh "+
			"prefers whatever binary it finds", landed)
	}
}

func calledWith(log []string, argument string) bool {
	for _, called := range log {
		if strings.Contains(called, argument) {
			return true
		}
	}
	return false
}

// The single line of a refusal carrying the wording, so a case can ask what else that line says.
func refusalLine(refused runtest.Run, wording string) string {
	for _, line := range strings.Split(refused.Stdout+"\n"+refused.Stderr, "\n") {
		if strings.Contains(line, wording) {
			return line
		}
	}
	return ""
}
