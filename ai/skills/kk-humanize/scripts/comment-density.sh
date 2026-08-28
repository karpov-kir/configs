#!/usr/bin/env bash
# Comment-density detector — flags changed source files whose ADDED lines are comment-heavy.
#   usage: comment-density.sh [<git-diff revisions>]   # defaults to HEAD (all uncommitted changes);
#          a path argument is refused with exit 2, never scanned
#   env:   COMMENT_MAX_RATIO — flag above this comments/(comments+code) share of added lines (default 0.3)
#          COMMENT_MIN_LINES — ignore files with fewer added comment lines than this (default 5)
#          DENSITY_MAX_FILE_BYTES — skip untracked files larger than this (default 262144)
# Prints each outlier with its counts. Exits 1 when any found, 0 when clean, 2 when git rejected the
# arguments. Prose/data files (md, txt, json, lockfiles) don't count. With no diff args, untracked
# text files are scanned too; the index is never touched.
# A targeting aid, not a bar: it counts ADDED lines, so rewording a comment the base already carried
# moves it into the added set, and the ratio can rise across a pass that cut comments.
# tested by: comment-density-test.sh, whose 66 cases are each proven able to fail by shell-mutate.sh
# — one guard broken at a time in a copy, the named case required to redden and nothing else with it.
set -uo pipefail
export LC_ALL=C

max_ratio="${COMMENT_MAX_RATIO:-0.3}"
min_lines="${COMMENT_MIN_LINES:-5}"
max_file_bytes="${DENSITY_MAX_FILE_BYTES:-262144}"

# Arguments are git-diff *revisions*, never paths — `git diff <path>` is legal and diffs against the
# index, so a path silently scans the wrong change set and exits 0, indistinguishable from clean.
if [ "$#" -gt 0 ]; then
  for arg in "$@"; do
    [ "$arg" = "--" ] && break
    # Refused, not skipped: `--output=` alone drains the pipe, so the scan exits 0 over a real outlier.
    case "$arg" in
      -*)
        echo "comment-density.sh: '$arg' is an option, not a git-diff revision — the scan did NOT run." >&2
        echo "  this script takes revisions only (HEAD, origin/main, a..b); paths go after '--'." >&2
        exit 2
        ;;
    esac
    if [ -e "$arg" ] && ! git rev-parse --verify --quiet "$arg^{}" >/dev/null 2>&1; then
      echo "comment-density.sh: '$arg' is a path, not a git-diff revision — the scan did NOT run." >&2
      echo "  pass a revision (HEAD, origin/main, a..b); paths, if you must, go after '--'." >&2
      exit 2
    fi
  done
fi

emit_untracked_as_diff() {
  git ls-files --others --exclude-standard -z | while IFS= read -r -d '' file; do
    # Binary = NUL in the first 8KB. if-guard, not &&: a skipped file must not fail the loop.
    bytes="$(wc -c < "./$file" 2>/dev/null || echo 0)"
    # A newline in the name would write a second line into this stream — the one forged header a
    # tracked branch cannot produce. `$'\n'`, not `"$(printf '\n')"`: substitution strips the
    # newline, the pattern collapses to `*`, and every untracked file is skipped.
    case "$file" in
      *$'\n'*)
        echo "comment-density.sh: skipping an untracked path whose name contains a newline; it was NOT scanned." >&2
        continue
        ;;
    esac
    if [ "$bytes" -le "$max_file_bytes" ] &&
      [ "$(head -c 8192 "./$file" 2>/dev/null | tr -cd '\000' | wc -c)" -eq 0 ]; then
      printf 'diff --git a/%s b/%s\n+++ b/%s\n' "$file" "$file" "$file"
      # awk, not `sed 's/^/+/'`: sed passes a missing final newline through, fusing this file's
      # last line with the next file's `diff --git` header so the anchor below never fires.
      # || true: a file that vanished mid-scan contributes nothing.
      awk '{ print "+" $0 }' "./$file" 2>/dev/null || true
    fi
  done
}

{
  # `|| exit 2`: this group's status is the trailing `if`'s, not git's, so without it a scan that
  # never ran reads as clean. Safe — the group is a pipeline's left side, already a subshell. The `if`
  # stays an `if` for that same status: `&&` would leave the group at 1 whenever revisions were passed.
  # The flags pin the shape the awk keys off (`+++ b/<path>`, a leading `+`), which `diff.noprefix`,
  # `color.diff=always` or an external diff driver break. `core.quotePath=false`: else a non-ASCII
  # path arrives C-quoted and fails the `b/` test. `--text`: else `* -diff` in the PR author's
  # `.gitattributes`, or one NUL byte, yields `Binary files … differ`. Each exits 0 over a real hit.
  git -c core.quotePath=false diff --no-ext-diff --no-textconv --no-color --text --src-prefix=a/ --dst-prefix=b/ "${@:-HEAD}" || {
    echo "comment-density.sh: git rejected these arguments — exit 2, the scan did NOT run. Not a clean result." >&2
    exit 2
  }
  if [ "$#" -eq 0 ]; then
    emit_untracked_as_diff
  fi
} | awk -v max_ratio="$max_ratio" -v min_lines="$min_lines" '
  # `diff --git` is the anchor: every line in a diff *body* carries a `+`, `-` or space prefix, so no
  # file content can forge one. Without it an added line `++ b/x.txt` reassigns the file, and every
  # added line after it disappears.
  /^diff --git / { file = ""; pending = 1; next }
  /^\+\+\+ / { if (pending) { pending = 0; if ($0 ~ /^\+\+\+ b\//) file = substr($0, 7) } next }
  /^\+/ {
    line = substr($0, 2)
    gsub(/^[[:space:]]+/, "", line)
    if (line == "" || file == "") next
    if (file ~ /\.(md|markdown|txt|json|lock)$/ || file ~ /(^|\/)[^\/]*lock[^\/]*\.(yaml|yml)$/) next
    if (line ~ /^(\/\/|\/\*|\*\/?([[:space:]]|$)|#)/) comments[file]++
    else code[file]++
  }
  END {
    found = 0
    shown = 0
    # Path and line count are both bounded — under kk-pr-review they come from a branch someone else
    # wrote. A suppressed outlier is announced, never dropped, which holds only while this cap and
    # the one in the announcement are the same number.
    max_shown = 200
    for (file in comments) {
      total = comments[file] + code[file]
      ratio = comments[file] / total
      if (comments[file] >= min_lines && ratio > max_ratio) {
        found++
        if (++shown <= max_shown)
          printf "%.200s: %d comment / %d code added lines (%.2f)\n", file, comments[file], code[file], ratio
      }
    }
    if (found > max_shown) printf "… and %d further outlier(s), not shown\n", found - max_shown
    exit (found > 0)
  }
'
