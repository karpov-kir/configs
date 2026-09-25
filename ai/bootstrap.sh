#!/usr/bin/env bash
#
# Install this repository's agent ecosystem on this machine: the shared ~/.kk-flavor bucket, the skills
# the chosen tier takes, the client's instruction file, and the packages and registries those need.
#
#   usage: bootstrap.sh --agent=claude|codex [--dry-run] [--relocate] [--maintainer] [--owner] [--skip-brew] [--skip-tools] [--skip-mcp] [--skip-rtk] [--skip-verify] [--skip-models] [--uninstall]
#
# The recipe is Go, in `ai/tools/ai-bootstrap/`. What is left here is the part of a machine install
# that cannot be Go: reaching a Go binary on a machine that has none.
#
# tested by: the Go suite in ai/tools/ai-bootstrap/, the stub region by the Go suite in ai/tools/reach/.
set -euo pipefail

tool="ai-bootstrap"
# How far THIS file sits above the tools directory.
tools_offset="."

# --- the toolchain, and why it is still shell -------------------------------------------------------

# Everything past the exec at the end of this file is a Go subcommand, and a binary has to exist
# first. `ai/tools/install.sh` puts one there on a machine with no Go, downloading the release this
# repository cut with `gh` alone. `ai/tools/resolve.sh` builds from source where a toolchain is
# present instead.

# A fresh clone on a machine that lacks Go would otherwise get the resolver's refusal, and the
# install would stop there. That is the machine a bootstrap is for.

# The guard fires only when a binary is absent and Go is too, so a machine already set up pays two
# tests, and no network call goes out. `|| true`: install.sh exits 3 where this repository has cut
# no release, and that is a normal state the tools step inside the run reports. Every other way it
# fails leaves no binary, and the resolver at the end of this file is what says so.

# This uses a directory of its own, and not the stub region's `$here`. That region is copied byte for
# byte into every stub, and a line added inside it would break the scan that holds the copies
# identical.
prologue_here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" || prologue_here=""
if [ -n "$prologue_here" ] &&
  [ ! -x "$prologue_here/tools/bin/$tool" ] &&
  ! command -v go >/dev/null 2>&1 &&
  [ -x "$prologue_here/tools/install.sh" ]; then
  "$prologue_here/tools/install.sh" || true
fi

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
