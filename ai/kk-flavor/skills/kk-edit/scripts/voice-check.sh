#!/usr/bin/env bash
# Comment-density detector. By default it flags changed source files whose ADDED lines are
# comment-heavy; with `--bar` it holds the whole change set to the host repo's own comment rate; with
# `--voice` it reads the comments instead of counting them.
#
#   usage: voice-check.sh [--bar | --voice [--profile=comment|prose|instruction]] [<git-diff revisions>] [-- <paths>]
#          # revisions default to HEAD (all uncommitted changes); a bare path argument is refused with
#          exit 2, never scanned, and paths after `--` narrow the scan to them
#   env:   COMMENT_MAX_RATIO — flag above this comments/(comments+code) share of added lines (default 0.3)
#          COMMENT_MIN_LINES — ignore files with fewer added comment lines than this (default 5)
#          DENSITY_MAX_FILE_BYTES — skip a file larger than this unread: only untracked files in the
#          default mode, every file under --bar (default 262144)
#
# Exits 1 with findings, 0 when clean, 2 when the scan did not run — git rejecting the arguments, a
# path passed where a revision belongs, or a threshold that is no number. Prose/data files (md, txt,
# json, lockfiles) don't count. With no diff args, untracked text files are scanned too; the index is
# never touched.
#
# The default mode prints each outlier with its counts, then on stderr its denominator — files reached,
# files with countable added lines, outliers, untracked files skipped unread — and one line saying which
# run this was: nothing reached, nothing countable, or a targeting aid and not a bar. It counts ADDED
# lines, so rewording a comment the base already carried moves it into the added set, and the ratio can
# rise across a pass that cut comments.
#
# `--bar` counts each changed file as it will land, against the rate the repo's untouched files run at,
# and says how far over it sits and which files carry it. Two runs over one tree print one report. How
# many comment lines the set owes is this reading; the judge opens the cut that pays it.
# Files are read as they sit in the working tree; revisions only choose which files. Only a file new
# since the diff's base is held to the per-file ceiling: one the repo already carried has the repo's own
# density, and its added lines are the default mode's to flag. The two COMMENT_* thresholds do not
# apply to it, and it exits 2 as well when no file outside the change set carries countable lines.
#
# `--voice` prints one finding per line as `<file>:<line>: <check>: <matched text>`, exit 1 with
# findings, 0 clean, 2 when the scan did not run. It counts nothing: each check names a shape a reader
# stumbles on, so a finding is an edit to make and never a number to drive down. The rule it enforces
# is `~/.kk-flavor/standards/code-style.md` -> Comments, and the tells are
# `~/.kk-flavor/standards/human-writing.md` -> AI tells -> House voice.
#
# Three profiles. `comment` (the default) reads the comment lines a diff added to source files, and
# takes `-` to read a unified diff on stdin, which is how a branch this checkout does not hold is
# scanned: `gh pr diff <N> | voice-check.sh --voice -`. `prose` reads a markdown or plain-text
# file named as a path or `-`: a PR body, a review comment, a reply. `instruction` reads a rule file
# under `ai/kk-flavor/` and skips its frontmatter, its fenced code and its headings.
#
# Checks: bold, contrast, counterfactual-opener, no-subject, intensifier, positional, long-block,
# coined, long-sentence, clause-depth, double-negative, semicolon. Three of them are scoped by profile. `long-block` is the comment profile's, since only there
# is a block a thing. `bold` is not the instruction profile's: a rule file IS markdown, so its bold is
# structure rather than the tell. `coined` is not the instruction profile's either: a coined word is a
# codebase's invented vocabulary, and a rule file is prose about writing that uses the ordinary English
# word a codebase may have coined. `coined` also carries a built-in list of the phrases every
# repository coins by accident, the house idiom of naming, so each conf can leave them out. `comment-voice.conf` names the words this repository coined and the findings it has decided
# to keep, looked for at COMMENT_VOICE_CONF, then `<repo>/.kk-flavor/comment-voice.conf`, then
# `${XDG_CONFIG_HOME:-~/.config}/kk-flavor/comment-voice.conf`. An allow entry with no reason is
# refused, and a missing file is no error.
#
# tested by: the Go suite beside the tool, `ai/tools/voice-check/`; the shared stub region below
# by tool-stub-test.sh, and the resolver it calls by resolve-test.sh.

set -euo pipefail

tool="voice-check"
# How far THIS file sits above the tools directory.
tools_offset="../../../.."

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
