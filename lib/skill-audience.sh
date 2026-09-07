#!/usr/bin/env bash
#
# Who a skill is for, read from its own frontmatter — the one question both installers ask of every
# skill they find, and the reason neither carries a list of names.
#
# Sourced, never executed. It reports nothing itself: `unknown_audience` prints a value for its
# caller to refuse over, and `is_maintainer_only` answers yes or no. Which of those is fatal, and in
# whose words, is the caller's.
#
# tested by: the two installers' suites, ai/bootstrap-test.sh and ai/install-project-test.sh, because
# an audience is only real once an install has acted on it.
set -uo pipefail

is_maintainer_only() { # <SKILL.md>
  [ -r "$1" ] || return 1
  awk '
    NR == 1 { if ($0 !~ /^---[[:space:]]*$/) exit; next }
    /^---[[:space:]]*$/ { closed = 1; exit }
    tolower($0) ~ /^audience:[[:space:]]*maintainer[[:space:]]*$/ { found = 1 }
    END { if (closed && found) exit 0; exit 1 }
  ' "$1"
}

unknown_audience() { # <SKILL.md>, prints the value and exits 0 when there is one
  [ -r "$1" ] || return 1
  awk '
    NR == 1 { if ($0 !~ /^---[[:space:]]*$/) exit; next }
    /^---[[:space:]]*$/ { closed = 1; exit }
    tolower($0) ~ /^audience:/ && tolower($0) !~ /^audience:[[:space:]]*maintainer[[:space:]]*$/ {
      # The first one only, and the raw text rather than the lowered line: it is echoed back to
      # whoever typed it, and a reader hunting `Maintainr` should find what they wrote.
      if (!found) { found = 1; value = substr($0, index($0, ":") + 1) }
    }
    END {
      if (!closed || !found) exit 1
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
      print value
      exit 0
    }
  ' "$1"
}
