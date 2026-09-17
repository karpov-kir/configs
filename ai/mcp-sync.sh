#!/usr/bin/env bash
#
# Sync the MCP servers declared in ai/mcp.jsonc — and ai/mcp.private.jsonc (gitignored, same shape),
# when present — into the selected client's user scope. The client is never inferred.
# Edit either file, then re-run. It adds and updates but does not prune servers removed from a file.
# Codex accepts stdio command/args/env and streamable HTTP URLs; unsupported fields are refused.
#
#   usage: mcp-sync.sh --agent=claude|codex
#
# Every run writes a live registry, so an argument this does not understand stops it rather than being
# ignored: `mcp-sync.sh --help`, run expecting usage text, once performed a real registration instead.
#
# The recipe is Go, in `ai/tools/mcp-sync/`.
#
# tested by: the Go suite in ai/tools/mcp-sync/; stub region by the Go suite in ai/tools/reach/.
set -euo pipefail

tool="mcp-sync"
# How far THIS file sits above the tools directory.
tools_offset="."

# --- shared:tool-stub ---
# Byte-identical in every stub, held so by the wiring check's shared-region scan. Copied rather than
# sourced because sourcing a file is executing it, and these run from whatever repo the human is in —
# so only what cannot move is here: a stub has to find the resolver before the resolver can decide
# anything. `ai/tools/resolve.sh` owns the rest, argv[0] included, and its header states why each line
# below is the shape it is — the `cd -P`, the one declared offset, the two guards, the exec.
die() {
  printf '%s: %s\n' "${0##*/}" "$1" >&2
  exit 2
}

here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" ||
  die "cannot resolve my own directory, so $tool could not be located"

resolver="$here/$tools_offset/tools/resolve.sh"
[ -e "$resolver" ] ||
  die "no resolver at $resolver — this skill is mounted from a checkout that does not ship ai/tools/, and $tool did NOT run"
[ -x "$resolver" ] ||
  die "$resolver is not executable, so $tool did NOT run — chmod +x it"

exec "$resolver" --run "$tool" "$0" "$@"
# --- end shared:tool-stub ---
