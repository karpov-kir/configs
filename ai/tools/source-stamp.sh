#!/usr/bin/env bash
# Print the SHA-256 of the Go source <tool> is built from, so a binary in bin/ can be held against the
# source sitting beside it.
#
#   usage: source-stamp.sh <tool>     # <tool> is a directory name under ai/tools
#
# Content, because nothing else survives the trip from the build machine to this one. git does not
# preserve mtimes, and a release binary is written into bin/ long after the checkout it lands in, so a
# timestamp reads a stale binary as fresh. The revision Go embeds — `go version -m`, readable even
# without a toolchain — names a commit and not the bytes, so two different edits of one commit record
# the same revision.
#
# The set is every non-test Go file in the module, plus go.mod — not a per-tool subset. A directory
# that cmd/ holds a main for looks like that tool's private source and can still be a library this one
# imports: eco-report imports tree-fingerprint, which cmd/tree-fingerprint also backs. A subset that
# guesses wrong goes blind in silence, which is the defect this file exists to end, while covering too
# much costs only a rebuild the next run would make anyway. `_test.go` is the one exclusion, because
# no test file reaches a binary.
#
# tested by: source-stamp-test.sh
set -euo pipefail

die() {
  printf 'source-stamp.sh: %s\n' "$1" >&2
  exit 2
}

# `CDPATH=` and `pwd -P` for the reasons resolve.sh states over the same two lines.
tools="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" ||
  die "cannot resolve my own directory, so no source can be found"

[ $# -eq 1 ] || die "usage: source-stamp.sh <tool>"
tool="$1"

case "$tool" in
  "" | *[!a-z0-9-]*) die "'$tool' is not a tool name — expected lowercase letters, digits and dashes" ;;
esac

# macOS ships shasum, most Linux images ship sha256sum, and a release is stamped on one and read on
# the other — so this rests on the two writing a digest and a name the same way, which
# source-stamp-test.sh asserts.
if command -v shasum >/dev/null 2>&1; then
  hasher=(shasum -a 256)
elif command -v sha256sum >/dev/null 2>&1; then
  hasher=(sha256sum)
else
  die "no shasum or sha256sum on this machine, so the source of $tool cannot be stamped"
fi

# The tool still has to be one, so that a mistyped name is refused rather than answered with the
# module's hash. Its main lives under cmd/ or in its own directory; either makes it a tool here.
[ -d "$tools/$tool" ] || [ -d "$tools/cmd/$tool" ] ||
  die "no source for $tool under $tools, so there is nothing to stamp"

[ -f "$tools/go.mod" ] || die "no go.mod at $tools, so the source of $tool cannot be stamped"

# Sorted under LC_ALL=C and named relatively, so the same source stamps the same on the release runner
# and on the machine that installs what it built. bin/ and dist/ are pruned because they hold the
# binaries, and walking them would stat a few megabytes per run for files that cannot match.
sources=()
while IFS= read -r path; do
  sources+=("$path")
done < <(
  CDPATH= cd "$tools" &&
    {
      printf './go.mod\n'
      find . \( -name bin -o -name dist \) -prune -o \
        -type f -name '*.go' ! -name '*_test.go' -print
    } | LC_ALL=C sort
)
[ ${#sources[@]} -gt 1 ] || die "found no Go source for $tool under $tools, so the stamp would say nothing"

# One digest over every file's digest and name, so a file added, removed or renamed moves the stamp
# as surely as an edited one.
(CDPATH= cd "$tools" && "${hasher[@]}" "${sources[@]}") | "${hasher[@]}" | cut -d' ' -f1
