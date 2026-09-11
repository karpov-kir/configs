#!/usr/bin/env bash
# Tests for run-tests.sh — the runner CI calls, so a bug here is a gate that stops gating quietly.
#   usage: run-tests-test.sh   # prints one line per case; exit 0 when all pass, 1 otherwise
#
# The two that matter most are the cases a green run cannot otherwise be told apart from: `no suites
# found`, where discovery silently matches nothing, and a suite that exits 0 having run no case at
# all. Both report a clean tree that was never read.
#
# The concurrency cases are ai/run-tests-concurrency-test.sh; the fixtures both halves need are in
# lib/run-tests-fixtures.sh rather than copied into each.
set -uo pipefail
export LC_ALL=C

here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
suite_name="ai/run-tests-test.sh"
runner="$here/run-tests.sh"

# shellcheck source=../lib/test-check.sh
. "$here/../lib/test-check.sh" ||
  { printf '%s: lib/test-check.sh did not load to the end — nothing was measured\n' "$suite_name" >&2; exit 2; }
# shellcheck source=../lib/run-tests-fixtures.sh
. "$here/../lib/run-tests-fixtures.sh" ||
  { printf '%s: lib/run-tests-fixtures.sh did not load to the end — nothing was measured\n' "$suite_name" >&2; exit 2; }
mkdir -p "$tmp/green/nested"
new_suite "$tmp/green/a-test.sh" "2 passed, 0 failed"
new_suite "$tmp/green/nested/b-test.sh" "1 passed, 0 failed"
out="$("$runner" "$tmp/green" 2>&1)"; rc=$?
check "a root of passing suites exits 0" "0" "$rc"
check "both suites are found, including a nested one" "2" "$(matching_output_lines '^ok   ')"
check "the count is reported, so a shrinking tree is visible" "1" \
  "$(matching_output_lines '2 suite(s) found: 2 passed, 0 failed, 0 unmeasured')"

check "successful suites create no diagnostic logs" "0" "$(find "$TMPDIR" -type f | wc -l | tr -d ' ')"

