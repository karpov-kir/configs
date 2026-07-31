#!/usr/bin/env bash
#
# Sync the MCP servers declared in ai/mcp.json (and ai/mcp.private.json, when present)
# into Claude Code's user scope (~/.claude.json), making them available in every
# project and every launch method (terminal CLI and the IDE extension alike).
#
# ai/mcp.json is the public source of truth; ai/mcp.private.json (gitignored) holds
# machine-private servers — internal hosts and the like — in the same shape. Edit
# either file, then re-run this script. The sync is idempotent — each declared server
# is removed and re-added, so re-running re-applies the current definitions. Note it
# adds and updates but does not prune: a server you delete from a file stays
# registered until you remove it with `claude mcp remove <name> -s user`.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PUBLIC_MCP_FILE="$SCRIPT_DIR/mcp.json"
PRIVATE_MCP_FILE="$SCRIPT_DIR/mcp.private.json"

command -v jq >/dev/null || {
  echo "error: jq is required (brew install jq)" >&2
  exit 1
}
command -v claude >/dev/null || {
  echo "error: claude CLI not found on PATH" >&2
  exit 1
}
[ -f "$PUBLIC_MCP_FILE" ] || {
  echo "error: $PUBLIC_MCP_FILE not found" >&2
  exit 1
}

for mcp_file in "$PUBLIC_MCP_FILE" "$PRIVATE_MCP_FILE"; do
  if [ ! -f "$mcp_file" ]; then
    continue
  fi
  jq -r '.mcpServers | keys[]' "$mcp_file" | while IFS= read -r name; do
    config="$(jq -c --arg name "$name" '.mcpServers[$name]' "$mcp_file")"
    claude mcp remove -s user -- "$name" >/dev/null 2>&1 || true
    claude mcp add-json -s user -- "$name" "$config"
    echo "synced: $name ($(basename "$mcp_file"))"
  done
done
