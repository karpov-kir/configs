#!/usr/bin/env bash
#
# The owner's machine: everything ai/bootstrap.sh installs, plus the maintainer skills, rtk, and this
# checkout's own CLAUDE.md as the machine's instruction file.
#
#   usage: ai/bootstrap-owner.sh [any ai/bootstrap.sh option]
#
# There is nothing here but the exec below, and that is the point. Every step this tier adds is a
# conditional inside ai/bootstrap.sh behind `--owner`, so all of them — the rtk formula, the RTK.md
# leftover, the instruction-file mount — take part in the same mount survey and the same report as
# everything else.
#
# Doing the extras HERE and then exec'ing would put the instruction-file mount outside
# lib/mount.sh's second-checkout guard: a write before the survey, on precisely the mount the guard
# exists to protect. `exec` also replaces this process, so anything after it would never run.
#
# `exec` rather than a call for a second reason: lib/mount.sh recognises a foreign checkout by finding
# a file of the RUNNING script's name at the same depth under the root a link resolves to. Exec'ing
# leaves that name `bootstrap.sh`, which is what every machine's mounts were written by. Sourcing
# ai/bootstrap.sh from here instead would make the guard hunt for `bootstrap-owner.sh` in the foreign
# root, never match, and report "no mount comes from another checkout" before repointing all of them.
#
# tested by: bootstrap-test.sh, through the --owner flag this passes.
set -uo pipefail

repo="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"

[ -x "$repo/bootstrap.sh" ] || {
  printf 'ai/bootstrap-owner.sh: %s/bootstrap.sh is missing or not executable — nothing was installed\n' "$repo" >&2
  exit 2
}

exec "$repo/bootstrap.sh" --owner "$@"
