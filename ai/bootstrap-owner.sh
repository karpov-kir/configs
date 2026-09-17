#!/usr/bin/env bash
#
# Install the maintainer tier, RTK, and shared owner instructions for either client.
# Owner memory lives separately; ai/README.md names the store and what an install does with it.
#
#   usage: ai/bootstrap-owner.sh --agent=claude|codex [--uninstall] [other ai/bootstrap.sh options]
#
# tested by: bootstrap-test.sh, through the --owner flag this passes.
set -uo pipefail

repo="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"

[ -x "$repo/bootstrap.sh" ] || {
  printf 'ai/bootstrap-owner.sh: %s/bootstrap.sh is missing or not executable — nothing was installed\n' "$repo" >&2
  exit 2
}

exec "$repo/bootstrap.sh" --owner "$@"
