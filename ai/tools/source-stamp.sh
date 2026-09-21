#!/usr/bin/env bash
# Print the SHA-256 of the Go source <tool> is built from, so a binary in bin/ can be held against the
# source sitting beside it.
#
#   usage: source-stamp.sh <tool>     # <tool> is a directory name under ai/tools
#
# tested by: the Go suite in ai/tools/reach/, which execs this script once per case.

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
# go.mod sits at the repository root, two levels up from ai/tools/, so the listing starts at the root.
# The offset is declared here for the reason resolve.sh gives about the stubs. A walk upward finds
# whatever ancestor happens to carry a go.mod, and this machine keeps checkouts inside checkouts, so
# that ancestor can be a different module.
set -euo pipefail

die() {
  printf 'source-stamp.sh: %s\n' "$1" >&2
  exit 2
}

# `CDPATH=` and `pwd -P` for the reasons resolve.sh states over the same two lines.
tools="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" ||
  die "cannot resolve my own directory, so no source can be found"

# The module root, as an offset from this directory. resolve.sh declares the same offset, and the two
# have to move together.
module="$tools/../.."

[ $# -eq 1 ] || die "usage: source-stamp.sh <tool>"
tool="$1"

case "$tool" in
  "" | *[!a-z0-9-]*) die "'$tool' is not a tool name — expected lowercase letters, digits and dashes" ;;
esac

# macOS ships shasum and most Linux images ship sha256sum. A release is stamped on one and read on
# the other, so the two have to write a digest and a name the same way.
# The Go suite in ai/tools/reach/ checks that they do.
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

[ -f "$module/go.mod" ] || die "no go.mod at $module, so the source of $tool cannot be stamped"

# The list is sorted under LC_ALL=C and named relative to the module root. The same source then stamps
# the same on the release runner and on the machine that installs what it built.

# git holds the list inside a checkout. `git ls-files` stops at a nested repository's edge, so the
# worktrees a machine keeps inside its checkout fall out by git's own rule. An ignore entry would do
# the same job and anyone can edit one away.

# `--others` is passed because source that is written but still unstaged has to move the stamp. A
# binary built before that source would otherwise read as current. `-z` is passed because git
# C-quotes a path holding a quote or a non-ASCII byte.

# Outside a checkout the walk is all there is. It prunes bin/ and dist/ for the binaries and `.git`
# for holding no Go. Both spellings name every file `./…`, so an unchanged tree stamps as it did when
# this only walked.
sources=()
while IFS= read -r -d '' path; do
  sources+=("./${path#./}")
done < <(
  CDPATH= cd "$module" &&
    if command -v git >/dev/null 2>&1 &&
      [ "$(git rev-parse --is-inside-work-tree 2>/dev/null)" = true ]; then
      printf 'go.mod\0'
      git ls-files --cached --others --exclude-standard -z -- '*.go' ':(exclude)*_test.go'
    else
      printf './go.mod\0'
      find . \( -name .git -o -name bin -o -name dist \) -prune -o \
        -type f -name '*.go' ! -name '*_test.go' -print0
    fi | LC_ALL=C sort -z
)
[ ${#sources[@]} -gt 1 ] || die "found no Go source for $tool under $module, so the stamp would say nothing"

# One digest over every file's digest and name, so a file added, removed or renamed moves the stamp
# as surely as an edited one.
(CDPATH= cd "$module" && "${hasher[@]}" "${sources[@]}") | "${hasher[@]}" | cut -d' ' -f1
