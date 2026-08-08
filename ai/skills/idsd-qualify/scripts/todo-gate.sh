#!/usr/bin/env bash
# Open-TODO gate — scan a markdown file for unchecked `- [ ]`, fence/comment-aware so an
# example checkbox never reads as a real TODO. Prints each open item (with its `## Section`)
# and exits 1 if any are found, 0 if none, and 2 when the scan could not run (no argument, or no
# such file) — 2 prints nothing, so never read it as clean. The one scanner both gates rely on:
# the qualify report gate (`report.sh`) and idsd-build's Phase 5 archive gate.
set -uo pipefail
# Byte-oriented, like every sibling script: macOS awk aborts on the first non-UTF-8 byte under a
# UTF-8 locale, and a report or ICE quoting code can carry one. The guard below turns such an abort
# into exit 2. Without it the abort yields empty output, which gate, carry, state and stamp all read
# as "no open items".
export LC_ALL=C

file="${1:-}"
[ -n "$file" ] || {
  echo "usage: todo-gate.sh <file>" >&2
  exit 2
}
[ -f "$file" ] || {
  echo "error: no such file: $file" >&2
  exit 2
}

open_items=$(awk '
  /^[[:space:]]*```/ || /^[[:space:]]*~~~/ { in_fence = !in_fence; next }
  in_fence { next }
  /<!--/ { in_comment = 1 }
  in_comment { if (/-->/) { in_comment = 0 } next }
  /^#+ / { section = $0; next }
  /^[[:space:]]*- \[ \]/ { line = $0; sub(/^[[:space:]]*/, "", line); print section " | " line }
' "$file")
scan_status=$?
[ "$scan_status" -eq 0 ] || {
  echo "error: could not scan $file (awk exited $scan_status)" >&2
  exit 2
}

[ -n "$open_items" ] || exit 0
echo "$open_items"
exit 1
