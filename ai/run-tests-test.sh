#!/usr/bin/env bash
# Tests for run-tests.sh — the runner CI calls, so a bug here is a gate that stops gating quietly.
#   usage: run-tests-test.sh   # prints one line per case; exit 0 when all pass, 1 otherwise
#
# Every case builds its own root under mktemp and points the runner at it. None of them run the
# runner over this repository, which is what keeps this file — discovered by that runner like any
# other suite — from recursing into itself.
# The cases that earn the rest are the two a green run cannot otherwise distinguish itself from:
# `no suites found`, where discovery silently matches nothing, and a suite that exits 0 having run
# no case at all. Both report a clean tree that was never read.
set -uo pipefail
export LC_ALL=C

here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
runner="$here/run-tests.sh"

pass=0
fail=0
check() {
  local name="$1" expected="$2" actual="$3"
  if [ "$expected" = "$actual" ]; then
    echo "ok   — $name"
    pass=$((pass + 1))
  else
    echo "FAIL — $name"
    printf '       expected: %s\n       actual:   %s\n' "$expected" "$actual"
    fail=$((fail + 1))
  fi
}

tmp="$(mktemp -d)" || exit 1
trap 'rm -rf "$tmp"' EXIT

mkdir -p "$tmp/green/nested"
printf '#!/usr/bin/env bash\necho "2 passed, 0 failed"\n' > "$tmp/green/a-test.sh"
printf '#!/usr/bin/env bash\necho "1 passed, 0 failed"\n' > "$tmp/green/nested/b-test.sh"
out="$("$runner" "$tmp/green" 2>&1)"; rc=$?
check "a root of passing suites exits 0" "0" "$rc"
check "both suites are found, including a nested one" "2" "$(printf '%s' "$out" | grep -c '^ok   ')"
check "the count is reported, so a shrinking tree is visible" "1" \
  "$(printf '%s' "$out" | grep -c '2 suite(s) found: 2 passed, 0 failed, 0 unmeasured')"

mkdir -p "$tmp/red"
printf '#!/usr/bin/env bash\necho "1 passed, 0 failed"\n' > "$tmp/red/good-test.sh"
printf '#!/usr/bin/env bash\necho "something broke" >&2\nexit 1\n' > "$tmp/red/bad-test.sh"
out="$("$runner" "$tmp/red" 2>&1)"; rc=$?
check "one failing suite fails the run" "1" "$rc"
check "the failing suite is named" "1" "$(printf '%s' "$out" | grep -c '^FAIL bad-test.sh')"
check "its output is shown, not swallowed" "1" "$(printf '%s' "$out" | grep -c 'something broke')"
check "the passing sibling still ran" "1" "$(printf '%s' "$out" | grep -c '^ok   good-test.sh')"

mkdir -p "$tmp/vacuous"
printf '#!/usr/bin/env bash\necho "0 passed, 0 failed"\n' > "$tmp/vacuous/hollow-test.sh"
out="$("$runner" "$tmp/vacuous" 2>&1)"; rc=$?
check "control: a suite that exits 0 having run no case fails the run" "1" "$rc"
check "control: and is named vacuous rather than passing" "1" "$(printf '%s' "$out" | grep -c '^VACUOUS hollow-test.sh')"

mkdir -p "$tmp/skipped"
printf '#!/usr/bin/env bash\necho "0 passed, 0 failed, 3 skipped"\n' > "$tmp/skipped/declined-test.sh"
out="$("$runner" "$tmp/skipped" 2>&1)"; rc=$?
check "zero passed with a skip count is not vacuous — the cases exist" "0" "$rc"
check "and it reports as passing, not skipped away" "1" "$(printf '%s' "$out" | grep -c '^ok   declined-test.sh')"

mkdir -p "$tmp/fields"
printf '#!/usr/bin/env bash\necho "0 failed, 5 passed, 1 skipped"\n' > "$tmp/fields/reordered-test.sh"
out="$("$runner" "$tmp/fields" 2>&1)"; rc=$?
check "the count is read by field name, not by position" "0" "$rc"

mkdir -p "$tmp/nomeasure"
printf '#!/usr/bin/env bash\necho "0 passed, 0 failed" >&2\nexit 2\n' > "$tmp/nomeasure/loaded-test.sh"
out="$("$runner" "$tmp/nomeasure" 2>&1)"; rc=$?
check "a suite exiting 2 makes the run exit 2, never 0" "2" "$rc"
check "it is reported as unmeasured, not as a failure" "1" "$(printf '%s' "$out" | grep -c '^NOMEASURE loaded-test.sh')"
check "and it is counted apart from failures" "1" \
  "$(printf '%s' "$out" | grep -c '1 suite(s) found: 0 passed, 0 failed, 1 unmeasured')"

