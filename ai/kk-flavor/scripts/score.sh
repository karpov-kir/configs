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
# Thresholds come from `../thresholds.conf`, which is tracked, overlaid per lane by an untracked
# `${XDG_CONFIG_HOME:-~/.config}/kk-flavor/thresholds.conf` where that exists — so a bar can be tuned
# on one machine without dirtying the repo. An override in effect is always announced: a bar moved
# locally produces a verdict no other machine reproduces, and silence about it is the whole hazard.
#
# Four things here are the enforcement rather than the convenience. An unknown lane exits instead of
# falling back to `default`, because a caller that cannot find its number invents one, and an invented
# threshold is indistinguishable in the output from the ruled one. `cut` refuses to run without the
# anchor argument, which makes the caller write what a 10 is before any score is read. A run that
# cuts nothing exits 3, because that is what scoring against no anchor produces and the anchor
# refusal cannot see it — the anchor is a free string written before the scores exist. And the
# override may only move a lane the tracked file rules, malformed lines in it being as fatal as
# anywhere else: an override allowed to add a lane, or quietly skipped when unreadable, would put the
# invented threshold back by another door.
#
# tested by: score-test.sh
set -euo pipefail

# `CDPATH=`: set in the environment, it makes `cd` resolve a relative path against it and echo the
# directory it landed on, so `here` comes back two lines long and the config resolves nowhere.
here="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
config="$here/../thresholds.conf"
# Machine-local overrides, outside the repo so tweaking a bar is never a dirty working tree — and
# outside `~/.kk-flavor`, which is a symlink into it. Absent is the common case and costs one stat.
# It may only move a lane the tracked config already rules; see the allow-list in `scan_config`.
override="${XDG_CONFIG_HOME:-$HOME/.config}/kk-flavor/thresholds.conf"

# Set by `scan_config` and `threshold_for` rather than printed, because a caller reading them through
# `$(...)` would put the config loop in a subshell — see `scan_config`.
lanes= found= level= override_note=

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

# Either config is read by this one function, and read with a redirect rather than a pipe or a command
# substitution: either of those puts the loop in a subshell, where `die` exits that subshell and the
# caller carries on with an empty result. A malformed config would then be reported as a missing lane.
# It sets `found` and appends to `lanes` rather than printing them for that same reason — a caller
# reading the result back through `$(...)` would reintroduce the subshell it avoids.
#
# Field-by-field, never a pattern built from a config or from `$want`: a lane name is data, and
# reaching a regexp it would be metacharacters. That is also why `$name` stays quoted inside the
# `case` patterns below — unquoted, a lane named `*` would match every lane there is.
#
# `allow`, when set, is the lane list the tracked config rules, and a lane outside it is refused. An
# override free to add a lane would defeat the unknown-lane exit this script exists for: the caller
# would receive a threshold no tracked file states. It also makes a typo loud — `instructions` for
# `instruction` would otherwise tune nothing at all, silently, which is the one failure a local
# override is most likely to have.
scan_config() {
  local path="$1" want="$2" allow="${3-}" name verb op lvl rest
  found=
  # `|| [ -n "$name" ]` for the same reason the stdin loop carries it: a config whose last lane has no
  # trailing newline would otherwise lose that lane, and the failure reads as "no lane 'x'" — a
  # missing-lane message for a lane that is right there.
  while read -r name verb op lvl rest || [ -n "$name" ]; do
    case "$name" in '' | '#'*) continue ;; esac
    [ "$verb" = cut ] && [ "$op" = '<=' ] && [ -z "$rest" ] ||
      die "$path: cannot read the line naming '$name' — the form is '<lane> cut <= <n>'"
    case "$lvl" in
      '' | *[!0-9]*) die "$path: '$name' has a non-numeric level" ;;
    esac
    [ "$lvl" -le 10 ] || die "$path: '$name' is $lvl, over the 0-10 scale"
    if [ -n "$allow" ]; then
      case " $allow " in
        *" $name "*) ;;
        *) die "$path: '$name' is not a lane $config rules — an override moves a lane, never adds one" ;;
      esac
    fi
    lanes="$lanes $name"
    if [ "$name" = "$want" ]; then
      found="$lvl"
    fi
  done <"$path"
}

# Resolves one lane into `level`, and into `override_note` when the machine-local file moved it. The
# note is built here and printed by the caller, because where it can go differs: `threshold` mode's
# stdout is the bare number a caller reads back, so it goes to stderr there and into the report body
# under `cut`, where the verdict it governs is what a reader keeps.
threshold_for() {
  local want="$1" ruled=
  lanes= found= level= override_note=
  scan_config "$config" "$want"
  [ -n "$found" ] || die "no lane '$want' in $config — it lists:$lanes"
  level="$found"
  [ -f "$override" ] || return 0
  ruled="$lanes"
  lanes=
  scan_config "$override" "$want" "$ruled"
  # A lane the override does not name keeps the tracked number: the overlay is per lane, so tuning
  # one bar never silently detaches the rest from the file that rules them.
  [ -n "$found" ] || return 0
  override_note="lane $want overridden by $override: $level ruled, $found in effect"
  level="$found"
}

[ -f "$config" ] || die "no threshold config at $config"
[ $# -ge 2 ] || die "usage: score.sh threshold <lane> | score.sh cut <lane> <what a 10 is here>"

case "$1" in
  threshold)
    [ $# -eq 2 ] || die "threshold takes one lane"
    threshold_for "$2"
    # stderr, never stdout: this mode's stdout is the number, read straight back by its caller.
    [ -z "$override_note" ] || printf 'score.sh: %s\n' "$override_note" >&2
    printf '%s\n' "$level"
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
    threshold_for "$lane"
    printf 'lane %s, cutting at or below %s\n' "$lane" "$level"
    # In the report body here, not on stderr: the bar a verdict was judged against belongs beside the
    # verdict, and stderr is exactly what a caller piping this report to a file loses.
    [ -z "$override_note" ] || printf '%s\n' "$override_note"
    printf '10 here means: %s\n\n' "$anchor"
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
