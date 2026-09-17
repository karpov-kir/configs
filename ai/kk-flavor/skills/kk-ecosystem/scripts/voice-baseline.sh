#!/usr/bin/env bash
# Holds the instruction tree to `ai/kk-flavor/voice-baseline.txt`, the ratchet the register campaign
# pays down. Before this script its only reader was a sentence in a skill telling an agent to read it.
#
#   usage: voice-baseline.sh [--regenerate]
#
# It refuses in both directions. A file measuring over its line has risen, so the change that raised
# it repairs it. A file measuring under its line has left slack, so the change lowers the line too. A
# file that carries a count without a baseline line is a new file starting dirty. `--regenerate`
# rewrites the file from what the tree measures now, in the same change that lowers a count.
#
# Exits 1 with findings, 0 when every file sits exactly on its line, 2 when the check did not run.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../.." && pwd)"
baseline="$root/ai/kk-flavor/voice-baseline.txt"
check="$root/ai/kk-flavor/skills/kk-edit/scripts/voice-check.sh"

[ -x "$check" ] || { echo "voice-baseline: $check is not executable, so nothing was measured — exit 2" >&2; exit 2; }
[ -f "$baseline" ] || { echo "voice-baseline: $baseline is missing, so there is no ratchet to hold — exit 2" >&2; exit 2; }

cd "$root"
mapfile -t files < <(find ai/kk-flavor/standards ai/kk-flavor/workers ai/kk-flavor/skills ai/kk-flavor/templates \
  -name '*.md' 2>/dev/null | sort)
[ "${#files[@]}" -gt 0 ] || { echo "voice-baseline: no instruction file was found — exit 2" >&2; exit 2; }

# Measured one file at a time, reading each run's own summary line. The report truncates its findings
# at a display cap, so counting printed lines undercounts any file that runs past it.
declare -A measured
for f in "${files[@]}"; do
  summary="$("$check" --profile=instruction "$f" 2>&1 >/dev/null | grep -o 'instruction profile: [0-9]* finding' || true)"
  n="${summary//[!0-9]/}"
  measured["$f"]="${n:-0}"
done

if [ "${1:-}" = "--regenerate" ]; then
  { grep '^#' "$baseline"; for f in "${files[@]}"; do echo "${measured[$f]} $f"; done | sort -rn -k1,1 -k2,2; } > "$baseline.new"
  mv "$baseline.new" "$baseline"
  echo "voice-baseline: regenerated over ${#files[@]} file(s)"
  exit 0
fi

declare -A recorded
while read -r count file; do
  [ -z "${file:-}" ] && continue
  recorded["$file"]="$count"
done < <(grep -v '^#' "$baseline" | grep -v '^$')

findings=0
for f in "${files[@]}"; do
  now="${measured[$f]}"
  was="${recorded[$f]:-}"
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
