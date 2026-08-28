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
# machine-local file. An override in effect is always announced: a bar moved locally produces a
# verdict no other machine reproduces, and silence about it is the hazard.
#
# The refusals below are the enforcement, not convenience: read the comment at one before removing it.
# An unknown lane exits rather than falling back to `default`, because a caller that cannot find its
# number invents one, and an invented threshold reads exactly like the ruled one.
#
# tested by: score-test.sh
set -euo pipefail

# `CDPATH=`: set in the environment, it makes `cd` echo the directory it landed on, so `here` comes
# back two lines long and the config resolves nowhere.
here="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
config="$here/../thresholds.conf"
# Machine-local overrides live outside the repo, so tweaking a bar is never a dirty working tree, and
# outside `~/.kk-flavor`, which is a symlink into it.
override="${XDG_CONFIG_HOME:-$HOME/.config}/kk-flavor/thresholds.conf"

# Set rather than printed, for the subshell reason `scan_config` states.
lanes= found= level= override_note=

die() {
  printf 'score.sh: %s\n' "$1" >&2
  exit 2
}

# Exit 3, not 2: a caller that cannot tell "refused" from "did not run" treats a refusal as a broken
# tool and moves on.
die3() {
  printf 'score.sh: %s\n' "$1" >&2
  exit 3
}

# Either config is read here, with a redirect rather than a pipe or `$(...)`: either of those puts the
# loop in a subshell, where `die` exits only that subshell and the caller carries on with an empty
# result. A malformed config is then reported as a missing lane. `found` and `lanes` are set rather
# than printed for that same reason.
#
# Field-by-field, never a pattern built from a config or from `$want`: a lane name is data, and in a
# regexp it would be metacharacters. Same reason `$name` stays quoted in the `case` patterns below;
# unquoted, a lane named `*` matches every lane there is.
#
# `allow`, when set, is the lane list the tracked config rules; a lane outside it is refused. What
# that buys is a loud typo: `instructions` for `instruction` would otherwise tune nothing, silently.
# Not the threshold a caller gets. `threshold_for` refuses any lane the tracked config omits first.
scan_config() {
  local path="$1" want="$2" allow="${3-}" name verb op line_level rest
  found=
  # `|| [ -n "$name" ]`, as in the stdin loop below: a config whose last lane has no trailing newline
  # would otherwise lose it, and the failure reads as "no lane 'x'" for a lane that is right there.
  while read -r name verb op line_level rest || [ -n "$name" ]; do
    case "$name" in '' | '#'*) continue ;; esac
    # A lane name is data, and every message below prints it back. Raw, a control byte in it overwrites
    # the line already on the reader's terminal, and an `\033]` sequence reaches the terminal itself.
    # That is the hazard the label loop neutralises. Refused here rather than neutralised, because no
    # lane a config can legitimately rule carries one, and refusing keeps every message downstream clean
    # by construction, `lanes` included. Shown neutralised, so the developer can still find the line.
    case "$name" in
      *[[:cntrl:]]*) die "$path: the lane name '${name//[[:cntrl:]]/ }' carries a control character" ;;
    esac
    [ "$verb" = cut ] && [ "$op" = '<=' ] && [ -z "$rest" ] ||
      die "$path: cannot read the line naming '$name' — the form is '<lane> cut <= <n>'"
    case "$line_level" in
      '' | *[!0-9]*) die "$path: '$name' has a non-numeric level" ;;
    esac
    [ "$line_level" -le 10 ] || die "$path: '$name' is $line_level, over the 0-10 scale"
    if [ -n "$allow" ]; then
      case " $allow " in
        *" $name "*) ;;
        *) die "$path: '$name' is not a lane $config rules — an override moves a lane, never adds one" ;;
      esac
    fi
    lanes="$lanes $name"
    if [ "$name" = "$want" ]; then
      found="$line_level"
    fi
  done <"$path"
}

