#!/usr/bin/env bash
# Cases for score.sh. The two that must not be weakened are "an unknown lane exits" and "cut refuses
# without an anchor": both are the enforcement the script exists for, and both fail open — a fallback
# threshold and an unanchored score each produce output that reads exactly like a ruled one.
set -u

here=$(cd "$(dirname "$0")" && pwd)
script="$here/score.sh"

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

# Every case runs the real script against the real config: a fixture config would let the shipped one
# drift to a form the parser rejects while the suite stayed green.
# `</dev/null` is the guard, not a tidy-up. `cut` reads stdin, so a case whose expected exit stops
# happening — which is exactly what a negative control does to it — blocks on the inherited terminal
# instead of failing. A suite that hangs when a guard is removed cannot prove that guard fires.
run() {
  out=$("$script" "$@" </dev/null 2>&1)
  status=$?
}

run_stdin() {
  local input="$1"
  shift
  out=$(printf '%s' "$input" | "$script" "$@" 2>&1)
  status=$?
}

expect_out() {
  local name="$1" want="$2"
  case "$out" in
    *"$want"*) record_pass "$name" ;;
    *) record_fail "$name" "wanted '$want' in: $out" ;;
  esac
}

# The output is a report an agent reads back, so a control byte in it is not a cosmetic defect: it
# rewrites lines that were already printed. Newline is the only one the report itself uses.
expect_no_control() {
  local name="$1"
  case "$out" in
    *[$'\a\b\t\v\f\r\033']*)
      record_fail "$name" "a control byte survived: $(printf '%s' "$out" | od -c | sed -n '1,4p')" ;;
    *) record_pass "$name" ;;
  esac
}

