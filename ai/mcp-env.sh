#!/usr/bin/env bash
#
# Launch an MCP server with a chosen environment instead of an inherited one.
#   usage: mcp-env.sh <command> [<arg>...]
#
# A stdio MCP server is a child of Claude Code, so without this it inherits every variable exported
# in the shell that launched the session — every API key, token and credential — and two of the
# servers in mcp.jsonc are unpinned `npx` packages whose code changes on every launch.
#
# So this re-execs behind `env -i`, passing only the variables below, by NAME rather than by value.
# The values still come from whatever launched it, so nothing here hardcodes a machine. The list is
# what these servers need to start, not what they might like to have, and a variable absent from the
# launching environment stays absent.
#
# Adding a name here widens what an unreviewed release can read, so add one only when a server fails
# without it, and say which server in the same edit.
#
# tested by: mcp-env-test.sh
set -euo pipefail

# PATH/HOME: find mise and node at all, and locate mise's data dir, the npm cache and the browser
# profiles under $HOME. USER/LOGNAME: node and Chrome both read them for per-user paths.
# TMPDIR/TMP/TEMP: npx unpacks into them. LANG/LC_*: text handling, and their absence makes node
# fall back to a different default than the terminal is using.
# XDG_*: where mise and npm look on Linux. MISE_*: only the ones that relocate its dirs.
# NODE_EXTRA_CA_CERTS/SSL_CERT_*: a machine behind a corporate CA cannot fetch a package without them.
# *_PROXY: same, and the one entry on this list that can itself carry a credential — a proxy URL may
# embed one. It is here because dropping it breaks every fetch on a proxied network; a machine that
# does not use a proxy passes nothing.
ALLOWED=(
  PATH HOME USER LOGNAME
  TMPDIR TMP TEMP
  LANG LC_ALL LC_CTYPE
  XDG_CONFIG_HOME XDG_CACHE_HOME XDG_DATA_HOME XDG_STATE_HOME
  MISE_DATA_DIR MISE_CONFIG_DIR MISE_CACHE_DIR
  NODE_EXTRA_CA_CERTS SSL_CERT_FILE SSL_CERT_DIR
  HTTP_PROXY HTTPS_PROXY NO_PROXY http_proxy https_proxy no_proxy
)

if [ "$#" -eq 0 ]; then
  echo "mcp-env.sh: no command given — nothing was launched." >&2
  echo "  usage: mcp-env.sh <command> [<arg>...]" >&2
  exit 2
fi

if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
  sed -n '3,4p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
  printf 'passes only: %s\n' "${ALLOWED[*]}"
  exit 0
fi

# `${!name+set}` distinguishes unset from set-and-empty, which `-n` and `-z` cannot: an empty TMPDIR
# passed through as `TMPDIR=` makes npx write to a path that is the empty string rather than falling
# back to /tmp.
keep=()
for name in "${ALLOWED[@]}"; do
  if [ -n "${!name+set}" ]; then
    keep+=("$name=${!name}")
  fi
done

# `env -i` and not `env -u` per unwanted name: the list of variables to remove is unknowable, since
# it is whatever the human happened to export. Only an allow-list is a claim this file can make.
exec env -i "${keep[@]}" "$@"
