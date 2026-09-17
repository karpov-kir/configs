// Cases for install.sh end to end: what a caller sees when a release cannot be downloaded, cannot be
// named, cannot be trusted, or installs.
//
// A faked `gh` answers what this script asked for. What a real `gh release download` returns is not
// faked — install.sh's header says why — so nothing here claims anything about the network. Its argv is
// faked, because which repository and tag this asks for is the script's own decision.
//
// Exit 0 is every tool installed, 2 a refusal, and 3 the repository having cut no release: there was
// nothing to download and nothing went wrong. The third is separate because a caller cannot otherwise
// tell it from a download that failed, and the two send a reader to different places.
package reach

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The tools the fixture release carries. Two, not this repository's twenty-two: every case here turns on
// the all-or-nothing loop over that list rather than on its contents, and `ai/tools/shipped_test.go` is
// what holds the real list against the packages that back it.
var fixtureTools = []string{"alpha", "bravo"}

// What this machine's uname pair resolves to. Named rather than derived from uname here — the mapping
// from uname to asset name is install.sh's, and every row of it is held in install_decisions_test.go.
var thisSuffix = runtime.GOOS + "-" + runtime.GOARCH

// The workflow identity the attestation is pinned to. The literal upstream path and never one built from
// the remote: this is the trust anchor, so it must not move with whatever origin a checkout happens to
// have. A fork cutting its own release is refused here and builds from source through resolve.sh.
const signerWorkflow = "--signer-workflow karpov-kir/configs/.github/workflows/release-tools.yml"

// `gh` resolves a release against the current directory's repository when it is not told which one, and
// this script runs from wherever the caller stands — `../bootstrap.sh` invokes it without changing
// directory. Inside someone else's checkout that means downloading their assets and the SHA256SUMS they
// wrote to match, after which resolve.sh prefers those binaries for every stub exec.
func TestTheDownloadIsPinnedToTheOriginOfTheCheckoutInstallLivesIn(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	checkout := newCheckout(t, sandbox, "https://github.com/unreachable/target.git")
	refused, log := install(t, checkout, nil)

	expectRefusal(t, refused, "could not download")
	// The control the assertion below needs: a gh that was never invoked leaves no log, and a search over
	// nothing finds nothing, which reads exactly like a search that found the wrong thing.
	if len(log) == 0 {
		t.Fatalf("gh was never invoked, so its argv is not a measurement\n%v", refused)
	}
	if !calledWith(log, "--repo unreachable/target") {
		t.Errorf("the download was not pinned to the checkout's own origin, so gh resolves the release "+
			"against whatever repository the caller was standing in\n%s", strings.Join(log, "\n"))
	}
	// This gh fails the release listing too, so the repository's release state is unreadable — which is
	// not the same fact as having no release, and must not be reported as one.
	if refused.said("has cut no release") {
		t.Errorf("a listing gh could not answer was reported as a repository with no release, which tells "+
			"an offline machine there is nothing to download\n%v", refused)
	}
}

// A repository that has never cut a release and a download that failed are different facts with
// different consequences, and gh reports both by failing the download. The release listing is what
// separates them.
func TestARepositoryThatHasCutNoReleaseExitsThreeRatherThanTwo(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	checkout := newCheckout(t, sandbox, "https://github.com/no-release/target.git")
	refused, _ := install(t, checkout, nil)

	if refused.code != 3 {
		t.Errorf("wanted exit 3, which is the one outcome that is neither an install nor a refusal\n%v", refused)
	}
	for _, wording := range []string{"has cut no release", "from source on first use", "Go toolchain"} {
		if !refused.said(wording) {
			t.Errorf("the message does not say %q, so a reader is not told what happens instead or what it "+
				"needs\n%v", wording, refused)
		}
	}
	if refused.said("could not download") {
		t.Errorf("a repository with no release was reported as a failed download, which sends a reader to "+
			"this machine's network rather than to the repository's releases\n%v", refused)
	}
	expectNothingInstalled(t, checkout)
}

