#!/usr/bin/env bash
#
# The one assertion that had been written out three times over — ai/mcp-env-test.sh,
# ai/mcp-sync-test.sh and ai/run-tests-test.sh each carried its own `check`, two of them
# byte-identical and the third differing only in a format verb. Sourced, never executed: the filename
# does not end in `-test.sh`, so run-tests.sh's discovery does not pick it up as a suite of its own.
#
# It holds `check` and the two counters it moves, and deliberately NOT the summary line. Under
# `~/.kk-flavor/standards/testing.md` → **7. What a suite reports**, the shape of that line is itself
# an assertion about the suite printing it: two fields say "no case in here is conditional", three say
# the opposite. A shared summary would erase that per-suite claim rather than remove a duplicate, so
# each suite still prints its own and this file stays out of it.
#
# Nothing here creates a directory, writes a file or installs a trap. That is what lets a suite with no
# scratch of its own — ai/mcp-env-test.sh says so in its header, and means it — source this without
# acquiring either. lib/test-harness.sh is the other shared file and is not that: it is the bootstrap
# suites' fixtures, it mktemps at source time, and it has no `check` at all.
#
# tested by: ai/mcp-env-test.sh, ai/mcp-sync-test.sh, ai/run-tests-test.sh,
# ai/run-tests-concurrency-test.sh — a break here goes red in all four at once, which is the point of
# there being one copy.

pass=0
fail=0

# %q rather than %s on the two values: it is the difference between a reader seeing "these look
# identical" and seeing the trailing newline, the empty string or the tab that actually differed. Two
# printfs rather than one, so a multi-line expected value cannot carry the `actual:` label off past
# where anyone is still reading.
check() { # <name> <expected> <actual>
  local name="$1" expected="$2" actual="$3"
  if [ "$expected" = "$actual" ]; then
    echo "ok   — $name"
    pass=$((pass + 1))
  else
    echo "FAIL — $name"
    printf '       expected: %q\n' "$expected"
    printf '       actual:   %q\n' "$actual"
    fail=$((fail + 1))
  fi
}
