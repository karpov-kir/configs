#!/usr/bin/env bash
# Print the path to a runnable binary for <tool>, or exit 2 saying which way it could not be reached.
#
# Usage: resolve.sh <tool>            # <tool> is a directory name under ai/tools
#
# Callers are the stubs in each skill's scripts/ directory. They pass the tool name, take the path
# from stdout, and exec it. Everything about *finding* a binary lives here so the stubs stay a name
# and an exec.
#
# Order: a binary already at bin/<tool>, then a local `go build` when the source is here. The first
# branch is what a release install lands on, and it is why installing these skills does not need a Go
# toolchain. Set ECO_TOOLS_BUILD=1 to skip it — a downloaded binary is preferred over source that may
# be newer, because git does not preserve mtimes and a fresh clone would otherwise rebuild every
# tool on a machine that has no Go.
#
# Every failure exits 2 and names what did not happen. These tools report findings, and exit 0 with
# no findings is what a clean tree looks like, so a tool that could not run must never reach a caller
# as silence. Printing nothing on stdout is not enough: the caller has to be told, and told loudly.
#
# tested by: resolve-test.sh
set -euo pipefail

die() {
  printf 'resolve.sh: %s\n' "$1" >&2
  exit 2
}

# `CDPATH=` because `cd` consults it for a relative path and echoes where it landed, which would put
# a second line into the command substitution and corrupt every path built from it. `pwd -P` so the
# tools directory is found through the symlink each skill is mounted by, never from cwd — this runs
# from whatever repository the human is working in.
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

if [ "${ECO_TOOLS_BUILD:-}" != 1 ] && [ -e "$binary" ]; then
  # An existing-but-unrunnable binary is a half-finished install. Reported rather than built over:
  # building needs Go, so papering over it would work on a developer's machine and fail on the
  # machine the install was for.
  [ -f "$binary" ] || die "$binary exists but is not a regular file — remove it and install again"
  [ -x "$binary" ] || die "$binary is not executable — the install did not complete; chmod +x it or install again"
  printf '%s\n' "$binary"
  exit 0
fi

[ -d "$tools/$tool" ] ||
  die "no prebuilt binary at $binary and no source at $tools/$tool — this skill is mounted from a checkout that ships neither, and $tool did NOT run"
command -v go >/dev/null 2>&1 ||
  die "no prebuilt binary at $binary and go is not installed, so $tool did NOT run — that is unchecked, not clean"

mkdir -p "$tools/bin" || die "cannot create $tools/bin, so $tool did NOT run"

# A package directory holding cmd/ keeps its main there, so the library beside it can be driven by
# the suite without a process per case.
package="./$tool/"
[ -d "$tools/$tool/cmd" ] && package="./$tool/cmd/"

# Built to a temp name and moved, because `go build -o` writes the binary in place and two skills
# running at once would otherwise let one exec what the other is half way through writing. The move
# is within one directory, so it is atomic.
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

printf '%s\n' "$binary"
