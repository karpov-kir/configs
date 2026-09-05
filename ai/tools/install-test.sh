#!/usr/bin/env bash
# Cases for install.sh's decisions: which asset this machine needs, which tools a release carries,
# which hash each one is checked against, and whether it carries provenance. A faked `gh` answers
# what this script asked for; what a real `gh release download` returns is not faked, and install.sh's
# header says why.
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
# --- shared:expect-refusal ---
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
# --- end shared:expect-refusal ---

# The refusals below are all-or-nothing, so each one has to show that bin/ stayed empty. Named once:
# the raw `ls -A` reads as a directory listing rather than as the claim the case is making.
expect_nothing_installed() { # <name>, over $bin
  if [ -n "$(ls -A "$bin" 2>/dev/null)" ]; then
    record_fail "$1" "bin/ holds $(ls -A "$bin" | tr '\n' ' ')"
  else
    record_pass "$1"
  fi
}

# bash 3.2 — macOS's own, and one leg of the gates workflow — mis-parses a `case` written inline in a
# `$( )`, ending the substitution at the first pattern's `)`. A needle test that has to run inside one
# goes through a function instead.
held() { # <needle> <text> — 'held' when the text holds the needle, the whole text when it does not
  case "$2" in
    *"$1"*) printf 'held' ;;
    *) printf '%s' "$2" ;;
  esac
}

lacked() { # <needle> <text> — 'lacked' when the text is free of the needle, the whole text otherwise
  case "$2" in
    *"$1"*) printf '%s' "$2" ;;
    *) printf 'lacked' ;;
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

# `gh` resolves a release against the current directory's repository when it is not told which one,
# and this script runs from wherever the caller stands — so the repository has to come out of this
# checkout's own origin. Every form a GitHub remote is written in, and every shape that must not
# become a `gh` argument.
expect_eq "reads owner/name out of an https remote" "$(origin_repo https://github.com/kk/configs.git)" kk/configs
expect_eq "and out of one without the .git suffix" "$(origin_repo https://github.com/kk/configs)" kk/configs
expect_eq "and out of an scp-style ssh remote" "$(origin_repo git@github.com:kk/configs.git)" kk/configs
expect_eq "and out of an ssh:// remote" "$(origin_repo ssh://git@github.com/kk/configs.git)" kk/configs
expect_eq "and out of one carrying a username" "$(origin_repo https://kk@github.com/kk/configs.git)" kk/configs
expect_eq "and out of one with a trailing slash" "$(origin_repo https://github.com/kk/configs/)" kk/configs

# The controls. Each of these would otherwise reach `gh` as an argument, and a refusal that resolved
# to a near-miss is worse than one that fails: the fall back `gh` makes on its own is the defect.
# A dot segment passes the character class and is still not a name: `../repo` reaches `gh --repo` as
# a path that walks out of the owner it names.
for bad in "/home/me/configs" "https://github.com/kk" "https://github.com/kk/configs/extra" \
  "https://github.com/-kk/configs" "https://github.com/kk/con figs" "https://github.com/kk/con;figs" \
  "https://github.com//configs" "https://github.com/../repo" "https://github.com/kk/.." \
  "https://github.com/./repo" "https://github.com/kk/." "" ; do
  out=$(origin_repo "$bad")
  status=$?
  if [ "$status" -ne 0 ] && [ -z "$out" ]; then
    record_pass "refuses the remote url '$bad'"
  else
    record_fail "refuses the remote url '$bad'" "exit $status, printed '$out'"
  fi
done

# A tag reaches `gh release download` as its first positional argument, where a leading dash makes it
# an option instead. The accepted forms first, so the refusals below are not a function that says no
# to everything.
for good in v1.0.0 v1.2.3-rc.1 release/2024.01 1.0; do
  if is_safe_tag "$good"; then
    record_pass "accepts the tag '$good'"
  else
    record_fail "accepts the tag '$good'" "refused a tag a release really carries"
  fi
done
# `/` is legal in a tag and `..` is not, and git settles both: `git check-ref-format refs/tags/a..b`
# exits 1, so nothing below is a tag a release could carry, while `release/2024.01` above exits 0.
for bad in "--repo=evil/pwn" "-v1.0.0" "v1.0.0;id" "v1 0" "\$(id)" "" \
  "../../etc" "../../../evil/repo/releases/tags/v1" "v1.0.0/../../evil" "a..b"; do
  if is_safe_tag "$bad"; then
    record_fail "refuses the tag '$bad'" "accepted something that is not a tag"
  else
    record_pass "refuses the tag '$bad'"
  fi
done

