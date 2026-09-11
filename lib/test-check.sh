#!/usr/bin/env bash
#
# The shared `check`, and the two counters it moves. Sourced, never executed: the filename does not end
# in `-test.sh`, so run-tests.sh's discovery passes over it.
#
# Not the summary line. Under `~/.kk-flavor/standards/testing.md` → **7. What a suite reports** that
# line's shape is itself an assertion about the suite printing it, and the suites sourcing this print
# three different shapes — so a shared one would take the shape as an argument, which is the suite
# stating it either way.
#
# Nothing here creates a directory, writes a file or installs a trap, which is what lets a suite with
# no scratch of its own source it. lib/test-harness.sh mktemps and traps EXIT at source time, so it
# could not grow a `check` without forcing both on every caller.
#
# tested by: ai/mcp-env-test.sh, ai/mcp-sync-test.sh, ai/run-tests-test.sh,
# ai/run-tests-concurrency-test.sh — a break here goes red in all four at once.

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
