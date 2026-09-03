#!/usr/bin/env bash
# Print the path to a runnable binary for <tool>, or exit 2 saying which way it could not be reached.
#
#   usage: resolve.sh <tool>          # <tool> is a directory name under ai/tools
#
# Callers are the stubs in each skill's scripts/ directory. Everything about *finding* a binary lives
# here, so a stub stays a tool name and an exec.
#
# Order: a binary already at bin/<tool>, then a local `go build` when the source is here. The first
# branch is what a release install lands on, and why installing these skills needs no Go toolchain.
# ECO_TOOLS_BUILD=1 skips it. That binary is served only while no Go source in the module is newer
# than it; otherwise this rebuilds, so an edit is not measured through the build before it. bin/ is
# gitignored, so no checkout carries a binary: install.sh downloads one afterwards, newer than every
# source, and an install still pays no build. Where it is older and nothing here can rebuild, it is
# served with a warning on stderr.
#
# Every failure exits 2 and names what did not happen. These tools report findings, so exit 0 with
# none is what a clean tree looks like, and a tool that could not run must never reach a caller as
# silence: an empty stdout is not enough, the caller has to be told.
#
# tested by: resolve-test.sh
set -euo pipefail

die() {
  printf 'resolve.sh: %s\n' "$1" >&2
  exit 2
}

# `CDPATH=` because `cd` echoes where it landed when the path is relative, which would put a second
# line into this substitution and corrupt every path built from it. `pwd -P` finds the tools directory
# through the symlink each skill is mounted by, never from cwd: this runs from the human's own repo.
tools="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" ||
  die "cannot resolve my own directory, so no tool can be located"

[ $# -eq 1 ] || die "usage: resolve.sh <tool>"
tool="$1"

# A tool name is a directory name here, so anything that could climb out of this directory or name
# something other than a plain entry is refused before it reaches a path.
case "$tool" in
  "" | *[!a-z0-9-]*) die "'$tool' is not a tool name — expected lowercase letters, digits and dashes" ;;
esac

binary="$tools/bin/$tool"

serve() {
  printf '%s\n' "$binary"
  exit 0
}

# The whole module, never just `$tools/$tool`: every tool imports `kk-flavor/tools/shell` and most
# import `eco-root`, so an edit there leaves the tool's directory untouched and its binary stale.
#
# Three answers: 0 newer, 1 nothing newer, 2 could not tell. The third earns its place because a
# `find` that runs and fails prints nothing, and on the output alone that is byte-for-byte what a
# clean tree looks like. Reading the status is what stops "it broke" from being served as "nothing
# is newer" — the defect the branch below exists to close, one layer further in.
sources_newer_than() {
  local listing status=0
  listing=$(find "$tools" -name '*.go' -newer "$1" -print 2>/dev/null) || status=$?
  [ "$status" -eq 0 ] || return 2
  [ -n "$listing" ]
}

if [ "${ECO_TOOLS_BUILD:-}" != 1 ] && [ -e "$binary" ]; then
  # An existing-but-unrunnable binary is a half-finished install, reported rather than built over:
  # building needs Go, so papering over it works on a developer's machine and fails on the install's.
  [ -f "$binary" ] || die "$binary exists but is not a regular file — remove it and install again"
  [ -x "$binary" ] || die "$binary is not executable — the install did not complete; chmod +x it or install again"
  # What this catches: someone edits the Go, and every run afterwards measures the previous build
  # while reading exactly like a run against the edit. It rests on install.sh landing bin/<tool>
  # after the checkout; an installer that preserved the asset's own timestamp would invert it.
  #
  # `|| staleness=$?` because 1 and 2 are answers, and `set -e` would take the shell down on both.
  staleness=0
  sources_newer_than "$binary" || staleness=$?
  # A `find` that is absent lands here alongside one that ran and failed: the missing one exits 127
  # out of the command substitution, which is a status like any other. Either way the question went
  # unanswered, and answering it "nothing is newer" would be the stale-binary defect above wearing
  # the shape of a pass. So the cause is named and the gap printed instead of guessed at.
  if [ "$staleness" -eq 2 ]; then
    if command -v find >/dev/null 2>&1; then
      cause='find failed here'
    else
      cause='no find on PATH'
    fi
    printf 'resolve.sh: %s, so whether %s is older than its sources was NOT checked — it is served unchecked\n' \
      "$cause" "$binary" >&2
    serve
  fi
  if [ "$staleness" -eq 1 ]; then
    serve
  fi
  # Rebuilding needs both the source and a toolchain. Without them this binary is the only way to
  # run the tool at all, so it is served and the doubt printed alongside it.
  if [ ! -d "$tools/$tool" ] || ! command -v go >/dev/null 2>&1; then
    printf 'resolve.sh: %s is older than the Go sources beside it and cannot be rebuilt here — what %s reports may not be the code you are reading\n' \
      "$binary" "$tool" >&2
    serve
  fi
fi

[ -d "$tools/$tool" ] ||
  die "no prebuilt binary at $binary and no source at $tools/$tool — this skill is mounted from a checkout that ships neither, and $tool did NOT run"
command -v go >/dev/null 2>&1 ||
  die "no prebuilt binary at $binary and go is not installed, so $tool did NOT run — that is unchecked, not clean"

mkdir -p "$tools/bin" || die "cannot create $tools/bin, so $tool did NOT run"

# A tool whose main lives under cmd/ keeps its library in `<tool>/`, so the suite can drive that
# package without a process per case. One directory per tool under cmd/, never a `cmd/` inside each
# tool: `go build -o <dir>/ ./...` names every binary after its own directory, so three mains in
# directories all called `cmd` overwrite one another and the build stays green two tools short.
package="./$tool/"
[ -d "$tools/cmd/$tool" ] && package="./cmd/$tool/"

# Built to a temp name and moved: `go build -o` writes in place, so two skills running at once would
# let one exec what the other is half way through writing. The move stays in one directory, so atomic.
staging="$tools/bin/.$tool.$$"
# stdout is the path this prints and nothing else, so the build's own chatter goes to stderr.
if ! (CDPATH= cd "$tools" && go build -o "$staging" "$package") >&2; then
  rm -f "$staging"
  die "$tool did not build, so it did NOT run"
fi
mv -f "$staging" "$binary" || {
  rm -f "$staging"
  die "$tool built but could not be moved to $binary, so it did NOT run"
}

serve
