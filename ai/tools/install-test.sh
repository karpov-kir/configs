#!/usr/bin/env bash
# Cases for install.sh's decisions: which asset this machine needs, which tools a release carries, and
# which hash each one is checked against. The download itself is a `gh` call against a real release
# and is not faked here; install.sh's header says why.
#
# The controls that make the rest mean something: asset_suffix must FAIL on a platform the release
# does not carry, and recorded_sha256 must return nothing for an absent name. Both would otherwise
# hand the installer a plausible-looking wrong answer, and a wrong asset installs a binary that
# cannot execute while a missing hash installs one that was never verified.
#
# Sourcing install.sh reaches the functions without downloading anything; the guard in that script is
# what makes sourcing safe.
set -uo pipefail
export LC_ALL=C

here="$(CDPATH= cd -P "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
# shellcheck source=./install.sh
. "$here/install.sh"

base=$(mktemp -d) || exit 1
trap 'rm -rf "$base"' EXIT

passed=0
failed=0

record_pass() {
  passed=$((passed + 1))
  echo "  pass  $1"
}

record_fail() {
  failed=$((failed + 1))
  echo "  FAIL  $1  — $2"
}

expect_eq() {
  local name="$1" got="$2" want="$3"
  [ "$got" = "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "got '$got', wanted '$want'"
}

# A refusal asserted on what it says, never on its exit code alone: every refusal install.sh has exits
# 2, so the code says one happened and never which — a case checking only the code passes on whatever
# the fixture broke first, while its name claims the cause. The needle has to be wording no other
# refusal shares. `command not found` is a stripped PATH killing the script before it reaches any check
# at all, which install.sh never writes and no case ever means.
expect_refusal() { # <name> <the wording only this cause produces>, over $status and $out
  local name="$1" want="$2"
  if [ "$status" -ne 2 ]; then
    record_fail "$name" "exit $status, wanted 2 — output: $out"
    return
  fi
  case "$out" in
    *"command not found"* | *": not found"*)
      record_fail "$name" "a missing command produced this refusal, not '$want' — output: $out" ;;
    *"$want"*) record_pass "$name" ;;
    *) record_fail "$name" "wanted '$want' in: $out" ;;
  esac
}

echo "install.sh"

# Every platform the release workflow builds, named the way uname names it on that platform.
expect_eq "Darwin arm64 takes the darwin-arm64 asset" "$(asset_suffix Darwin arm64)" darwin-arm64
expect_eq "Darwin x86_64 takes the darwin-amd64 asset" "$(asset_suffix Darwin x86_64)" darwin-amd64
expect_eq "Linux aarch64 takes the linux-arm64 asset" "$(asset_suffix Linux aarch64)" linux-arm64
expect_eq "Linux x86_64 takes the linux-amd64 asset" "$(asset_suffix Linux x86_64)" linux-amd64
expect_eq "Linux amd64 is accepted as well" "$(asset_suffix Linux amd64)" linux-amd64

# The controls. A release carries four targets and nothing else, so anything else has to fail rather
# than resolve to a near-miss.
for bad in "FreeBSD arm64" "Darwin i386" "Windows_NT x86_64" "Darwin ''"; do
  # shellcheck disable=SC2086
  set -- $bad
  out=$(asset_suffix "${1:-}" "${2:-}")
  status=$?
  if [ "$status" -ne 0 ] && [ -z "$out" ]; then
    record_pass "refuses $bad"
  else
    record_fail "refuses $bad" "exit $status, printed '$out'"
  fi
done

# This machine's own platform has to resolve, or the installer refuses every real run.
if asset_suffix "$(uname -s)" "$(uname -m)" >/dev/null; then
  record_pass "resolves the platform this suite is running on"
else
  record_fail "resolves the platform this suite is running on" "$(uname -s)/$(uname -m) is unsupported"
fi

# The tool list comes out of the workflow, so a release and an install always agree on it.
workflow="$here/../../.github/workflows/release-tools.yml"
tools=$(shipped_tools "$workflow")
count=$(printf '%s\n' "$tools" | wc -l | tr -d ' ')
if [ "$count" -ge 1 ] && [ -n "$tools" ]; then
  record_pass "reads $count tool(s) out of the release workflow"
else
  record_fail "reads the tools out of the release workflow" "got '$tools'"
fi

# Each name it reads has to be a directory that really builds, or the install asks for an asset no
# release carries. This is the pairing that catches a typo in the workflow's list.
missing=""
for tool in $tools; do
  [ -d "$here/$tool" ] || missing="$missing $tool"
done
expect_eq "and every one of them is a package in this module" "$missing" ""

# A workflow with no list yields nothing, rather than a single empty name the caller would treat as a
# tool called "".
printf 'jobs:\n  build:\n    runs-on: ubuntu-latest\n' >"$base/no-list.yml"
expect_eq "a workflow with no SHIPPED list yields nothing" "$(shipped_tools "$base/no-list.yml")" ""

# SHA256SUMS is written with `sha256sum ./*`, so the recorded names carry a ./ prefix.
sums="$base/SHA256SUMS"
{
  printf '%s  ./eco-check-darwin-arm64\n' aaaa1111
  printf '%s  ./eco-stats-linux-amd64\n' bbbb2222
  printf '%s  SHA256SUMS\n' cccc3333
} >"$sums"
expect_eq "finds a hash recorded with a ./ prefix" "$(recorded_sha256 "$sums" eco-check-darwin-arm64)" aaaa1111
expect_eq "finds one recorded without a prefix" "$(recorded_sha256 "$sums" SHA256SUMS)" cccc3333
expect_eq "returns nothing for a name the file does not record" "$(recorded_sha256 "$sums" eco-report-darwin-arm64)" ""
# A name that is a suffix of a recorded one must not match it: the comparison is on the whole name.
expect_eq "does not match on a partial name" "$(recorded_sha256 "$sums" darwin-arm64)" ""

# This machine can hash a file at all, or every verification would fail closed.
printf 'x' >"$base/one-byte"
digest=$(sha256_of "$base/one-byte")
if [ ${#digest} -eq 64 ]; then
  record_pass "hashes a file to 64 hex characters"
else
  record_fail "hashes a file to 64 hex characters" "got '$digest'"
fi

# The refusal a machine without gh gets. A PATH holding only what install.sh needs to reach its gh
# check: without `dirname` it dies at self-resolution instead, which exits 2 as well, so the case is
# asserted on the wording rather than the code.
bare="$base/bare-path"
mkdir -p "$bare"
for needed in bash dirname; do
  ln -s "$(command -v "$needed")" "$bare/$needed"
done
out=$(PATH="$bare" "$here/install.sh" 2>&1)
status=$?
expect_refusal "a machine without gh exits 2" "gh is not installed"
# One glob over both halves. `go build` on its own is in the refusal for an unsupported platform too,
# so alone it passes on that one and claims the gh refusal carries a way out when it may not.
case "$out" in
  *"gh is not installed"*"go build"*) record_pass "and names the way out that needs no gh" ;;
  *) record_fail "and names the way out that needs no gh" "output: $out" ;;
esac

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]