mkdir -p "$tmp/both"
printf '#!/usr/bin/env bash\nexit 2\n' > "$tmp/both/unmeasured-test.sh"
printf '#!/usr/bin/env bash\nexit 1\n' > "$tmp/both/failing-test.sh"
out="$("$runner" "$tmp/both" 2>&1)"; rc=$?
check "a red outranks a non-measurement" "1" "$rc"

# bootstrap-test.sh linked a temp HOME at this repository and wrote through the link, replacing real
# config files while reporting every case green. A suite like that has measured something and the
# measurement was not the whole effect, so the run has to notice the checkout moved under it.
if command -v git >/dev/null; then
  mkdir -p "$tmp/repo"
  ( cd "$tmp/repo" && git init -q . && git config user.email t@t && git config user.name t &&
    printf 'real config\n' > kept.conf && git add kept.conf && git commit -qm seed ) >/dev/null 2>&1
  printf '#!/usr/bin/env bash\nprintf "clobbered\\n" > "$(dirname "$0")/kept.conf"\necho "1 passed, 0 failed"\n' \
    > "$tmp/repo/greedy-test.sh"
  out="$("$runner" "$tmp/repo" 2>&1)"; rc=$?
  check "control: a checkout that moved under the run exits 3, never 0" "3" "$rc"
  check "control: and the run says the checkout changed, without naming a culprit" "1" \
    "$(printf '%s' "$out" | grep -c 'the checkout changed while the suites ran')"
  check "control: even though the suite itself reported passing" "1" "$(printf '%s' "$out" | grep -c '^ok   greedy-test.sh')"

  # The counters stay a tally of suites. Folded into `failed` this read `1 suite(s) found: 1 passed,
  # 1 failed` — a sum larger than what was found, asserting a red suite that does not exist.
  # run-tests.sh's guard carries why that phantom was worth a case.
  check "and a moved checkout is named on the summary rather than counted as a failing suite" "1" \
    "$(printf '%s' "$out" | grep -c '1 suite(s) found: 1 passed, 0 failed, 0 unmeasured, the checkout moved')"

  # Which of the three the exit carries when more than one is true at once; the ranking and its reason
  # are in run-tests.sh. Both cases need a checkout of their own: the greedy suite writes the same
  # bytes every run, so a repository it has already clobbered no longer moves under it.
  mkdir -p "$tmp/moved-red"
  ( cd "$tmp/moved-red" && git init -q . && git config user.email t@t && git config user.name t &&
    printf 'real config\n' > kept.conf && git add kept.conf && git commit -qm seed ) >/dev/null 2>&1
  printf '#!/usr/bin/env bash\nprintf "clobbered\\n" > "$(dirname "$0")/kept.conf"\necho "1 passed, 0 failed"\n' \
    > "$tmp/moved-red/greedy-test.sh"
  printf '#!/usr/bin/env bash\nexit 1\n' > "$tmp/moved-red/broken-test.sh"
  out="$("$runner" "$tmp/moved-red" 2>&1)"; rc=$?
  check "a red suite outranks a checkout that moved" "1" "$rc"
  check "control: and the checkout really did move, so this is not a red on its own" "1" \
    "$(printf '%s' "$out" | grep -c 'the checkout changed while the suites ran')"

  mkdir -p "$tmp/moved-nomeasure"
  ( cd "$tmp/moved-nomeasure" && git init -q . && git config user.email t@t && git config user.name t &&
    printf 'real config\n' > kept.conf && git add kept.conf && git commit -qm seed ) >/dev/null 2>&1
  printf '#!/usr/bin/env bash\nprintf "clobbered\\n" > "$(dirname "$0")/kept.conf"\necho "1 passed, 0 failed"\n' \
    > "$tmp/moved-nomeasure/greedy-test.sh"
  printf '#!/usr/bin/env bash\nexit 2\n' > "$tmp/moved-nomeasure/absent-test.sh"
  out="$("$runner" "$tmp/moved-nomeasure" 2>&1)"; rc=$?
  check "a checkout that moved outranks a suite that did not measure" "3" "$rc"
  check "control: and that suite really did decline to measure" "1" \
    "$(printf '%s' "$out" | grep -c '^NOMEASURE absent-test.sh')"
fi

# Two runs that checked different things must not print one line. A run outside a checkout cannot
# verify that no suite wrote into it, and the summary is where a reader looks.
out="$("$runner" "$tmp/green" 2>&1)"
check "outside a checkout the summary says containment went unchecked" "1" \
  "$(printf '%s' "$out" | grep -c 'unmeasured, containment unchecked')"
