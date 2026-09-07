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

# Whether a skill exists to maintain this instruction tree rather than to work in any repository. The
# audience is declared in the skill's own frontmatter, so discovery below stays discovery: a
# maintainer-only skill added tomorrow is excluded without anyone editing this file, and a list of
# three names here would be wrong the day a fourth is marked.
#
# The block opens on line 1 and has to close before any line in it counts, which is the rule the Go
# reader states in ai/tools/shell/markdown.go — an `audience:` line in the prose is prose, and an
# unterminated block is not frontmatter. The pattern below is that reader's, character for character,
# because eco-check's mount scan asks the same question of the same files and the two answers cannot
# be allowed to differ.
is_maintainer_only() { # <SKILL.md>
  [ -r "$1" ] || return 1
  awk '
    NR == 1 { if ($0 !~ /^---[[:space:]]*$/) exit; next }
    /^---[[:space:]]*$/ { closed = 1; exit }
    tolower($0) ~ /^audience:[[:space:]]*maintainer[[:space:]]*$/ { found = 1 }
    END { if (closed && found) exit 0; exit 1 }
  ' "$1"
}

# The value on an `audience:` line neither reader knows, printed for the refusal below. Its block rule
# is the one above, character for character, for the same reason.
#
# Asked at all because "is this the marker" cannot tell `audience: maintainr` from a skill that
# declared nothing: both answer no, and the typo installs for everyone while the human who wrote it
# believes they marked it. Nothing on the resulting machine looks wrong. `maintainer` is the only
# value there is, so anything else is refused by name — the same answer ai/tools/bloat-judge's
# deadline override gives an option it does not understand, for the same reason.
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
