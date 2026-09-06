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
# ECO_TOOLS_BUILD=1 skips it. That binary is served only while it was built from the source beside
# it: source-stamp.sh hashes that source, and the stamp written next to the binary says what the
# binary came from. When the two differ this rebuilds, so an edit is never measured through the
# build that came before it.
#
# A hash and not a timestamp, because the binary that has to be caught is one NEWER than the source
# it disagrees with, and every downloaded release binary is. source-stamp.sh's header has the rest.
#
# Where the binary came from something else, or cannot be compared at all, and nothing here can
# rebuild, it is served with a warning on stderr.
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

# Written by whoever puts the binary there — this script after a build, install.sh after a download.
stamp="$binary.stamp"

# Three answers: 0 built from this source, 1 built from something else, 2 could not tell. The third
# earns its place for the reason the whole file exists: a check that could not run must never be
# served as one that came back clean.
#
# The stamp covers every non-test Go file in the module rather than a guess at which of them this
# tool compiles. source-stamp.sh carries why: the guess went blind on a library that cmd/ also backs.
built_from_this_source() {
  local want held
  # A checkout carrying binaries with no Go source beside them — the shape a skill mounted from a
  # source-less checkout has — leaves nothing to compare against, so there is no doubt to report. Not
  # a release install: that lands its binaries in a full checkout, which does carry the source.
  #
  # What says which shape this is, is the module file, not the missing directory. A checkout that
  # ships Go source and has none for THIS name holds an orphan: a binary from a tool since renamed,
  # or one nothing here put there. bin/ is gitignored, so an orphan appears in no diff and no
  # `git status`, and answering "built from this source" for it serves it at exit 0 in silence — the
  # one way a binary nobody can account for keeps being exec'd over the human's repositories. Report
  # it as the unknown it is instead, and let the caller below rebuild it or warn.
  if [ ! -d "$tools/$tool" ] && [ ! -d "$tools/cmd/$tool" ]; then
    if [ -f "$tools/go.mod" ]; then
      return 2
    fi
    return 0
  fi
  want="$("$tools/source-stamp.sh" "$tool")" || return 2
  held="$(cat "$stamp" 2>/dev/null)" || return 2
  [ "$held" = "$want" ]
}

if [ "${ECO_TOOLS_BUILD:-}" != 1 ] && [ -e "$binary" ]; then
  # An existing-but-unrunnable binary is a half-finished install, reported rather than built over:
  # building needs Go, so papering over it works on a developer's machine and fails on the install's.
  [ -f "$binary" ] || die "$binary exists but is not a regular file — remove it and install again"
  [ -x "$binary" ] || die "$binary is not executable — the install did not complete; chmod +x it or install again"
  # What this catches: someone edits the Go, and every run afterwards measures the previous build
  # while reading exactly like a run against the edit.
  #
  # `|| verdict=$?` because 1 and 2 are answers, and `set -e` would take the shell down on both.
  verdict=0
  built_from_this_source || verdict=$?
  if [ "$verdict" -eq 0 ]; then
    serve
  fi
  # Past here the binary is either wrong or unproven, and a rebuild settles both outright, so
  # reporting is only what is left when one is impossible. Rebuilding needs the source and a
  # toolchain; without them this binary is the only way to run the tool at all, so it is served and
  # the doubt printed alongside it. Keep the two messages apart: one says the bytes are wrong, the
  # other says nobody knows.
  if [ ! -d "$tools/$tool" ] || ! command -v go >/dev/null 2>&1; then
    if [ "$verdict" -eq 1 ]; then
      printf 'resolve.sh: %s was not built from the source beside it and cannot be rebuilt here — what %s reports may not be the code you are reading\n' \
        "$binary" "$tool" >&2
    else
      printf 'resolve.sh: %s could NOT be compared with the source beside it, so it is served unchecked — a check that did not run is not a clean one\n' \
        "$binary" >&2
    fi
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

# What these bytes were built from, so the next run can hold them against the source without
# rebuilding. A stamp that cannot be written is removed rather than left: an old one beside new bytes
# is a wrong answer, and a missing one only costs the next run a rebuild.
if source_stamp="$("$tools/source-stamp.sh" "$tool")"; then
  printf '%s\n' "$source_stamp" >"$stamp" || rm -f "$stamp"
else
  rm -f "$stamp"
fi

serve