// The control that stops the arm above swallowing the refusal below it: same fake, a release in the
// listing, and the download still fails. Without it, every download failure could report an absent
// release and this suite would agree.
func TestADownloadThatFailsWhereAReleaseExistsStillExitsTwo(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	checkout := newCheckout(t, sandbox, "https://github.com/pinned/target.git")
	refused, _ := install(t, checkout, nil)

	expectRefusal(t, refused, "could not download")
	if refused.said("has cut no release") {
		t.Errorf("a download that failed against a repository with releases was reported as one with "+
			"none\n%v", refused)
	}
}

// A tag reaches `gh release download` as its first positional argument, where a leading dash makes it an
// option instead — `--repo=someone/else` redirects the whole download. Driven end to end rather than
// through is_safe_tag alone, because what matters is that the refusal happens before the download.
func TestATagThatIsReallyAnOptionIsRefusedBeforeGhIsReached(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	checkout := newCheckout(t, sandbox, "https://github.com/pinned/target.git")
	refused, log := install(t, checkout, []string{"--repo=evil/pwn"})

	expectRefusal(t, refused, "is not a release tag")
	if len(log) != 0 {
		t.Errorf("gh ran anyway:\n%s", strings.Join(log, "\n"))
	}
}

// A repository it cannot name is a refusal, never a fall back to whatever gh would have guessed —
// guessing is the whole defect, so a checkout that cannot answer must stop rather than downgrade to the
// behaviour this pinning replaced.
func TestACheckoutWithNoOriginRemoteRefusesRatherThanLettingGhGuess(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	checkout := newCheckout(t, sandbox, "")
	refused, log := install(t, checkout, nil)

	expectRefusal(t, refused, "not a checkout with an 'origin' remote")
	if len(log) != 0 {
		t.Errorf("gh ran anyway:\n%s", strings.Join(log, "\n"))
	}
}

// The refusal a machine without gh gets. It carries the way out, which is the whole reason a Go
// toolchain and a release are alternatives rather than both required.
func TestAMachineWithoutGhIsToldTheWayOutThatNeedsNoGh(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	checkout := newCheckout(t, sandbox, "https://github.com/pinned/target.git")
	// A PATH holding only what install.sh needs to reach its gh check: without `dirname` it dies at
	// self-resolution instead, which exits 2 as well, so this is asserted on the wording.
	refused := launch(t, newLaunch(t, filepath.Join(checkout, "ai", "tools", "install.sh"),
		newPathDir(t, sandbox, "no-gh", "bash", "dirname")))

	expectRefusal(t, refused, "gh is not installed")
	// One line carrying both halves. `resolve.sh` is in the refusal for an unsupported platform too, so
	// asserting on it alone passes on that one and claims this refusal carries a way out when it may not.
	if !strings.Contains(refusalLine(refused, "gh is not installed"), "resolve.sh") {
		t.Errorf("the gh refusal does not name the way that needs no gh, so a machine without it is told "+
			"only what failed\n%v", refused)
	}
}

