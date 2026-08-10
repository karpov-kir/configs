#!/usr/bin/env bash
# Comment-density detector — flags changed source files whose ADDED lines are comment-heavy, so the
# comment pass starts knowing where to hunt.
#   usage: comment-density.sh [<git-diff revisions>]   # defaults to HEAD (all uncommitted changes);
#          a path argument is refused with exit 2, never scanned
#   env:   COMMENT_MAX_RATIO — flag above this comments/(comments+code) share of added lines (default 0.3)
#          COMMENT_MIN_LINES — ignore files with fewer added comment lines than this (default 5)
#          DENSITY_MAX_FILE_BYTES — skip untracked files larger than this (default 262144; that large
#          is machine-generated, not authored comments)
# Prints each outlier with its counts. Exits 1 when any found, 0 when clean, and 2 when git rejected
# the arguments. A scan that did not run is not evidence, so never read 2 as clean. Prose/data files
# (md, txt, json, lockfiles) don't count, because their "comments" are content. With no diff args,
# untracked text files are scanned too; the index is never touched.
set -uo pipefail
# Byte-level text processing. The C locale keeps sed/awk from choking on stray non-UTF-8 bytes.
export LC_ALL=C

max_ratio="${COMMENT_MAX_RATIO:-0.3}"
min_lines="${COMMENT_MIN_LINES:-5}"
max_file_bytes="${DENSITY_MAX_FILE_BYTES:-262144}"

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
    # outlier.
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

{
  # `|| exit 2`: this group's status is the trailing `if`'s, not git's. Without it, a rejected
  # argument leaves the status at 0 and a scan that never ran reads as a clean one. Exiting here
  # empties the pipe, so awk finds nothing and exits 0, and pipefail surfaces the 2. `exit` is safe
  # because the group is the left side of a pipeline, and so already a subshell.
  # The awk below keys off `+++ b/<path>` and a leading `+`. Git configs that silently change that
  # shape: `diff.noprefix`, `diff.mnemonicPrefix` (i/ w/), `color.diff = always`, an external diff
  # driver. Under any of them the scan exits 0 with nothing found while an outlier sits in the diff.
  # Pin the shape here; the caller's own flags still come after and win.
  # `core.quotePath=false` is part of that shape. By default git C-quotes any path holding a
  # non-ASCII byte, a quote or a backslash, so `café.ts` arrives as `+++ "b/caf\303\251.ts"`, fails
  # the `b/` test below, and every added line in it is attributed to no file and dropped. The scan
  # then exits 0 over a file that is almost all comment. Under kk-pr-review the filename is chosen
  # by the author of the branch under review.
  # `--text` because the PR author owns `.gitattributes`: `* -diff` there, or one NUL byte in the
  # file, makes git emit `Binary files … differ` instead of a diff — the scan then finds nothing
  # and exits 0 over a real hit. Forcing text pushes genuine binaries through too; every printed
  # span is length-bounded already.
  git -c core.quotePath=false diff --no-ext-diff --no-textconv --no-color --text --src-prefix=a/ --dst-prefix=b/ "${@:-HEAD}" || {
    echo "comment-density.sh: git rejected these arguments — exit 2, the scan did NOT run. Not a clean result." >&2
    exit 2
  }
  if [ "$#" -eq 0 ]; then
    git ls-files --others --exclude-standard -z | while IFS= read -r -d '' file; do
      # Binary = NUL in the first 8KB. if-guard, not &&: a skipped file must not fail the loop —
      # pipefail would read that as "found".
      bytes="$(wc -c < "./$file" 2>/dev/null || echo 0)"
      # A newline in the name lets the path itself write a second line into this stream, which is
      # the one forged header a tracked branch cannot produce — there the shape comes from git.
      # Skip such a file, and say so on stderr rather than dropping it silently.
      # `$'\n'` and not `"$(printf '\n')"`: command substitution strips trailing newlines, so that
      # form is the empty string, the pattern collapses to `*`, and every untracked file is skipped.
      case "$file" in
        *$'\n'*)
          echo "comment-density.sh: skipping an untracked path whose name contains a newline; it was NOT scanned." >&2
          continue
          ;;
      esac
      if [ "$bytes" -le "$max_file_bytes" ] &&
        [ "$(head -c 8192 "./$file" 2>/dev/null | tr -cd '\000' | wc -c)" -eq 0 ]; then
        printf 'diff --git a/%s b/%s\n+++ b/%s\n' "$file" "$file" "$file"
        # awk rather than `sed 's/^/+/'`, because awk's print always terminates the record and sed
        # passes a missing final newline straight through. Unterminated, this file's last line fuses
        # with the next file's `diff --git` header, which then no longer starts a line: the anchor
        # below never fires, and every added line of that next file is counted against this one.
        # || true: a file that vanished mid-scan contributes nothing, same as any other skip.
        awk '{ print "+" $0 }' "./$file" 2>/dev/null || true
      fi
    done
  fi
} | awk -v max_ratio="$max_ratio" -v min_lines="$min_lines" '
  # Which file the lines below belong to is read only from a header git actually emitted.
  # `diff --git` is the anchor because every line in a diff *body* carries a `+`, `-` or space
  # prefix, so no file content can forge one. Without the anchor, an added line reading `++ b/x.txt`
  # renders as `+++ b/x.txt` and reassigns the file to a name the prose filter below then skips.
  # Every added line after it disappears, and the scan exits 0 over a file that is almost all comment.
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
    # The path comes from the diff, so under kk-pr-review it is a name the branch author chose, and
    # this output is read by an agent that drafts a review. Both the path and the number of lines are
    # bounded for that reason. A suppressed outlier is announced, never dropped: the count stays
    # exact, which holds only while the cap below and the one in the announcement are the same
    # number. The `%.200s` on the path is a separate bound that happens to share the value.
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
