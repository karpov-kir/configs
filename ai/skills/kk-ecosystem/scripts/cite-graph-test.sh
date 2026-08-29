#!/usr/bin/env bash
# Cases specific to cite-graph: it reads a real tree and measures the shape of it. This tool never
# refuses a tree, so exit 0 is what a working run and a silently dead wrapper both look like — every
# case below pins a number the fixture really produces, never exit 0 on its own.
# Never weaken "runs from an unrelated cwd". That is the defect the wrapper exists for.
# Never weaken "a root holding no markdown exits 2". Three skills read exit 2 as "the measurement did
# not run"; collapsing it into the 0 a flat tree exits is what makes an unread tree read as flat.
# The wrapper's body is covered by tool-stub-test.sh, the resolver behind it by resolve-test.sh.
#
# No case here rests on a mode bit, and none may be added that does. CI's Linux runner is root, which
# reads and writes whatever the mode says, so a case built on `chmod 000` passes there having proven
# nothing. Every unreachable thing below is unreachable by construction: a path that does not exist,
# or a directory holding none of what the tool reads.
set -u

here=$(cd "$(dirname "$0")" && pwd)
script="$here/cite-graph.sh"
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

# Every citation-shaped string is assembled here at run time and never written as a literal line in
# this file. The wiring check scans each `.sh` under the root, this suite among them, so a citation
# spelled out here is read as one of the checkout's own and reported against it — from a case that
# passes. What reaches the temp tree is byte-identical to a real citation, which is the whole point:
# that string is what the tool under test has to find. `assert_no_citation_literals` at the end of
# this file is the mechanism holding the rule, so a later edit cannot quietly undo it.
arrow=$(printf '\342\206\222')

new_citation() {
  printf '`%s` %s **%s**' "$1" "$arrow" "$2"
}

new_link_citation() {
  printf '[%s](%s) %s **%s**' "$1" "$1" "$arrow" "$2"
}

new_chained_citation() {
  printf 'and %s **%s**' "$arrow" "$1"
}

# The fixture has to really hold each shape the tool reports. With a tree that measures to nothing,
# every case below passes on a wrapper that ran nothing.
#   The consumer cites the middle file, which cites the deep one: that is the two-hop chain DEPTH
#   reports. The consumer enters the deep file at two sections, so the deep file carries a
#   two-section door surface. The consumer also names the middle file whole, in a link, so it cites
#   that one precisely rather than entering it. The deep file cites back into the consumer, and that
#   is the cycle. Two headings are cited by nothing, and those are the unentered sections.
consumer_name=consumer.md
middle_name=middle.md
deep_name=deep.md
flat_name=plain.md
door_heading='Handing off'
first_heading='The last rule'
second_heading='A second door'
back_heading='Doing the work'
unentered_deep='Unreached corner'
unentered_middle='Nothing enters this'
unentered_flat='A section nobody cites'

root="$base/root"
mkdir -p "$root/flat"
{
  printf '# Consumer\n\n## %s\n\n' "$back_heading"
  printf 'Follow %s before you start.\n' "$(new_link_citation "$middle_name" "$door_heading")"
  printf 'Then apply %s %s.\n' "$(new_citation "$deep_name" "$first_heading")" "$(new_chained_citation "$second_heading")"
} >"$root/$consumer_name"
{
  printf '# Middle\n\n## %s\n\n' "$door_heading"
  printf 'Read %s when the handoff lands.\n\n' "$(new_citation "$deep_name" "$first_heading")"
  printf '## %s\n' "$unentered_middle"
} >"$root/$middle_name"
{
  printf '# Deep\n\n## %s\n\n' "$first_heading"
  printf 'Back to %s once the rule is applied.\n\n' "$(new_citation "$consumer_name" "$back_heading")"
  printf '## %s\n\nAnother rule.\n\n' "$second_heading"
  printf '## %s\n' "$unentered_deep"
} >"$root/$deep_name"
# A tree with a file but no citation between any: flat, and the tool says so at exit 0.
printf '# Plain\n\n## %s\n' "$unentered_flat" >"$root/flat/$flat_name"
# A directory that exists and holds content, none of it markdown. Nothing to measure is not flat.
mkdir -p "$base/no-markdown"
printf 'not markdown\n' >"$base/no-markdown/notes.txt"

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

