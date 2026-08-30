#!/usr/bin/env bash
# Cases for todo-gate.sh. Its caller's side — that report.sh refuses rather than reading a failed scan
# as clean — is pinned by eco-report's Go suite, which copies this script into each fixture and also
# stubs it to exit 3. What that cannot reach, and what this file is for, is the scan itself: fence and
# comment awareness, section attribution, and both refusals.
#
# The pairing that matters is each ignored-context case with its negative control: the same checkbox
# outside the fence has to be reported, or the case would pass against a script that found nothing
# anywhere.
set -u

here=$(CDPATH= cd -P "$(dirname "$0")" && pwd -P)
gate="$here/todo-gate.sh"
# Exit 2, and it says why: a fixture root that cannot be created is a suite that did not measure,
# which run-tests.sh counts apart from a failure. Exit 1 there would claim the script under test is
# broken — a different claim, and a false one (`~/.kk-flavor/standards/testing.md` -> **7. What a
# suite reports**).
base=$(mktemp -d) || {
  echo "todo-gate-test: could not create a temporary directory — nothing was tested" >&2
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

# Runs the gate over a file built from the given body, leaving $out and $status.
run_over() {
  local body="$1"
  local file="$base/case.md"
  printf '%s' "$body" >"$file"
  out=$("$gate" "$file" 2>&1)
  status=$?
}

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

expect_no_out() {
  local name="$1"
  [ -z "$out" ] &&
    record_pass "$name" ||
    record_fail "$name" "expected no output, got: $out"
}

echo "todo-gate.sh"

# The two ways it cannot run. Both print to stderr and exit 2, and 2 prints no items, so a caller
# reading it as clean is the failure this guards.
out=$("$gate" 2>&1)
status=$?
expect_status "no argument exits 2" 2
expect_out "and prints its usage" "usage:"

out=$("$gate" "$base/absent.md" 2>&1)
status=$?
expect_status "a file that is not there exits 2" 2
expect_out "and names the file" "no such file"

# The negative control for every ignored-context case below: this exact item, in the open, is found.
item='- [ ] wire the resolver into the skills'
run_over "## Follow-ups

$item
"
expect_status "an open item exits 1" 1
expect_out "and prints the item" "wire the resolver into the skills"
expect_out "and attributes it to its section" "## Follow-ups"

run_over "## Follow-ups

- [x] this one is done
"
expect_status "a checked item alone exits 0" 0
expect_no_out "and prints nothing"

run_over "## Follow-ups

nothing open here
"
expect_status "a file with no checkboxes exits 0" 0

# Fence awareness, both fence characters. An example checkbox in a code block is documentation, and
# reading it as a real TODO would block a merge on prose.
run_over "## Report

\`\`\`
$item
\`\`\`
"
expect_status "an item inside a backtick fence exits 0" 0
expect_no_out "and is not reported"

run_over "## Report

~~~
$item
~~~
"
expect_status "an item inside a tilde fence exits 0" 0
expect_no_out "and is not reported either"

# An unclosed fence swallows the rest of the file, which is the shape that hides a real TODO behind
# an example one. Pinned as the behaviour it has, so a change to it is a decision rather than a
# surprise.
run_over "## Report

\`\`\`
$item
"
expect_status "an unclosed fence hides what follows it" 0

# Comment awareness, including the multi-line form: a commented-out item is not an open one.
run_over "## Report

<!--
$item
-->
"
expect_status "an item inside an HTML comment exits 0" 0
expect_no_out "and is not reported"

# A commented item and a real one together. This is the case that fails if the comment state is never
# cleared: the real item after the comment closes has to still be found.
run_over "## Report

<!--
- [ ] an example, not a real item
-->

$item
"
expect_status "a real item after a closed comment is still found" 1
expect_out "and it is the one outside the comment" "wire the resolver into the skills"
case "$out" in
  *"an example, not a real item"*)
    record_fail "and the commented one is not reported" "the commented item appears in: $out" ;;
  *) record_pass "and the commented one is not reported" ;;
esac

# Several items under different headings: each carries the section it sits under, not the first one.
run_over "## First

- [ ] item in the first section

## Second

- [ ] item in the second section
"
expect_status "two items in two sections exit 1" 1
expect_out "and the first carries its own heading" "## First | - [ ] item in the first section"
expect_out "and the second carries its own heading" "## Second | - [ ] item in the second section"

# Indented items are still items: a nested checkbox under a bullet is how a sub-task is written.
run_over "## Follow-ups

- something
  $item
"
expect_status "an indented item exits 1" 1
expect_out "and is printed without its indentation" "- [ ] wire the resolver into the skills"

# A path awk would read as a variable assignment rather than a file. Handed to awk as an operand,
# `intent=v2.md` sets `intent` and opens nothing; awk then falls through to stdin, takes EOF from the
# `/dev/null` an agent hands it, prints nothing and exits 0 — a clean merge gate over a file it never
# opened. Relative and identifier-shaped is the whole condition: an absolute path is not a valid awk
# name, which is why every case above misses this.
#
# `</dev/null` is load-bearing here: without it the defect blocks on the terminal instead of reporting
# a false clean, and the case could not tell the two apart.
assignment_dir="$base/assignment"
mkdir -p "$assignment_dir"
printf '## Follow-ups\n\n%s\n' "$item" >"$assignment_dir/intent=v2.md"
out=$(cd "$assignment_dir" && "$gate" "intent=v2.md" </dev/null 2>&1)
status=$?
expect_status "a relative path shaped like an awk assignment is still opened" 1
expect_out "and its open item is reported" "wire the resolver into the skills"

# The control, and the reason the case above is not enough alone: the same file reached by a path awk
# could never misread has to behave identically. Without it, a gate that had stopped reading anything
# at all would satisfy neither, and a gate that reported every path as an item would satisfy both.
out=$(cd "$assignment_dir" && "$gate" "./intent=v2.md" </dev/null 2>&1)
status=$?
expect_status "and the same file reached by an unambiguous path agrees" 1
expect_out "and reports the same item" "wire the resolver into the skills"

# The other half of the shape: an identifier-shaped path holding nothing open must still exit 0, so
# the fix cannot be a gate that reports an item for every path it cannot parse.
printf '## Follow-ups\n\n- [x] done\n' >"$assignment_dir/intent=v3.md"
out=$(cd "$assignment_dir" && "$gate" "intent=v3.md" </dev/null 2>&1)
status=$?
expect_status "an identifier-shaped path with nothing open exits 0" 0
expect_no_out "and prints nothing"

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]
