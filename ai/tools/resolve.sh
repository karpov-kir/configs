#!/usr/bin/env bash
# Reach a runnable binary for <tool>, or exit 2 saying which way it could not be reached.
#
#   usage: resolve.sh <tool>                            # print the path, and nothing else
#          resolve.sh --run <tool> <argv0> [argument …] # and exec it, under that argv[0]
#
# Callers are the stubs in each skill's scripts/ directory, which take the second spelling. Everything
# about reaching a binary lives here, so a stub is a tool name, its depth, and one exec: a stub has to
# find THIS file before this file can decide anything, and that is the whole of what it can hold.
#
# `--run` replaces the calling stub rather than answering it: this file runs IN the stub's process and
# execs the binary from there, where a stub that read a path back forked one shell to get it. With the
# identity reads that went with it, a warm invocation of a stub fell from 15 processes to 12, measured.
# `<argv0>` is the stub's own `$0`, passed rather than inherited: the tools derive their skill directory
# from argv[0], so a skill reached through its symlink mount still finds its own ledger and siblings.
#
# What the stub region still holds, and why each line of it is the shape it is:
#
#   `CDPATH=`, because `cd` echoes where it landed when the path is relative, which would put a second
#   line into that substitution and corrupt every path built from it. `pwd -P`, because it resolves the
#   symlink the skill is mounted by: the resolver is found from the stub's real location, never cwd.
#
#   One declared offset, never a search. The stubs sit at four depths, and both ways of guessing
#   between them reach a tools directory the stub does not name: an upward walk execs the first
#   `tools/resolve.sh` in any ancestor of a checkout that ships none, and a list of relative candidates
#   resolves outside the repository for the shallowest stubs. Either runs a stranger's binary at exit 0.
#
#   Two guards and two messages, because the fixes differ: no resolver means a checkout that ships no
#   `ai/tools/`, and one without its exec bit means a half-finished install. Both exit 2, and both say
#   the tool did NOT run — these tools report findings, so silence from one reads as a clean tree.
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
# A build takes bin/<tool>.lock and holds it over the stamping, the build, the move and the stamp write,
# because two builds of one tool run at once: the gate runs its checks concurrently and every stub execs
# this file. The move is atomic within the directory and the stamp write beside it is not, so without the
# lock the binary of one build ends up beside the stamp of the other and is served as current. A lock is
# broken on its age, because a build killed outright runs no trap of its own.
#
# Where the binary came from something else, or cannot be compared at all, and nothing here can
# rebuild, it is served with a warning on stderr.
#
# Every failure exits 2 and names what did not happen. These tools report findings, so exit 0 with
# none is what a clean tree looks like, and a tool that could not run must never reach a caller as
# silence: an empty stdout is not enough, the caller has to be told.
#
# tested by: the Go suite in ai/tools/reach/, which execs this script once per case.
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

# How far the module root sits above this directory. go.mod is at the repository root rather than here,
# so that Go's test cache hashes every file a case opens: that cache is keyed on the module, and a file
# above the module root is skipped rather than hashed, which is how a suite reading the checkout used to
# answer `ok (cached)` over an edit it should have gone red on. source-stamp.sh declares the same
# offset, and the two have to move together.
module="$tools/../.."

