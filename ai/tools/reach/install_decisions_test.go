// The decisions install.sh makes before and around the download. The asset this machine needs, the
// repository and tag it asks for, the hash an asset is checked against, and whether a release exists.
//
// Each of these is a mapping, so every row of it is here. The near miss is the danger. A wrong asset
// installs a binary that cannot execute, and a missing hash installs one that was never verified. A
// remote or tag reaching `gh` as an option redirects the download to somebody else's release, whose
// SHA256SUMS then verifies their binaries.
//
// One process answers the lot. install.sh's sourcing guard makes that safe: sourcing reaches the
// functions, and only a direct run downloads anything. The shell suite spent a process per row.
package reach

import (
	"configs/ai/tools/runtest"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// One call into the sourced script, and what it has to answer.
type probe struct {
	name string
	call string
	args []string
	// What the call prints. A row that must refuse leaves this empty and sets refuses instead. A refusal
	// is a non-zero status with an empty stdout and stderr. A refusal that printed a plausible answer is
	// the near miss every one of these rows exists to catch.
	want    string
	refuses bool
}

func TestInstallReadsEveryAnswerTheWayTheReleaseWroteIt(t *testing.T) {
	t.Parallel()
	sandbox := runtest.Sandbox(t)
	probes := probeTable(t, sandbox)
	answered := ask(t, sandbox, probes)
	// A row the driver never reached is one whose absence reads exactly like a pass.
	if len(answered) != len(probes) {
		t.Fatalf("%d rows were asked and %d came back, so the table below is not the table that ran",
			len(probes), len(answered))
	}

	for _, asked := range probes {
		t.Run(asked.name, func(t *testing.T) {
			got := answered[asked.name]
			if asked.refuses {
				if got.Code == 0 || got.Stdout != "" {
					t.Errorf("%s %v was accepted, and what it resolves to reaches gh as an argument\n%v",
						asked.call, asked.args, got)
				}
				return
			}
			if got.Code != 0 || got.Stdout != asked.want {
				t.Errorf("%s %v answered %q at exit %d, wanted %q", asked.call, asked.args, got.Stdout,
					got.Code, asked.want)
			}
		})
	}
}

func probeTable(t *testing.T, sandbox string) []probe {
	t.Helper()
	// SHA256SUMS is written with `sha256sum ./*`, so the recorded names carry a ./ prefix. install.sh
	// strips that prefix, because the basename match is what lets an asset verify wherever it landed.
	manifest := filepath.Join(sandbox, "SHA256SUMS")
	runtest.WriteFile(t, manifest, "aaaa1111  ./eco-check-darwin-arm64\nbbbb2222  ./eco-stats-linux-amd64\ncccc3333  SHA256SUMS\n", 0o644)
	hashed := filepath.Join(sandbox, "one-byte")
	runtest.WriteFile(t, hashed, "x", 0o644)
	digest := sha256.Sum256([]byte("x"))
	// A workflow with no SHIPPED list has to yield an empty result. A single empty name would reach the
	// caller as a tool called "".
	noList := filepath.Join(sandbox, "no-list.yml")
	runtest.WriteFile(t, noList, "jobs:\n  build:\n    runs-on: ubuntu-latest\n", 0o644)

	probes := []probe{
		// Every platform the release workflow builds, named the way uname names it there.
		{name: "Darwin arm64 takes the darwin-arm64 asset", call: "asset_suffix", args: []string{"Darwin", "arm64"}, want: "darwin-arm64"},
		{name: "Darwin x86_64 takes the darwin-amd64 asset", call: "asset_suffix", args: []string{"Darwin", "x86_64"}, want: "darwin-amd64"},
		{name: "Linux aarch64 takes the linux-arm64 asset", call: "asset_suffix", args: []string{"Linux", "aarch64"}, want: "linux-arm64"},
		{name: "Linux x86_64 takes the linux-amd64 asset", call: "asset_suffix", args: []string{"Linux", "x86_64"}, want: "linux-amd64"},
		{name: "Linux amd64 is accepted as well", call: "asset_suffix", args: []string{"Linux", "amd64"}, want: "linux-amd64"},
		// The platform this suite is running on, which every case in install_test.go builds its fixture
		// release for. A guess here installs a binary that cannot execute.
		{name: "the platform this suite runs on resolves to its own asset", call: "this_machine", want: thisSuffix},

		// The hash a two-column manifest records for a name. install.sh reads both of the release's
		// manifests through this one function — SHA256SUMS over the assets, STAMPS over the source each
		// tool was built from.
		{name: "a hash recorded with a ./ prefix is found", call: "recorded_sha256", args: []string{manifest, "eco-check-darwin-arm64"}, want: "aaaa1111"},
		{name: "a hash recorded without one is found too", call: "recorded_sha256", args: []string{manifest, "SHA256SUMS"}, want: "cccc3333"},
		{name: "a name the manifest does not record answers nothing", call: "recorded_sha256", args: []string{manifest, "eco-report-darwin-arm64"}, want: ""},
		// A name that is a suffix of a recorded one must not match it: the comparison is on the whole name.
		{name: "a partial name matches nothing", call: "recorded_sha256", args: []string{manifest, "darwin-arm64"}, want: ""},

		{name: "a file hashes to its own SHA-256", call: "sha256_of", args: []string{hashed}, want: hex.EncodeToString(digest[:])},
		{name: "a workflow with no SHIPPED list yields nothing", call: "shipped_tools", args: []string{noList}, want: ""},

		// Whether the repository has cut a release at all. Two of the three answers leave gh's stdout
		// empty, so the exit code is all that separates them. An unreadable listing read as "none" tells
		// an offline machine the release has no tools to download.
		{name: "a listing with a release in it reads as some", call: "releases_state", args: []string{"pinned/target"}, want: "some"},
		{name: "an empty listing reads as none", call: "releases_state", args: []string{"no-release/target"}, want: "none"},
		{name: "a listing gh could not answer reads as unknown, never as none", call: "releases_state", args: []string{"unreachable/target"}, want: "unknown"},
	}

	// Every form a GitHub remote is written in.
	for _, accepted := range []string{
		"https://github.com/kk/configs.git",
		"https://github.com/kk/configs",
		"git@github.com:kk/configs.git",
		"ssh://git@github.com/kk/configs.git",
		"https://kk@github.com/kk/configs.git",
		"https://github.com/kk/configs/",
	} {
		probes = append(probes, probe{name: "origin_repo reads kk/configs out of " + accepted,
			call: "origin_repo", args: []string{accepted}, want: "kk/configs"})
	}
	// And every shape that must not become a `gh` argument. A dot segment passes the character class and
	// is still not a name: `../repo` reaches `gh --repo` as a path that walks out of the owner it names.
	for _, refused := range []string{
		"/home/me/configs", "https://github.com/kk", "https://github.com/kk/configs/extra",
		"https://github.com/-kk/configs", "https://github.com/kk/con figs", "https://github.com/kk/con;figs",
		"https://github.com//configs", "https://github.com/../repo", "https://github.com/kk/..",
		"https://github.com/./repo", "https://github.com/kk/.", "",
	} {
		probes = append(probes, probe{name: "origin_repo refuses the remote url " + strconv.Quote(refused),
			call: "origin_repo", args: []string{refused}, refuses: true})
	}

	// The tags a release really carries, first, so the refusals that follow cannot come from a function
	// that refuses everything.
	for _, accepted := range []string{"v1.0.0", "v1.2.3-rc.1", "release/2024.01", "1.0"} {
		probes = append(probes, probe{name: "is_safe_tag accepts the tag " + accepted,
			call: "is_safe_tag", args: []string{accepted}})
	}
	// `/` is legal in a tag and `..` is not. git decides both: `git check-ref-format refs/tags/a..b`
	// exits 1, and the same check on `release/2024.01` exits 0. No refused tag here is one a release
	// could carry.
	for _, refused := range []string{
		"--repo=evil/pwn", "-v1.0.0", "v1.0.0;id", "v1 0", "$(id)", "",
		"../../etc", "../../../evil/repo/releases/tags/v1", "v1.0.0/../../evil", "a..b",
	} {
		probes = append(probes, probe{name: "is_safe_tag refuses the tag " + strconv.Quote(refused),
			call: "is_safe_tag", args: []string{refused}, refuses: true})
	}
	return probes
}

// Sources install.sh once and runs every row through it, keyed by name. Each row's arity is written
// out as a field of its own, because `read` gives an empty field and an absent field the same way. One
// row asks about the empty string.
const probeDriver = `#!/usr/bin/env bash
set -uo pipefail
. "$1"

# The platform this machine really is, so a row can hold the installer to resolving it.
this_machine() {
  asset_suffix "$(uname -s)" "$(uname -m)"
}

while IFS=$'\t' read -r name arity call first second; do
  [ -n "$name" ] || continue
  if [ "$arity" = 0 ]; then
    answer="$("$call" 2>&1)"
  elif [ "$arity" = 1 ]; then
    answer="$("$call" "$first" 2>&1)"
  else
    answer="$("$call" "$first" "$second" 2>&1)"
  fi
  status=$?
  printf '%s\t%s\t%s\n' "$name" "$status" "$answer"
done <"$2"
`

func ask(t *testing.T, sandbox string, probes []probe) map[string]runtest.Run {
	t.Helper()
	var table strings.Builder
	for _, asked := range probes {
		fields := append([]string{asked.name, strconv.Itoa(len(asked.args)), asked.call}, asked.args...)
		table.WriteString(strings.Join(fields, "\t") + "\n")
	}
	rows := filepath.Join(sandbox, "probes")
	runtest.WriteFile(t, rows, table.String(), 0o644)

	driver := filepath.Join(sandbox, "probe-driver.sh")
	runtest.WriteFile(t, driver, probeDriver, 0o755)
	command := newLaunch(t, driver, newGhPath(t, sandbox)+":"+os.Getenv("PATH"),
		runtest.Runnable(t, installScript), rows)
	command.Env = append(command.Env, "GH_FAKE_LOG="+filepath.Join(sandbox, "gh-argv"))
	ran := runtest.Launch(t, command)
	if ran.Code != 0 {
		t.Fatalf("the driver that sources install.sh exited %d, so no row was answered\n%v", ran.Code, ran)
	}

	answered := map[string]runtest.Run{}
	for _, line := range strings.Split(strings.TrimSuffix(ran.Stdout, "\n"), "\n") {
		name, rest, held := strings.Cut(line, "\t")
		if !held {
			continue
		}
		status, said, _ := strings.Cut(rest, "\t")
		code, err := strconv.Atoi(status)
		if err != nil {
			t.Fatalf("the driver answered %q for %s, which is not a status", status, name)
		}
		answered[name] = runtest.Run{Stdout: said, Code: code}
	}
	return answered
}
