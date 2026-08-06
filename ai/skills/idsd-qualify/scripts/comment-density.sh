#!/usr/bin/env bash
# Comment-density detector — flags changed source files whose ADDED lines are comment-heavy, so the
# comment pass starts knowing where to hunt instead of strolling the diff evenly.
#   usage: comment-density.sh [<git-diff args>]   # defaults to HEAD (all uncommitted changes)
#   env:   COMMENT_MAX_RATIO — flag above this comments/(comments+code) share of added lines (default 0.3)
#          COMMENT_MIN_LINES — ignore files with fewer added comment lines than this (default 5)
#          DENSITY_MAX_FILE_BYTES — skip untracked files larger than this (default 262144; that large
#          is machine-generated, not authored comments)
# Prints each outlier with its counts; exits 1 when any found, 0 when clean, and 2 when git rejected
# the arguments — a scan that did not run is not evidence, so never read 2 as clean. Prose/data files
# (md, txt, json, lockfiles) don't count — their "comments" are content. With no diff args, untracked
# text files are scanned too; the index is never touched.
set -uo pipefail
# Byte-level text processing — the C locale keeps sed/awk from choking on stray non-UTF-8 bytes.
export LC_ALL=C

max_ratio="${COMMENT_MAX_RATIO:-0.3}"
min_lines="${COMMENT_MIN_LINES:-5}"
max_file_bytes="${DENSITY_MAX_FILE_BYTES:-262144}"

{
  # `|| exit 2`: this group's status is the trailing `if`'s, not git's, so a rejected argument would
  # leave it at 0 and a scan that never ran would be indistinguishable from a clean one. Exiting here
  # leaves the pipe empty, so awk finds nothing and exits 0 and pipefail surfaces the 2. Exiting works
  # because the group is the left side of a pipeline, and so already a subshell.
  git diff "${@:-HEAD}" || exit 2
  if [ "$#" -eq 0 ]; then
    git ls-files --others --exclude-standard -z | while IFS= read -r -d '' file; do
      # Binary = NUL in the first 8KB. if-guard, not &&: a skipped file must not fail the loop —
      # pipefail would read that as "found".
      bytes="$(wc -c < "./$file" 2>/dev/null || echo 0)"
      if [ "$bytes" -le "$max_file_bytes" ] &&
        [ "$(head -c 8192 "./$file" 2>/dev/null | tr -cd '\000' | wc -c)" -eq 0 ]; then
        printf '+++ b/%s\n' "$file"
        sed 's/^/+/' "./$file" 2>/dev/null || true
      fi
    done
  fi
} | awk -v max_ratio="$max_ratio" -v min_lines="$min_lines" '
  /^\+\+\+ b\// { file = substr($0, 7); next }
  /^\+\+\+/ { next }
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
    for (file in comments) {
      total = comments[file] + code[file]
      ratio = comments[file] / total
      if (comments[file] >= min_lines && ratio > max_ratio) {
        found = 1
        printf "%s: %d comment / %d code added lines (%.2f)\n", file, comments[file], code[file], ratio
      }
    }
    exit found
  }
'
