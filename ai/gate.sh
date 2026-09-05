#!/usr/bin/env bash
# The pre-commit gate: every check this repo gates on, run only where the change could have moved it.
#
#   usage: gate.sh [--full] [--mutants] [--units] [--why <unit>]
#          (no flag)  the fast path — run what is stale, skip what is not, defer the mutation harnesses
#          --full     run everything from cold, ignoring and then refreshing every cached verdict
#          --mutants  settle the deferred mutation units, and nothing else
#          --units    print the unit table with each unit's freshness, and stop
#          --why      print the input files one unit is keyed on, and stop
#
# Skipping is sound, not a sample: every check is a pure function of a declared set of input files
# plus the toolchain, so a unit whose inputs hash to the last green run's already has its verdict.
#
# It may never report a pass for a unit it did not run, resolve a unit to an empty input set, finish
# having resolved nothing, or skip anything quietly. Each of those exits 2 and says so.
#
# A fast path beside the full sweep, never instead of it: `.github/workflows/gates.yml` still runs
# every command from cold on every push, and `--full` is the same sweep on demand.
#
# The gate is Go, in `ai/tools/gate/`. Don't put it back in shell: keying 60-odd units there measured
# 9.5s for `--units`, against 1.0s here.
#
# tested by: the Go suite beside the tool, `ai/tools/gate/`; the shared stub region below by
# tool-stub-test.sh, and the resolver it calls by resolve-test.sh.

set -euo pipefail

tool="gate"
# How far THIS file sits above the tools directory.
tools_offset="."

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

# `-a "$0"` keeps argv[0] as the path this was invoked by. The tools derive their skill directory from
# it, so a skill reached through its symlink mount still finds its own ledger, template and siblings.
exec -a "$0" "$binary" "$@"
# --- end shared:tool-stub ---
