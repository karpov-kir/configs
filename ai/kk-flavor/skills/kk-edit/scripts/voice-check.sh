#!/usr/bin/env bash
# Register check for comments and prose. By default it reads the comments a change set added and says
# which sentences are written in the register the rule forbids. With `--density` it reports how many
# comment lines the set carries beside the host repository's own rate.
#
#   usage: voice-check.sh [--density | --per-file | --profile=comment|prose|instruction] [<git-diff revisions>] [-- <paths>]
#          # revisions default to HEAD (all uncommitted changes); a bare path argument is refused with
#          exit 2, never scanned, and paths after `--` narrow the scan to them
#   env:   DENSITY_MAX_FILE_BYTES — skip a file larger than this unread (default 262144)
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
# `--density` counts each changed file as it will land, against the rate the repo's untouched files run at,
# and says how far over it sits and which files carry it. Two runs over one tree print one report. How
# The figure is reported and gates no edit. It always exits 0, because the bar that used to gate
# on it is what drove comments into compression, and a compressed comment is what this tool catches.
# Files are read as they sit in the working tree; revisions only choose which files. Only a file new
# since the diff's base is held to the per-file ceiling: one the repo already carried has the repo's own
# density. It exits 2 when every file outside the change set is free of countable lines, because the
# repository then has no rate to report against.
#
# The default mode prints one finding per line as `<file>:<line>: <check>: <matched text>`, exit 1 with
# findings, 0 clean, 2 when the scan did not run. It counts nothing: each check names a shape a reader
# stumbles on, so a finding is an edit to make and never a number to drive down. The rule it enforces
# is `~/.kk-flavor/standards/code-style.md` -> Comments, and the tells are
# `~/.kk-flavor/standards/human-writing.md` -> AI tells -> House voice.
#
# Three profiles. `comment` (the default) reads the comment lines a diff added to source files, and
# takes `-` to read a unified diff on stdin, which is how a branch this checkout does not hold is
# scanned: `gh pr diff <N> | voice-check.sh -`. `prose` reads a markdown or plain-text
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
# tested by: the Go suite in ai/tools/voice-check/. The shared stub region and the resolver it
# calls have their own cases in the Go suite in ai/tools/reach/.

set -euo pipefail

tool="voice-check"
# How far THIS file sits above the tools directory.
tools_offset="../../../.."

# --- shared:tool-stub ---
# Byte-identical in every stub, which the wiring check's shared-region scan enforces.
#
# Each stub carries its own copy. One shared file would be executed by the source call that read it,
# and a stub runs from whatever repository the human is standing in.

# What lives here is the part that cannot move: a stub has to find the resolver before the resolver
# can decide anything. ai/tools/resolve.sh owns the rest, argv[0] included. Its header says why each
# line here has the shape it has: the `cd -P`, the declared offset, the two guards, the exec.
die() {
  printf '%s: %s\n' "${0##*/}" "$1" >&2
  exit 2
}

here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" ||
  die "cannot resolve my own directory, so $tool could not be located"

resolver="$here/$tools_offset/tools/resolve.sh"
[ -e "$resolver" ] ||
  die "no resolver at $resolver — this skill is mounted from a checkout that does not ship ai/tools/, and $tool did NOT run"
[ -x "$resolver" ] ||
  die "$resolver is not executable, so $tool did NOT run — chmod +x it"

exec "$resolver" --run "$tool" "$0" "$@"
# --- end shared:tool-stub ---
