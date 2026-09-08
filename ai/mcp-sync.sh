#!/usr/bin/env bash
#
# Sync the MCP servers declared in ai/mcp.jsonc — and ai/mcp.private.jsonc (gitignored, same shape),
# when present — into the selected client's user scope. An explicit --agent=claude|codex is required.
# Edit either file, then re-run. It adds and updates but does not prune servers removed from a file.
# Codex accepts stdio command/args/env and streamable HTTP URLs; unsupported fields are refused.
# tested by: mcp-sync-test.sh, including an isolated Codex registry when its CLI is available.

# The sed is anchored at the line start: blanking from any `//` onwards truncates a URL.
strip_comments() {
  sed -e 's|^[[:space:]]*//.*$||' "$1"
}

substitute_configs_dir() { # <json> <dir>
  local rest="$1" out=""
  while [ "${rest#*@CONFIGS@}" != "$rest" ]; do
    out="$out${rest%%@CONFIGS@*}$2"
    rest="${rest#*@CONFIGS@}"
  done
  printf '%s' "$out$rest"
}

# Refused rather than escaped, because the mangling is silent. A `\` lands inside the JSON string as
# an escape, so the entry still parses and the command names a different path: `/opt/a\b` reaches
# the CLI as a backspace. A crafted `"` closes the string early, so the command is no longer
# mcp-env.sh and the rest of the directory becomes further keys, an `env` one being enough. Either
# way what goes missing is the environment stripping, so the directory is named and the sync stops.
configs_dir_is_substitutable() { # <dir>
  case "$1" in
    *'"'* | *'\'*) return 1 ;;
  esac
}

# Shell arguments cannot carry NUL; refusing it prevents a JSON value changing during conversion.
validate_codex_config() {
  jq -e '
    def argument: type == "string" and (contains("\u0000") | not);
    type == "object" and
    if (.type // "stdio") == "stdio" then
      (keys - ["type", "command", "args", "env"] | length == 0) and
      (.command | argument and length > 0) and
      ((has("args") | not) or (.args | type == "array" and all(.[]; argument))) and
      ((has("env") | not) or (.env | type == "object" and
        all(to_entries[]; (.key | test("^[A-Za-z_][A-Za-z0-9_]*$")) and (.value | argument))))
    elif .type == "http" then
      (keys - ["type", "url"] | length == 0) and
      (.url | type == "string" and test("^https?://[^[:space:][:cntrl:]]+$"))
    else false end
  ' <<<"$1" >/dev/null 2>&1
}

sync_codex_server() { # <name> <config>
  local name="$1" config="$2" argument command
  local cli_args=(mcp add "$name")
  if [ "$(jq -r '.type // "stdio"' <<<"$config")" = http ]; then
    cli_args+=(--url "$(jq -r '.url' <<<"$config")")
  else
    while IFS= read -r -d '' argument; do
      cli_args+=(--env "$argument")
    done < <(jq -j '(.env // {}) | to_entries[] | .key, "=", .value, "\u0000"' <<<"$config")
    IFS= read -r -d '' command < <(jq -j '.command, "\u0000"' <<<"$config")
    cli_args+=(-- "$command")
    while IFS= read -r -d '' argument; do
      cli_args+=("$argument")
    done < <(jq -j '.args // [] | .[] | ., "\u0000"' <<<"$config")
  fi
  codex "${cli_args[@]}"
}

# mcp-sync-test.sh sources this file to reach strip_comments, so sourcing stops here. Only a direct
# run syncs.
if [ "${BASH_SOURCE[0]}" != "${0}" ]; then
  return 0
fi

set -euo pipefail

agent=""
for arg in "$@"; do
  case "$arg" in
    --agent=claude | --agent=codex) agent="${arg#*=}" ;;
    -h | --help)
      sed -n '3,6p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      printf 'usage: mcp-sync.sh --agent=claude|codex\n'
      exit 0
      ;;
    *)
      printf 'mcp-sync.sh: unknown argument %s. Nothing was synced.\n' "$arg" >&2
      exit 2
      ;;
  esac
done
if [ -z "$agent" ]; then
  printf 'mcp-sync.sh: select --agent=claude|codex. Nothing was synced.\n' >&2
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
command -v "$agent" >/dev/null || {
  echo "error: $agent CLI not found on PATH" >&2
  exit 1
}
[ -f "$public_mcp_file" ] || {
  echo "error: $public_mcp_file not found" >&2
  exit 1
}
configs_dir_is_substitutable "$script_dir" || {
  echo "error: $script_dir contains a quote or a backslash, so @CONFIGS@ cannot be substituted into" >&2
  echo "       valid JSON naming it. Nothing was synced — move the checkout somewhere without one." >&2
  exit 1
}
[ -x "$script_dir/mcp-env.sh" ] || {
  echo "error: $script_dir/mcp-env.sh is missing or not executable, and every stdio server here is" >&2
  echo "       launched through it. Registering them anyway would leave each one failing to start," >&2
  echo "       so nothing was synced." >&2
  exit 1
}

documents=()
document_files=()
for mcp_file in "$public_mcp_file" "$private_mcp_file"; do
  [ -f "$mcp_file" ] || continue
  json="$(substitute_configs_dir "$(strip_comments "$mcp_file")" "$script_dir")"
  if ! jq -e 'type == "object" and (.mcpServers | type == "object")' <<<"$json" >/dev/null 2>&1; then
    echo "error: $mcp_file must contain an mcpServers object. Nothing was synced." >&2
    exit 1
  fi
  if [ "$agent" = codex ]; then
    if ! jq -e '(keys == ["mcpServers"]) and (.mcpServers | keys | all(.[]; test("^[A-Za-z0-9_][A-Za-z0-9_-]*$")))' <<<"$json" >/dev/null; then
      echo "error: $mcp_file has unsupported fields or Codex server names. Nothing was synced." >&2
      exit 1
    fi
    while IFS= read -r name; do
      config="$(jq -c --arg name "$name" '.mcpServers[$name]' <<<"$json")"
      if ! validate_codex_config "$config"; then
        echo "error: Codex cannot preserve the transport fields for '$name' in $mcp_file. Nothing was synced." >&2
        exit 1
      fi
    done < <(jq -r '.mcpServers | keys[]' <<<"$json")
  fi
  documents+=("$json")
  document_files+=("$mcp_file")
done

for ((i = 0; i < ${#documents[@]}; i++)); do
  json="${documents[i]}"
  mcp_file="${document_files[i]}"
  while IFS= read -r name; do
    config="$(jq -c --arg name "$name" '.mcpServers[$name]' <<<"$json")"
    if [ "$agent" = codex ]; then
      sync_codex_server "$name" "$config" || {
        echo "error: syncing '$name' to Codex failed. No later entries were synced." >&2
        exit 1
      }
    else
      claude mcp remove -s user -- "$name" >/dev/null 2>&1 || true
      claude mcp add-json -s user -- "$name" "$config" || {
        echo "error: re-adding '$name' failed — it was removed first, so it is now UNREGISTERED." >&2
        echo "       Fix its entry and re-run this script. No later entries were synced." >&2
        exit 1
      }
    fi
    echo "synced: $name ($(basename "$mcp_file"))"
  done < <(jq -r '.mcpServers | keys[]' <<<"$json")
done