# This machine can hash a file at all, or every verification would fail closed.
printf 'x' >"$base/one-byte"
digest=$(sha256_of "$base/one-byte")
if [ ${#digest} -eq 64 ]; then
  record_pass "hashes a file to 64 hex characters"
else
  record_fail "hashes a file to 64 hex characters" "got '$digest'"
fi

# The wiring, not the download. `origin_repo` returning the right pair proves nothing about whether
# the pair reaches `gh`, and that hand-off is the whole defect: without `--repo`, gh resolves the
# release against the caller's current directory. A `gh` that records its own argv and then fails is
# a fake at a true external edge, and what it asserts is what this script asked for rather than what
# GitHub would have answered — so nothing is downloaded and nothing about the network is claimed.
# A checkout shaped like this repository, holding the install.sh under test and the workflow it reads
# its tool list out of. The remote is each caller's own, because that is what the case is about.
new_checkout() { # <directory>
  mkdir -p "$1/ai/tools" "$1/.github/workflows"
  cp "$here/install.sh" "$1/ai/tools/install.sh"
  chmod 755 "$1/ai/tools/install.sh"
  cp "$workflow" "$1/.github/workflows/release-tools.yml"
}

pinned="$base/pinned"
new_checkout "$pinned"
( cd "$pinned" && git init -q . && git remote add origin https://github.com/pinned/target.git ) >/dev/null 2>&1

fake_gh="$base/fake-gh"
mkdir -p "$fake_gh"
cat >"$fake_gh/gh" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$@" >"$GH_ARGV_LOG"
exit 1
FAKE
chmod 755 "$fake_gh/gh"

argv_log="$base/gh-argv"
out=$(PATH="$fake_gh:$PATH" GH_ARGV_LOG="$argv_log" "$pinned/ai/tools/install.sh" 2>&1)
status=$?
expect_refusal "a download gh refuses exits 2" "could not download"
# The control the assertion below needs: without it, a gh that was never invoked leaves no log and
# the awk that reads it prints nothing, which compares equal to nothing and reads as a pass.
if [ -s "$argv_log" ]; then
  record_pass "control: gh really was invoked, so its argv is a measurement"
else
  record_fail "control: gh really was invoked, so its argv is a measurement" "nothing at $argv_log"
fi
expect_eq "the download is pinned to the origin of the checkout install.sh lives in" \
  "$(awk '$0 == "--repo" { getline; print; exit }' "$argv_log")" "pinned/target"

# And a tag that is really an option never reaches that call at all.
absent_log="$base/gh-argv-never"
out=$(PATH="$fake_gh:$PATH" GH_ARGV_LOG="$absent_log" "$pinned/ai/tools/install.sh" --repo=evil/pwn 2>&1)
status=$?
expect_refusal "a tag that is really an option exits 2" "is not a release tag"
if [ -e "$absent_log" ]; then
  record_fail "control: and gh was never reached" "gh ran anyway and wrote $absent_log"
else
  record_pass "control: and gh was never reached"
fi

# Nor does a tag carrying a dot segment. `--repo` pins the repository, and a tag reaches GitHub
# inside an API path: were the segments resolved there, the pin would be undone by the argument it
# was meant to constrain, and the assets would be verified against that other repository's own
# SHA256SUMS. Driven end to end rather than through is_safe_tag alone, because what matters is that
# the refusal happens before the download.
traversal_log="$base/gh-argv-traversal"
out=$(PATH="$fake_gh:$PATH" GH_ARGV_LOG="$traversal_log" \
  "$pinned/ai/tools/install.sh" ../../../evil/repo/releases/tags/v1 2>&1)
status=$?
expect_refusal "a tag that walks out of the pinned repository exits 2" "is not a release tag"
if [ -e "$traversal_log" ]; then
  record_fail "control: and that one reached no download" "gh ran anyway and wrote $traversal_log"
else
  record_pass "control: and that one reached no download"
fi

# A repository it cannot name is a refusal, never a fall back to whatever gh would have guessed —
# guessing is the whole defect, so a checkout that cannot answer must stop rather than downgrade to
# the behaviour this pinning replaced. Both ways the answer can be missing, each with its own fix.
unnamed() { # <label> <the remote to add, or nothing> <the wording only this cause produces>
  local label="$1" remote="$2" want="$3"
  local dir log
  dir="$base/unnamed-$label"
  log="$base/gh-argv-$label"
  new_checkout "$dir"
  (
    cd "$dir" && git init -q .
    if [ -n "$remote" ]; then git remote add origin "$remote"; fi
  ) >/dev/null 2>&1
  out=$(PATH="$fake_gh:$PATH" GH_ARGV_LOG="$log" "$dir/ai/tools/install.sh" 2>&1)
  status=$?
  expect_refusal "$label exits 2 rather than letting gh guess" "$want"
  if [ -e "$log" ]; then
    record_fail "control: $label reached no download" "gh ran anyway and wrote $log"
  else
    record_pass "control: $label reached no download"
  fi
}
unnamed "a checkout with no origin remote" "" "not a checkout with an 'origin' remote"
unnamed "an origin that names no owner/name pair" "/srv/mirrors/configs.git" "cannot read an owner/name pair"

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
# One glob over both halves. `resolve.sh` is in the refusal for an unsupported platform too, so alone
# it passes on that one and claims the gh refusal carries a way out when it may not.
case "$out" in
  *"gh is not installed"*"resolve.sh"*) record_pass "and names the way out that needs no gh" ;;
  *) record_fail "and names the way out that needs no gh" "output: $out" ;;
esac

# The provenance gate. Every case above stops at the download, because the gh they drive fails on its
# first call, so none of them reaches the attestation check and without what follows it is a changed
# behaviour nothing runs.
#
# A gh that answers the download for real and then answers the attestation the way each case asks.
# One line per invocation, because `--repo` reaches both calls and a case that only greps for it
# cannot tell which one it found.
release_gh="$base/release-gh"
mkdir -p "$release_gh"
# `if` rather than a `case` on $1: eco-check reads a column-0 `case "$1…"` in any .sh file as a
# subcommand dispatch and asks the tree for a call site per arm, which a stub inside a heredoc can
# never have.
cat >"$release_gh/gh" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$GH_CALL_LOG"
if [ "$1 $2" = "release download" ]; then
  dir="" prev=""
  for arg in "$@"; do
    if [ "$prev" = "--dir" ]; then dir="$arg"; fi
    prev="$arg"
  done
  [ -n "$dir" ] || exit 1
  for tool in $GH_STUB_TOOLS; do
    printf 'fake %s binary\n' "$tool" >"$dir/$tool-$GH_STUB_SUFFIX"
  done
  # A stamp the case can read back by eye. install.sh records whatever the release recorded and never
  # reads it, so a real digest here would only hide which tool's stamp landed where.
  if [ -z "${GH_STUB_NO_STAMPS:-}" ]; then
    for tool in $GH_STUB_TOOLS; do
      if [ "$tool" != "${GH_STUB_UNSTAMPED_TOOL:-}" ]; then
        printf 'stamp-for-%s  %s\n' "$tool" "$tool"
      fi
    done >"$dir/STAMPS"
  fi
  # `*` and not `*-$GH_STUB_SUFFIX`, so STAMPS is covered the way the release workflow covers it. The
  # redirect creates SHA256SUMS before the glob runs, so the file has to skip itself.
  (
    cd "$dir" || exit 1
    for file in *; do
      if [ "$file" = SHA256SUMS ]; then continue; fi
      if command -v shasum >/dev/null 2>&1; then
        printf '%s  ./%s\n' "$(shasum -a 256 "$file" | cut -d' ' -f1)" "$file"
      else
        printf '%s  ./%s\n' "$(sha256sum "$file" | cut -d' ' -f1)" "$file"
      fi
    done
  ) >"$dir/SHA256SUMS"
  exit 0
fi
if [ "$1 $2" = "attestation verify" ]; then
  unattested=" ${GH_STUB_UNATTESTED:-} "
  asked=" $(basename "$3") "
  if [ "${unattested%"$asked"*}" != "$unattested" ]; then
    echo "stub-gh: no attestation matching the artifact was found" >&2
    exit 1
  fi
  exit 0
fi
exit 1
FAKE
chmod 755 "$release_gh/gh"

stub_suffix="$(asset_suffix "$(uname -s)" "$(uname -m)")"

# A fresh checkout per case, so "bin/ holds nothing" is this run's answer and not the previous one's.
# The two GH_STUB_* knobs a case wants are set on the call itself — `release_no_stamps=1 release_run
# …` — because bash restores a variable assigned in front of a function call once that call returns.
# Set-then-reset around the call would leave one forgotten line able to run a later case under this
# one's stub, and what that corrupts is the "nothing was installed" control, which reads as a pass
# when the fixture is wrong.
release_run() { # <label> <the asset basenames with no attestation>, over $out, $status, $bin, $log
  local label="$1" unattested="$2" dir
  dir="$base/release-$label"
  bin="$dir/ai/tools/bin"
  log="$base/gh-calls-$label"
  new_checkout "$dir"
  (cd "$dir" && git init -q . && git remote add origin https://github.com/pinned/target.git) >/dev/null 2>&1
  out=$(PATH="$release_gh:$PATH" GH_CALL_LOG="$log" GH_STUB_TOOLS="$tools" \
    GH_STUB_SUFFIX="$stub_suffix" GH_STUB_UNATTESTED="$unattested" \
    GH_STUB_NO_STAMPS="${release_no_stamps:-}" GH_STUB_UNSTAMPED_TOOL="${release_unstamped_tool:-}" \
    "$dir/ai/tools/install.sh" 2>&1)
  status=$?
}

# The control the refusal below needs. Without a run that installs, "nothing was installed" passes on
# any fixture broken enough to stop the script early, and the case name would still claim provenance.
release_run attested ""
if [ "$status" -eq 0 ]; then
  record_pass "a release whose assets all carry an attestation installs"
else
  record_fail "a release whose assets all carry an attestation installs" "exit $status — output: $out"
fi
installed=""
for tool in $tools; do
  [ -x "$bin/$tool" ] || installed="$installed $tool"
done
expect_eq "and every shipped tool lands in bin/ executable" "$installed" ""
# And the check really ran on that green run: one call per tool, each pinned to the same repository
# the download was. A refusal proves nothing about a check that was never reached.
#
# The whole argv, anchored at both ends, because `--repo` alone leaves the signer unpinned — every
# workflow in the repository that can request an id-token signs attestations gh would accept.
# `--signer-workflow` is matched literally, so dropping it goes red here.
# One per asset rather than one per tool: STAMPS decides, on every run afterwards, whether a binary is
# reported as built from the source beside it, so an unverified STAMPS is a stale binary nobody is
# ever warned about.
expect_eq "control: one attestation check per asset, on the run that passed" \
  "$(grep -c "^attestation verify .* --repo pinned/target --signer-workflow karpov-kir/configs/\.github/workflows/release-tools\.yml$" "$log")" \
  "$(($(printf '%s\n' "$tools" | wc -l) + 1))"

# The stamp the release recorded, landing beside the binary it belongs to. Without it a release
# install has nothing to hold its binaries against, and resolve.sh reports every one of them as
# unknown for as long as it is installed.
mislaid=""
for tool in $tools; do
  [ "$(cat "$bin/$tool.stamp" 2>/dev/null)" = "stamp-for-$tool" ] || mislaid="$mislaid $tool"
done
expect_eq "and each tool's own source stamp lands beside it" "$mislaid" ""

# One asset without an attestation, and the whole install stops — the same all-or-nothing the hash
# loop already had, because resolve.sh prefers whatever binary it finds.
unattested_tool="$(printf '%s\n' "$tools" | tail -1)"
release_run unattested "$unattested_tool-$stub_suffix"
expect_refusal "an asset with no attestation exits 2" "carries no provenance attestation"
expect_eq "and names the tool it refused" \
  "$(held "$unattested_tool-$stub_suffix carries no provenance" "$out")" "held"
# gh's own reason, carried into the refusal rather than left to a re-run: it is the only thing in the
# output that tells an unattested binary from an offline machine.
expect_eq "and carries gh's own reason, since the staging path it names is already deleted" \
  "$(held 'stub-gh: no attestation matching' "$out")" "held"
# The one that matters: refused *before* anything became executable.
expect_nothing_installed "control: and nothing was installed"
# The hash passed on that run, so the attestation is what refused it and not a broken fixture.
expect_eq "control: the hashes all matched, so it is provenance that refused" \
  "$(lacked 'does not match its recorded hash' "$out")" "lacked"

# A release cut before source stamps existed. Refused rather than installed unstamped, because an
# unstamped binary is one resolve.sh reports as unknown on every run, and a warning that never goes
# away is one that stops being read.
release_no_stamps=1 release_run no-stamps ""
# The needle is this refusal's own wording and not the loop's, which says "carries no STAMPS" for any
# asset that failed to arrive — with the check above deleted that loop still exits 2, so a case
# matching it would pass while claiming to cover a check nothing runs.
expect_refusal "a release carrying no STAMPS exits 2" "has no STAMPS asset"
expect_eq "and names a way to get the tools anyway" "$(held 'take a later one' "$out")" "held"
expect_nothing_installed "control: and nothing was installed"

# And one tool missing from a STAMPS that is otherwise whole. All or nothing, for the same reason the
# hash loop is: the one tool nobody could be told about is the one you would never think to check.
unstamped_tool="$(printf '%s\n' "$tools" | tail -1)"
release_unstamped_tool="$unstamped_tool" release_run one-unstamped ""
expect_refusal "a release whose STAMPS omits a tool exits 2" "records no source stamp"
expect_eq "and names the tool it could not stamp" \
  "$(held "records no source stamp for $unstamped_tool" "$out")" "held"
expect_nothing_installed "control: and that one installed nothing either"

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]