# The override note is built here and printed by the caller, because where it may go differs per mode.
threshold_for() {
  local want="$1" ruled=
  lanes= found= level= override_note=
  scan_config "$config" "$want"
  [ -n "$found" ] || die "no lane '$want' in $config — it lists:$lanes"
  level="$found"
  # Absent is the common case, and the only one that falls back silently. A path that exists but is not
  # a readable file is refused: a directory, a fifo, a dangling symlink, or a mode denying us. Falling
  # back there would restore the tracked bar while its owner believed their tuning was live, which is
  # the same silent-default hole a malformed line is refused for.
  #
  # `[` reports ENOENT and EACCES alike, so one case stays open: an override under a directory this user
  # cannot search reads as absent and falls back without a word. Closing it needs a stat this script
  # forks for nowhere else. The refusal above is about the file's own mode, never the path leading to it.
  [ -e "$override" ] || [ -L "$override" ] || return 0
  # Neutralised for the same reason the note below is. This message carries `$XDG_CONFIG_HOME`, and a
  # control byte in it rewrites the line already on the reader's terminal.
  [ -f "$override" ] && [ -r "$override" ] ||
    die "${override//[[:cntrl:]]/ } is not a readable file. Fix or remove it; skipping it would restore the tracked bar without saying so"
  ruled="$lanes"
  lanes=
  scan_config "$override" "$want" "$ruled"
  # A lane the override does not name keeps the tracked number: the overlay is per lane, so tuning
  # one bar never silently detaches the rest from the file that rules them.
  [ -n "$found" ] || return 0
  # Neutralised rather than refused, because `$override` carries `$XDG_CONFIG_HOME`, a path its owner
  # may legitimately hold. Under `cut` this note prints into the report body above the verdict lines, so
  # a newline in that variable would put a forged `keep 10 <item>` among the real verdicts while the
  # counts below still said the item was cut. `$want` stays raw wherever it is matched: neutralising it
  # there would let `reply\r` resolve as `reply` and defeat the unknown-lane exit.
  override_note="lane $want overridden by $override: $level ruled, $found in effect"
  override_note="${override_note//[[:cntrl:]]/ }"
  level="$found"
}

# `-r` as well as `-f`, matching the override's own test above: without it an unreadable tracked config
# reached the redirect and exited 1 on bash's error rather than the ruled 2. Closing that on the
# override alone would have left the pair asymmetric, with the older half the weaker one.
[ -f "$config" ] && [ -r "$config" ] || die "no readable threshold config at $config"
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
    # returns non-zero, so the plain form drops the last item, and a caller piping a heredoc has none.
    # The item is then neither kept nor cut while the counts still add up.
    while IFS= read -r line || [ -n "$line" ]; do
      [ -n "$line" ] || continue
      score="${line%%	*}"
      label="${line#*	}"
      [ "$label" != "$line" ] ||
        die "no tab in '${line//[[:cntrl:]]/ }' — each line is '<score><TAB><label>'"
      # A label is text from the artifact under review, printed back into a report its caller reads.
      # A control character rewrites that report rather than appearing in it: `\r` overwrites the
      # verdict on the line and `\v` opens a second one, so an item this cut renders as one it kept
      # while the counts still say it was cut.
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
    # Everything clearing the bar is what scoring against no anchor looks like: the scale never gets
    # used, every item lands mid-band, and the run reads as a pass. The anchor refusal cannot catch
    # it, because the anchor is a free string written before the scores exist. This exits rather than
    # prints, because a notice at the end of a list is the one thing a caller skims.
    #
    # A tight artifact really can cut nothing, so the exit is refusable, but only by writing down why.
    # Exit 3, never 2: the scan ran.
    if [ "$gone" -eq 0 ] && [ "$kept" -gt 0 ]; then
      [ -n "$kept_all_why" ] ||
        die3 "nothing scored at or below $level. Re-score against the anchor, or re-run with --kept-all '<why nothing fell below it>'"
      printf 'nothing cut, accepted: %s\n' "${kept_all_why//[[:cntrl:]]/ }"
    fi
    ;;
  *) die "unknown command '$1' — threshold or cut" ;;
esac
