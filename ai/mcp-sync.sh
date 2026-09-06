#!/usr/bin/env bash
#
# Sync the MCP servers declared in ai/mcp.jsonc — and ai/mcp.private.jsonc (gitignored, same shape),
# when present — into Claude Code's user scope, so they reach every project and every launch method.
# Edit either file, then re-run. It adds and updates but does not prune: a server you delete from a
# file stays registered until `claude mcp remove <name> -s user`.
# tested by: mcp-sync-test.sh — the comment stripping, the @CONFIGS@ substitution, and the two guards
# that refuse to register at all.
# untested: what `claude mcp add-json` does with an entry once it has one. That is a call against the
# real user scope, and faking the CLI to assert its answer would only assert the fake. The suite fakes
# it to read what this script *asked* the CLI for, which is this script's own decision.
# A private server's config, secrets included, passes to `claude mcp add-json` as a positional
# argument, where `ps` can read it for the length of the call. There's no stdin or file path to pass
# it through: `add-json` takes `<name> <json>` and nothing else — read out of `claude mcp add-json
# --help`, not assumed.
#
# It is not only a shared host that reads argv. Every process running as you can, and this repo starts
# two of those at every session from an unpinned `npx` (ai/mcp.jsonc). One of them alive while this
# runs sees the config.
# So keep secrets out of these files: export the variable in the shell that starts Claude Code and
# name it in mcp-env.sh's allow-list, where it reaches the server without ever crossing argv. Treat
# anything that has already been in one of these files as exposed.

# The sed is anchored at the line start: blanking from any `//` onwards truncates a URL.
strip_comments() {
  sed -e 's|^[[:space:]]*//.*$||' "$1"
}

# `@CONFIGS@` in a server's command is this directory. It cannot be written into mcp.jsonc, which is
# committed and read on machines that keep the checkout somewhere else, and it cannot be left to the
# claude CLI, which is given a literal string and does no expansion of its own.
#
# Split on the token rather than `${json//@CONFIGS@/$dir}`, and not sed either. sed is out because the
# directory is a path and every sed delimiter is a character a path may contain. `//` is out because
# it has the same defect one layer down: from bash 5.2 an unquoted `&` in the replacement expands to
# the text that matched, so a checkout under `/opt/R&D` registered `/opt/R@CONFIGS@D/mcp-env.sh` — a
# command that does not exist, on a machine whose only symptom is servers that never start. And it is
# version-dependent, so it looks fine under macOS's own bash 3.2, which substitutes `&` literally.
# `%%` and `#` treat every character of the path as itself.
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

for mcp_file in "$public_mcp_file" "$private_mcp_file"; do
  if [ ! -f "$mcp_file" ]; then
    continue
  fi
  json="$(substitute_configs_dir "$(strip_comments "$mcp_file")" "$script_dir")"
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
