#!/usr/bin/env bash
# Cases for todo-gate.sh. Its caller's side — that report.sh refuses rather than reading a failed scan
# as clean — is pinned by eco-report's Go suite, which copies this script into each fixture and also
# stubs it to exit 3. What that cannot reach, and what this file is for, is the scan itself: the
# fence and comment awareness this script's header carried as owed.
#
# The pairing that matters is each ignored-context case with its negative control: the same checkbox
# outside the fence has to be reported, or the case would pass against a script that found nothing
# anywhere.
set -u

here=$(CDPATH= cd -P "$(dirname "$0")" && pwd -P)
gate="$here/todo-gate.sh"
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

# Indented items are still items — a nested checkbox under a bullet is how a sub-task is written.
run_over "## Follow-ups

- something
  $item
"
expect_status "an indented item exits 1" 1
expect_out "and is printed without its indentation" "- [ ] wire the resolver into the skills"

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]
