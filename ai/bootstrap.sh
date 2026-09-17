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
# Byte-identical in every stub, held so by the wiring check's shared-region scan. Copied rather than
# sourced because sourcing a file is executing it, and these run from whatever repo the human is in.
die() {
  printf '%s: %s\n' "${0##*/}" "$1" >&2
  exit 2
}

# `CDPATH=` because `cd` echoes where it landed when the path is relative, which would put a second
# line into this substitution and corrupt every path built from it. `pwd -P` resolves the symlink the
# skill is mounted by, so the tools directory is found from this file's real location, not from cwd.
here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" ||
  die "cannot resolve my own directory, so $tool could not be located"

# Exactly one path, named by the stub above rather than searched for here. The stubs sit at three
# depths, so anything that guesses between them is a stub reaching a directory it does not name: an
# upward walk leaves a checkout shipping no `ai/tools/` and execs the first `tools/resolve.sh` in any
# ancestor, and a list of relative candidates resolves outside the repository for the stubs one level
# above the tools directory. Either runs a stranger's binary at exit 0.
resolver="$here/$tools_offset/tools/resolve.sh"
[ -e "$resolver" ] ||
  die "no resolver at $resolver — this skill is mounted from a checkout that does not ship ai/tools/, and $tool did NOT run"
[ -x "$resolver" ] ||
  die "$resolver is not executable, so $tool did NOT run — chmod +x it"

# The resolver names its own failures on stderr, so nothing is re-reported here. Its status is NOT
# passed through: the 2 below is deliberate rather than a copy of it. Every way a resolver can fail
# means the tool did not run, which is 2 in this repo's vocabulary, and 3 (ran, and refuses a result)
# must never reach a caller for a binary that never started. `ai/tools/resolve.sh` exits 2 for all of
# them today, so keep the literal 2 if it ever grows a code.
binary="$("$resolver" "$tool")" || exit 2
[ -n "$binary" ] && [ -x "$binary" ] ||
  die "the resolver named no runnable binary for $tool, so it did NOT run"

# The build about to answer, handed to the tool rather than printed: a stub's own output is a value
# callers parse. Empty when nothing stamped it. `ai/tools/resolve.sh` carries why.
export ECO_TOOL_BUILD="$(cat "$binary.stamp" 2>/dev/null || true)"

# The checkout that answered, which the build stamp does not name: the stamp hashes source, so it moves
# when the source does and says nothing about which commit the tree sits on. `ai/tools/resolve.sh`
# carries why the two are both needed.
export ECO_TOOL_TREE="$(git -C "${resolver%/*}" rev-parse HEAD 2>/dev/null || true)"

# `-a "$0"` keeps argv[0] as the path this was invoked by. The tools derive their skill directory from
# it, so a skill reached through its symlink mount still finds its own ledger, template and siblings.
exec -a "$0" "$binary" "$@"
# --- end shared:tool-stub ---
