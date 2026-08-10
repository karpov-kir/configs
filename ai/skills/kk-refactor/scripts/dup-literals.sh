#!/usr/bin/env bash
# Repeated-long-literal detector — byte-identical long strings that appear 2+ times among the diff's
# ADDED lines: copy-pasted tokens, keys, fixtures, and the like. Run by kk-refactor's setup, and by
# a pipeline orchestrator before the refactor stage.
#   usage: dup-literals.sh [<git-diff revisions>]   # defaults to HEAD (all uncommitted changes);
#          a path argument is refused with exit 2, never scanned
#   env:   DUP_MIN_LEN — minimum literal length in chars (default 100)
#          DUP_MAX_FILE_BYTES — skip untracked files larger than this (default 262144; that large
#          is machine-generated, and its own repetition says nothing about the change)
# Prints each duplicate (count, length, 60-char prefix). Exits 1 when any found, 0 when clean, and 2
# when git rejected the arguments. A scan that did not run is not evidence, so never read 2 as clean.
# With no diff args, untracked text files are scanned too (they never appear in `git diff HEAD`);
# the index is never touched.
set -uo pipefail
# Byte-level text processing throughout. The C locale keeps tr/sed/awk from choking on stray
# non-UTF-8 bytes in diffs and fixtures; every pattern here is ASCII.
export LC_ALL=C

min_length="${DUP_MIN_LEN:-100}"
max_file_bytes="${DUP_MAX_FILE_BYTES:-262144}"

# Arguments are git-diff *revisions*. `git diff <path>` is legal: it diffs that path against the
# index. So feeding it `git diff --name-only` output scans the wrong change set, finds nothing, and
# exits 0, which is indistinguishable from clean. Reject a path before the scan instead. A range
# (`origin/main..HEAD`) names no file, so it never reaches the revision probe.
if [ "$#" -gt 0 ]; then
  for arg in "$@"; do
    [ "$arg" = "--" ] && break
    # Options are refused outright, not skipped. The contract is "revisions only", and passing an
    # option through hands the caller git's whole option surface. `--output=` alone overwrites an
    # arbitrary file and drains the pipe, so awk sees nothing and the scan exits 0 over a real
    # duplicate.
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

{
  # `|| exit 2`: this group's status is the trailing `if`'s, not git's. Without it, a rejected
  # argument leaves the status at 0 and a scan that never ran reads as a clean one. Exiting here
  # empties the pipe, so awk finds nothing and exits 0, and pipefail surfaces the 2. `exit` is safe
  # because the group is the left side of a pipeline, and so already a subshell.
  # The awk below reads a leading `+` as "added line". `color.diff = always` puts ANSI bytes in
  # front of it and an external diff driver (`diff.external`, `GIT_EXTERNAL_DIFF`) replaces the
  # output wholesale; under either, the scan finds nothing and exits 0 over a real duplicate.
  # Pin the shape here; the caller's own flags still come after and win.
  # `--text` because the PR author owns `.gitattributes`: `* -diff` there, or one NUL byte in the
  # file, makes git emit `Binary files … differ` instead of a diff — the scan then finds nothing
  # and exits 0 over a real hit. Forcing text pushes genuine binaries through too; every printed
  # span is length-bounded already.
  git diff --no-ext-diff --no-textconv --no-color --text "${@:-HEAD}" || {
    echo "dup-literals.sh: git rejected these arguments — exit 2, the scan did NOT run. Not a clean result." >&2
    exit 2
  }
  if [ "$#" -eq 0 ]; then
    git ls-files --others --exclude-standard -z | while IFS= read -r -d '' file; do
      # Binary = NUL in the first 8KB. if-guard, not &&: a skipped file must not fail the loop —
      # pipefail would read that as "found".
      bytes="$(wc -c < "./$file" 2>/dev/null || echo 0)"
      if [ "$bytes" -le "$max_file_bytes" ] &&
        [ "$(head -c 8192 "./$file" 2>/dev/null | tr -cd '\000' | wc -c)" -eq 0 ]; then
        # awk rather than `sed 's/^/+/'`, because awk's print always terminates the record and sed
        # passes a missing final newline straight through — fusing this file's last line with the
        # next file's first, so neither is ever compared as the literal it is.
        # || true: a file that vanished mid-scan contributes nothing, same as any other skip.
        awk '{ print "+" $0 }' "./$file" 2>/dev/null || true
      fi
    done
  fi
} | awk -v min_length="$min_length" '
  # Skip the `+++` of a real header only. Unanchored, this also drops any added line whose content
  # starts with `++`, so a duplicated literal shaped like that slips past. `diff --git` is the anchor
  # because every line in a diff *body* carries a `+`, `-` or space prefix, so no file content can
  # forge one.
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
    # `%.60s` already bounds each line; this bounds how many there are. Under kk-pr-review both the
    # literals and their count come from a branch someone else wrote, and this output is read by an
    # agent that drafts a review. A suppressed duplicate is announced, never dropped — which holds
    # only while the cap on the two loops and the one in the announcement stay the same number.
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
