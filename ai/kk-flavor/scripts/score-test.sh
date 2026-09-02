#!/usr/bin/env bash
# Cases for score.sh. The two that must not be weakened are "an unknown lane exits" and "cut refuses
# without an anchor": both are the enforcement the script exists for, and both fail open. A fallback
# threshold and an unanchored score each produce output that reads exactly like a ruled one.
set -u

# `CDPATH=`: set in the environment, `cd` echoes where it landed whenever the path it is given is
# relative and not dot-led, so `here` comes back two lines long and `$script` names nothing.
#
# A suite is not covered by the script it covers: this guard is the harness's own, and the CDPATH case
# at the bottom of this file cannot reach it. Run relatively with CDPATH set, an unguarded `here`
# reddened all but thirteen cases — that one among them.
here=$(CDPATH= cd "$(dirname "$0")" && pwd)
script="$here/score.sh"

# One temp root for the whole suite, and one EXIT trap: a second `trap ... EXIT` added later replaces
# this one rather than adding to it, and leaks whatever the first had created.
# Exit 2, and it says why: a fixture root that cannot be created is a suite that did not measure,
# which run-tests.sh counts apart from a failure. Exit 1 there would claim the script under test is
# broken — a different claim, and a false one (`~/.kk-flavor/standards/testing.md` -> **7. What a
# suite reports**).
tmp=$(mktemp -d) || {
  echo "score-test: could not create a temporary directory — nothing was tested" >&2
  exit 2
}
trap 'rm -rf "$tmp"' EXIT

# The machine-local override resolves under XDG_CONFIG_HOME, so the suite pins that at a directory
# holding none. Left alone, every baseline level below moves with whatever this machine has in
# ~/.config/kk-flavor, and the tracked thresholds this suite protects go unchecked. Exported as well
# as named, because the fixture cases at the end run the script directly rather than through a helper.
no_override_home="$tmp/no-override"
export XDG_CONFIG_HOME="$no_override_home"
mkdir -p "$no_override_home"

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
# The config home leads, because it is what a case varies: the override-free one below, the built
# override, or a hostile path. Naming it at the call rather than exporting it per case keeps the
# threshold a case is judged against readable from the case.
#
# `</dev/null` is a guard: `cut` reads stdin, so without it a case whose expected exit stops happening
# blocks on the terminal instead of failing, and a suite that hangs cannot prove a guard fires.
run_with() {
  local config_home="$1"
  shift
  out=$(XDG_CONFIG_HOME="$config_home" "$script" "$@" </dev/null 2>&1)
  status=$?
}

run_stdin_with() {
  local config_home="$1" input="$2"
  shift 2
  out=$(printf '%s' "$input" | XDG_CONFIG_HOME="$config_home" "$script" "$@" 2>&1)
  status=$?
}

run() { run_with "$no_override_home" "$@"; }
run_stdin() { run_stdin_with "$no_override_home" "$@"; }

expect_out() {
  local name="$1" want="$2"
  case "$out" in
    *"$want"*) record_pass "$name" ;;
    *) record_fail "$name" "wanted '$want' in: $out" ;;
  esac
}

# A control byte in the output is not cosmetic: it rewrites lines already printed. Newline is the only
# one this report uses.
expect_no_control() {
  local name="$1"
  case "$out" in
    *[$'\a\b\t\v\f\r\033']*)
      record_fail "$name" "a control byte survived: $(printf '%s' "$out" | od -c | sed -n '1,4p')" ;;
    *) record_pass "$name" ;;
  esac
}

# Substring is the wrong test for a bare number: the failure path prints the config's path, and a
# `mktemp` directory carrying that digit passes a substring test with the guard removed.
expect_exactly() {
  local name="$1" want="$2"
  [ "$out" = "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "got '$out', wanted exactly '$want'"
}

# The absence of a whole line, not of a substring: the offending text is expected to survive
# neutralised, and what must not survive is its position at the start of a line a verdict is read from.
expect_not_out() {
  local name="$1" unwanted="$2"
  case "$out" in
    *"$unwanted"*) record_fail "$name" "found '$unwanted' in: $out" ;;
    *) record_pass "$name" ;;
  esac
}

