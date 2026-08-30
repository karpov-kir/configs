#!/usr/bin/env bash
# Cases specific to rule-echo: it reads a real tree and finds a restatement in it. A broken wrapper
# reports no pairs, and so does a clean tree, so nothing here can rest on exit 0 alone.
# Never weaken "runs from an unrelated cwd". That is the defect the wrapper exists for.
# The wrapper's body is covered by tool-stub-test.sh, the resolver behind it by resolve-test.sh.
set -u

# `CDPATH=`: set in the environment, `cd` echoes where it landed whenever the path it is given is
# relative and not dot-led, so `here` comes back two lines long and `$script` names nothing. A suite
# is not covered by the script it covers: this guard is the harness's own, and nothing under test
# reaches it.
here=$(CDPATH= cd "$(dirname "$0")" && pwd)
script="$here/ruleecho.sh"
# Exit 2, and it says why: a fixture root that cannot be created is a suite that did not measure,
# which run-tests.sh counts apart from a failure. Exit 1 there would claim the script under test is
# broken — a different claim, and a false one (`~/.kk-flavor/standards/testing.md` -> **7. What a
# suite reports**).
base=$(mktemp -d) || {
  echo "ruleecho-test: could not create a temporary directory — nothing was tested" >&2
  exit 2
}
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

# The same property for this suite's own root, which no case about the script under test reaches.
# A corrupt `here` does not announce itself as one: it reports having measured something else.
#
# The resolve line is extracted from this file rather than written out again, so this reddens when
# the guard leaves the real line and not when the line is reworded. Driven by a relative invocation
# from the directory above, the only shape that consults CDPATH at all. The control is what stops an
# extraction that has stopped matching from letting the case below pass over an empty probe.
cdpath_line=$(grep -m1 '^here=' "$0")
if [ -n "$cdpath_line" ]; then
  record_pass "control: this suite's own root resolution was found, so the case below drives something"
else
  record_fail "control: this suite's own root resolution was found, so the case below drives something" "no 'here=' line in $0"
fi
mkdir -p "$base/cdpath-probe/scripts"
printf '#!/usr/bin/env bash\n%s\necho "$here"\n' "$cdpath_line" >"$base/cdpath-probe/scripts/probe.sh"
cdpath_lines=$( (cd "$base/cdpath-probe" && CDPATH=. bash scripts/probe.sh 2>/dev/null) | grep -c '')
if [ "$cdpath_lines" = "1" ]; then
  record_pass "CDPATH in the environment does not corrupt this suite's own root"
else
  record_fail "CDPATH in the environment does not corrupt this suite's own root" "the resolve line came back $cdpath_lines line(s) long"
fi

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]