# The report ends in a line of `<figure> <number>` pairs, and the number is the fact these cases are
# for. Everything above that line explains what a figure means, and an explanation gets rewritten —
# the DEPTH paragraph already has been. A case spelled against one of those sentences goes red when
# the sentence improves, so it teaches the next writer to leave the wording alone. Read the number by
# the name in front of it instead: that survives the rewrite and still goes red when the number moves.
# Take the last occurrence, because a lower-bound warning on stderr names depth too and the trailer
# is the end of the report.
# An absent figure fails, and never reads as zero. That is what keeps the flat-tree case from passing
# on a run that refused the tree instead of measuring it as flat.
expect_figure() {
  local name="$1" figure="$2" want="$3" tail got
  tail=${out##*"$figure "}
  if [ "$tail" = "$out" ]; then
    record_fail "$name" "no '$figure' figure in the report, so nothing measured it — output: $out"
    return
  fi
  got=${tail%%[!0-9]*}
  [ -n "$got" ] && [ "$got" = "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "$figure is '${got:-not a number}', wanted $want — output: $out"
}

echo "cite-graph.sh"

out=$(CDPATH= cd "$base" && "$script" "$root" 2>&1)
status=$?
expect_status "runs from an unrelated cwd and measures the tree" 0
expect_figure "and reports the chain depth the fixture holds" depth 2
expect_figure "and reports the widest door surface" "widest door surface" 2
expect_out "and counts a file named whole as a precision citer, not a door" "1 precision citer(s)"
expect_out "and names the section nothing enters" "$unentered_deep"
expect_out "and reports the cycle" "$(printf '%s %s %s %s %s' "$consumer_name" "$arrow" "$deep_name" "$arrow" "$consumer_name")"

# A root named outside this system: the wrapper hands its argument on whole, or it measures
# somewhere else and says so at exit 0.
cp -R "$root" "$base/rôot with spaces"
out=$("$script" "$base/rôot with spaces" 2>&1)
status=$?
expect_status "a root path holding a space and a non-ASCII character reaches the tool whole" 0
expect_figure "and measures that tree rather than another" depth 2

out=$("$script" "$root/flat" 2>&1)
status=$?
expect_status "a root with markdown but no citation exits 0" 0
expect_figure "and reports a flat tree, which is a measurement and not a refusal" depth 0

out=$("$script" "$base/no-markdown" 2>&1)
status=$?
expect_status "a root holding no markdown exits 2, never the 0 a flat tree exits" 2
expect_out "and says it read nothing rather than reporting a flat tree" "read nothing under"

out=$("$script" "$base/absent" 2>&1)
status=$?
expect_status "a root that does not exist exits 2" 2

out=$("$script" 2>&1)
status=$?
expect_status "no root exits 2" 2
expect_out "and prints the usage grammar" "usage: cite-graph <root>"

out=$("$script" "$root" "$root" 2>&1)
status=$?
expect_status "a second root exits 2 — this takes exactly one" 2

orphan="$base/orphan/skills/kk-ecosystem/scripts"
mkdir -p "$orphan"
cp "$script" "$orphan/cite-graph.sh"
chmod 755 "$orphan/cite-graph.sh"
out=$("$orphan/cite-graph.sh" "$root" 2>&1)
status=$?
expect_status "a checkout without the tool exits 2" 2
expect_out "and names what is missing" "does not ship ai/tools/"

# The guard on the rule the fixture above is built to keep. A later edit that spells a citation out
# here hands the wiring check a citation of this checkout's own, and the finding lands on the tree
# rather than on this file — so the suite goes on passing while the gate it is meant to sit under
# goes red. Matching what that check matches: a markdown filename and an arrow on one line.
assert_no_citation_literals() {
  local name="no citation is spelled out in this suite's own source"
  if grep -q "\.md.*$arrow" "$0"; then
    record_fail "$name" "assemble it from variables instead — $(grep -c "\.md.*$arrow" "$0") line(s) carry one"
  else
    record_pass "$name"
  fi
}
assert_no_citation_literals

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]
