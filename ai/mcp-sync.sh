#!/usr/bin/env bash
#
# Sync the MCP servers declared in ai/mcp.jsonc — and ai/mcp.private.jsonc (gitignored, same shape),
# when present — into Claude Code's user scope, so they reach every project and every launch method.
# Edit either file, then re-run. It adds and updates but does not prune: a server you delete from a
# file stays registered until `claude mcp remove <name> -s user`.
# tested by: mcp-sync-test.sh, which covers strip_comments.
# untested: every other effect here is a `claude mcp` call against the real user scope. Faking that
# CLI would only assert the fake, so run it and read the server list instead.
# A private server's config, secrets included, passes to `claude mcp add-json` as a positional
# argument, where `ps` can read it for the length of the call. There's no stdin or file path to pass
# it through: `add-json` takes `<name> <json>` and nothing else. Don't run this on a shared host.

# The sed is anchored at the line start: blanking from any `//` onwards truncates a URL.
strip_comments() {
  sed -e 's|^[[:space:]]*//.*$||' "$1"
}

# mcp-sync-test.sh sources this file to reach strip_comments, so sourcing stops here. Only a direct
# run syncs.
if [ "${BASH_SOURCE[0]}" != "${0}" ]; then
  return 0
fi

set -euo pipefail

# Every invocation form this script has writes to the live MCP registry, so an argument it does not
# understand is refused rather than ignored. `bash mcp-sync.sh --help`, run expecting usage text,
# silently performed a real registration instead: there is no argument meaning "tell me and stop"
# unless this file says so. Refused ahead of the probes below, so the wording names the argument
# rather than whatever this machine happens to be missing.
if [ "$#" -gt 0 ]; then
  if [ "$#" -eq 1 ] && { [ "$1" = "-h" ] || [ "$1" = "--help" ]; }; then
    sed -n '3,6p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
    printf 'usage: mcp-sync.sh   # takes no arguments; every run writes to the live registry\n'
    exit 0
  fi
  printf 'mcp-sync.sh: unknown argument %s — this script takes none, and every run writes to the live MCP registry. Nothing was synced.\n' "$1" >&2
  exit 2
fi

# `CDPATH=`: set in the environment, `cd` echoes the directory it landed on, so `script_dir` comes
# back two lines long and every file path built from it resolves nowhere.
script_dir="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
public_mcp_file="$script_dir/mcp.jsonc"
private_mcp_file="$script_dir/mcp.private.jsonc"

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
  json="$(strip_comments "$mcp_file")"
  jq -r '.mcpServers | keys[]' <<<"$json" | while IFS= read -r name; do
    config="$(jq -c --arg name "$name" '.mcpServers[$name]' <<<"$json")"
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
