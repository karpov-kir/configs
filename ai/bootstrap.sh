#!/usr/bin/env bash
#
# Install this repository's agent ecosystem on the owner's machine: the shared ~/.kk-flavor bucket, every
# skill, the owner instruction copy, and the packages and registries those need.
#
#   usage: bootstrap.sh --agent=claude|codex [--dry-run] [--relocate] [--skip-brew] [--skip-mcp] [--skip-rtk] [--skip-verify] [--skip-models] [--uninstall]
#
# The recipe is Go, in `ai/tools/ai-bootstrap/`, and this stub builds it from source on first use.
# Install Go with mise first: the repository's mise.toml pins it.
#
# tested by: the Go suite in ai/tools/ai-bootstrap/, the stub region by the Go suite in ai/tools/reach/.
set -euo pipefail

tool="ai-bootstrap"
# How far THIS file sits above the tools directory.
tools_offset="."

# --- shared:tool-stub ---
# Byte-identical in every stub, which the wiring check's shared-region scan enforces.
#
# Each stub carries its own copy. One shared file would be executed by the source call that read it,
# and a stub runs from whatever repository the human is standing in.

# What lives here is the part that cannot move: a stub has to find the resolver before the resolver
# can decide anything. ai/tools/resolve.sh owns the rest, argv[0] included. Its header says why each
# line here has the shape it has: the `cd -P`, the declared offset, the two guards, the exec.
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