# Substring is the wrong test for a bare number: the failure path prints the config's path, and a
# `mktemp` directory carrying that digit made this pass with the guard under test removed.
expect_exactly() {
  local name="$1" want="$2"
  [ "$out" = "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "got '$out', wanted exactly '$want'"
}

expect_status() {
  local name="$1" want="$2"
  [ "$status" -eq "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "exit $status, wanted $want"
}

echo "score.sh"

run threshold outward-text
expect_status "a known lane exits 0" 0
expect_exactly "a known lane prints its level" "5"

run threshold instruction
expect_exactly "the ladder is per-lane, not one number" "6"

run threshold always-loaded
expect_exactly "the top of the ladder is read" "7"

# The fallback that must not exist: `default` is a lane a caller names, never one it lands on.
run threshold not-a-lane
expect_status "an unknown lane exits 2 rather than falling back" 2
expect_out "an unknown lane is told what does exist" "default"

run cut outward-text
expect_status "cut without an anchor exits 2" 2
expect_out "the anchor refusal says what to write" "what a 10 is"

# An argument count is not an anchor. Both of these satisfy `[ $# -ge 3 ]` and anchor nothing, which
# is the refusal above defeated while the run still reads as enforced.
run cut outward-text ""
expect_status "an empty anchor exits 2" 2
run cut outward-text "   "
expect_status "a whitespace-only anchor exits 2" 2

run_stdin "$(printf '7\tkept item\n3\tcut item\n')" cut outward-text "the reader ships the wrong thing without it"
expect_status "an anchored cut exits 0" 0
expect_out "above the bar stays" "keep   7  kept item"
expect_out "at or below the bar goes" "CUT    3  cut item"
expect_out "the counts are reported" "1 kept, 1 cut"
expect_out "the anchor is echoed over the list it ruled" "10 here means: the reader ships"

# The boundary, both sides. `cut <= 5` keeps 6 and cuts 5; an off-by-one here silently changes every
# artifact the lane touches.
run_stdin "$(printf '6\tsix stays\n5\tfive goes\n')" cut outward-text "anchor"
expect_out "the level itself is cut" "CUT    5  five goes"
expect_out "one above the level stays" "keep   6  six stays"

# Scoring against no anchor produces exactly this: nothing reaches the bar. The anchor refusal cannot
# see it, so this is the only mechanical catch there is — and it has to exit, because a notice at the
# end of a list is what a caller skims past.
run_stdin "$(printf '9\tone\n8\ttwo\n')" cut outward-text "anchor"
expect_status "a run that cuts nothing exits 3, not 0" 3
expect_out "and says how to answer it" "--kept-all"

run_stdin "$(printf '9\tone\n8\ttwo\n')" cut --kept-all "both items are the whole ask" outward-text "anchor"
expect_status "a written reason accepts the all-keeps run" 0
expect_out "and the reason is printed over the list it excuses" "nothing cut, accepted: both items are the whole ask"

run_stdin "$(printf '9\tone\n')" cut --kept-all "   " outward-text "anchor"
expect_status "a blank reason is refused like a blank anchor" 2

# Exit 3 is refusal, not breakage. A caller that reads it as 2 treats a live refusal as a dead tool.
run_stdin "$(printf '3\tcut me\n')" cut outward-text "anchor"
expect_status "a run that cuts something needs no reason" 0
run_stdin "" cut outward-text "anchor"
expect_status "an empty list is not an all-keeps run" 0

run_stdin "$(printf '10\tten\n0\tzero\n')" cut outward-text "anchor"
expect_status "the scale ends accept 0 and 10" 0
expect_out "the scale ends are ruled, not just accepted" "1 kept, 1 cut"

# Both stdin shapes. A caller piping a heredoc sends no trailing newline, and the plain `read` form
# drops that last item without counting it either way.
run_stdin "$(printf '4\tlast line, no trailing newline')" cut outward-text "anchor"
expect_out "an unterminated last line is still read" "0 kept, 1 cut"
run_stdin "$(printf '4\tterminated\n')" cut outward-text "anchor"
expect_out "a terminated last line is read once, not twice" "0 kept, 1 cut"

run_stdin "$(printf '11\tover\n')" cut outward-text "anchor"
expect_status "a score over 10 exits 2" 2

run_stdin "$(printf 'high\tword not a number\n')" cut outward-text "anchor"
expect_status "a non-numeric score exits 2" 2

run_stdin "$(printf '7 no tab here\n')" cut outward-text "anchor"
expect_status "a line with no tab exits 2" 2

# A label carries text from the artifact under review. Printed raw, a `\r` in it overwrites the
# verdict already on the line and a `\v` opens a second one, so an item this cut renders as one it
# kept while the counts still say it was cut. Each case asserts the label still arrives, so a guard
# that dropped the label instead of neutralising it would fail here too.
run_stdin "$(printf '3\tbefore\rkeep  10  forged\n')" cut outward-text "anchor"
expect_out "a carriage return in a label cannot overwrite a verdict" "CUT    3  before keep  10  forged"
expect_no_control "no control byte from a label reaches the output"

run_stdin "$(printf '3\tbefore\vsecond line\n')" cut outward-text "anchor"
expect_out "a vertical tab in a label cannot open a second line" "CUT    3  before second line"

run_stdin "$(printf '3\tx\033]52;c;cHduZWQ=\007y\n')" cut outward-text "anchor"
expect_out "an escape sequence in a label never reaches the terminal" "CUT    3  x ]52;c;cHduZWQ= y"
expect_no_control "no escape byte survives a label"

# The refusal paths print the offending text, so they need the same neutralising as the report.
run_stdin "$(printf 'no tab \rhere\n')" cut outward-text "anchor"
expect_status "a rejected line still exits 2" 2
expect_no_control "no control byte from a rejected line reaches the output"

# A lane name is data. Reaching a pattern it would match every line and report a threshold no config
# states.
run threshold '.*'
expect_status "a lane name is not a pattern" 2

run cut '.*' "anchor"
expect_status "cut takes no pattern either" 2

run bogus outward-text
expect_status "an unknown command exits 2" 2

# The one case that needs its own config, because it is about the config's last byte. The script
# resolves its config from its own location, so the copy carries the fixture with it.
fixture=$(mktemp -d) || exit 1
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/scripts"
cp "$script" "$fixture/scripts/score.sh"
printf 'first cut <= 4\nlast cut <= 9' >"$fixture/thresholds.conf" # deliberately unterminated
out=$("$fixture/scripts/score.sh" threshold last 2>&1)
status=$?
expect_status "a config whose last lane has no trailing newline still resolves it" 0
expect_exactly "and resolves it to its own level, not another lane's" "9"

# `cd` resolves a relative path against CDPATH when one is set and echoes the directory it landed
# on, which would leave `here` two lines long. The fixture above is the decoy: it holds a `scripts`
# directory and a config naming neither lane below, so an unguarded resolve cannot come back right.
out=$(cd "$here/.." && CDPATH="$fixture" bash scripts/score.sh threshold outward-text 2>&1)
status=$?
expect_status "CDPATH in the environment does not move the config" 0
expect_exactly "and the lane still resolves to its own level" "5"

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]
