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
# that fails the check is deleted rather than installed: these run over the human's repositories.
#
# The release comes from this checkout's own `origin`, named to gh explicitly. Left to itself gh
# resolves a release against the *caller's* current directory, and this script runs from wherever the
# caller was standing — `../bootstrap.sh` invokes it without changing directory.
#
# tested by: install-test.sh
# untested: the download itself, which is a `gh release download` against a real release — faking gh
# would only assert the fake, so run it and read what lands in bin/. Its argv is faked, because which
# repository and tag this asks for is this script's decision rather than an answer from GitHub. What
# happens to an asset once it lands is this script's decision too, so the hash and attestation checks
# run against a faked download.
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

# The `<owner>/<repo>` a git remote URL names, or nothing. What the unnamed release resolves to is at
# the head of this file; inside someone else's checkout it means downloading their assets and the
# SHA256SUMS they wrote to match, so every hash verifies and `resolve.sh` then prefers those binaries
# over the source beside it for every stub exec afterwards. Parsed rather than passed through: the
# result becomes a `gh` argument, so anything that is not a plain owner/name pair of safe characters
# is refused.
origin_repo() {
  local url="$1" path owner name
  case "$url" in
    *://*)
      path="${url#*://}"
      path="${path#*@}"
      path="${path#*/}"
      ;;
    *:*) path="${url#*:}" ;;
    *) return 1 ;;
  esac
  path="${path%/}"
  path="${path%.git}"
  case "$path" in
    */*) ;;
    *) return 1 ;;
  esac
  owner="${path%%/*}"
  name="${path#*/}"
  case "$name" in
    */*) return 1 ;;
  esac
  # A dot segment is not a name. `https://github.com/../repo` parses to the pair `../repo`, which
  # reaches `gh --repo` as a path that walks out of the owner it names.
  case "$owner" in
    "" | -* | . | ..) return 1 ;;
  esac
  case "$name" in
    "" | -* | . | ..) return 1 ;;
  esac
  # One test over both halves, so neither can carry a shell or option character the other is checked
  # for. A `/` cannot appear here: the two were split on the only one.
  case "$owner$name" in
    *[!A-Za-z0-9._-]*) return 1 ;;
  esac
  printf '%s/%s\n' "$owner" "$name"
}

# A release tag reaches `gh release download` as its first positional argument, where a leading dash
# makes it an option instead — `--repo=someone/else` redirects the whole download. Refused rather
# than escaped, because there is no quoting that turns an option back into a tag.
#
# `..` is refused and `/` is not, because a tag reaches GitHub inside an API path and a dot segment
# there could walk back out of the repository `--repo` pinned — reinstating the cross-repo redirect,
# with the other repository's SHA256SUMS then verifying the other repository's binaries. Whether any
# edge really normalises those segments is untested here, and the refusal does not rest on it. What
# settles the shape is git: `git check-ref-format refs/tags/../../etc` exits 1 and so does `a..b`, so
# no tag that can exist is lost, while `refs/tags/release/2024.01` exits 0, so banning `/` would cost
# a real one.
is_safe_tag() {
  case "$1" in
    "" | -* | *..* | *[!A-Za-z0-9._/-]*) return 1 ;;
  esac
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
  die "gh is not installed, so nothing was downloaded. Install it, or build from source instead: $here/resolve.sh <tool>, which needs Go"

suffix="$(asset_suffix "$(uname -s)" "$(uname -m)")" ||
  die "no release binaries for $(uname -s)/$(uname -m) — build from source instead: $here/resolve.sh <tool>, which needs Go"

tag="${1:-}"
[ -z "$tag" ] || is_safe_tag "$tag" ||
  die "'$tag' is not a release tag — expected letters, digits, dots, dashes and slashes, with no leading dash and no '..' anywhere. Nothing was installed"

# A failure to derive this is a refusal, never a fall back to whatever `gh` would have guessed:
# guessing is the defect.
origin_url="$(git -C "$here" remote get-url origin 2>/dev/null)" ||
  die "$here is not a checkout with an 'origin' remote, so the release repository could not be determined — nothing was installed"
release_repo="$(origin_repo "$origin_url")" ||
  die "cannot read an owner/name pair out of origin's url '$origin_url', so the release repository could not be determined — nothing was installed"

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

printf 'install.sh: downloading %s assets for %s from %s\n' \
  "$(printf '%s\n' "$tools" | wc -l | tr -d ' ')" "$suffix" "$release_repo" >&2
if [ -n "$tag" ]; then
  gh release download --repo "$release_repo" "$tag" --dir "$staging" "${patterns[@]}" ||
    die "gh could not download the assets for release $tag of $release_repo — nothing was installed"
else
  gh release download --repo "$release_repo" --dir "$staging" "${patterns[@]}" ||
    die "gh could not download the assets for the latest release of $release_repo — nothing was installed"
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
  # The hash and the attestation answer different questions. The hash proves the asset matches the
  # SHA256SUMS shipped beside it, but whoever publishes a release publishes both, so an attacker's
  # assets verify against the attacker's own list. Only the attestation, signed by GitHub against the
  # release workflow's identity, says the binary came from here.
  #
  # `--repo` alone is not that identity: any workflow in the repository holding `id-token: write` can
  # sign for it. `--signer-workflow` pins the file, and its value is the literal upstream path rather
  # than one built from `$release_repo`. This is the trust anchor, so it must not move with whatever
  # remote the checkout happens to have. A fork cutting its own release is refused here and builds
  # from source through resolve.sh.
  #
  # gh's message goes into the refusal because there is no reading it afterwards: the asset sits in a
  # staging directory the EXIT trap deletes, so re-running the command would only report a missing file.
  attestation_error="$(gh attestation verify "$asset" --repo "$release_repo" \
    --signer-workflow karpov-kir/configs/.github/workflows/release-tools.yml 2>&1 >/dev/null)" ||
    die "$tool-$suffix carries no provenance attestation from $release_repo, or it could not be checked — nothing was installed. A release built before the release workflow attested, an offline or unauthenticated machine, and a substituted binary all land here; gh said: $attestation_error"
done

for tool in $tools; do
  chmod 755 "$staging/$tool-$suffix"
  mv -f "$staging/$tool-$suffix" "$here/bin/$tool" ||
    die "could not move $tool into $here/bin — the install is incomplete"
  printf 'install.sh: installed %s\n' "$here/bin/$tool"
done
