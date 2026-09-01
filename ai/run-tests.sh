#!/usr/bin/env bash
# Runs every shell test suite in this repository — the `*-test.sh` files sitting beside the scripts
# they cover. CI calls this, and so can you.
#   usage: run-tests.sh [<root>]   # <root> defaults to the repository this script lives in
#
# Discovery rather than a list, so a suite written tomorrow runs without anyone remembering to
# register it. The cost of discovery is a gate that finds nothing and reports success, so finding
# zero suites exits 2 here rather than passing empty.
# tested by: run-tests-test.sh
set -uo pipefail
export LC_ALL=C

# `CDPATH=`: set in the environment, `cd` echoes the directory it landed on, so the default root comes
# back two lines long and the check below refuses a directory that is really there.
root="${1:-$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)}"
[ -d "$root" ] || {
  echo "run-tests.sh: not a directory: $root" >&2
  exit 2
}

# Discovery asks git first, because every file this finds is then executed. `--cached --others
# --exclude-standard` is tracked files plus new untracked ones and nothing else: a suite written five
# minutes ago still runs without anyone registering it, while a build artefact, a vendored tree or
# anything else .gitignore already excludes does not get to execute as a suite.
#
# `find` stays as the fallback, because the suites can legitimately be run over a directory that is
# not a checkout, and that path keeps the node_modules exclusion it always had. Which one answered is
# reported: two runs that read different file sets must not print one line.
suites=()
if git -C "$root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  discovery="git"
  while IFS= read -r suite; do
    [ -n "$suite" ] || continue
    suites+=("$root/$suite")
  done < <(git -C "$root" ls-files --cached --others --exclude-standard -- '*-test.sh' | sort -u)
else
  discovery="find"
  while IFS= read -r suite; do
    suites+=("$suite")
  done < <(find "$root" -name "*-test.sh" -type f -not -path "*/node_modules/*" | sort)
fi

[ "${#suites[@]}" -gt 0 ] || {
  echo "run-tests.sh: no *-test.sh under $root — read this as discovery broken, never as a clean run" >&2
  exit 2
}

# A suite's own count for a named field, read by name rather than position: score-test.sh reports
# three fields where most report two, so counting words in would take the wrong number.
summary_field() { # <field> <the suite's last line>
  RT_FIELD="$1" RT_LINE="$2" awk 'BEGIN {
    n = split(ENVIRON["RT_LINE"], word, /[ ,]+/)
    for (i = 2; i <= n; i++) {
      if (word[i] == ENVIRON["RT_FIELD"] && word[i - 1] ~ /^[0-9]+$/) { print word[i - 1]; exit }
    }
  }'
}

# The repository's own dirty set, so a suite that writes into the checkout is caught once here
# rather than needing a case in every suite. Empty output with a non-zero status means git could not
# answer, which is not the same as a clean tree — the caller distinguishes them.
tree_state() {
  git -C "$root" status --porcelain 2>/dev/null
}

passed=0
failed=0
unmeasured=0
# A flag rather than a fourth count: containment is one fact about the run, not a tally of suites, and
# a counter whose only values are 0 and 1 is a boolean wearing a measurement's shape
# (`~/.kk-flavor/standards/testing.md` → **7. What a suite reports**). It rides out on the same
# summary slot as `containment unchecked`, so the three things this run can say about containment
# read as three spellings of one field.
checkout_moved=0