mkdir -p "$tmp/noisy"
cat > "$tmp/noisy/early-test.sh" <<'SUITE'
#!/usr/bin/env bash
echo 'early failure evidence' >&2
for ((i=1; i<=60; i++)); do echo "later output $i"; done
exit 1
SUITE
out="$("$runner" "$tmp/noisy" 2>&1)"; rc=$?
check "a noisy failure retains its failing exit" "1" "$rc"
log="$(printf '%s\n' "$out" | sed -n 's/^     full log: //p')"
check "a noisy failure reports one readable full log" "yes" "$([ -f "$log" ] && echo yes)"
check "the full log retains the early failure before the tail" "1" "$(grep -c '^early failure evidence$' "$log" 2>/dev/null || true)"
check "failure console output remains bounded" "yes" "$([ "$(printf '%s\n' "$out" | wc -l)" -le 20 ] && echo yes)"
check "the log path survives an outer tail" "1" "$(printf '%s\n' "$out" | tail -5 | grep -c 'full log: ')"
check "the full log is outside the subject root" "yes" "$([ -n "$log" ] && [ "${log#"$tmp/noisy"/}" = "$log" ] && echo yes)"
check "the full log is private" "-rw-------" "$(LC_ALL=C ls -l "$log" 2>/dev/null | awk '{print substr($1, 1, 10)}')"
first_log="$log"
out="$("$runner" "$tmp/noisy" 2>&1)"
log="$(printf '%s\n' "$out" | sed -n 's/^     full log: //p')"
check "separate failed runs keep distinct logs" "yes" "$([ "$first_log" != "$log" ] && [ -f "$first_log" ] && [ -f "$log" ] && echo yes)"
ln -s "$tmp/noisy" "$tmp/log-link"
out="$(TMPDIR="$tmp/log-link" "$runner" "$tmp/noisy" 2>&1)"
log="$(printf '%s\n' "$out" | sed -n 's/^     full log: //p')"
check "a temporary directory resolving inside the subject does not receive logs" "0" "$(find "$tmp/noisy" -name 'kk-suite-logs.*' | wc -l | tr -d ' ')"
check "the outside fallback still retains the full diagnostics" "1" "$(grep -c '^early failure evidence$' "$log" 2>/dev/null || true)"
if [ -f "$log" ]; then rm -rf "$(dirname "$log")"; fi
newline_root="$tmp/subject"$'\n'
mkdir -p "$newline_root/logs"
cp "$tmp/noisy/early-test.sh" "$newline_root/early-test.sh"
out="$(TMPDIR="$newline_root/logs" "$runner" "$newline_root" 2>&1)"
log="$(printf '%s\n' "$out" | sed -n 's/^     full log: //p')"
check "a newline-ending subject root cannot hide an inside-root temporary directory" "0" "$(find "$newline_root" -name 'kk-suite-logs.*' | wc -l | tr -d ' ')"
if [ -f "$log" ]; then rm -rf "$(dirname "$log")"; fi
cp "$tmp/noisy/early-test.sh" "$tmp/noisy/second-test.sh"
out="$("$runner" "$tmp/noisy" 2>&1)"
log_dir="$(printf '%s\n' "$out" | sed -n 's/^full logs: //p')"
check "the final directory pointer survives a many-failure outer tail" "1" "$(printf '%s\n' "$out" | tail -5 | grep -c '^full logs: ')"
check "the final directory contains every failed suite log" "2" "$(find "$log_dir" -type f 2>/dev/null | wc -l | tr -d ' ')"
check "retained logs identify both originating suites after console truncation" "2" "$(grep -lE '^suite: (early|second)-test.sh$' "$log_dir"/* | wc -l | tr -d ' ')"
check "retained logs carry the failing status" "2" "$(grep -l '^status: 1$' "$log_dir"/* | wc -l | tr -d ' ')"
rm "$tmp/noisy/second-test.sh"
out="$(TMPDIR="$tmp/missing/logs" "$runner" "$tmp/noisy" 2>&1)"; rc=$?
check "failed log persistence preserves the suite failure" "1" "$rc"
check "failed log persistence prints the otherwise lost evidence" "1" "$(matching_output_lines '^early failure evidence$')"
check "failed log persistence is explicit" "1" "$(matching_output_lines 'could not retain full log')"
check "unavailable diagnostic storage remains explicit after outer truncation" "1" "$(printf '%s\n' "$out" | tail -5 | grep -c '^warning: full logs unavailable for 1 suite(s)')"

mkdir -p "$tmp/red"
new_suite "$tmp/red/good-test.sh" "1 passed, 0 failed"
printf '#!/usr/bin/env bash\necho "something broke" >&2\nexit 1\n' > "$tmp/red/bad-test.sh"
out="$("$runner" "$tmp/red" 2>&1)"; rc=$?
check "one failing suite fails the run" "1" "$rc"
check "the failing suite is named" "1" "$(matching_output_lines '^FAIL bad-test.sh')"
check "its output is shown, not swallowed" "1" "$(matching_output_lines 'something broke')"
check "the passing sibling still ran" "1" "$(matching_output_lines '^ok   good-test.sh')"

mkdir -p "$tmp/vacuous"
new_suite "$tmp/vacuous/hollow-test.sh" "0 passed, 0 failed"
out="$("$runner" "$tmp/vacuous" 2>&1)"; rc=$?
check "control: a suite that exits 0 having run no case fails the run" "1" "$rc"
check "control: and is named vacuous rather than passing" "1" "$(matching_output_lines '^VACUOUS hollow-test.sh')"

mkdir -p "$tmp/skipped"
new_suite "$tmp/skipped/declined-test.sh" "0 passed, 0 failed, 3 skipped"
out="$("$runner" "$tmp/skipped" 2>&1)"; rc=$?
check "zero passed with a skip count is not vacuous — the cases exist" "0" "$rc"
check "and it reports as passing, not skipped away" "1" "$(matching_output_lines '^ok   declined-test.sh')"

mkdir -p "$tmp/fields"
new_suite "$tmp/fields/reordered-test.sh" "0 failed, 5 passed, 1 skipped"
out="$("$runner" "$tmp/fields" 2>&1)"; rc=$?
check "the count is read by field name, not by position" "0" "$rc"

mkdir -p "$tmp/nomeasure"
printf '#!/usr/bin/env bash\necho "missing prerequisite evidence" >&2\necho "0 passed, 0 failed" >&2\nexit 2\n' > "$tmp/nomeasure/loaded-test.sh"
out="$("$runner" "$tmp/nomeasure" 2>&1)"; rc=$?
check "a suite exiting 2 makes the run exit 2, never 0" "2" "$rc"
check "it is reported as unmeasured, not as a failure" "1" "$(matching_output_lines '^NOMEASURE loaded-test.sh')"
check "and it is counted apart from failures" "1" \
  "$(matching_output_lines '1 suite(s) found: 0 passed, 0 failed, 1 unmeasured')"

log="$(printf '%s\n' "$out" | sed -n 's/^     full log: //p')"
check "unmeasured suite diagnostics also survive in a full log" "1" "$(grep -c '^missing prerequisite evidence$' "$log" 2>/dev/null || true)"

mkdir -p "$tmp/both"
new_unmeasured_suite "$tmp/both/unmeasured-test.sh"
new_failing_suite "$tmp/both/failing-test.sh"
out="$("$runner" "$tmp/both" 2>&1)"; rc=$?
check "a red outranks a non-measurement" "1" "$rc"

# A suite can measure something real and still not have measured its whole effect: writing outside its
# own fixtures while reporting every case green. So the run has to notice the checkout moved under it.
# guarded-block: this machine may have no git, and discovery cannot be driven through it there.
if command -v git >/dev/null; then
  new_greedy_checkout "$tmp/repo"
  out="$("$runner" "$tmp/repo" 2>&1)"; rc=$?
  check "control: a checkout that moved under the run exits 3, never 0" "3" "$rc"
  check "control: and the run says the checkout changed, without naming a culprit" "1" \
    "$(matching_output_lines 'the checkout changed while the suites ran')"
  check "control: even though the suite itself reported passing" "1" "$(matching_output_lines '^ok   greedy-test.sh')"

  # The counters stay a tally of suites. Folded into `failed`, the summary sums to more suites than it
  # found and asserts a red suite that does not exist. run-tests.sh's guard carries why.
  check "and a moved checkout is named on the summary rather than counted as a failing suite" "1" \
    "$(matching_output_lines '1 suite(s) found: 1 passed, 0 failed, 0 unmeasured, the checkout moved')"

  # Which of the three the exit carries when more than one is true at once; the ranking and its reason
  # are in run-tests.sh. Both cases need a checkout of their own: the greedy suite writes the same
  # bytes every run, so a repository it has already clobbered no longer moves under it.
  new_greedy_checkout "$tmp/moved-red"
  new_failing_suite "$tmp/moved-red/broken-test.sh"
  out="$("$runner" "$tmp/moved-red" 2>&1)"; rc=$?
  check "a red suite outranks a checkout that moved" "1" "$rc"
  check "control: and the checkout really did move, so this is not a red on its own" "1" \
    "$(matching_output_lines 'the checkout changed while the suites ran')"

  new_greedy_checkout "$tmp/moved-nomeasure"
  new_unmeasured_suite "$tmp/moved-nomeasure/absent-test.sh"
  out="$("$runner" "$tmp/moved-nomeasure" 2>&1)"; rc=$?
  check "a checkout that moved outranks a suite that did not measure" "3" "$rc"
  check "control: and that suite really did decline to measure" "1" \
    "$(matching_output_lines '^NOMEASURE absent-test.sh')"
else
  record_skip 8 "this machine has no git, so discovery could not be driven through it"
fi

# Two runs that checked different things must not print one line. A run outside a checkout cannot
# verify that no suite wrote into it, and the summary is where a reader looks.
out="$("$runner" "$tmp/green" 2>&1)"
check "outside a checkout the summary says containment went unchecked" "1" \
  "$(matching_output_lines 'unmeasured, containment unchecked')"
# guarded-block: this machine may have no git, and discovery cannot be driven through it there.
if command -v git >/dev/null; then
  out="$("$runner" "$tmp/repo" 2>&1)"
  check "inside one it does not, so a clean tail means the tree was checked" "0" \
    "$(matching_output_lines 'containment unchecked')"
else
  record_skip 1 "this machine has no git, so discovery could not be driven through it"
fi

mkdir -p "$tmp/empty/deep"
printf '#!/usr/bin/env bash\necho hi\n' > "$tmp/empty/deep/not-a-suite.sh"
out="$("$runner" "$tmp/empty" 2>&1)"; rc=$?
check "control: finding no suites exits 2 rather than passing empty" "2" "$rc"
check "control: and says discovery is what broke" "1" "$(matching_output_lines 'discovery broken')"

out="$("$runner" "$tmp/nope" 2>&1)"; rc=$?
check "a root that does not exist exits 2" "2" "$rc"

# `cd` echoes the directory it landed on when CDPATH is set and the path it is given is relative, so
# the default root comes back two lines long and the runner refuses a directory that is really there.
# Invoked by a relative path from a directory above it, because that is the only shape that consults
# CDPATH at all — and it is the shape the gates workflow uses, `run: ai/run-tests.sh`.
mkdir -p "$tmp/cdpath/sub"
cp "$runner" "$tmp/cdpath/sub/run-tests.sh"
chmod +x "$tmp/cdpath/sub/run-tests.sh"
new_suite "$tmp/cdpath/a-test.sh" "1 passed, 0 failed"
out="$(cd "$tmp/cdpath" && CDPATH=. bash sub/run-tests.sh 2>&1)"; rc=$?
check "CDPATH in the environment does not corrupt the default root" "0" "$rc"
check "control: and the suite beside it really was found, so this is not an empty pass" "1" \
  "$(matching_output_lines '^ok   a-test.sh')"

# Every file discovery finds is then executed, so inside a checkout the list comes from git: tracked
# files plus new untracked ones, and nothing .gitignore already excludes. The untracked half is the
# property that must survive the narrowing — a suite written five minutes ago is untracked, and this
# runner's whole claim is that it runs without anyone registering it. The ignored suite exits 1, so
# it is a control rather than an assertion about a name: if it ran at all, the run goes red.
# guarded-block: this machine may have no git, and discovery cannot be driven through it there.
if command -v git >/dev/null; then
  mkdir -p "$tmp/ignored/vendor"
  ( cd "$tmp/ignored" && git init -q . && git config user.email t@t && git config user.name t ) >/dev/null 2>&1
  printf 'vendor/\n' > "$tmp/ignored/.gitignore"
  new_suite "$tmp/ignored/tracked-test.sh" "1 passed, 0 failed"
  ( cd "$tmp/ignored" && git add .gitignore tracked-test.sh && git commit -qm seed ) >/dev/null 2>&1
  new_suite "$tmp/ignored/fresh-test.sh" "1 passed, 0 failed"
  new_failing_suite "$tmp/ignored/vendor/dropped-test.sh"
  out="$("$runner" "$tmp/ignored" 2>&1)"; rc=$?
  check "control: a gitignored suite is never executed" "0" "$rc"
  check "control: and is not among the suites found" "0" "$(matching_output_lines 'dropped-test.sh')"
  check "a tracked suite still runs" "1" "$(matching_output_lines '^ok   tracked-test.sh')"
  check "and an untracked one written since the last commit runs too" "1" \
    "$(matching_output_lines '^ok   fresh-test.sh')"
  check "and the summary says git answered discovery" "1" "$(matching_output_lines 'discovered by git')"

  # Why a tracked suite missing from the working tree must not go red is in run-tests.sh, at the
  # guard. `git rm` is not an option for the caller either: the index is shared, so a deletion waits
  # unstaged until they commit.
  rm "$tmp/ignored/tracked-test.sh"
  out="$("$runner" "$tmp/ignored" 2>&1)"; rc=$?
  check "a tracked suite absent from the working tree does not fail the run" "0" "$rc"
  check "and is reported as absent rather than as a failure" "1" \
    "$(matching_output_lines '^ABSENT .*tracked-test.sh')"
  check "and never reaches bash, so nothing reports it as not found" "0" \
    "$(matching_output_lines 'No such file or directory')"
  check "and the summary carries it, so a run that read fewer files does not print the same line" "1" \
    "$(matching_output_lines 'tracked but absent from the working tree')"
  check "and the suites that are there still run" "1" \
    "$(matching_output_lines '^ok   fresh-test.sh')"
  new_suite "$tmp/ignored/tracked-test.sh" "1 passed, 0 failed"

  # A suite that is PRESENT but cannot be run. The fixture is untracked, so landing in the absent arm
  # above would announce "tracked by git" about a file git does not track. Why `-f` alone cannot tell
  # the two apart is in run-tests.sh, at the guard.
  ln -s no-such-target.sh "$tmp/ignored/dangling-test.sh"
  out="$("$runner" "$tmp/ignored" 2>&1)"; rc=$?
  check "a suite present but not runnable does not pass green" "2" "$rc"
  check "and is reported as broken rather than as absent" "1" \
    "$(matching_output_lines '^BROKEN .*dangling-test.sh')"
  check "and nothing claims git tracks it, because git does not" "0" \
    "$(matching_output_lines '^ABSENT .*dangling-test.sh')"
  check "and the summary carries it, apart from the unmeasured count" "1" \
    "$(matching_output_lines 'present but not runnable')"
  check "and the suites that are there still run" "1" \
    "$(matching_output_lines '^ok   fresh-test.sh')"
  rm "$tmp/ignored/dangling-test.sh"

  # The same hazard through the door that costs a real suite: one git tracks, replaced by a dangling
  # symlink. git reports ` T` and still lists it, so discovery finds it, and a green run here means a
  # suite that used to be covered stopped being covered with nothing saying so.
  rm "$tmp/ignored/tracked-test.sh"
  ln -s gone.sh "$tmp/ignored/tracked-test.sh"
  out="$("$runner" "$tmp/ignored" 2>&1)"; rc=$?
  check "a tracked suite replaced by a dangling symlink does not pass green" "2" "$rc"
  check "and is reported as broken, not as an ordinary deletion" "1" \
    "$(matching_output_lines '^BROKEN .*tracked-test.sh')"
  rm "$tmp/ignored/tracked-test.sh"
  new_suite "$tmp/ignored/tracked-test.sh" "1 passed, 0 failed"

  # -s executes whatever it names, so it has to refuse what discovery refuses. Without this it runs
  # exactly the suite the control above proves is never executed, and the guarantee holds only for
  # callers that did not pass -s.
  out="$("$runner" -s vendor/dropped-test.sh "$tmp/ignored" 2>&1)"; rc=$?
  check "-s refuses a gitignored suite discovery would never run" "2" "$rc"
  check "and says git does not know it" "1" "$(matching_output_lines 'ignored or unknown to git')"
  out="$("$runner" -s tracked-test.sh "$tmp/ignored" 2>&1)"; rc=$?
  check "control: -s still runs a tracked suite in the same repo" "0" "$rc"

  # The shape ai/gate.sh sends every run: a suite that exists and is not ignored but has never been
  # committed, which is what every suite is for as long as it is being written. Without this case,
  # tightening the guard to `--cached` alone leaves every case above green while breaking the gate for
  # every newly written suite.
  out="$("$runner" -s fresh-test.sh "$tmp/ignored" 2>&1)"; rc=$?
  check "-s runs an untracked suite git does not ignore" "0" "$rc"
  check "and reports it as the one suite found" "1" "$(matching_output_lines '^1 suite(s) found')"

  # A tracked suite whose name is not plain ASCII, which `ls-files` C-quotes. Why that ends in a run
  # announcing a suite that is plainly there as absent, and exiting 0 having never run it, is in
  # run-tests.sh, at the `-z`. Last in this block, so nothing after it depends on the tree this leaves.
  new_suite "$tmp/ignored/café-test.sh" "1 passed, 0 failed"
  ( cd "$tmp/ignored" && git add "café-test.sh" && git commit -qm accented ) >/dev/null 2>&1
  out="$("$runner" "$tmp/ignored" 2>&1)"; rc=$?
  check "a tracked suite with a non-ASCII name is run rather than reported absent" "1" \
    "$(matching_output_lines '^ok   café-test.sh')"
  check "control: and the run does not pass green over it" "0" "$rc"
else
  record_skip 24 "this machine has no git, so discovery could not be driven through it"
fi

# Two runs that read different file sets must not print one line, for the same reason the containment
# tail exists: outside a checkout git cannot answer and the fallback reads every file under the root.
out="$("$runner" "$tmp/green" 2>&1)"
check "outside a checkout the summary says find answered instead" "1" \
  "$(matching_output_lines 'discovered by find')"

mkdir -p "$tmp/skip/node_modules/pkg"
new_failing_suite "$tmp/skip/node_modules/pkg/vendor-test.sh"
new_suite "$tmp/skip/mine-test.sh" "1 passed, 0 failed"
out="$("$runner" "$tmp/skip" 2>&1)"; rc=$?
check "a vendored suite under node_modules is not run" "0" "$rc"

# -s names one suite instead of discovering them all. It exists so a caller that already knows which
# suite a change could have moved — ai/gate.sh — still gets this file's reading of the result rather
# than running `bash <suite>` itself and inheriting neither the exit-2 nor the vacuity rule.
mkdir -p "$tmp/named"
new_suite "$tmp/named/wanted-test.sh" "3 passed, 0 failed"
new_suite "$tmp/named/other-test.sh" "9 passed, 0 failed"
out="$("$runner" -s wanted-test.sh "$tmp/named" 2>&1)"; rc=$?
check "-s runs the suite it names" "0" "$rc"
check "and reports exactly one suite found" "1" "$(matching_output_lines '^1 suite(s) found')"
check "and does not run the sibling it did not name" "0" "$(matching_output_lines 'other-test.sh')"
check "and the summary says discovery was by name" "1" "$(matching_output_lines 'discovered by named')"

# The same rule discovery lives by: naming a file that is not there is the caller's typo, and an
# empty run over it would report a clean tree for a suite nothing executed.
out="$("$runner" -s no-such-test.sh "$tmp/named" 2>&1)"; rc=$?
check "-s naming a suite that is not there exits 2" "2" "$rc"
check "and says discovery is broken rather than clean" "1" \
  "$(matching_output_lines 'never as a clean run')"

# The vacuity rule above, concretely: a suite emptied to zero bytes exits 0 in silence, and a caller
# running it directly reads that as a pass. Through here it is VACUOUS and a failure.
: > "$tmp/named/empty-test.sh"
out="$("$runner" -s empty-test.sh "$tmp/named" 2>&1)"; rc=$?
check "-s over a suite that ran no case at all fails" "1" "$rc"
check "and names it vacuous" "1" "$(matching_output_lines '^VACUOUS')"

# Everything this script finds it then executes, so a named path that leaves the root is refused
# rather than run: nothing else stops `-s ../../../x` executing a file outside the repository.
# The target has to EXIST outside the root, or the missing-file refusal fires first and the case
# passes on the right exit code for the wrong reason.
new_suite "$tmp/outside-test.sh" "1 passed, 0 failed"
out="$("$runner" -s ../outside-test.sh "$tmp/named" 2>&1)"; rc=$?
check "-s naming a path outside the root exits 2" "2" "$rc"
check "and says it is not this root's to run" "1" "$(matching_output_lines "not this root's to run")"
check "and does not run it" "0" "$(matching_output_lines '1 passed')"

out="$("$runner" -z "$tmp/named" 2>&1)"; rc=$?
check "an unknown flag exits 2" "2" "$rc"

# Held on every machine, not only the one a guard is for: the skip counts are literals nothing derives.
# skip_count_drift takes the file to scan because both suites carry guarded blocks now.
check "every record_skip count matches the cases its block holds" "" "$(skip_count_drift "$0")"

echo "$pass passed, $fail failed, $skipped skipped"
[ "$fail" -eq 0 ]
