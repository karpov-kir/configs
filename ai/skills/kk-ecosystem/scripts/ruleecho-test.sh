#!/usr/bin/env bash
# Cases specific to rule-echo: it reads a real tree and finds a restatement in it. A broken wrapper
# reports no pairs, and so does a clean tree, so nothing here can rest on exit 0 alone.
# Never weaken "runs from an unrelated cwd". That is the defect the wrapper exists for.
# The wrapper's body is covered by tool-stub-test.sh, the resolver behind it by resolve-test.sh.
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

# The fixture has to really hold a restatement. With nothing to find, every case below passes on a
# broken wrapper.
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
