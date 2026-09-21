#!/usr/bin/env bash
# Holds the instruction tree to `ai/kk-flavor/voice-baseline.txt`, the ratchet the register campaign
# pays down. Before this script its only reader was a sentence in a skill telling an agent to read it.
#
#   usage: voice-baseline.sh [--regenerate] [<root>]
#
# It refuses in both directions. A file over its line has risen, so the change that raised it repairs
# it. A file under its line has left slack, so the change lowers the line too. A file that carries a
# count without a baseline line is a new file starting dirty. Exits 1 with findings, 0 when every
# file is on its line, and 2 when the check did not run.
#
# tested by: the Go suite in ai/tools/voicebaseline/, which execs this script once per case.
set -euo pipefail

regenerate=""
[ "${1:-}" = "--regenerate" ] && { regenerate=1; shift; }

# The checkout to measure. Named, it is a fixture root carrying its own baseline and its own stub
# checker, which is how the suite drives both refusal directions without the Go tool.
root="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../.." && pwd)}"
baseline="$root/ai/kk-flavor/voice-baseline.txt"
check="$root/ai/kk-flavor/skills/kk-edit/scripts/voice-check.sh"

[ -x "$check" ] || { echo "voice-baseline: $check is not executable, so nothing was measured — exit 2" >&2; exit 2; }
[ -f "$baseline" ] || { echo "voice-baseline: $baseline is missing, so there is no ratchet to hold — exit 2" >&2; exit 2; }

cd "$root"
# An indexed array filled by a read loop, and a second one beside it. macOS ships bash 3.2, which has
# `mapfile` and `declare -A` in neither, and every other script here already runs on it.
files=()
while IFS= read -r f; do
  files+=("$f")
done < <(find ai/kk-flavor/standards ai/kk-flavor/workers ai/kk-flavor/skills ai/kk-flavor/templates \
  -name '*.md' 2>/dev/null | sort)
[ "${#files[@]}" -gt 0 ] || { echo "voice-baseline: no instruction file was found — exit 2" >&2; exit 2; }

# Measured one file at a time, reading each run's own summary line. The report truncates its findings
# at a display cap, so counting printed lines undercounts any file that runs past it, and one run over
# every file at once would hit that cap long before the last file.
#
# The runs go concurrently, in batches. Sequentially this was the slowest thing in the gate at 102
# seconds over 72 files, which is most of the whole budget spent on startup — each run reads one file
# and shares nothing with the others, so there is nothing here to serialize. Batched with `wait` rather
# than `wait -n`, and with an indexed array rather than an associative one, because macOS ships bash
# 3.2 and has neither.
#
# Exit 2 from a run is fatal here. It means that run did not measure, and its empty summary would
# otherwise read as a file with no findings — which is a count under its baseline, and the regenerate
# path would then write that zero in as the new floor.
measured=()
work="$(mktemp -d "${TMPDIR:-/tmp}/voice-baseline.XXXXXX")" || {
  echo "voice-baseline: no temp directory, so nothing was measured — exit 2" >&2; exit 2; }
trap 'rm -rf "$work"' EXIT
batch="$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 8)"
i=0
while [ "$i" -lt "${#files[@]}" ]; do
  j=0
  while [ "$j" -lt "$batch" ] && [ "$i" -lt "${#files[@]}" ]; do
    (
      # The run's own status, taken from the substitution rather than from PIPESTATUS. PIPESTATUS
      # there reports the assignment, which is a one-element pipeline, so it read 0 whatever the
      # checker did — and the guard below never fired.
      status=0
      summary="$("$check" --profile=instruction "${files[$i]}" 2>&1 >/dev/null)" || status=$?
      summary="$(printf '%s' "$summary" | grep -o 'instruction profile: [0-9]* finding' || true)"
      [ "$status" = 2 ] && { printf 'refused\n' >"$work/$i"; exit 0; }
      n="${summary//[!0-9]/}"
      printf '%s\n' "${n:-0}" >"$work/$i"
    ) &
    i=$((i + 1)); j=$((j + 1))
  done
  wait
done
i=0
while [ "$i" -lt "${#files[@]}" ]; do
  n="$(cat "$work/$i" 2>/dev/null || true)"
  [ "$n" = refused ] || [ -z "$n" ] && {
    echo "voice-baseline: $check did not measure ${files[$i]}, so this is not a clean run — exit 2" >&2; exit 2; }
  measured+=("$n")
  i=$((i + 1))
done

# Rewritten from what the tree measures now, which belongs in the same change that lowered a count.
# A later change spends slack the baseline still records, and the count never reaches the floor it
# already stood on.
if [ -n "$regenerate" ]; then
  { grep '^#' "$baseline"
    i=0
    while [ "$i" -lt "${#files[@]}" ]; do
      echo "${measured[$i]} ${files[$i]}"
      i=$(( i + 1 ))
    done | sort -rn -k1,1 -k2,2; } > "$baseline.new"
  mv "$baseline.new" "$baseline"
  echo "voice-baseline: regenerated over ${#files[@]} file(s)"
  exit 0
fi

# The recorded count for one file, or empty where the baseline holds no line for it. Read per file out
# of the baseline, because the lookup table a whole-file read would build needs an associative array.
recorded_for() {
  awk -v want="$1" '$1 ~ /^[0-9]+$/ && $2 == want { print $1; exit }' "$baseline"
}

findings=0
i=0
while [ "$i" -lt "${#files[@]}" ]; do
  f="${files[$i]}"
  now="${measured[$i]}"
  was="$(recorded_for "$f")"
  i=$(( i + 1 ))
  if [ -z "$was" ]; then
    if [ "$now" -gt 0 ]; then
      echo "$f: $now finding(s) and no baseline line — a new instruction file starts clean"
      findings=$(( findings + 1 ))
    fi
    continue
  fi
  if [ "$now" -gt "$was" ]; then
    echo "$f: $now finding(s), over its baseline of $was — the change that raised it repairs it"
    findings=$(( findings + 1 ))
  elif [ "$now" -lt "$was" ]; then
    echo "$f: $now finding(s), under its baseline of $was — run --regenerate so the slack is not left for a later change"
    findings=$(( findings + 1 ))
  fi
done

if [ "$findings" -gt 0 ]; then
  echo "voice-baseline: $findings file(s) off their line. The baseline only goes down." >&2
  exit 1
fi
echo "voice-baseline: ${#files[@]} file(s), every one on its line." >&2