// The provenance gate, and the only run here that installs anything. Every refusal above stops at the
// download, so without this the attestation check is a behaviour nothing exercises.
//
// The hash and the attestation answer different questions. The hash proves the asset matches the
// SHA256SUMS shipped beside it, but whoever publishes a release publishes both, so an attacker's assets
// verify against the attacker's own list. Only the attestation, signed against the release workflow's
// identity, says the binary came from here.
func TestAReleaseWhoseAssetsAllCarryAnAttestationInstallsWithTheirStamps(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	checkout := newCheckout(t, sandbox, "https://github.com/pinned/target.git")
	installed, log := install(t, checkout, nil, "GH_FAKE_DOWNLOAD=serve")

	if installed.code != 0 {
		t.Fatalf("the install refused a release whose assets all carry an attestation\n%v", installed)
	}
	bin := filepath.Join(checkout, "ai", "tools", "bin")
	for _, name := range fixtureTools {
		info, err := os.Stat(filepath.Join(bin, name))
		if err != nil || info.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s did not land in bin/ executable (%v)", name, err)
		}
		// The stamp the release recorded, beside the binary it belongs to. Without it a release install has
		// nothing to hold its binaries against, and resolve.sh reports every one of them as unknown for as
		// long as it is installed.
		if stamp := strings.TrimSpace(read(t, filepath.Join(bin, name+".stamp"))); stamp != "stamp-for-"+name {
			t.Errorf("%s was installed with the stamp %q rather than the one the release recorded", name, stamp)
		}
	}

	// One check per asset on the run that passed, each pinned to the same repository the download was. A
	// refusal proves nothing about a check that was never reached. Matched on the whole argv, because
	// `--repo` alone leaves the signer unpinned: every workflow in the repository that can request an
	// id-token signs attestations gh would accept. STAMPS is counted with the binaries — it decides, on
	// every run afterwards, whether a binary is reported as built from the source beside it, so an
	// unverified STAMPS is a stale binary nobody is ever warned about.
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

// Every way a release is refused after it has been downloaded. All-or-nothing in each: a half-installed
// set is worse than none, because resolve.sh prefers whatever binary it finds.
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
			// The hash passed on that run, so it is provenance that refused and not a broken fixture.
			neverSays: "does not match its recorded hash",
		},
		{
			// A release cut before source stamps existed. Refused rather than installed unstamped, because
			// an unstamped binary is one resolve.sh reports as unknown on every run, and a warning that
			// never goes away is one that stops being read. The needle is this refusal's own wording and not
			// the per-tool loop's, which fires for any asset that failed to arrive.
			name:     "the release carries no STAMPS at all",
			fake:     []string{"GH_FAKE_NO_STAMPS=1"},
			says:     "has no STAMPS asset",
			alsoSays: "take a later one",
		},
		{
			// One tool missing from a STAMPS that is otherwise whole, refused for the same all-or-nothing
			// reason: the one tool nobody could be told about is the one you would never think to check.
			name: "STAMPS omits one of the tools",
			fake: []string{"GH_FAKE_UNSTAMPED=bravo"},
			says: "records no source stamp for bravo",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			sandbox := newSandbox(t)
			checkout := newCheckout(t, sandbox, "https://github.com/pinned/target.git")
			refused, _ := install(t, checkout, nil, append([]string{"GH_FAKE_DOWNLOAD=serve"}, scenario.fake...)...)

			expectRefusal(t, refused, scenario.says)
			if scenario.alsoSays != "" && !refused.said(scenario.alsoSays) {
				// gh's own reason is carried into the refusal rather than left to a re-run: the asset sits in
				// a staging directory the EXIT trap deletes, so running the command again reports a missing
				// file, and gh's reason is the only thing telling an unattested binary from an offline machine.
				t.Errorf("the refusal does not carry %q\n%v", scenario.alsoSays, refused)
			}
			if scenario.neverSays != "" && refused.said(scenario.neverSays) {
				t.Errorf("the refusal says %q, so something other than the check this case names refused "+
					"it\n%v", scenario.neverSays, refused)
			}
			expectNothingInstalled(t, checkout)
		})
	}
}

// One run of install.sh in a fixture checkout, with the fake gh on PATH, and every line of argv that
// fake was called with.
func install(t *testing.T, checkout string, arguments []string, fake ...string) (outcome, []string) {
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
	ran := launch(t, command)

	body, err := os.ReadFile(log)
	if err != nil {
		return ran, nil
	}
	return ran, strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
}

// A refusal is all-or-nothing, so each one has to show bin/ empty. Named, because a bare directory
// listing in a case reads as a listing rather than as the claim the case is making.
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

// The one line of a refusal carrying the wording, so a case can ask what else that same line says.
func refusalLine(refused outcome, wording string) string {
	for _, line := range strings.Split(refused.stdout+"\n"+refused.stderr, "\n") {
		if strings.Contains(line, wording) {
			return line
		}
	}
	return ""
}