# `exec` is the mode a stub takes and `print` the one a human or another script takes. Held in a word of
# its own rather than inferred from argv0 being set: read that way, `--run <tool> ""` would quietly fall
# back to printing a path to a caller that is waiting to be replaced, and exit 0 having run nothing.
mode="print"
argv0=""
forward=()
if [ "${1:-}" = "--run" ]; then
  [ $# -ge 3 ] || die "usage: resolve.sh --run <tool> <argv0> [argument …]"
  mode="exec"
  tool="$2"
  argv0="$3"
  shift 3
  forward=("$@")
else
  [ $# -eq 1 ] || die "usage: resolve.sh <tool>"
  tool="$1"
fi

# A tool name is a directory name here, so anything that could climb out of this directory or name
# something other than a plain entry is refused before it reaches a path.
case "$tool" in
  "" | *[!a-z0-9-]*) die "'$tool' is not a tool name — expected lowercase letters, digits and dashes" ;;
esac

binary="$tools/bin/$tool"

# The one way out that is not a refusal. Under `--run` nothing is printed at all: stdout belongs to the
# tool from here on, and a path on it would be a line every caller of every stub had to learn to drop.
#
# `${forward[@]+…}` because bash 3.2 — which is still /bin/bash on macOS — reads an empty array under
# `set -u` as unbound, and a tool invoked with no arguments is the common case, not the odd one.
serve() {
  if [ "$mode" = exec ]; then
    exec -a "$argv0" "$binary" ${forward[@]+"${forward[@]}"}
  fi
  printf '%s\n' "$binary"
  exit 0
}

# Written by whoever puts the binary there — this script after a build, install.sh after a download.
#
# It is also the build identity every stub exports as ECO_TOOL_BUILD, so a tool that reports a
# measurement can name what produced it. `992662a` settled the other half — a scanner number names the
# commit it was read off — and a reading whose instrument is unnamed cannot be compared with one taken
# later: a rebuild here moved voice-check's attribution figures on an unchanged tree, with the output
# silent about it. The stamp carries this. It moves exactly when the build does, and costs one file
# read. A hash of the binary on every invocation would cost more. Never the stub: that file barely
# changes, so its identity would say little about the build.
#
# Read by the tool that reports it, off its own `os.Executable()`, and never handed down from here: the
# stub used to export it beside a `git rev-parse`, which cost every one of the 23 tools two processes
# per invocation to carry a line one of them prints. voice-check/bar.go is that one, and it reads
# the checkout's own commit there too — a different fact from this one, because a tree can hold a stamp
# that matches its own source perfectly and still be a commit nobody else has. The mount resolves to one
# checkout's working tree, so a session reading source, running a binary or loading a skill through it
# gets whatever that tree currently holds. Observed: a skill appeared in a live session's list and
# vanished two turns later as that checkout moved and moved back.
#
# It names the SOURCE, not the bytes: identical source built under two Go toolchains stamps the same and
# can still behave differently. Narrow, and stated rather than built for — but do not read a matching
# stamp as a bytes-identical guarantee.
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
    if [ -f "$module/go.mod" ]; then
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

# The stamping, the build, the move and the stamp write are one critical section per tool. The move is
# atomic within the directory and the stamp write beside it is not, so two builds over source that changed
# between them otherwise leave the binary of one beside the stamp of the other: the run after holds that
# stamp against the source, finds it current, and serves the older binary at exit 0 in silence.
#
# `mkdir` is the mutex, because it is atomic on every filesystem this runs on. It is taken here rather than
# at the top of the file: a binary already built from this source is served far above, so no warm
# invocation of any stub reaches this line or pays anything for it.
lock="$binary.lock"

# How long a lock may go on existing before a waiter takes it for abandoned. A build of one of these tools
# takes seconds, the trap below gives the lock up on every signal a shell can catch, and only a process
# killed outright leaves one behind. Whole minutes, because `find -mmin` is what asking a file's age costs
# without GNU stat. Waiting on it instead would wedge every session on the machine behind a lock nobody
# holds.
lock_abandoned_minutes=5

# A waiter polls five times a second. There is nothing in bash 3.2 — which is still /bin/bash on macOS —
# to wait on a directory being removed, and the fractional sleep both machines this runs on accept is what
# keeps a queued tool off a whole second it did not need.
waited=""
while ! mkdir "$lock" 2>/dev/null; do
  [ -d "$lock" ] || die "cannot create $lock, so $tool did NOT run"
  if [ -n "$(find "$lock" -maxdepth 0 -mmin "+$lock_abandoned_minutes" 2>/dev/null)" ]; then
    rmdir "$lock" 2>/dev/null || :
  else
    waited=1
    sleep 0.2
  fi
done

# The build that failed, and the one a signal stopped, both give the lock up here. `serve` does not: it
# execs, and an exec runs no trap, so every path that reaches it gives the lock up on the line before.
trap 'rmdir "$lock" 2>/dev/null || :' EXIT
trap 'rmdir "$lock" 2>/dev/null || :; exit 2' HUP INT TERM

release_lock() {
  trap - EXIT HUP INT TERM
  rmdir "$lock" 2>/dev/null || :
}

# The build this run waited for may be the one it needed, and past the lock the stamp says so. Compiling
# the same source a second time is all that skipping this saves, and it is what every tool the gate
# launches at once would otherwise pay. Never under ECO_TOOLS_BUILD=1: that flag is a caller saying the
# bytes have to come from a build in this tree, and a stamp names the source rather than the bytes.
if [ "${ECO_TOOLS_BUILD:-}" != 1 ] && [ -n "$waited" ] && [ -x "$binary" ] && built_from_this_source; then
  release_lock
  serve
fi

# A tool whose main lives under cmd/ keeps its library in `<tool>/`, so the suite can drive that
# package without a process per case. One directory per tool under cmd/, never a `cmd/` inside each
# tool: `go build -o <dir>/ ./...` names every binary after its own directory, so three mains in
# directories all called `cmd` overwrite one another and the build stays green two tools short.
package="./$tool/"
[ -d "$tools/cmd/$tool" ] && package="./cmd/$tool/"

# Taken from the source the compiler is about to read, and written after the move below. An edit landing
# while the build runs would otherwise be stamped over bytes it never reached, which is the pair this lock
# exists to prevent, one process wide. Stamping first errs the other way, toward a rebuild nobody needed.
source_stamp="$("$tools/source-stamp.sh" "$tool")" || source_stamp=""

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
if [ -n "$source_stamp" ]; then
  printf '%s\n' "$source_stamp" >"$stamp" || rm -f "$stamp"
else
  rm -f "$stamp"
fi

release_lock
serve
