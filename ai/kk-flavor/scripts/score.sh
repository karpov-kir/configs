#!/usr/bin/env bash
# Apply `~/.kk-flavor/standards/writing.md` → **Score what survives**: hold the per-lane thresholds,
# and cut a scored list against one. It never produces a score — "how much this reader needs it" is a
# judgment, so the caller scores and this decides what that score buys.
#
# Usage: score.sh threshold <lane>
#        score.sh cut [--kept-all <why>] <lane> <what a 10 is here>
#            reads `<score><TAB><label>` lines on stdin; exit 3 if nothing fell below the line
#
# Prints to stdout. Exit 2 means it did not run; exit 3 means it ran and refuses the result.
#
# Three things here are the enforcement rather than the convenience. An unknown lane exits instead of
# falling back to `default`, because a caller that cannot find its number invents one, and an invented
# threshold is indistinguishable in the output from the ruled one. `cut` refuses to run without the
# anchor argument, which makes the caller write what a 10 is before any score is read. And a run that
# cuts nothing exits 3, because that is what scoring against no anchor produces and the anchor
# refusal cannot see it — the anchor is a free string written before the scores exist.
#
# tested by: score-test.sh
set -euo pipefail

# `CDPATH=`: set in the environment, it makes `cd` resolve a relative path against it and echo the
# directory it landed on, so `here` comes back two lines long and the config resolves nowhere.
here="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
config="$here/../thresholds.conf"

die() {
  printf 'score.sh: %s\n' "$1" >&2
  exit 2
}

# Exit 3 is "the scan ran and its result is refused", against 2's "the scan did not run". A caller
# that cannot tell those apart treats a refusal as a broken tool and moves on.
die3() {
  printf 'score.sh: %s\n' "$1" >&2
  exit 3
}

# The config is read by this one function, and read with a redirect rather than a pipe or a command
# substitution: either of those puts the loop in a subshell, where `die` exits that subshell and the
# caller carries on with an empty result. A malformed config would then be reported as a missing lane.
#
# Field-by-field, never a pattern built from the config or from `$want`: a lane name is data, and
# reaching a regexp it would be metacharacters.
threshold_for() {
  local want="$1" name verb op level rest found= known=
  # `|| [ -n "$name" ]` for the same reason the stdin loop carries it: a config whose last lane has no
  # trailing newline would otherwise lose that lane, and the failure reads as "no lane 'x'" — a
  # missing-lane message for a lane that is right there.
  while read -r name verb op level rest || [ -n "$name" ]; do
    case "$name" in '' | '#'*) continue ;; esac
    [ "$verb" = cut ] && [ "$op" = '<=' ] && [ -z "$rest" ] ||
      die "$config: cannot read the line naming '$name' — the form is '<lane> cut <= <n>'"
    case "$level" in
      '' | *[!0-9]*) die "$config: '$name' has a non-numeric level" ;;
    esac
    [ "$level" -le 10 ] || die "$config: '$name' is $level, over the 0-10 scale"
    known="$known $name"
    if [ "$name" = "$want" ]; then
      found="$level"
    fi
  done <"$config"
  [ -n "$found" ] || die "no lane '$want' in $config — it lists:$known"
  printf '%s\n' "$found"
}

[ -f "$config" ] || die "no threshold config at $config"
[ $# -ge 2 ] || die "usage: score.sh threshold <lane> | score.sh cut <lane> <what a 10 is here>"

case "$1" in
  threshold)
    [ $# -eq 2 ] || die "threshold takes one lane"
    threshold_for "$2"
    ;;
  cut)
    shift
    kept_all_why=
    if [ "${1:-}" = --kept-all ]; then
      [ $# -ge 2 ] && [ -n "${2//[[:space:]]/}" ] ||
        die "--kept-all needs the reason nothing fell below the line, in your own words"
      kept_all_why="$2"
      shift 2
    fi
    [ $# -ge 2 ] || die "cut needs the anchor: what a 10 is for this artifact, in your own words"
    lane="$1"
    shift
    anchor="$*"
    # Whitespace, not just absence: `cut prose ""` satisfies an argument count and anchors nothing,
    # which is the refusal above defeated while still reading as enforced.
    [ -n "${anchor//[[:space:]]/}" ] ||
      die "the anchor is blank — write what a 10 is for this artifact before any score is read"
    level="$(threshold_for "$lane")"
    printf 'lane %s, cutting at or below %s\n10 here means: %s\n\n' "$lane" "$level" "$anchor"
    kept=0 gone=0
    # `|| [ -n "$line" ]`: at EOF without a trailing newline `read` fills the variable and still
    # returns non-zero, so the plain form drops the last item — and a caller piping a heredoc has no
    # trailing newline. Dropping it silently is the worst shape this could fail in: the item is
    # neither kept nor cut, and the counts still add up.
    while IFS= read -r line || [ -n "$line" ]; do
      [ -n "$line" ] || continue
      score="${line%%	*}"
      label="${line#*	}"
      [ "$label" != "$line" ] ||
        die "no tab in '${line//[[:cntrl:]]/ }' — each line is '<score><TAB><label>'"
      # A label is text from the artifact under review, printed back into a report its caller
      # reads. A control character in it rewrites that report rather than appearing in it: `\r`
      # overwrites the verdict already on the line and `\v` opens a second one, so an item this
      # cut can be made to render as one it kept while the counts below still say it was cut.
      score="${score//[[:cntrl:]]/ }"
      label="${label//[[:cntrl:]]/ }"
      case "$score" in
        '' | *[!0-9]*) die "'$score' is not a score 0-10" ;;
      esac
      [ "$score" -le 10 ] || die "'$score' is over the 0-10 scale"
      if [ "$score" -le "$level" ]; then
        printf 'CUT   %2s  %s\n' "$score" "$label"
        gone=$((gone + 1))
      else
        printf 'keep  %2s  %s\n' "$score" "$label"
        kept=$((kept + 1))
      fi
    done
    printf '\n%s kept, %s cut\n' "$kept" "$gone"
    # Everything clearing the bar is what scoring against no anchor looks like — the scale never gets
    # used, every item lands mid-band, and the run reads as a pass. The anchor refusal cannot catch
    # it, because the anchor is a free string written before the scores exist. This can, and it must
    # exit rather than print: a notice at the end of a list is the one thing a caller skims.
    #
    # A tight artifact really can cut nothing, so the exit is refusable — but only by writing down
    # why, which is the judgment the score was avoiding. Exit 3, never 2: the scan ran.
    if [ "$gone" -eq 0 ] && [ "$kept" -gt 0 ]; then
      [ -n "$kept_all_why" ] ||
        die3 "nothing scored at or below $level. Re-score against the anchor, or re-run with --kept-all '<why nothing fell below it>'"
      printf 'nothing cut, accepted: %s\n' "${kept_all_why//[[:cntrl:]]/ }"
    fi
    ;;
  *) die "unknown command '$1' — threshold or cut" ;;
esac
