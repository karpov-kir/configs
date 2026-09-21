#!/usr/bin/env bash
#
# Install this repository's agent ecosystem on this machine: the shared ~/.kk-flavor bucket, the skills
# the chosen tier takes, the client's instruction file, and the packages and registries those need.
#
#   usage: bootstrap.sh --agent=claude|codex [--dry-run] [--relocate] [--maintainer] [--owner] [--skip-brew] [--skip-tools] [--skip-mcp] [--skip-rtk] [--skip-verify] [--uninstall]
#
# The recipe is Go, in `ai/tools/ai-bootstrap/`. What is left here is the one part of a machine install
# that cannot be: reaching a Go binary on a machine that has none.
#
# tested by: the Go suite in ai/tools/ai-bootstrap/; stub region by the Go suite in ai/tools/reach/.
set -euo pipefail

tool="ai-bootstrap"
# How far THIS file sits above the tools directory.
tools_offset="."

# --- the toolchain, and why it is still shell -------------------------------------------------------
#
# Everything past the handover below is a Go subcommand, and none of it can run until a binary exists.
# `ai/tools/install.sh` is what puts one there on a machine with no Go — it downloads the release this
# repository cut, needing nothing but `gh` — and `ai/tools/resolve.sh` builds from source where there
# is a toolchain instead. Without this call a fresh clone on a machine with no Go gets the resolver's
# refusal and no install at all, which is the one machine a bootstrap is for.
#
# Only when nothing can answer yet, so a machine that is already set up pays two tests and no network.
# `|| true`: install.sh exits 3 when this repository has cut no release, which is not a failure and is
# reported properly by the tools step inside the run. Every other way it fails leaves no binary, and
# the resolver below is what says so.
#
# Its own directory rather than the stub region's `$here`, because that region is copied byte for byte
# into every stub and a line added inside it would break the scan that holds the copies identical.
prologue_here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" || prologue_here=""
if [ -n "$prologue_here" ] &&
  [ ! -x "$prologue_here/tools/bin/$tool" ] &&
  ! command -v go >/dev/null 2>&1 &&
  [ -x "$prologue_here/tools/install.sh" ]; then
  "$prologue_here/tools/install.sh" || true
fi

# --- shared:tool-stub ---
# Byte-identical in every stub. The wiring check's shared-region scan holds it so.
#
# Each stub carries a copy instead of sourcing one file. Sourcing a file executes it, and a stub runs
# from whatever repository the human is standing in. So the only part that lives here is the part that
# cannot move: a stub has to find the resolver before the resolver can decide anything.
#
# ai/tools/resolve.sh owns everything after that, argv[0] included. Its header says why each line
# below has the shape it has: the `cd -P`, the declared offset, the two guards, the exec.
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