before_tree="$(tree_state)"
tree_readable=$?
containment=""
for suite in "${suites[@]}"; do
  name="${suite#"$root"/}"
  output="$(bash "$suite" 2>&1)"
  status=$?
  last="$(printf '%s' "$output" | tail -1)"

  # Exit 2 is a suite saying it did not measure — a dependency missing, a machine too loaded to time
  # anything. Counted apart and never folded into failures: unproven is not disproven.
  if [ "$status" -eq 2 ]; then
    printf 'NOMEASURE %-47s %s\n' "$name" "$last"
    unmeasured=$((unmeasured + 1))
    continue
  fi

  if [ "$status" -ne 0 ]; then
    printf 'FAIL %s\n' "$name"
    printf '%s\n' "$output" | tail -15 | sed 's/^/     /'
    failed=$((failed + 1))
    continue
  fi

  # Exiting 0 having passed nothing and skipped nothing means no case ran, and a green exit over no
  # cases is the one failure discovery cannot otherwise see. Zero passed *with* a skip count is a
  # different fact: the cases exist and this machine declined them by name.
  ran="$(summary_field passed "$last")"
  declined="$(summary_field skipped "$last")"
  if [ "${ran:-0}" -eq 0 ] && [ "${declined:-0}" -eq 0 ]; then
    printf 'VACUOUS %-49s %s\n' "$name" "$last"
    failed=$((failed + 1))
    continue
  fi

  printf 'ok   %-52s %s\n' "$name" "$last"
  passed=$((passed + 1))
done

# A suite that passes while corrupting the checkout has measured something, and the measurement was
# not the whole effect. bootstrap-test.sh linked a temp HOME at this repository and wrote through the
# link, replacing real config files while reporting 43 passed.
#
# It cannot say which caused a delta: a concurrent editor looks the same from here. Naming a suite
# would be a false diagnosis, which sends someone hunting through code that is fine. Where the
# checkout belongs to one run — a fresh GitHub-hosted runner, which is what gates.yml uses — the only
# candidate is a suite. That is a property of the workspace and not of CI: a self-hosted runner
# reusing its workspace, or two jobs sharing a checkout, is as ambiguous as a laptop. Nothing here
# detects which it is, and the ambient variables that would guess at it — CI, GITHUB_ACTIONS, set by
# any runner and by anyone who exports them — would make that same false diagnosis wearing the
# authority of code. So this reports what it saw and diagnoses nothing.
#
# What it saw is its own result rather than a failure. `failed` counts suites that went red, and
# folding a delta no suite need have caused into it makes the summary assert a red suite that does
# not exist — `16 suite(s) found: 16 passed, 1 failed`, which does not even add up. Several sessions
# in one checkout fire this on ordinary editing, and a reader who meets a phantom failure often
# enough stops reading the line: that is the run where a suite really did write into its own
# repository, rendered identically to the noise.
if [ "$tree_readable" -eq 0 ]; then
  after_tree="$(tree_state)"
  if [ "$before_tree" != "$after_tree" ]; then
    echo
    echo "the checkout changed while the suites ran, so this result is not trustworthy. Either a suite"
    echo "wrote into the repository it is testing, or something else edited the tree during the run —"
    echo "the second is common with several sessions in one checkout and impossible here to tell apart:"
    diff <(printf '%s\n' "$before_tree") <(printf '%s\n' "$after_tree") | sed 's/^/     /'
    containment=", the checkout moved"
    checkout_moved=1
  fi
else
  # Not a defect: the suites can be run outside a checkout, and they did measure. But the summary must
  # not read the same as a run that did verify containment — two runs that checked different things
  # printing one line is how the difference stops being visible.
  containment=", containment unchecked"
fi

printf '\n%s suite(s) found: %s passed, %s failed, %s unmeasured%s, discovered by %s\n' \
  "${#suites[@]}" "$passed" "$failed" "$unmeasured" "$containment" "$discovery"

# A red outranks a non-measurement: something is known to be wrong, which is the more urgent fact. A
# moved checkout sits between the two — it refuses every line of the result rather than one of them,
# which outranks a single suite declining to measure, while a suite known to be red is more urgent
# still.
#
# Exit 3 for it, on `kk-flavor/scripts/score.sh`'s vocabulary: 2 is did-not-measure, 3 is
# ran-and-refuses-the-result, and a caller that cannot tell those apart reads a live refusal as a dead
# tool. Non-zero either way, so a workspace that really is exclusive still reddens gates.yml, which
# runs this bare and fails the step on any status.
[ "$failed" -eq 0 ] || exit 1
[ "$checkout_moved" -eq 0 ] || exit 3
[ "$unmeasured" -eq 0 ] || exit 2
