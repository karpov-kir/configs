#!/usr/bin/env bash
# Offer cadence — idsd-ship offers a periodic pass at most once per interval, and this file owns both
# the intervals and where each one's date is kept.
#   usage: cadence.sh <topic> due     0 = offer one, 1 = not yet, 2 = undetermined
#          cadence.sh <topic> asked   record that the offer was made today, whatever the human answered
# Topics, and why their records sit where they do:
#   retro  how a run was conducted — a habit of the human's, not of one project, so the date lives
#          in this skill's own directory, the one path that is identical from every repo.
#          Git-ignored, so a ship run in another repo never dirties this one.
#   audit  the .idsd/ intent set of THIS repo, so the date belongs to this repo. It goes under .git/,
#          never in .idsd/: `report.sh discard` wipes a throwaway .idsd/ at the end of every ship, and
#          a cadence the ship itself deletes can never come due.
# Every path prints one line saying why. Exit 2 never means "not due" — it means nothing was decided.
set -uo pipefail
export LC_ALL=C

topic="${1:-}"
action="${2:-}"

skill_dir=$(cd "$(dirname "$0")/.." 2>/dev/null && pwd) || skill_dir=""
[ -n "$skill_dir" ] || {
  echo "cadence.sh: could not resolve idsd-ship's own directory — nothing was determined; the cadence is unknown." >&2
  exit 2
}

interval_days=7
# Dispatched on "${1:-}" rather than on $topic so kk-ecosystem's check.sh recognises this as the
# top-level case and holds every arm to having a call site somewhere an agent reads.
case "${1:-}" in
  retro)
    state="$skill_dir/last-offer-retro.txt"
    ;;
  audit)
    # --git-path resolves correctly from a worktree and a submodule too, where a hardcoded .git/ does not.
    state=$(git rev-parse --git-path idsd-audit-offer 2>/dev/null) || state=""
    [ -n "$state" ] || {
      echo "cadence.sh: not inside a git repository, so there is no per-repo record — nothing was determined." >&2
      exit 2
    }
    ;;
  *)
    echo "usage: cadence.sh {retro|audit} {due|asked}" >&2
    exit 2
    ;;
esac

# Days since 1970-01-01 for a YYYY-MM-DD date; prints nothing when the argument is not one. Done in
# awk because neither `date -d` (GNU) nor `date -j -f` (BSD) is portable across the shells this
# script runs in.
day_number() {
  printf '%s\n' "$1" | awk '
    NR == 1 {
      if ($0 !~ /^[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]$/) exit 1
      y = substr($0, 1, 4) + 0; m = substr($0, 6, 2) + 0; d = substr($0, 9, 2) + 0
      if (m < 1 || m > 12 || d < 1 || d > 31) exit 1
      # days-from-civil: shift the year to start in March, which puts the leap day at its end.
      if (m <= 2) y--
      era = int((y >= 0 ? y : y - 399) / 400)
      yoe = y - era * 400
      doy = int((153 * (m + (m > 2 ? -3 : 9)) + 2) / 5) + d - 1
      doe = yoe * 365 + int(yoe / 4) - int(yoe / 100) + doy
      print era * 146097 + doe - 719468
      exit
    }'
}

case "$action" in
  due)
    if [ ! -e "$state" ]; then
      echo "due: no $topic has ever been offered (no $state)."
      exit 0
    fi
    if [ ! -r "$state" ] || ! recorded=$(head -n 1 "$state" 2>/dev/null); then
      echo "undetermined: $state exists but could not be read — nothing was determined; this is not a 'not due'." >&2
      exit 2
    fi
    stamp_day=$(day_number "$recorded")
    [ -n "$stamp_day" ] || {
      echo "undetermined: $state holds '$recorded', which is no YYYY-MM-DD date — nothing was determined; this is not a 'not due'." >&2
      exit 2
    }
    today=$(date +%Y-%m-%d)
    today_day=$(day_number "$today")
    [ -n "$today_day" ] || {
      echo "undetermined: date printed '$today', which is no YYYY-MM-DD date — nothing was determined; this is not a 'not due'." >&2
      exit 2
    }
    elapsed=$((today_day - stamp_day))
    [ "$elapsed" -ge 0 ] || {
      echo "undetermined: the last $topic offer is recorded as $recorded, which is later than today — nothing was determined; this is not a 'not due'." >&2
      exit 2
    }
    if [ "$elapsed" -ge "$interval_days" ]; then
      echo "due: $topic last offered $recorded, $elapsed days ago (interval $interval_days days)."
      exit 0
    fi
    echo "not due: $topic last offered $recorded, $elapsed days ago (interval $interval_days days)."
    exit 1
    ;;

  asked)
    today=$(date +%Y-%m-%d)
    mkdir -p "$(dirname "$state")" 2>/dev/null
    printf '%s\n' "$today" >"$state" || {
      echo "cadence.sh: could not write $state — the $topic offer was NOT recorded, so the next run will offer again." >&2
      exit 2
    }
    echo "recorded the $topic offer on $today."
    ;;

  *)
    echo "usage: cadence.sh {retro|audit} {due|asked}" >&2
    exit 2
    ;;
esac
