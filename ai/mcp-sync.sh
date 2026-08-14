#!/usr/bin/env bash
#
# Sync the MCP servers declared in ai/mcp.json — and ai/mcp.private.json (gitignored, same shape),
# when present — into Claude Code's user scope, so they reach every project and every launch method.
# Edit either file, then re-run. It adds and updates but does not prune: a server you delete from a
# file stays registered until `claude mcp remove <name> -s user`.

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
public_mcp_file="$script_dir/mcp.json"
private_mcp_file="$script_dir/mcp.private.json"

command -v jq >/dev/null || {
  echo "error: jq is required (brew install jq)" >&2
  exit 1
}
command -v claude >/dev/null || {
  echo "error: claude CLI not found on PATH" >&2
  exit 1
}
[ -f "$public_mcp_file" ] || {
  echo "error: $public_mcp_file not found" >&2
  exit 1
}

for mcp_file in "$public_mcp_file" "$private_mcp_file"; do
  if [ ! -f "$mcp_file" ]; then
    continue
  fi
  jq -r '.mcpServers | keys[]' "$mcp_file" | while IFS= read -r name; do
    config="$(jq -c --arg name "$name" '.mcpServers[$name]' "$mcp_file")"
    claude mcp remove -s user -- "$name" >/dev/null 2>&1 || true
    # The remove already happened, so a failed add leaves the server gone, and `set -e` would end the
    # run with nothing said.
    claude mcp add-json -s user -- "$name" "$config" || {
      echo "error: re-adding '$name' failed — it was removed first, so it is now UNREGISTERED." >&2
      echo "       The sync stopped here: nothing after '$name' was synced, in this file or any later one." >&2
      echo "       Fix its entry and re-run this script." >&2
      exit 1
    }
    echo "synced: $name ($(basename "$mcp_file"))"
  done
done
