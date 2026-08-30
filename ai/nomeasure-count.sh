#!/usr/bin/env bash
# What a `shell-mutate.sh` exit status does to the count of consecutive runs that measured nothing,
# and whether that count is past the point where "the runner was loaded" is still the likelier
# explanation.
#   usage: nomeasure-count.sh <harness-exit-status> <count-file>
#          0  it measured — the count is cleared, and the caller exits on the harness's own status
#          1  it did not measure, and the count is still under the threshold — warn and pass
#          3  it did not measure <threshold> runs running — fail, and say why
#          2  nothing was decided and nothing was written: a bad argument, or a count file that
#             would not take the write
#
# A gate that warns and passes on every did-not-measure is an honest chain ending in a green tick
# forever, over guards nothing has ever proved. The count is what turns that chain into something a
# run can act on.
#
# Exit 2 is the harness saying a loaded machine killed mutants on its watchdog, not that a guard
# failed to redden. Every other status clears the count: 0 and 1 are it reporting on the guards
# themselves, and a failing guard proves it reached them, which is the only thing this counts. A
# status the harness never defines, a 127 from a harness that is not there, clears the count too. It
# cannot buy a silent pass that way, because the caller exits on that status and the job goes red.
#
# Nothing is written but <count-file>, and its directory is not created: the caller names the path,
# and a script that makes up the parents is writing somewhere nobody named.
#
# tested by: nomeasure-count-test.sh
set -uo pipefail
export LC_ALL=C

# Three, because two in a row on a shared runner is ordinary and three is not. Changing it changes
# how long the gate stays green over unproven guards, so it lives here rather than in a workflow.
escalate_at=3

status="${1:-}"
count_file="${2:-}"

if [ "$#" -lt 2 ] || [ -z "$status" ] || [ -z "$count_file" ]; then
  echo "nomeasure-count.sh: needs the harness's exit status and a count file — nothing was decided." >&2
  echo "usage: nomeasure-count.sh <harness-exit-status> <count-file>" >&2
  exit 2
fi

case "$status" in
  *[!0-9]*)
    echo "nomeasure-count.sh: '$status' is no exit status — nothing was decided, and the count was left as it was." >&2
    exit 2
    ;;
esac

# `>|` rather than `>`: under a shell with noclobber set, plain `>` refuses an existing file and
# leaves the old count in place, which reads exactly like a count this run wrote.
record() { # <count>
  printf '%s' "$1" >|"$count_file" || {
    echo "nomeasure-count.sh: could not write $count_file — the count is unchanged, so the next run reads stale history and this one proved nothing." >&2
    return 1
  }
}

if [ "$status" -ne 2 ]; then
  record 0 || exit 2
  echo "shell-mutate reached the guards on this run (exit $status), so the did-not-measure count is back to 0."
  exit 0
fi

# A stored count this script cannot parse starts the history over, so this run counts as the first.
# What it must never do is carry on from the garbage.
#
# Ten digits or more is refused with the rest, and the length is what refuses it rather than any
# arithmetic on the value: bash wraps at 64 bits silently, so a corrupt entry of nineteen digits can
# increment to a NEGATIVE count that stays under the threshold for good, switching off the escalation
# by the very garbage this guard is here for. A real count never leaves single figures.
stored=$(cat "$count_file" 2>/dev/null || true)
case "$stored" in
  '' | *[!0-9]* | ??????????*) stored=0 ;;
esac

count=$((stored + 1))
record "$count" || exit 2

if [ "$count" -ge "$escalate_at" ]; then
  echo "shell-mutate has not measured on $count consecutive runs. That is no longer machine load — nothing is proving these guards, and this stops passing until one run measures."
  exit 3
fi

echo "shell-mutate did not measure every guard on this runner ($count consecutive). Nothing was proved for those, and this is not a pass for them."
exit 1
