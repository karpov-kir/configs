#!/usr/bin/env bash
# Configure public MCP servers in one project's client files; never user settings.
# Usage: project-mcp.sh --agent=claude|codex [--dry-run] [--uninstall] <project>
# tested by: project-mcp-test.sh
set -euo pipefail
here="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
agent=""
project=""
is_dry_run=false
is_uninstall=false
for arg in "$@"; do
  case "$arg" in
    --agent=claude | --agent=codex) agent="${arg#*=}" ;;
    --dry-run) is_dry_run=true ;;
    --uninstall) is_uninstall=true ;;
    -h | --help) sed -n '2,3p' "${BASH_SOURCE[0]}"; exit 0 ;;
    -*) echo "project MCP: unknown option $arg" >&2; exit 2 ;;
    *)
      [ -z "$project" ] || { echo 'project MCP: select one project' >&2; exit 2; }
      project="$arg"
      ;;
  esac
done
[ -n "$agent" ] && [ -n "$project" ] && [ -d "$project" ] || {
  echo 'project MCP: --agent=claude|codex and an existing project directory are required' >&2
  exit 2
}
if command -v node >/dev/null 2>&1; then
  exec node "$here/project-mcp.mjs" "$@"
fi
# Lookup and provisioning must not evaluate the caller's mise configuration or hooks.
node_dir="$(MISE_NO_CONFIG=1 MISE_NO_ENV=1 MISE_NO_HOOKS=1 mise where node@lts 2>/dev/null || true)"
if [ -x "$node_dir/bin/node" ]; then
  exec "$node_dir/bin/node" "$here/project-mcp.mjs" "$@"
fi
if $is_dry_run; then
  echo 'project MCP: would update public browser servers; validation deferred until Node is installed through mise'
  exit 0
fi
if $is_uninstall; then
  echo 'project MCP: Node is required to uninstall safely; install Node through mise, then retry' >&2
  exit 1
fi
command -v mise >/dev/null 2>&1 || { echo 'project MCP: mise is required to provide Node' >&2; exit 1; }
MISE_NO_CONFIG=1 MISE_NO_ENV=1 MISE_NO_HOOKS=1 exec mise exec node@lts -- node "$here/project-mcp.mjs" "$@"
