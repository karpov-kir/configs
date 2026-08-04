#!/usr/bin/env bash
# Repeated-long-literal detector — finds byte-identical long strings appearing 2+ times among the
# diff's ADDED lines (copy-pasted tokens, keys, fixtures). Human reviewers reliably miss these:
# one run shipped nine identical ~900-char DRM tokens past two review stages. Mechanical, so it
# runs before the refactor stage and its hits ride the spawn prompt's tool-output slot as evidence.
#   usage: dup-literals.sh [<git-diff args>]   # defaults to HEAD (all uncommitted changes)
#   env:   DUP_MIN_LEN — minimum literal length in chars (default 100)
#          DUP_MAX_FILE_BYTES — skip untracked files larger than this (default 262144)
# Prints each duplicate (count, length, 60-char prefix); exits 1 when any found, 0 when clean.
# With no diff args, untracked text files are scanned too (they never appear in `git diff HEAD`);
# the index is never touched. Untracked files above DUP_MAX_FILE_BYTES are skipped: a file that large is
# machine-generated (a test-run dump, a coverage report), and its internal repetition says nothing about
# the change while crowding out the real hits — one run's evidence was three-quarters dump noise.
set -uo pipefail
# Byte-level text processing throughout — the C locale keeps tr/sed/awk from choking on
# stray non-UTF-8 bytes in diffs and fixtures. All patterns here are ASCII.
export LC_ALL=C

min="${DUP_MIN_LEN:-100}"
max_file_bytes="${DUP_MAX_FILE_BYTES:-262144}"

{
  git diff "${@:-HEAD}"
  if [ "$#" -eq 0 ]; then
    git ls-files --others --exclude-standard -z | while IFS= read -r -d '' file; do
      # Binary = NUL in the first 8KB (BSD grep -I is unreliable here). if-guard, not &&: a
      # skipped file must not fail the loop — pipefail would read that as "found".
      bytes="$(wc -c < "./$file" 2>/dev/null || echo 0)"
      if [ "$bytes" -le "$max_file_bytes" ] &&
        [ "$(head -c 8192 "./$file" 2>/dev/null | tr -cd '\000' | wc -c)" -eq 0 ]; then
        # || true: a file that vanished mid-scan contributes nothing, same as any other skip.
        sed 's/^/+/' "./$file" 2>/dev/null || true
      fi
    done
  fi
} | awk -v min="$min" '
  /^\+\+\+/ { next }
  /^\+/ {
    raw = substr($0, 2)
    # Whole trimmed line, for copy-pasted statements.
    trimmed = raw
    gsub(/^[[:space:]]+|[[:space:]]+$/, "", trimmed)
    if (length(trimmed) >= min) lines[trimmed]++
    # Whitespace/quote-delimited tokens, for long literals embedded in differing lines.
    count = split(raw, parts, /[[:space:]"'"'"'`,;()]+/)
    for (i = 1; i <= count; i++) if (length(parts[i]) >= min) tokens[parts[i]]++
  }
  END {
    found = 0
    for (token in tokens) if (tokens[token] >= 2) {
      found = 1
      printf "%dx token (%d chars): %.60s…\n", tokens[token], length(token), token
    }
    for (line in lines) if (lines[line] >= 2 && !(line in tokens)) {
      found = 1
      printf "%dx line  (%d chars): %.60s…\n", lines[line], length(line), line
    }
    exit found
  }
'