if command -v git >/dev/null; then
  out="$("$runner" "$tmp/repo" 2>&1)"
  check "inside one it does not, so a clean tail means the tree was checked" "0" \
    "$(printf '%s' "$out" | grep -c 'containment unchecked')"
fi

mkdir -p "$tmp/empty/deep"
printf '#!/usr/bin/env bash\necho hi\n' > "$tmp/empty/deep/not-a-suite.sh"
out="$("$runner" "$tmp/empty" 2>&1)"; rc=$?
check "control: finding no suites exits 2 rather than passing empty" "2" "$rc"
check "control: and says discovery is what broke" "1" "$(printf '%s' "$out" | grep -c 'discovery broken')"

out="$("$runner" "$tmp/nope" 2>&1)"; rc=$?
check "a root that does not exist exits 2" "2" "$rc"

# `cd` echoes the directory it landed on when CDPATH is set and the path it is given is relative, so
# the default root comes back two lines long and the runner refuses a directory that is really there.
# Invoked by a relative path from a directory above it, because that is the only shape that consults
# CDPATH at all — and it is the shape the gates workflow uses, `run: ai/run-tests.sh`.
mkdir -p "$tmp/cdpath/sub"
cp "$runner" "$tmp/cdpath/sub/run-tests.sh"
chmod +x "$tmp/cdpath/sub/run-tests.sh"
printf '#!/usr/bin/env bash\necho "1 passed, 0 failed"\n' > "$tmp/cdpath/a-test.sh"
out="$(cd "$tmp/cdpath" && CDPATH=. bash sub/run-tests.sh 2>&1)"; rc=$?
check "CDPATH in the environment does not corrupt the default root" "0" "$rc"
check "control: and the suite beside it really was found, so this is not an empty pass" "1" \
  "$(printf '%s' "$out" | grep -c '^ok   a-test.sh')"

# Every file discovery finds is then executed, so inside a checkout the list comes from git: tracked
# files plus new untracked ones, and nothing .gitignore already excludes. The untracked half is the
# property that must survive the narrowing — a suite written five minutes ago is untracked, and this
# runner's whole claim is that it runs without anyone registering it. The ignored suite exits 1, so
# it is a control rather than an assertion about a name: if it ran at all, the run goes red.
if command -v git >/dev/null; then
  mkdir -p "$tmp/ignored/vendor"
  ( cd "$tmp/ignored" && git init -q . && git config user.email t@t && git config user.name t ) >/dev/null 2>&1
  printf 'vendor/\n' > "$tmp/ignored/.gitignore"
  printf '#!/usr/bin/env bash\necho "1 passed, 0 failed"\n' > "$tmp/ignored/tracked-test.sh"
  ( cd "$tmp/ignored" && git add .gitignore tracked-test.sh && git commit -qm seed ) >/dev/null 2>&1
  printf '#!/usr/bin/env bash\necho "1 passed, 0 failed"\n' > "$tmp/ignored/fresh-test.sh"
  printf '#!/usr/bin/env bash\nexit 1\n' > "$tmp/ignored/vendor/dropped-test.sh"
  out="$("$runner" "$tmp/ignored" 2>&1)"; rc=$?
  check "control: a gitignored suite is never executed" "0" "$rc"
  check "control: and is not among the suites found" "0" "$(printf '%s' "$out" | grep -c 'dropped-test.sh')"
  check "a tracked suite still runs" "1" "$(printf '%s' "$out" | grep -c '^ok   tracked-test.sh')"
  check "and an untracked one written since the last commit runs too" "1" \
    "$(printf '%s' "$out" | grep -c '^ok   fresh-test.sh')"
  check "and the summary says git answered discovery" "1" "$(printf '%s' "$out" | grep -c 'discovered by git')"
fi

# Two runs that read different file sets must not print one line, for the same reason the containment
# tail exists: outside a checkout git cannot answer and the fallback reads every file under the root.
out="$("$runner" "$tmp/green" 2>&1)"
check "outside a checkout the summary says find answered instead" "1" \
  "$(printf '%s' "$out" | grep -c 'discovered by find')"

mkdir -p "$tmp/skip/node_modules/pkg"
printf '#!/usr/bin/env bash\nexit 1\n' > "$tmp/skip/node_modules/pkg/vendor-test.sh"
printf '#!/usr/bin/env bash\necho "1 passed, 0 failed"\n' > "$tmp/skip/mine-test.sh"
out="$("$runner" "$tmp/skip" 2>&1)"; rc=$?
check "a vendored suite under node_modules is not run" "0" "$rc"

echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
