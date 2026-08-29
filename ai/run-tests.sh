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

root="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
[ -d "$root" ] || {
  echo "run-tests.sh: not a directory: $root" >&2
  exit 2
}

suites=()
while IFS= read -r suite; do
  suites+=("$suite")
done < <(find "$root" -name "*-test.sh" -type f -not -path "*/node_modules/*" | sort)

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
# would be a false diagnosis, which sends someone hunting through code that is fine. In CI the
# checkout is exclusive, so there the only candidate is a suite.
if [ "$tree_readable" -eq 0 ]; then
  after_tree="$(tree_state)"
  if [ "$before_tree" != "$after_tree" ]; then
    echo
    echo "the checkout changed while the suites ran, so this result is not trustworthy. Either a suite"
    echo "wrote into the repository it is testing, or something else edited the tree during the run —"
    echo "the second is common with several sessions in one checkout and impossible here to tell apart:"
    diff <(printf '%s\n' "$before_tree") <(printf '%s\n' "$after_tree") | sed 's/^/     /'
    failed=$((failed + 1))
  fi
else
  # Not a defect: the suites can be run outside a checkout, and they did measure. But the summary must
  # not read the same as a run that did verify containment — two runs that checked different things
  # printing one line is how the difference stops being visible.
  containment=", containment unchecked"
fi

printf '\n%s suite(s) found: %s passed, %s failed, %s unmeasured%s\n' \
  "${#suites[@]}" "$passed" "$failed" "$unmeasured" "$containment"

# A red outranks a non-measurement: something is known to be wrong, which is the more urgent fact.
[ "$failed" -eq 0 ] || exit 1
[ "$unmeasured" -eq 0 ] || exit 2
