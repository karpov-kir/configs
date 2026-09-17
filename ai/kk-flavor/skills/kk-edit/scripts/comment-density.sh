#!/usr/bin/env bash
# Comment-density detector. By default it flags changed source files whose ADDED lines are
# comment-heavy; with `--bar` it holds the whole change set to the host repo's own comment rate; with
# `--voice` it reads the comments instead of counting them.
#
#   usage: comment-density.sh [--bar | --voice [--profile=comment]] [<git-diff revisions>] [-- <paths>] | comment-density.sh --voice --profile=prose|instruction <path|->...
#          # revisions default to HEAD (all uncommitted changes); a bare path argument is refused with
#          exit 2, never scanned, and paths after `--` narrow the scan to them; the prose and
#          instruction profiles take their paths bare, and refuse a `--` as a path they cannot read
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
# scanned: `gh pr diff <N> | comment-density.sh --voice -`. `prose` reads a markdown or plain-text
# file named as a path or `-`: a PR body, a review comment, a reply. `instruction` reads a rule file
# under `ai/kk-flavor/` and skips its frontmatter, its fenced code and its headings.
#
# Checks: bold, contrast, counterfactual-opener, no-subject, intensifier, positional, long-block,
# coined. Three of them are scoped by profile. `long-block` is the comment profile's, since only there
# is a block a thing. `bold` is not the instruction profile's: a rule file IS markdown, so its bold is
# structure rather than the tell. `coined` is not the instruction profile's either: a coined word is a
# codebase's invented vocabulary, and a rule file is prose about writing that uses the ordinary English
# word a codebase may have coined. `comment-voice.conf` names the words this repository coined and the findings it has decided
# to keep, looked for at COMMENT_VOICE_CONF, then `<repo>/.kk-flavor/comment-voice.conf`, then
# `${XDG_CONFIG_HOME:-~/.config}/kk-flavor/comment-voice.conf`. An allow entry with no reason is
# refused, and a missing file is no error.
#
# tested by: the Go suite beside the tool, `ai/tools/comment-density/`; the shared stub region below
# by the Go suite in ai/tools/reach/, which covers the resolver it calls too.

set -euo pipefail

tool="comment-density"
# How far THIS file sits above the tools directory.
tools_offset="../../../.."

# --- shared:tool-stub ---
# Byte-identical in every stub, held so by the wiring check's shared-region scan. Copied rather than
# sourced because sourcing a file is executing it, and these run from whatever repo the human is in —
# so only what cannot move is here: a stub has to find the resolver before the resolver can decide
# anything. `ai/tools/resolve.sh` owns the rest, argv[0] included, and its header states why each line
# below is the shape it is — the `cd -P`, the one declared offset, the two guards, the exec.
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
