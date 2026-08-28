#!/usr/bin/env bash
# Repeated-long-literal detector — byte-identical long strings appearing 2+ times among the diff's
# ADDED lines: copy-pasted tokens, keys, fixtures. Run by kk-refactor's setup, and by a pipeline
# orchestrator before the refactor stage.
#   usage: dup-literals.sh [<git-diff revisions>]   # defaults to HEAD (all uncommitted changes);
#          a path argument is refused with exit 2, never scanned
#   env:   DUP_MIN_LEN — minimum literal length in chars (default 100)
#          DUP_MAX_FILE_BYTES — skip untracked files larger than this (default 262144)
# Prints each duplicate (count, length, 60-char prefix). Exits 1 when any found, 0 when clean, 2 when
# git rejected the arguments. With no diff args, untracked text files are scanned too; the index is
# never touched. Why revisions-only, why these `git diff` flags, and why the `|| exit 2` below:
# `~/.claude/skills/kk-humanize/scripts/comment-density.sh`, which runs the same shape.
# tested by: dup-literals-test.sh, whose 65 cases are each proven able to fail by shell-mutate.sh —
# one guard broken at a time in a copy, the named case required to redden and nothing else with it.
set -uo pipefail
export LC_ALL=C

min_length="${DUP_MIN_LEN:-100}"
max_file_bytes="${DUP_MAX_FILE_BYTES:-262144}"

if [ "$#" -gt 0 ]; then
  for arg in "$@"; do
    [ "$arg" = "--" ] && break
    case "$arg" in
      -*)
        echo "dup-literals.sh: '$arg' is an option, not a git-diff revision — the scan did NOT run." >&2
        echo "  this script takes revisions only (HEAD, origin/main, a..b); paths go after '--'." >&2
        exit 2
        ;;
    esac
    if [ -e "$arg" ] && ! git rev-parse --verify --quiet "$arg^{}" >/dev/null 2>&1; then
      echo "dup-literals.sh: '$arg' is a path, not a git-diff revision — the scan did NOT run." >&2
      echo "  pass a revision (HEAD, origin/main, a..b); paths, if you must, go after '--'." >&2
      exit 2
    fi
  done
fi

emit_untracked_as_added_lines() {
  git ls-files --others --exclude-standard -z | while IFS= read -r -d '' file; do
    # Binary = NUL in the first 8KB. if-guard, not &&: a skipped file must not fail the loop.
    bytes="$(wc -c < "./$file" 2>/dev/null || echo 0)"
    if [ "$bytes" -le "$max_file_bytes" ] &&
      [ "$(head -c 8192 "./$file" 2>/dev/null | tr -cd '\000' | wc -c)" -eq 0 ]; then
      # awk, not `sed 's/^/+/'`: sed passes a missing final newline through, fusing this file's
      # last line with the next file's first, so neither is compared as the literal it is.
      awk '{ print "+" $0 }' "./$file" 2>/dev/null || true
    fi
  done
}

{
  # The `if` below stays an `if` for the group's exit status: `&&` would leave the group at 1 whenever
  # revisions were passed, and under `pipefail` 1 is this script's "duplicates found".
  git diff --no-ext-diff --no-textconv --no-color --text "${@:-HEAD}" || {
    echo "dup-literals.sh: git rejected these arguments — exit 2, the scan did NOT run. Not a clean result." >&2
    exit 2
  }
  if [ "$#" -eq 0 ]; then
    emit_untracked_as_added_lines
  fi
} | awk -v min_length="$min_length" '
  # Skip the `+++` of a real header only: unanchored, this also drops any added line starting `++`,
  # so a duplicated literal shaped like that slips past. `diff --git` is the anchor — no line in a
  # diff *body* can forge one.
  /^diff --git / { pending = 1; next }
  /^\+\+\+ / { if (pending) { pending = 0; next } }
  /^\+/ {
    raw = substr($0, 2)
    # Whole trimmed line, for copy-pasted statements.
    trimmed = raw
    gsub(/^[[:space:]]+|[[:space:]]+$/, "", trimmed)
    if (length(trimmed) >= min_length) lines[trimmed]++
    # Whitespace/quote-delimited tokens, for long literals embedded in differing lines.
    count = split(raw, parts, /[[:space:]"'"'"'`,;()]+/)
    for (i = 1; i <= count; i++) if (length(parts[i]) >= min_length) tokens[parts[i]]++
  }
  END {
    found = 0
    shown = 0
    # `%.60s` bounds each line; this bounds how many there are — under kk-pr-review both come from a
    # branch someone else wrote. A suppressed duplicate is announced, never dropped, which holds only
    # while the cap on the two loops and the one in the announcement stay the same.
    max_shown = 200
    for (token in tokens) if (tokens[token] >= 2) {
      found++
      if (++shown <= max_shown) printf "%dx token (%d chars): %.60s…\n", tokens[token], length(token), token
    }
    for (line in lines) if (lines[line] >= 2 && !(line in tokens)) {
      found++
      if (++shown <= max_shown) printf "%dx line  (%d chars): %.60s…\n", lines[line], length(line), line
    }
    if (found > max_shown) printf "… and %d further duplicate(s), not shown\n", found - max_shown
    exit (found > 0)
  }
'
