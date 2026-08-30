#!/usr/bin/env bash
# Open-TODO gate — scan a markdown file for unchecked `- [ ]`, fence/comment-aware so an example
# checkbox never reads as a real TODO. Prints each open item with its `## Section`; exits 1 if any
# are found, 0 if none, 2 when the scan could not run — 2 prints nothing, so never read it as clean.
# tested by: todo-gate-test.sh, which pins the scan — fences, comments, section attribution, both
# refusals. The caller's side is pinned by the Go suite in `ai/tools/eco-report/`, which copies this
# script into each fixture and also stubs it to exit 3, so report.sh's state, carry and close refuse
# rather than read a failed scan as clean. Still unpinned: gate's call, and idsd-build's over an
# intent file.
set -uo pipefail
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

# A redirect, never an operand. awk reads an operand shaped like `name=value` as a variable
# assignment rather than a file, so a relative path whose leading component is a valid awk identifier
# — `intent=v2.md` — is never opened. awk then falls through to stdin, takes EOF from the `/dev/null`
# an agent hands it, prints nothing and exits 0: a clean merge gate over a file it never read. The
# redirect removes the operand, so no path can be parsed as anything but a path.
open_items=$(awk '
  /^[[:space:]]*```/ || /^[[:space:]]*~~~/ { in_fence = !in_fence; next }
  in_fence { next }
  /<!--/ { in_comment = 1 }
  in_comment { if (/-->/) { in_comment = 0 } next }
  /^#+ / { section = $0; next }
  /^[[:space:]]*- \[ \]/ { line = $0; sub(/^[[:space:]]*/, "", line); print section " | " line }
' <"$file")
scan_status=$?
[ "$scan_status" -eq 0 ] || {
  echo "error: could not scan $file (awk exited $scan_status)" >&2
  exit 2
}

[ -n "$open_items" ] || exit 0
echo "$open_items"
exit 1
