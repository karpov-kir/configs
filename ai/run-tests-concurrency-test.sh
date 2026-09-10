#!/usr/bin/env bash
# Cases for run-tests.sh's concurrency — the lanes, the job count a caller asks for, the downgrade on a
# bash without `wait -n`, the single lane bootstrap.sh --verify gets, and a suite whose runner subshell
# dies before it can report.
#   usage: run-tests-concurrency-test.sh   # one line per case; exit 0 when all pass, 1 otherwise
#
# There is no ai/run-tests-concurrency.sh. This covers ai/run-tests.sh, the same script its sibling
# ai/run-tests-test.sh covers. Every shell unit is keyed on that file already, so the split needed no
# new input; the gate names it this suite's sibling so its text is scanned too (ai/tools/gate/units.go).
set -uo pipefail
export LC_ALL=C

here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
suite_name="ai/run-tests-concurrency-test.sh"
runner="$here/run-tests.sh"

# shellcheck source=../lib/test-check.sh
. "$here/../lib/test-check.sh" ||
  { printf '%s: lib/test-check.sh did not load to the end — nothing was measured\n' "$suite_name" >&2; exit 2; }
# shellcheck source=../lib/run-tests-fixtures.sh
. "$here/../lib/run-tests-fixtures.sh" ||
  { printf '%s: lib/run-tests-fixtures.sh did not load to the end — nothing was measured\n' "$suite_name" >&2; exit 2; }

mkdir -p "$tmp/together"
for name in a b c; do new_marking_suite "$tmp/together/$name-test.sh" "$name" 3; done

# The count is named rather than left to the default, so this measures the mechanism on every machine.
# The default is half the cores, which is 1 on a two-core runner, and there the case would assert that
# concurrency is broken.
out="$(RUN_TESTS_JOBS=3 "$runner" "$tmp/together" 2>&1)"; rc=$?
check "suites asked to run three at a time all pass" "0" "$rc"
# guarded-block: `wait -n` is bash 4.3, and without it run-tests.sh runs the suites one at a time on
# purpose — "never wrong, only slower". On such a machine this assertion would report the runner broken
# for doing exactly what it documents, so it is skipped by name rather than left to fail there.
if [ "${BASH_VERSINFO[0]:-0}" -gt 4 ] ||
  { [ "${BASH_VERSINFO[0]:-0}" -eq 4 ] && [ "${BASH_VERSINFO[1]:-0}" -ge 3 ]; }; then
  check "and they overlap" "3" "$(most_seen "$tmp/together")"
else
  record_skip 1 "this bash has no \`wait -n\`, so the runner correctly ran the suites one at a time"
fi
check "and the report still names them in discovery order" "a b c" \
  "$(printf '%s\n' "$out" | awk '/^ok   /{ sub(/-test\.sh$/, "", $2); printf "%s%s", (seen++ ? " " : ""), $2 }')"

