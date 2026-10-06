#!/usr/bin/env bash
# Register check for prose. It reads a PR body, a review comment, a reply or a rule file, and says
# which sentences are written in the register the rule forbids.
#
#   usage: voice-check.sh [--profile=prose|instruction] [--kind=pr-body|ticket] [--per-file] [--] <path>|- ...
#          # `-` reads stdin; a path that starts with `-` goes after `--`
#   env:   DENSITY_MAX_FILE_BYTES — refuse a file larger than this. The default is in
#          `~/.kk-flavor/configs/voice-check.conf`.
#
# Prints one finding per line as `<file>:<line>: <check>: <matched text>`. Exits 1 with findings, 0
# clean, 2 when the scan did not run: an unknown flag, a path that cannot be read, a file over the
# byte cap, or a threshold that is no number. It counts nothing: each check names a shape a reader
# stumbles on, so a finding is an edit to make and never a number to drive down. The tells are
# `~/.kk-flavor/standards/human-writing.md` -> AI tells -> House voice.
#
# Two profiles. `prose` (the default) reads a markdown or plain-text file whole. `instruction` reads a
# rule file under `ai/kk-flavor/` and skips its frontmatter, its fenced code and its headings.
# `--kind` reads a prose body as a PR body or a ticket: it leaves the repository's template lines
# unread and adds the `tests-narration` check. `--per-file` prints a finding count per path instead of
# the findings.
#
# Checks: bold, contrast, counterfactual-opener, no-subject, intensifier, positional, coined,
# counterfactual-consequence, anthropomorphism, elided-verb, long-sentence, clause-depth,
# double-negative, semicolon. `bold` is not the instruction profile's: a rule file IS markdown, so its
# bold is structure rather than the tell. `coined` and the three sentence shapes are not the
# instruction profile's either: a rule file is prose about writing, and writing about a shape is not
# writing in it. `coined` carries a built-in list of the phrases every repository coins by accident,
# the house idiom of naming. A finding that must stand is a defect in the check, fixed there.
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
