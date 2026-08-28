#!/usr/bin/env bash
#
# Download the prebuilt tools for this machine from a GitHub release into ai/tools/bin/, which is
# where resolve.sh looks first. Run it once after cloning, and again after a release you want.
#
#   usage: install.sh [<tag>]   # <tag> defaults to the latest release
#
# This is what makes a Go toolchain optional: without it, resolve.sh falls back to building from
# source, and every machine that mounts these skills would need Go. With it, nothing but `gh` is
# needed.
#
# Verifies each download against the release's own SHA256SUMS before it becomes executable. A binary
# that fails the check is deleted rather than installed — these run over the human's repositories.
#
# tested by: install-test.sh, which covers the platform mapping, the tool list, and the refusals.
# untested: the download itself, which is a `gh release download` against a real release. Faking gh
# would only assert the fake, so run it and read what lands in bin/.
set -uo pipefail

# The tools a release carries. Read out of the workflow that builds them rather than restated here,
# so this cannot drift from what is actually published.
shipped_tools() {
  local workflow="$1"
  sed -n 's/^[[:space:]]*SHIPPED:[[:space:]]*//p' "$workflow" | tr ' ' '\n' | sed '/^$/d'
}

# The `<os>-<arch>` half of an asset name, from uname. Prints nothing and fails on anything the
# release does not carry, because guessing here installs a binary that cannot execute.
asset_suffix() {
  local kernel="$1" machine="$2" goos="" goarch=""
  case "$kernel" in
    Darwin) goos=darwin ;;
    Linux) goos=linux ;;
    *) return 1 ;;
  esac
  case "$machine" in
    arm64 | aarch64) goarch=arm64 ;;
    x86_64 | amd64) goarch=amd64 ;;
    *) return 1 ;;
  esac
  printf '%s-%s\n' "$goos" "$goarch"
}

# Whichever SHA-256 tool this machine has. macOS ships shasum, most Linux images ship sha256sum.
sha256_of() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    return 1
  fi
}

# The hash SHA256SUMS records for a basename. The file is written with `sha256sum ./*`, so the names
# in it carry a `./` prefix; matched on the basename so it verifies wherever the assets landed.
recorded_sha256() {
  local sums="$1" name="$2"
  awk -v want="$name" '{ path = $2; sub(/^\.\//, "", path); if (path == want) print $1 }' "$sums"
}

# install-test.sh sources this file to reach the functions above, so sourcing stops here. Only a
# direct run downloads anything.
if [ "${BASH_SOURCE[0]}" != "${0}" ]; then
  return 0
fi

set -euo pipefail

die() {
  printf 'install.sh: %s\n' "$1" >&2
  exit 2
}

here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" ||
  die "cannot resolve my own directory, so nothing was installed"
workflow="$here/../../.github/workflows/release-tools.yml"
[ -f "$workflow" ] ||
  die "no release workflow at $workflow, so the tool list cannot be read — nothing was installed"

command -v gh >/dev/null 2>&1 ||
  die "gh is not installed, so nothing was downloaded. Install it, or build from source instead: cd $here && go build -o bin/<tool> ./<tool>/"

suffix="$(asset_suffix "$(uname -s)" "$(uname -m)")" ||
  die "no release binaries for $(uname -s)/$(uname -m) — build from source instead: cd $here && go build -o bin/<tool> ./<tool>/"

tag="${1:-}"
tools="$(shipped_tools "$workflow")"
[ -n "$tools" ] || die "the workflow at $workflow lists no tools, so there is nothing to install"

staging="$(mktemp -d)" || die "cannot create a staging directory"
trap 'rm -rf "$staging"' EXIT

# One `gh` call for every asset this platform needs plus the checksums, so a partial release fails
# here rather than half way through installing.
patterns=()
for tool in $tools; do
  patterns+=(--pattern "$tool-$suffix")
done
patterns+=(--pattern SHA256SUMS)

printf 'install.sh: downloading %s assets for %s\n' "$(printf '%s\n' "$tools" | wc -l | tr -d ' ')" "$suffix" >&2
if [ -n "$tag" ]; then
  gh release download "$tag" --dir "$staging" "${patterns[@]}" ||
    die "gh could not download the assets for release $tag — nothing was installed"
else
  gh release download --dir "$staging" "${patterns[@]}" ||
    die "gh could not download the assets for the latest release — nothing was installed"
fi

[ -f "$staging/SHA256SUMS" ] ||
  die "the release carries no SHA256SUMS, so nothing could be verified — nothing was installed"

mkdir -p "$here/bin" || die "cannot create $here/bin"

# Verified before anything is made executable, and every asset before any of them is installed: a
# half-installed set is worse than none, because resolve.sh prefers whatever binary it finds.
for tool in $tools; do
  asset="$staging/$tool-$suffix"
  [ -f "$asset" ] || die "the release carries no $tool-$suffix — nothing was installed"
  want="$(recorded_sha256 "$staging/SHA256SUMS" "$tool-$suffix")"
  [ -n "$want" ] || die "SHA256SUMS records no hash for $tool-$suffix — nothing was installed"
  got="$(sha256_of "$asset")" ||
    die "no shasum or sha256sum on this machine, so $tool-$suffix could not be verified — nothing was installed"
  [ "$want" = "$got" ] ||
    die "$tool-$suffix does not match its recorded hash — nothing was installed"
done

for tool in $tools; do
  chmod 755 "$staging/$tool-$suffix"
  mv -f "$staging/$tool-$suffix" "$here/bin/$tool" ||
    die "could not move $tool into $here/bin — the install is incomplete"
  printf 'install.sh: installed %s\n' "$here/bin/$tool"
done