rm -f "$tmp/together"/*.saw "$tmp/together"/*.running
out="$(RUN_TESTS_JOBS=1 "$runner" "$tmp/together" 2>&1)"; rc=$?
check "RUN_TESTS_JOBS=1 puts them back on one lane" "0" "$rc"
check "and then no suite ever sees another running" "1" "$(most_seen "$tmp/together")"
check "and it still reports all three" "3" "$(matching_output_lines '^ok   ')"

# The downgrade to one lane, and the notice it prints. Every machine that runs this suite has
# `wait -n`, so the seam is what reaches the branch at all; the control below asserts the notice stays
# absent without the seam, so one that fired unconditionally could not pass as green either.
rm -f "$tmp/together"/*.saw
out="$(RUN_TESTS_NO_WAIT_N=1 RUN_TESTS_JOBS=3 "$runner" "$tmp/together" 2>&1)"; rc=$?
check "a bash without wait -n still runs every suite" "0" "$rc"
check "and says it downgraded, naming the count it did not get" "1" \
  "$(matching_output_lines 'has no .wait -n., so the suites run one at a time rather than 3')"
check "and really does run them one at a time" "1" "$(most_seen "$tmp/together")"

rm -f "$tmp/together"/*.saw
out="$(RUN_TESTS_JOBS=3 "$runner" "$tmp/together" 2>&1)"; rc=$?
check "and on a bash that has it, no downgrade notice appears" "0" \
  "$(matching_output_lines 'has no .wait -n.')"

# A count this does not understand refuses, rather than being read as zero and quietly restoring the
# serial run the caller was trying to move off.
out="$(RUN_TESTS_JOBS=two "$runner" "$tmp/together" 2>&1)"; rc=$?
check "a job count that is not a number exits 2" "2" "$rc"
check "and says so" "1" "$(matching_output_lines 'not a whole number of suites')"

# Zero passes the digits check and still names no run. Only an unset variable earns the default, so a
# spelled zero has to refuse.
out="$(RUN_TESTS_JOBS=0 "$runner" "$tmp/together" 2>&1)"; rc=$?
check "a job count of zero exits 2" "2" "$rc"
check "and says a run needs a lane" "1" "$(matching_output_lines 'a run needs at least one lane')"

out="$(RUN_TESTS_JOBS=00 "$runner" "$tmp/together" 2>&1)"; rc=$?
check "and zero spelled another way is refused too" "2" "$rc"

# Set but empty is the same mistake wearing a variable that did not expand.
out="$(RUN_TESTS_JOBS= "$runner" "$tmp/together" 2>&1)"; rc=$?
check "an empty job count exits 2" "2" "$rc"
check "and says it named no number" "1" "$(matching_output_lines 'set but empty')"

# The negative control for all four: naming nothing at all is the one case that does get the default.
rm -f "$tmp/together"/*.saw "$tmp/together"/*.running
out="$("$runner" "$tmp/together" 2>&1)"; rc=$?
check "naming no count at all still runs on the default" "0" "$rc"

# A suite whose runner subshell dies before it can write a status file. Nothing else drives it, and it
# decides between NOMEASURE and folding a suite that never reported into the pass count — a green over
# a suite nobody measured, which is the failure this whole file exists to refuse.
#
# The suite kills its own parent, which is the subshell running it, so `bash "$suite"` never returns and
# the `printf ... > .status` after it never runs.
mkdir -p "$tmp/nostatus"
new_suite "$tmp/nostatus/aa-good-test.sh" "1 passed, 0 failed"
printf '#!/usr/bin/env bash\nkill -9 "$PPID"\nsleep 30\n' > "$tmp/nostatus/zz-dies-test.sh"
out="$("$runner" "$tmp/nostatus" 2>&1)"; rc=$?
check "a suite whose runner died is unmeasured, not a pass" "2" "$rc"
check "and it is reported as NOMEASURE" "1" "$(matching_output_lines '^NOMEASURE .*zz-dies-test\.sh')"
check "and it is counted as unmeasured, never passed" "1" \
  "$(matching_output_lines '2 suite(s) found: 1 passed, 0 failed, 1 unmeasured')"
check "and the suite beside it still passes" "1" "$(matching_output_lines '^ok   .*aa-good-test\.sh')"

# The single lane bootstrap.sh --verify gets by default: it calls this runner immediately after writing
# the caller's real $HOME, and run-tests.sh's containment check is `git status` over the checkout, which
# cannot see a write that lands anywhere else. One lane does not stop a suite escaping its temp HOME —
# it stops one doing so beside five peers while that config is half-written.
rm -f "$tmp/together"/*.saw "$tmp/together"/*.running
out="$(BOOTSTRAP_VERIFYING=1 "$runner" "$tmp/together" 2>&1)"; rc=$?
check "under bootstrap --verify the suites still all pass" "0" "$rc"
check "and no suite there ever sees another running" "1" "$(most_seen "$tmp/together")"

# A default, not a ceiling: an explicit count still wins on that path.
rm -f "$tmp/together"/*.saw "$tmp/together"/*.running
out="$(BOOTSTRAP_VERIFYING=1 RUN_TESTS_JOBS=3 "$runner" "$tmp/together" 2>&1)"; rc=$?
check "an explicit count still wins under bootstrap --verify" "0" "$rc"
# guarded-block: `wait -n` is bash 4.3, and without it the runner serialises on purpose — so this
# machine cannot tell a count that was honoured from the carve-out it is meant to override.
if [ "${BASH_VERSINFO[0]:-0}" -gt 4 ] ||
  { [ "${BASH_VERSINFO[0]:-0}" -eq 4 ] && [ "${BASH_VERSINFO[1]:-0}" -ge 3 ]; }; then
  check "and they overlap despite the carve-out" "3" "$(most_seen "$tmp/together")"
else
  record_skip 1 "this bash has no \`wait -n\`, so an honoured count and the carve-out look alike"
fi

# Held on every machine, not only the one a guard is for: the skip counts are literals nothing derives.
# skip_count_drift takes the file to scan because both suites carry guarded blocks now.
check "every record_skip count matches the cases its block holds" "" "$(skip_count_drift "$0")"

echo "$pass passed, $fail failed, $skipped skipped"
[ "$fail" -eq 0 ]