# Anchored at the end, not a substring anywhere. The refusal prints the lane list last, so a lane
# appended to the config lands past what a substring test looks for and slips through.
expect_out_ends() {
  local name="$1" want="$2"
  [ "${out%"$want"}" != "$out" ] &&
    record_pass "$name" ||
    record_fail "$name" "wanted '$want' at the end of: $out"
}

expect_status() {
  local name="$1" want="$2"
  [ "$status" -eq "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "exit $status, wanted $want"
}

# Root reads a mode-000 file whatever its mode, so every permission-denied assertion below is false
# when the suite runs as root, which is what GitHub's Linux runners do. Reported as skipped rather
# than run: a case that cannot fail must never be counted among the ones that could, and a green line
# for a check that never applied is the failure this suite exists to make impossible.
skipped=0
record_skip() {
  skipped=$((skipped + 1))
  echo "  skip  $1  — $2"
}

can_deny_read() { [ "$(id -u)" -ne 0 ]; }

echo "score.sh"

run threshold outward-text
expect_status "a known lane exits 0" 0

tracked_lanes=(default reply outward-text report code-comment instruction always-loaded)

# Every lane in the tracked config. They all sit at the same level, so this block cannot tell a
# correct resolve from one that always returns the first line. The fixture case at the bottom does
# that.
for lane in "${tracked_lanes[@]}"; do
  run threshold "$lane"
  expect_exactly "the $lane lane is ruled" "7"
done

# The fallback that must not exist — `../thresholds.conf` states what `default` is at that lane.
run threshold not-a-lane
expect_status "an unknown lane exits 2 rather than falling back" 2
expect_out_ends "an unknown lane is told exactly which lanes exist" \
  "it lists: ${tracked_lanes[*]}"

run cut outward-text
expect_status "cut without an anchor exits 2" 2
expect_out "the anchor refusal says what to write" "what a 10 is"

# An argument count is not an anchor. Both of these satisfy it and anchor nothing, which is the
# refusal above defeated while the run still reads as enforced.
run cut outward-text ""
expect_status "an empty anchor exits 2" 2
run cut outward-text "   "
expect_status "a whitespace-only anchor exits 2" 2

run_stdin "$(printf '9\tkept item\n3\tcut item\n')" cut outward-text "the reader ships the wrong thing without it"
expect_status "an anchored cut exits 0" 0
expect_out "above the bar stays" "keep   9  kept item"
expect_out "at or below the bar goes" "CUT    3  cut item"
expect_out "the counts are reported" "1 kept, 1 cut"
expect_out "the anchor is echoed over the list it ruled" "10 here means: the reader ships"

# The boundary, both sides. `cut <= 7` keeps 8 and cuts 7; an off-by-one here silently changes every
# artifact the lane touches.
run_stdin "$(printf '8\teight stays\n7\tseven goes\n')" cut outward-text "anchor"
expect_out "the level itself is cut" "CUT    7  seven goes"
expect_out "one above the level stays" "keep   8  eight stays"

# Scoring against no anchor produces exactly this: nothing reaches the bar. The anchor refusal cannot
# see it, so this case is the only mechanical catch there is.
run_stdin "$(printf '9\tone\n8\ttwo\n')" cut outward-text "anchor"
expect_status "a run that cuts nothing exits 3, not 0" 3
expect_out "and says how to answer it" "--kept-all"

run_stdin "$(printf '9\tone\n8\ttwo\n')" cut --kept-all "both items are the whole ask" outward-text "anchor"
expect_status "a written reason accepts the all-keeps run" 0
expect_out "and the reason is printed over the list it excuses" "nothing cut, accepted: both items are the whole ask"

run_stdin "$(printf '9\tone\n')" cut --kept-all "   " outward-text "anchor"
expect_status "a blank reason is refused like a blank anchor" 2

run_stdin "$(printf '3\tcut me\n')" cut outward-text "anchor"
expect_status "a run that cuts something needs no reason" 0

# `0 kept, 0 cut` at exit 0 is the report that reads identically to a run that scored a real list and
# cut none of it, and it is the one this script must not produce: the scale never reached an artifact.
# Exit 2 rather than 3, because 3 says a result is being refused and there is no result to refuse.
run_stdin "" cut outward-text "anchor"
expect_status "an empty list exits 2 — nothing was scored" 2
expect_out "and says no line reached stdin" "nothing was scored"
expect_not_out "and prints no counts that would read as a clean run" "0 kept, 0 cut"

# Blank lines are skipped as items, so a list of them scores nothing either. Without this the skip
# above becomes a way to reach the vacuous report the case above closes.
run_stdin "$(printf '\n\n')" cut outward-text "anchor"
expect_status "a list of blank lines scores nothing and exits 2" 2

# `--kept-all` answers a list that was read and survived. It must not excuse one that never arrived,
# which is the vacuous report reached through the door built to accept a legitimate all-keeps run.
run_stdin "" cut --kept-all "the artifact is already tight" outward-text "anchor"
expect_status "--kept-all does not excuse a list that never arrived" 2

# Stdin closed, not merely empty. `read` returns without assigning `line`, so without the assignment
# before the loop `set -u` kills the run on the `[ -n "$line" ]` beside it: the caller gets bash's own
# unbound-variable error and exit 1, after a header that already reads like a report starting.
out=$(XDG_CONFIG_HOME="$no_override_home" "$script" cut outward-text "anchor" <&- 2>&1)
status=$?
expect_status "cut with stdin closed refuses rather than dying on bash's own error" 2
expect_out "and the refusal is the script's own, naming what did not arrive" "nothing was scored"
expect_not_out "and bash's unbound-variable error does not decide the exit" "unbound variable"

# A directory dups onto fd 0 happily and only fails at the read, so it reaches the same unset `line`
# by a route the closed-stdin case cannot see.
out=$(XDG_CONFIG_HOME="$no_override_home" "$script" cut outward-text "anchor" <"$tmp" 2>&1)
status=$?
expect_status "cut given a directory on stdin refuses too" 2

run_stdin "$(printf '10\tten\n0\tzero\n')" cut outward-text "anchor"
expect_status "the scale ends accept 0 and 10" 0
expect_out "the scale ends are ruled, not just accepted" "1 kept, 1 cut"

# Both stdin shapes: a heredoc sends no trailing newline, and the plain `read` form drops that item.
run_stdin "$(printf '4\tlast line, no trailing newline')" cut outward-text "anchor"
expect_out "an unterminated last line is still read" "0 kept, 1 cut"
run_stdin "$(printf '4\tterminated\n')" cut outward-text "anchor"
expect_out "a terminated last line is read once, not twice" "0 kept, 1 cut"

run_stdin "$(printf '11\tover\n')" cut outward-text "anchor"
expect_status "a score over 10 exits 2" 2
expect_out "and the refusal is the scale one" "over the 0-10 scale"

# The message, not only the exit. Remove the non-numeric guard and `[ high -le 10 ]` errors, the `||
# die` beside it catches that, and the exit is still 2 — so a status-only assertion passes while the
# refusal names the wrong cause and bash's own `integer expected` leaks beside it.
run_stdin "$(printf 'high\tword not a number\n')" cut outward-text "anchor"
expect_status "a non-numeric score exits 2" 2
expect_out "and says the score is not a number, not that it is out of range" "is not a score 0-10"
expect_not_out "and bash's own integer error does not leak" "integer expression expected"

run_stdin "$(printf '7 no tab here\n')" cut outward-text "anchor"
expect_status "a line with no tab exits 2" 2

# Five guards that were real but uncased, so removing any of them left the suite green. Each case
# below is the only one that reddens under its own guard's deletion; the earlier cases that look like
# they cover them are absorbed by a neighbouring guard instead. The no-tab line above goes through
# the non-numeric-score refusal, not the tab check.
run
expect_status "no arguments at all exits 2" 2

run threshold outward-text extra
expect_status "threshold with a second lane exits 2" 2

run_stdin "$(printf '7\n')" cut outward-text "anchor"
expect_status "a bare number with no tab exits 2" 2

run_stdin "$(printf '3\rX\tlabel\n')" cut outward-text "anchor"
expect_status "a control byte in the score field exits 2" 2
expect_no_control "no control byte from a score field reaches the output"

run_stdin "$(printf '3\tcut me\n\n4\talso cut\n')" cut outward-text "anchor"
expect_status "a blank line among items is skipped, not rejected" 0
expect_out "and the items around it are still counted" "0 kept, 2 cut"

# Each case asserts the label still arrives, so a guard that dropped a label instead of neutralising
# it would fail here too.
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

# The machine-local override is untracked by design, so these cases build one under a second
# XDG_CONFIG_HOME rather than writing to the developer's real one.
override_home="$tmp/with-override"
override_dir="$override_home/kk-flavor"
override_file="$override_dir/thresholds.conf"
mkdir -p "$override_dir"
write_override() { printf '%s\n' "$@" >"$override_file"; }

run_ovr() { run_with "$override_home" "$@"; }
run_stdin_ovr() { run_stdin_with "$override_home" "$@"; }

write_override 'outward-text cut <= 8'

run_ovr threshold outward-text
expect_status "an override resolves" 0
# Both numbers, not just the new one: a note saying only "8" cannot be checked against the bar the
# tracked file states.
expect_out "the note names both the ruled level and the one in effect" "7 ruled, 8 in effect"
expect_out "and names the file that moved it" "$override_file"

# The overlay is per lane. Whole-file replacement would detach every lane the override omits from the
# file that rules them, and the omission is invisible: the number still reads as ruled.
run_ovr threshold instruction
expect_exactly "a lane the override omits keeps its tracked level" "7"

# `threshold` mode's stdout is read straight back as the number, so the announcement must not be on
# it: concatenated, it becomes part of that number. Exact and stderr discarded, because substring
# would pass on the note alone.
out=$(XDG_CONFIG_HOME="$override_home" "$script" threshold outward-text 2>/dev/null)
expect_exactly "the announcement stays off threshold's stdout" "8"

# The bar has to move, not merely be reported as moved. Without this the note could be right while
# every item was still judged against the tracked number.
run_stdin_ovr "$(printf '8\teight goes\n9\tnine stays\n')" cut outward-text "anchor"
expect_status "an overridden cut exits 0" 0
expect_out "the overridden level itself is cut" "CUT    8  eight goes"
expect_out "one above the overridden level stays" "keep   9  nine stays"

# Under `cut` the note goes the other way, into the report on stdout: a caller piping the report to a
# file keeps stdout and loses stderr, and the bar belongs beside the verdict it ruled.
out=$(printf '8\tx\n' | XDG_CONFIG_HOME="$override_home" "$script" cut outward-text "anchor" 2>/dev/null)
expect_out "cut carries the override in the report body, not only on stderr" "7 ruled, 8 in effect"

# The hole this closes: a typo in the override (`instructions` for `instruction`) would tune nothing
# at all, and quietly. Not the threshold a caller gets — that lane is never the one returned anyway.
write_override 'instructions cut <= 2'
run_ovr threshold instruction
expect_status "an override naming a lane the tracked config does not rule exits 2" 2
expect_out "and says an override cannot add one" "never adds one"

# Malformed in the override is as fatal as malformed in the tracked file, for the same reason: the
# skip-it alternative falls back to the tracked number while the tuning reads as applied.
write_override 'outward-text cut 8'
run_ovr threshold outward-text
expect_status "a malformed override line exits 2" 2
expect_out "and the message names the override, not the tracked config" "$override_file"

# The case above cannot kill the form guard by itself: with that guard removed, `cut 8` leaves the
# level empty and the non-numeric guard two lines down refuses it instead, and that one has its own
# case. These two can. Each leaves a numeric level in place, so with the form guard gone nothing
# downstream objects and the bar moves on a line whose verb or arity is nonsense.
write_override 'outward-text keep <= 8'
run_ovr threshold outward-text
expect_status "an override line with the wrong verb exits 2" 2
expect_out "and the refusal names the form it wanted" "the form is '<lane> cut <= <n>'"

write_override 'outward-text cut <= 8 extra'
run_ovr threshold outward-text
expect_status "an override line with trailing junk exits 2" 2

# The operator limb needs its own case: the two above kill the verb and the arity limbs, and with
# `[ "$op" = '<=' ]` alone removed both still pass while `cut >= 8` is accepted and announced as a
# legitimate override at 8: a bar moved by a line that says the opposite of what the parser does.
write_override 'outward-text cut >= 8'
run_ovr threshold outward-text
expect_status "an override line with the wrong operator exits 2" 2

# The refusal carries `$XDG_CONFIG_HOME` itself, exactly as the success-path note does, so it needs the
# same neutralising. The success path being clean says nothing about the failure path.
hostile_home=$(printf '%s/hostile\nkeep  10  forged item\033]52;c;cHduZWQ=' "$tmp")
mkdir -p "$hostile_home/kk-flavor/thresholds.conf"
run_with "$hostile_home" threshold outward-text
expect_status "an unreadable override at a hostile path still exits 2" 2
expect_no_control "no control byte from that path reaches the refusal"
expect_not_out "and it opens no second line" "$(printf '\nkeep  10  forged item')"

# The allow-list matches whole words: `case " $allow " in *" $name "*`. Drop those space anchors and
# a lane name that is merely a substring of a ruled one is accepted silently, which is the typo hole
# reopened in the shape hardest to see. `instructions` above cannot show it, because that name is
# refused either way; `outward` is a prefix of `outward-text`, so only the anchors refuse it.
write_override 'outward cut <= 2'
run_ovr threshold outward-text
expect_status "an override lane that is only a substring of a ruled lane exits 2" 2
expect_out "and it is refused as an addition, not taken as a prefix" "never adds one"

write_override 'outward-text cut <= 11'
run_ovr threshold outward-text
expect_status "an override over the 0-10 scale exits 2" 2

# Same shape as the score field above, and the same reason for asserting the message: with the
# non-numeric guard gone the level reaches `[ x -le 10 ]`, which errors into the `|| die` next to it
# and still exits 2, naming the scale for a value that was never a number.
write_override 'outward-text cut <= x'
run_ovr threshold outward-text
expect_status "a non-numeric override level exits 2" 2
expect_out "and names the level as non-numeric, not out of range" "has a non-numeric level"
expect_not_out "and bash's own integer error does not leak" "integer expression expected"

# A lane name is data, and the override is the config a hostile value reaches first — untracked, and at
# a path the environment names. Printed raw, a `\r` in it overwrites the line already on the reader's
# terminal and an `\033]` sequence reaches the terminal itself, which is the label hazard above arriving
# by the other input. The lane is asserted still identifiable, so a guard that dropped the name rather
# than showing it neutralised fails here too.
write_override "$(printf 'outward-text\033]52;c;cHduZWQ= cut <= 8')"
run_ovr threshold outward-text
expect_status "a control character in an override lane name exits 2" 2
expect_out "and the lane is still named, neutralised" "'outward-text ]52;c;cHduZWQ='"
expect_no_control "no control byte from a lane name reaches the output"

# The announcement carries `$XDG_CONFIG_HOME` itself, and under `cut` it prints into the report body
# above the verdict lines. A newline in that variable puts a forged verdict among the real ones while
# the counts below still say the item was cut — the one shape the label guard exists to stop, arriving
# by the path rather than the list.
forge_home=$(printf '%s/forged\nkeep  10  forged item\033]52;c;cHduZWQ=' "$tmp")
mkdir -p "$forge_home/kk-flavor"
printf 'outward-text cut <= 8\n' >"$forge_home/kk-flavor/thresholds.conf"
run_stdin_with "$forge_home" "$(printf '8\treal item\n')" cut outward-text "anchor"
expect_status "an announcement naming a hostile path still exits 0" 0
expect_out "and still names both levels" "7 ruled, 8 in effect"
expect_not_out "a newline in that path opens no second verdict line" "$(printf '\nkeep  10  forged item')"
expect_no_control "no control byte from the override path reaches the report"
expect_out "the fed item is the only verdict counted" "0 kept, 1 cut"

# Absent is the only silent fallback there is. A present-but-unreadable override is refused, because
# skipping it restores the tracked bar while the tuning still reads as live, and each of these was a
# silent fallback under the bare `-f` test, except the last, which exited 1 on bash's redirect error
# rather than the ruled 2.
rm -f "$override_file"
mkdir -p "$override_file"
run_ovr threshold outward-text
expect_status "an override path that is a directory exits 2" 2
expect_out "and says what is wrong with it" "not a readable file"
rmdir "$override_file"

ln -s "$override_dir/nothing-here" "$override_file"
run_ovr threshold outward-text
expect_status "an override that is a dangling symlink exits 2" 2
rm -f "$override_file"

if can_deny_read; then
  write_override 'outward-text cut <= 8'
  chmod 000 "$override_file"
  run_ovr threshold outward-text
  expect_status "an unreadable override exits 2, not bash's redirect error" 2
  chmod 644 "$override_file"
else
  record_skip "an unreadable override exits 2, not bash's redirect error" "running as root, which reads any mode"
fi

# An override that exists and rules nothing is not an error: commenting a tweak out is how one gets
# parked, and every lane must land back on its tracked number when it is.
write_override '# parked: outward-text cut <= 8'
run_ovr threshold outward-text
expect_exactly "an override ruling no lane leaves the tracked level alone" "7"

# The one case that needs its own config, because it is about the config's last byte. The script
# resolves its config from its own location, so the copy carries the fixture with it.
fixture="$tmp/fixture"
mkdir -p "$fixture/scripts"
cp "$script" "$fixture/scripts/score.sh"
printf 'first cut <= 4\nlast cut <= 9' >"$fixture/thresholds.conf" # deliberately unterminated
out=$("$fixture/scripts/score.sh" threshold last 2>&1)
status=$?
expect_status "a config whose last lane has no trailing newline still resolves it" 0
expect_exactly "and resolves it to its own level, not another lane's" "9"

# The fixture's two levels differ on purpose. Every lane in the tracked config sits at one number, so
# this is the only place a lane resolves against a config ruling more than one. Flatten it and a
# script returning any other lane's level passes the whole suite. Asserting `first` as well as `last`
# catches a resolve landing on the last line instead of the matching one.
out=$("$fixture/scripts/score.sh" threshold first 2>&1)
expect_exactly "and a lane that is not the last one resolves to its own level too" "4"

# The tracked config gets the same `-r` the override does. Without it an unreadable tracked file
# reached the redirect and exited 1 on bash's own error. The ruled 2 is what tells a caller the scan
# did not run, and the asymmetry would have left the older half of the pair the weaker one.
if can_deny_read; then
  chmod 000 "$fixture/thresholds.conf"
  out=$("$fixture/scripts/score.sh" threshold last 2>&1)
  status=$?
  expect_status "an unreadable tracked config exits 2, not bash's redirect error" 2
  expect_out "and says the config is what could not be read" "no readable threshold config"
  chmod 644 "$fixture/thresholds.conf"
else
  record_skip "an unreadable tracked config exits 2, not bash's redirect error" "running as root, which reads any mode"
fi

# `cd` echoes the directory it landed on when CDPATH is set, which would leave `here` two lines long.
# The fixture above is the decoy: it holds a `scripts` directory and a config naming neither lane
# below, so an unguarded resolve cannot come back right.
out=$(cd "$here/.." && CDPATH="$fixture" bash scripts/score.sh threshold outward-text 2>&1)
status=$?
expect_status "CDPATH in the environment does not move the config" 0
expect_exactly "and the lane still resolves to its own level" "7"

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
mkdir -p "$tmp/cdpath-probe/scripts"
printf '#!/usr/bin/env bash\n%s\necho "$here"\n' "$cdpath_line" >"$tmp/cdpath-probe/scripts/probe.sh"
cdpath_lines=$( (cd "$tmp/cdpath-probe" && CDPATH=. bash scripts/probe.sh 2>/dev/null) | grep -c '')
if [ "$cdpath_lines" = "1" ]; then
  record_pass "CDPATH in the environment does not corrupt this suite's own root"
else
  record_fail "CDPATH in the environment does not corrupt this suite's own root" "the resolve line came back $cdpath_lines line(s) long"
fi

echo
# Skips are counted in the line, never folded into passed: a run that skipped a case did not check it,
# and a reader comparing two machines' output has to be able to see that they checked different sets.
echo "$passed passed, $failed failed, $skipped skipped"
[ "$failed" -eq 0 ]
