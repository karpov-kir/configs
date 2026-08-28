#!/usr/bin/env bash
# Cases for ruleecho.sh, covering what is specific to rule-echo: that it really reads a tree and finds
# a restatement in it. The one that must not be weakened is "runs from an unrelated cwd": that is the
# defect the wrapper exists for, and it fails silently — the scan reports nothing and nothing
# distinguishes that from a tree with nothing to report.
#
# How the wrapper reaches its binary is not tested here. That body is the shared tool-stub region, and
# its own cases are in tool-stub-test.sh; every way the resolver behind it can fail to produce a
# binary is in resolve-test.sh, which builds fixtures for each branch instead of depending on what
# this machine happens to have installed.
set -u

here=$(cd "$(dirname "$0")" && pwd)
script="$here/ruleecho.sh"
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

# A root holding one restatement, so a run that works reports exactly one pair and a run that silently
# did nothing reports zero. Without content to find, every case below would pass on a broken wrapper.
root="$base/root"
mkdir -p "$root/a" "$root/b"
rule='a shared rule stating several discriminating words plainly'
printf '# A\n\n**%s**\n' "$rule" >"$root/a/one.md"
printf '# B\n\n**%s**\n' "$rule" >"$root/b/two.md"

expect_status() {
  local name="$1" want="$2"
  [ "$status" -eq "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "exit $status, wanted $want — output: $out"
}

expect_out() {
  local name="$1" want="$2"
  case "$out" in
    *"$want"*) record_pass "$name" ;;
    *) record_fail "$name" "wanted '$want' in: $out" ;;
  esac
}

echo "ruleecho.sh"

# The regression, driven from a directory that is neither the repo nor the root under scan.
out=$(CDPATH= cd "$base" && "$script" "$root" 2>&1)
status=$?
expect_status "runs from an unrelated cwd and finds the pair" 1
expect_out "and reports the restatement it was pointed at" "rule stated twice"

out=$("$script" "$root/a" 2>&1)
status=$?
expect_status "a root with nothing restated exits 0" 0

out=$("$script" 2>&1)
status=$?
expect_status "no root exits 2" 2

# A checkout that mounts the skill without shipping the tools directory: still exit 2, because exit 0
# with no pairs is what a clean tree looks like.
orphan="$base/orphan/skills/kk-ecosystem/scripts"
mkdir -p "$orphan"
cp "$script" "$orphan/ruleecho.sh"
chmod 755 "$orphan/ruleecho.sh"
out=$("$orphan/ruleecho.sh" "$root" 2>&1)
status=$?
expect_status "a checkout without the tool exits 2" 2
expect_out "and names what is missing" "does not ship ai/tools/"

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]
