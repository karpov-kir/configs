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
# tested by: voice-baseline-test.sh
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
# at a display cap, so counting printed lines undercounts any file that runs past it.
measured=()
for f in "${files[@]}"; do
  summary="$("$check" --profile=instruction "$f" 2>&1 >/dev/null | grep -o 'instruction profile: [0-9]* finding' || true)"
  n="${summary//[!0-9]/}"
  measured+=("${n:-0}")
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
