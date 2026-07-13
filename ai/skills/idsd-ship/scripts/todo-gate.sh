#!/usr/bin/env bash
# Open-TODO gate — scan a markdown file for unchecked `- [ ]`, fence/comment-aware so an
# example checkbox never reads as a real TODO. Prints each open item (with its `## Section`)
# and exits 1 if any are found, 0 if none. The one scanner both idsd gates rely on:
# idsd-ship's report gate (`report.sh`) and idsd-build's Phase 5 archive gate.
set -uo pipefail

file="${1:-}"
[ -n "$file" ] || {
  echo "usage: todo-gate.sh <file>" >&2
  exit 2
}
[ -f "$file" ] || {
  echo "error: no such file: $file" >&2
  exit 2
}

found=$(awk '
  /^[[:space:]]*```/ || /^[[:space:]]*~~~/ { fence = !fence; next }
  fence { next }
  /<!--/ { incomment = 1 }
  incomment { if (/-->/) { incomment = 0 } next }
  /^## / { section = $0; next }
  /^[[:space:]]*- \[ \]/ { line = $0; sub(/^[[:space:]]*/, "", line); print section " | " line }
' "$file")

[ -n "$found" ] || exit 0
echo "$found"
exit 1
