// The fixture release: a checkout shaped like this repository, and a `gh` that answers what install.sh
// asked for rather than what GitHub would.
//
// The fake is one script for every case here. Its release listing answers by the repository it is asked
// about, so a single process can stand for a machine that is offline, a repository with no release and
// one with releases; everything else it does is chosen by the environment the case launches it in,
// because those cases are one process each anyway.
package reach

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fakeGh = `#!/usr/bin/env bash
set -u
printf '%s\n' "$*" >>"$GH_FAKE_LOG"

digest() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
  else
    sha256sum "$1" | cut -d' ' -f1
  fi
}

repo=""
previous=""
for argument in "$@"; do
  if [ "$previous" = "--repo" ]; then repo="$argument"; fi
  previous="$argument"
done

# The three answers a listing can give, chosen by the owner asked about. Two of them leave stdout empty
# — a repository with no release, and a listing that could not be read — so only the exit code separates
# them, and reading the second as the first tells an offline machine there is nothing to download.
if [ "${1:-} ${2:-}" = "release list" ]; then
  if [ "${repo%%/*}" = unreachable ]; then exit 1; fi
  if [ "${repo%%/*}" = no-release ]; then exit 0; fi
  printf 'v1.0.0\n'
  exit 0
fi

if [ "${1:-} ${2:-}" = "release download" ]; then
  [ "${GH_FAKE_DOWNLOAD:-fail}" = serve ] || exit 1
  dir=""
  previous=""
  for argument in "$@"; do
    if [ "$previous" = "--dir" ]; then dir="$argument"; fi
    previous="$argument"
  done
  [ -n "$dir" ] || exit 1
  for tool in $GH_FAKE_TOOLS; do
    printf 'fake %s binary\n' "$tool" >"$dir/$tool-$GH_FAKE_SUFFIX"
  done
  # A stamp a case can read back by eye. install.sh records whatever the release recorded and never reads
  # it, so a real digest here would only hide which tool's stamp landed where.
  if [ -z "${GH_FAKE_NO_STAMPS:-}" ]; then
    for tool in $GH_FAKE_TOOLS; do
      if [ "$tool" != "${GH_FAKE_UNSTAMPED:-}" ]; then
        printf 'stamp-for-%s  %s\n' "$tool" "$tool"
      fi
    done >"$dir/STAMPS"
  fi
  # Every file and not just the platform's assets, so STAMPS is covered the way the release workflow
  # covers it. The redirect creates SHA256SUMS before the glob runs, so the file has to skip itself.
  (
    cd "$dir" || exit 1
    for file in *; do
      if [ "$file" = SHA256SUMS ]; then continue; fi
      printf '%s  ./%s\n' "$(digest "$file")" "$file"
    done
  ) >"$dir/SHA256SUMS"
  exit 0
fi

if [ "${1:-} ${2:-}" = "attestation verify" ]; then
  unattested=" ${GH_FAKE_UNATTESTED:-} "
  asked=" $(basename "$3") "
  if [ "${unattested%"$asked"*}" != "$unattested" ]; then
    echo "stub-gh: no attestation matching the artifact was found" >&2
    exit 1
  fi
  exit 0
fi
exit 1
`

// A PATH directory holding the fake gh, to be put in front of this process's own: every case here is
// about what install.sh does with gh's answers, so nothing else on PATH is being modelled.
func newGhPath(t *testing.T, sandbox string) string {
	t.Helper()
	dir, err := os.MkdirTemp(sandbox, "gh-")
	if err != nil {
		t.Fatalf("building the gh fixture under %s: %v — nothing was measured", sandbox, err)
	}
	writeFile(t, filepath.Join(sandboxed(t, sandbox, dir), "gh"), fakeGh, 0o755)
	return dir
}

// A checkout shaped like this repository: the install.sh under test at ai/tools/, the workflow it reads
// its tool list out of, and an origin remote where the case wants one.
//
// The git directory is written rather than `git init`ed. What install.sh asks git for is one url, and
// four files answer it — a fixture that shells out to build itself is the cost this whole port is about.
func newCheckout(t *testing.T, sandbox, origin string) string {
	t.Helper()
	checkout, err := os.MkdirTemp(sandbox, "checkout-")
	if err != nil {
		t.Fatalf("building the checkout fixture under %s: %v — nothing was measured", sandbox, err)
	}
	sandboxed(t, sandbox, checkout)

	writeFile(t, filepath.Join(checkout, "ai", "tools", "install.sh"), read(t, runnable(t, installScript)), 0o755)
	writeFile(t, filepath.Join(checkout, ".github", "workflows", "release-tools.yml"),
		"jobs:\n  build:\n    env:\n      SHIPPED: "+strings.Join(fixtureTools, " ")+"\n", 0o644)

	writeFile(t, filepath.Join(checkout, ".git", "HEAD"), "ref: refs/heads/main\n", 0o644)
	config := "[core]\n\trepositoryformatversion = 0\n"
	if origin != "" {
		config += "[remote \"origin\"]\n\turl = " + origin + "\n"
	}
	writeFile(t, filepath.Join(checkout, ".git", "config"), config, 0o644)
	if err := os.MkdirAll(filepath.Join(checkout, ".git", "objects"), 0o755); err != nil {
		t.Fatalf("building the fixture git directory: %v — nothing was measured", err)
	}
	if err := os.MkdirAll(filepath.Join(checkout, ".git", "refs"), 0o755); err != nil {
		t.Fatalf("building the fixture git directory: %v — nothing was measured", err)
	}
	return checkout
}
