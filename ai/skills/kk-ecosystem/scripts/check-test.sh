#!/usr/bin/env bash
# Tests for check.sh's guards. The `echo` line above each group of cases names the block it covers.
#   usage: check-test.sh   # prints one line per case; exit 0 when all pass, 1 otherwise
#
# A change to check.sh needs a case here, and `~/.claude/skills/kk-ecosystem/scripts/check-mutate.sh`
# is what proves a case can fail. Write a mutation and assert it landed where you aimed it, the same
# way these cases assert that the guard fired.
# No case asserts on the exit code — a fixture root legitimately has findings of its own, so
# "clean" would pass every case for the wrong reason.
# Past the ~450-line guidance deliberately: `~/.claude/skills/kk-ecosystem/scripts/check-mutate.sh`
# copies the one suite beside it into its sandbox, so a split means teaching that harness to run
# several and to attribute a kill across them.
# Split it once that is solved, never on the line count alone.
set -uo pipefail
export LC_ALL=C

here="$(cd -P -- "$(dirname "$0")" && pwd -P)"
check="$here/check.sh"
[ -x "$check" ] || { echo "check-test: $check is not executable"; exit 1; }

installed_scripts="$(cd -P -- "${HOME:-}/.claude/skills/kk-ecosystem/scripts" 2>/dev/null && pwd -P)"
[ -n "$installed_scripts" ] && [ "$installed_scripts" = "$here" ] || {
  echo "check-test: run it as ~/.claude/skills/kk-ecosystem/scripts/check-test.sh — this copy is somewhere else" >&2
  echo "check-test: exit 2 — nothing was tested. It runs the check.sh beside it, so where it runs from decides whose code executes." >&2
  exit 2
}

base="$(mktemp -d)" || { echo "check-test: mktemp gave no fixture dir — nothing was tested"; exit 1; }
trap 'rm -rf "$base"' EXIT
passed=0
failed=0
case_number=0
check_home=""

# This sets `root` instead of echoing a path for the caller to substitute. Write
# `root="$(new_root_without_flavor)"` and the body runs in a subshell: `case_number` never
# increments, so every case reuses one directory.
new_root_without_flavor() {
  case_number=$((case_number + 1))
  root="$base/r$case_number"
  mkdir -p "$root/skills"
  check_home=""
}

new_home_without_flavor_mount() {
  check_home="$root/home"
  mkdir -p "$check_home/.claude"
}

new_home() {
  new_home_without_flavor_mount
  ln -s "$root/kk-flavor" "$check_home/.kk-flavor"
}

new_root() {
  new_root_without_flavor
  mkdir -p "$root/kk-flavor/standards"
  printf '# Flavor\n' >"$root/kk-flavor/inject.md"
}

# check.sh refuses to walk a symlinked `kk-flavor`, so a case writes into `$root/real-flavor` and never
# `$root/kk-flavor`.
new_root_with_symlinked_flavor() {
  new_root_without_flavor
  mkdir -p "$root/real-flavor/standards"
  printf '# Flavor\n' >"$root/real-flavor/inject.md"
  ln -s "$root/real-flavor" "$root/kk-flavor"
}

# A skill the bare-name half of the direction scan can resolve: it counts a name only when a skill
# answers to it.
new_mounted_skill() {
  mkdir -p "$root/skills/$1"
  printf '# %s\n' "$1" >"$root/skills/$1/SKILL.md"
}

# A committed filename holding a newline — the shape every forgery case here turns on. Writing a plain
# name instead would satisfy the assertion while testing nothing, so a filesystem that refuses one stops
# the run rather than leaving the case to pass on a fixture it never got.
# `2>/dev/null` goes before the redirect it silences, never after: redirections are applied left to
# right, so a trailing one is opened after the failure it was meant to swallow has already been printed.
new_file_with_newline_name() {
  printf '%s\n' "$2" 2>/dev/null >"$1" || {
    echo "check-test: this filesystem refused a newline in a filename — $3 cannot run here"
    exit 1
  }
}

new_script() {
  # `mkdir -p` on the parent, so a case can place a script under `<skill>/scripts/` — the real layout —
  # without the redirect failing and leaving the case asserting against a tree that has no script in it.
  mkdir -p "$(dirname "$root/skills/$1")"
  printf '%s\n' "$2" >"$root/skills/$1"
  chmod +x "$root/skills/$1"
}

# The lane fixture the citation and basename cases share: one mounted skill holding one script. The
# path it is cited by is written once, here, and passed as a `%s` argument at every use — check.sh
# scans every file under the root, this suite among them, so a cited path that does not resolve in the
# real checkout is a dangling-home-ref finding against the checkout itself.
lane_script_ref='~/.claude/skills/kk-humanize/scripts/comment-density.sh'
new_lane_with_script() {
  new_mounted_skill kk-humanize
  new_script kk-humanize/scripts/comment-density.sh 'true'
}

run_check() {
  local status
  check_output="$(HOME="${check_home:-$HOME}" "$check" "$root" 2>&1)"
  status=$?
  if [ "$status" = 2 ]; then
    echo "check-test: check.sh exited 2 on '$1' — nothing was checked, so no case here can be trusted"
    printf '%s\n' "$check_output" | sed 's/^/          /'
    exit 1
  fi
}

record_pass() {
  passed=$((passed + 1))
  echo "  pass  $1"
}

record_fail() {
  failed=$((failed + 1))
  echo "  FAIL  $1"
  printf '%s\n' "$check_output" | sed 's/^/          /'
}

# Both asserts take a here-string, never `printf … | grep -q` — check.sh's own here-string note says
# why, and a match swallowed here turns an `assert_does_not_report` case into a silent pass.
assert_reports() {
  run_check "$2"
  if grep -qF "$1" <<<"$check_output"; then record_pass "$2"; else record_fail "$2"; fi
}

assert_does_not_report() {
  run_check "$2"
  if grep -qF "$1" <<<"$check_output"; then record_fail "$2"; else record_pass "$2"; fi
}

# check.sh prints its budget lines before any finding, so a leak from a scan loop lands ahead of them.
assert_reported_via_findings() {
  local budget_line finding_line
  run_check "$2"
  budget_line="$(grep -nF -m1 'always-loaded:' <<<"$check_output" | cut -d: -f1)"
  finding_line="$(grep -nF -m1 "$1" <<<"$check_output" | cut -d: -f1)"
  if [ -n "$budget_line" ] && [ -n "$finding_line" ] && [ "$finding_line" -gt "$budget_line" ]; then
    record_pass "$2"
  else
    record_fail "$2"
  fi
}

# Ordering, never presence: the per-class cap on its own puts a real finding on screen alongside a
# flood, so asserting the real one is merely *present* passes with the rank patterns unanchored and
# observes nothing. What the `^` decides is which of the two lands first.
assert_ranks_above() {
  local above below
  run_check "$3"
  above="$(grep -nF -m1 "$1" <<<"$check_output" | cut -d: -f1)"
  below="$(grep -nF -m1 "$2" <<<"$check_output" | cut -d: -f1)"
  if [ -n "$above" ] && [ -n "$below" ] && [ "$above" -lt "$below" ]; then
    record_pass "$3"
  else
    record_fail "$3"
  fi
}

# One line repeated past a scan's own bound — what every budget case here needs and nothing else does.
flood_with_line() {
  local target="$1" remaining="$2" line="$3"
  while [ "$remaining" -gt 0 ]; do
    printf '%s\n' "$line" >>"$target"
    remaining=$((remaining - 1))
  done
}

cites="shared layer cites into a lane"
names="shared layer names a lane"
# Carries no other finding's text whole — check.sh's direction-scan comment says why.
basenames="shared layer reaches into a lane by basename"
unchecked="basename not checked"
never_ran="direction scan read no files"
refused="import refused"

echo "check.sh — direction scan"

new_root
printf 'the mechanics are `~/.claude/skills/kk-drive/SKILL.md`\n' >"$root/kk-flavor/standards/x.md"
assert_reports "$cites" "fires on a standard citing into a SKILL.md"

new_root
printf 'see `~/.claude/skills/kk-drive/SKILL.md` for the mechanics\n' >"$root/CLAUDE.md"
assert_reports "$cites" "fires on the root CLAUDE.md citing into a SKILL.md"

new_root
printf 'the mechanics are `~/.claude/skills/kk-drive/SKILL.md`\n' >"$root/kk-flavor/standards/x.md"
assert_reported_via_findings "$cites" "reports through the bounded findings path, not raw on stdout"

new_root
{
  printf 'a glob names the set: `~/.claude/skills/*/SKILL.md`\n'
  printf 'a bare name is not a path: run it per its SKILL.md\n'
  printf 'a placeholder: `~/.claude/skills/<skill name>/SKILL.md`\n'
  # Use a path that really exists, never an invented one — the home-ref scan resolves every
  # `~/.claude/skills/...` it finds, fixture text like this included.
  printf 'a file the skill owns that is not its body: `~/.claude/skills/idsd-qualify/templates/retro-spawn-prompt.md`\n'
} >"$root/kk-flavor/standards/legal.md"
# The fourth form is not legal: ecosystem.md → **One home** bans a file the skill owns too. It is
# quiet *here* only because the fixture mounts no skill — in any tree that holds `idsd-qualify`, this
# scan reports the path and the bare-name scan below reports the name.
assert_does_not_report "$cites" "stays quiet on the four forms that are not a path into a SKILL.md"

new_root
printf 'see `~/.claude/skills/kk-drive/SKILL.md`\n' >"$base/outside.md"
ln -s "$base/outside.md" "$root/CLAUDE.md"
assert_does_not_report "$cites" "does not read a violation through a symlinked CLAUDE.md"

# The bare-name half of the same rule. A name counts only when a skill answers to it, so the fixture
# has to mount one. The negative case is the same prose with nothing mounted: it pins that gate, not
# the prose's legality — check.sh's `unknown skill referenced` scan reports that prose anyway.
new_root
new_mounted_skill kk-drive
printf 'spawn `kk-drive` before any lens reads it\n' >"$root/kk-flavor/standards/x.md"
assert_reports "$names" "fires on a standard naming a skill that exists"

# `grep -o` keeps the trailing hyphen a token ends on, which a glob in prose produces. Without the
# strip the name matches no skill directory and the violation goes unreported.
new_root
new_mounted_skill kk-drive
printf 'every `kk-drive-*` invocation is the same lane\n' >"$root/kk-flavor/standards/x.md"
assert_reports "$names" "strips the trailing hyphen a glob leaves on a lane name"

# The suffix class carries `.` as well, so `grep -o` keeps the full stop of a sentence that ends on the
# lane name — the commonest way prose names one. The hyphen case above does not reach it: the strip has
# to take the whole trailing run, or the name matches no skill directory and the violation goes quiet.
new_root
new_mounted_skill kk-drive
printf 'the lane that owns this one is kk-drive.\n' >"$root/kk-flavor/standards/x.md"
assert_reports "$names" "strips the full stop a sentence ending on a lane name leaves"

new_root
printf 'spawn `kk-drive` before any lens reads it\n' >"$root/kk-flavor/standards/x.md"
assert_does_not_report "$names" "the names scan stays quiet on a token no skill answers to"

new_root
new_mounted_skill kk-drive
printf 'a template under `kk-flavor/` is this same layer, not a lane\n' >"$root/kk-flavor/standards/x.md"
assert_does_not_report "$names" "stays quiet on kk-flavor, which is the shared layer itself"

new_root
new_mounted_skill sonar-api
printf 'route it through `sonar-api`\n' >"$root/kk-flavor/standards/x.md"
assert_reports "$names" "fires on a skill named outside the kk-* and idsd-* families"

# The case above already covers a lane named outside the `kk-*`/`idsd-*` families. No script is mounted
# at the cited path here: this scan matches the shape of a path, it never resolves one.
new_root
new_mounted_skill kk-humanize
printf 'read `%s`\n' "$lane_script_ref" >"$root/kk-flavor/standards/x.md"
assert_reports "$cites" "fires on a path into a lane that is not a SKILL.md"

new_root
new_mounted_skill kk-drive
printf 'a `kk-drive-verified` claim is not a lane\n' >"$root/kk-flavor/standards/x.md"
assert_does_not_report "$names" "stays quiet on a hyphenated compound built off a real lane name"

# One NUL byte makes grep call the file binary, and it prints `Binary file … matches` or, on GNU grep
# >= 3.5, nothing at all. Without `-a` every grep in the scan then reads no violation out of a file
# `find` did hand them, and the did-not-run guard stays quiet because the file was still walked.
# The NUL goes in through `tr`: `printf '\000'` is truncated at the NUL by some printfs.
new_root
new_mounted_skill kk-drive
printf '# Rule\n' >"$root/kk-flavor/standards/x.md"
printf 'X' | tr 'X' '\000' >>"$root/kk-flavor/standards/x.md"
printf '\nspawn `kk-drive` before any lens reads it\n' >>"$root/kk-flavor/standards/x.md"
assert_reports "$names" "reads a lane name past a NUL byte that makes grep call the file binary"

new_root
printf '# Rule\n' >"$root/kk-flavor/standards/x.md"
printf 'X' | tr 'X' '\000' >>"$root/kk-flavor/standards/x.md"
printf '\nthe mechanics are `~/.claude/skills/kk-drive/SKILL.md`\n' >>"$root/kk-flavor/standards/x.md"
assert_reports 'kk-drive/SKILL.md — move the rule' "and reads a cited SKILL.md path past one too"

# The third grep needs its own case: the two above cover the cites and names greps, and `-a` is per-grep,
# so dropping it from this one alone would leave both of them green.
new_root
new_lane_with_script
printf '# Rule\n' >"$root/kk-flavor/standards/x.md"
printf 'X' | tr 'X' '\000' >>"$root/kk-flavor/standards/x.md"
printf '\nrun `comment-density.sh` before any lens reads it\n' >>"$root/kk-flavor/standards/x.md"
assert_reports "$basenames" "and reads a lane basename past one too"

# The scans outside the direction block need `-a` just as much, and had none: one committed NUL made
# BSD grep answer `Binary file X matches`, which both replaced the real finding and put a tree-chosen
# path inside the text of one. An agent drafts PR comments from these, so that is an injection, not
# only a miss. Two scans stand for the four, one per grep shape (`-oE` per file, `-rhoE` over the tree).
new_root
{
  printf 'see [x](nowhere.md) and kk-nonesuch\n'
  printf 'X' | tr 'X' '\000'
  printf '\n'
} >"$root/kk-flavor/standards/nul.md"
assert_reports 'dangling link: '"$root"'/kk-flavor/standards/nul.md -> nowhere.md' \
  "reads a markdown link past a NUL byte"
assert_reports 'unknown skill referenced: kk-nonesuch' \
  "and reads a skill name past one, rather than reporting grep's own notice as the name"

# The cited path is echoed whole. One trailing segment stops it at `.../kk-humanize/scripts` and drops
# the file the citation was about, which is the half that says what to go and move.
# Every path below resolves in the real checkout, for the reason `lane_script_ref` gives — and **run
# `check.sh` over this repo after editing this file, not before**: an invented fixture path is a finding
# against the checkout itself, and a run from before the edit cannot see it.
new_root
new_lane_with_script
printf 'read `%s`\n' "$lane_script_ref" >"$root/kk-flavor/standards/x.md"
assert_reports 'kk-humanize/scripts/comment-density.sh — move the rule' "echoes a cited path whole, not truncated at one segment"

# The third shape: a lane's file named by basename alone, carrying neither a lane name nor a path.
new_root
new_lane_with_script
printf 'run `comment-density.sh` before any lens reads it\n' >"$root/kk-flavor/standards/x.md"
assert_reports "$basenames" "fires on a lane's file named by its basename alone"

# `uniq -u` is the gate. A basename every lane carries names the kind of file, not one lane's copy, so
# two mounted skills are the fixture: with one, `SKILL.md` resolves uniquely and this case cannot fail.
new_root
new_mounted_skill kk-drive
new_mounted_skill kk-humanize
printf 'a bare name is not a path: run it per its SKILL.md\n' >"$root/kk-flavor/standards/x.md"
assert_does_not_report "$basenames" "stays quiet on a basename more than one lane carries"

# A name the shared layer carries is not in the set. Here nothing under skills/ carries it either, so it
# never enters the set; the subtraction case below covers the half where a lane does carry it.
new_root
new_mounted_skill kk-drive
printf '# Writing\n' >"$root/kk-flavor/standards/writing.md"
printf 'the shape is `writing.md`\n' >"$root/kk-flavor/standards/x.md"
assert_does_not_report "$basenames" "stays quiet on a shared-layer sibling's own basename"

new_root
new_lane_with_script
printf 'read `%s`\n' "$lane_script_ref" >"$root/kk-flavor/standards/x.md"
assert_does_not_report "$basenames" "does not report a basename that reached it inside a cited path"

# The basename set is built from NUL-delimited paths, never `find | sed`: a committed filename holding a
# newline splits in two, and each half is a forgery the reviewed tree chose. This is the muting half —
# the head becomes a second copy of a real basename, `uniq -u` drops it, and a genuine violation goes
# quiet. The control comes first: without the hostile file, the finding must be there to lose.
new_root
new_lane_with_script
printf 'run `comment-density.sh` before any lens reads it\n' >"$root/kk-flavor/standards/x.md"
assert_reports "$basenames" "reports a lane's basename (control for the case below)"
new_file_with_newline_name "$root/skills/kk-humanize/scripts/$(printf 'x\ncomment-density.sh')" \
  'not a script' 'the basename forgery cases'
assert_reports "$basenames" "a newline in a committed filename cannot mute a real basename finding"

# The forging half of the same split: the tail is a basename no lane carries, so a standard naming a file
# nothing under skills/ holds is reported against a file the branch never touched. The name is one the
# shared layer does not carry either, or the subtraction case below would silence this one for free.
new_root
new_mounted_skill kk-drive
printf 'run `report.sh` at the close\n' >"$root/kk-flavor/standards/x.md"
new_file_with_newline_name "$root/skills/kk-drive/$(printf 'q\nreport.sh')" x 'the basename forgery cases'
assert_does_not_report "$basenames" "a newline in a committed filename cannot forge a basename finding"

# The shared layer's own basenames are subtracted from the set. The reviewed tree fills skills/, so one
# committed file named after a standard would otherwise report every standard citing that sibling.
# Control first, again: the same lane file fires while the shared layer carries no file by that name.
new_root
new_mounted_skill kk-drive
mkdir -p "$root/skills/kk-drive/notes"
printf '# notes\n' >"$root/skills/kk-drive/notes/writing.md"
printf 'the shape is `writing.md`\n' >"$root/kk-flavor/standards/x.md"
assert_reports "$basenames" "reports a lane file the shared layer has no counterpart for (control)"
assert_does_not_report "$unchecked" "and calls nothing unchecked while one tier alone carries the name"
printf '# Writing\n' >"$root/kk-flavor/standards/writing.md"
assert_does_not_report "$basenames" "a lane file cannot forge a finding against a standard citing its own sibling"
# Subtracting the name narrows the scan, and a narrowing nothing says out loud is the mute this reports:
# any `.md` committed under kk-flavor/ named after a lane file would otherwise buy silence for free.
assert_reports "$unchecked" "and says the name went unchecked instead of going quiet"

# The unchecked-name notice is bounded like every other shape here: the tree picks how many times an
# ambiguous name appears, and each mention costs a fork to sanitise it.
new_root
new_mounted_skill kk-drive
mkdir -p "$root/skills/kk-drive/notes"
printf '# notes\n' >"$root/skills/kk-drive/notes/writing.md"
printf '# Writing\n' >"$root/kk-flavor/standards/writing.md"
flood_with_line "$root/kk-flavor/standards/x.md" 45 'the shape is `writing.md`'
assert_reports "$unchecked: $root/kk-flavor/standards/x.md — 40 already shown" \
  "bounds what the unchecked-name notice emits"

# This half emits a `find` per finding on top of the fork every hit costs, so its bound is the one that
# matters most of the three.
new_root
new_lane_with_script
flood_with_line "$root/kk-flavor/standards/x.md" 45 'run `comment-density.sh` now'
assert_reports "$basenames: $root/kk-flavor/standards/x.md — 40 already shown" \
  "bounds what the basename half of the scan emits"

# The bound on what each shape of the scan emits. Every finding costs a fork to sanitise its hit, so an
# unbounded emit lets one committed file turn this scan into tens of thousands of them.
new_root
new_mounted_skill kk-drive
flood_with_line "$root/kk-flavor/standards/x.md" 45 'spawn `kk-drive` now'
# The file is pinned, not just the tail: leading with it is what sorts the notice ahead of that file's
# own hits, so the printer's per-rank cap drops those before it drops the notice.
assert_reports "$names: $root/kk-flavor/standards/x.md — 40 already shown" \
  "bounds what the names half of the scan emits"

# The same bound over two files, each holding fewer hits than the cap. It can only fire while the budget
# outlives one file, which is what a hit list read by process substitution rather than through a pipe
# into a subshell buys. Which of the two files the notice names depends on the order `find` hands them
# over, so the assert leaves that free. The fixture mounts no skill, so the names half stays at zero
# and this tail can only have come from the cites half. The path is a `%s` argument for
# the reason the citation cases below give: written whole, it is a backticked in-repo path, and the
# dangling-path-ref scan reads this file and reports it against the repo.
new_root
spread_cites=1
while [ "$spread_cites" -le 45 ]; do
  printf 'the mechanics are `%s/SKILL.md`\n' path/to/other \
    >>"$root/kk-flavor/standards/x$((spread_cites % 2)).md"
  spread_cites=$((spread_cites + 1))
done
assert_reports "— 40 already shown across the shared layer" \
  "bounds the cites half across files, not once per file"

# `kk-flavor` names the shared layer, not a lane. The reviewed tree picks what is in `skills/`, so the
# exclusion cannot rest on no skill answering to that name.
new_root
new_mounted_skill kk-flavor
printf 'a template under `kk-flavor/` is this same layer, not a lane\n' >"$root/kk-flavor/standards/x.md"
assert_does_not_report "$names" "excludes kk-flavor even when the tree commits a skill by that name"

new_root
printf '# Root\n' >"$root/CLAUDE.md"
assert_does_not_report "$never_ran" "a root with no violation and no symlink does not trip the guard"

new_root_with_symlinked_flavor
printf 'see `~/.claude/skills/kk-drive/SKILL.md`\n' >"$root/real-flavor/standards/x.md"
printf '# Root\n' >"$root/CLAUDE.md"
assert_reports "$never_ran" "reports itself when a symlinked kk-flavor leaves it nothing to walk"
assert_does_not_report "$cites" "and does not read the violation behind that symlink"

echo "check.sh — @import resolved at the mount"

uncounted="uncounted import"

new_root
printf '# Root\n\n@FOO.md\n' >"$root/CLAUDE.md"
new_home
printf 'one two three\n' >"$check_home/.claude/FOO.md"
assert_does_not_report "$uncounted" "counts an import the mount really holds"
assert_reports "across 3 files" "and folds it into the budget's file count"

new_root
printf '# Root\n\n@FOO.md\n' >"$root/CLAUDE.md"
new_home_without_flavor_mount
printf 'one two three\n' >"$check_home/.claude/FOO.md"
assert_reports "$uncounted" "refuses to resolve when this checkout is not the installed one"

new_root
printf '# Root\n\n@../../escape.md\n' >"$root/CLAUDE.md"
new_home
# The file has to sit exactly where the traversal lands. Put it anywhere else and the case passes
# on "no such file", proving nothing.
printf 'secret words here\n' >"$root/escape.md"
assert_reports "$uncounted" "refuses a name that is a path rather than a bare filename"
assert_reports "$refused" "and reports that refusal instead of leaving it to read as drift"

new_root
printf '# Root\n\n@dir/file.md\n' >"$root/CLAUDE.md"
new_home
mkdir -p "$check_home/.claude/dir"
printf 'one two three\n' >"$check_home/.claude/dir/file.md"
assert_reports "$uncounted" "leaves a plain subdirectory import uncounted"
assert_does_not_report "$refused" "and does not report it as a probe"

new_root
printf '# Root\n\n@FOO.md\n' >"$root/CLAUDE.md"
new_home
printf 'one two three\n' >"$base/linked.md"
ln -s "$base/linked.md" "$check_home/.claude/FOO.md"
assert_reports "$uncounted" "refuses a symlink at the mount"
assert_reports "$refused" "and reports that one too, the shape nothing legitimate produces"

new_root
printf '# Root\n\n@FOO.md\n' >"$root/CLAUDE.md"
new_home
printf 'one two three\n' >"$check_home/.claude/FOO.md"
chmod 000 "$check_home/.claude/FOO.md"
assert_reports "$uncounted" "refuses a file at the mount it cannot read"
assert_reports "$refused" "and reports it, the shape that hid a short figure one tier down"

new_root_with_symlinked_flavor
new_home_without_flavor_mount
ln -s "$root/real-flavor" "$check_home/.kk-flavor"
printf '# Root\n\n@FOO.md\n' >"$root/CLAUDE.md"
printf 'one two three\n' >"$check_home/.claude/FOO.md"
assert_reports "$uncounted" "refuses a kk-flavor symlinked to the install, which would open the gate"

new_root
printf '# Root\n' >"$root/CLAUDE.md"
printf '@BAR.md\n' >>"$root/kk-flavor/inject.md"
new_home
printf 'one two three four five\n' >"$check_home/.claude/BAR.md"
assert_reports "$uncounted" "refuses an import its carrier does not put at this mount"

new_root
printf '# Root\n\n```\n@FOO.md\n```\n' >"$root/CLAUDE.md"
printf '@FOO.md\n' >>"$root/kk-flavor/inject.md"
new_home
printf 'one two three four five\n' >"$check_home/.claude/FOO.md"
assert_reports "$uncounted" "treats a fenced mention in CLAUDE.md as prose, not as its import"

new_root
printf '# Root\n\nnever write `@FOO.md` here\n' >"$root/CLAUDE.md"
printf '@FOO.md\n' >>"$root/kk-flavor/inject.md"
new_home
printf 'one two three four five\n' >"$check_home/.claude/FOO.md"
assert_reports "$uncounted" "treats a backticked mention in CLAUDE.md the same way"

new_root
new_home
{
  printf '# Root\n\n'
  capped=1
  while [ "$capped" -le 65 ]; do printf '@f%s.md\n' "$capped"; capped=$((capped + 1)); done
} >"$root/CLAUDE.md"
capped=1
while [ "$capped" -le 65 ]; do printf 'one\n' >"$check_home/.claude/f$capped.md"; capped=$((capped + 1)); done
assert_reports "$uncounted" "caps resolution attempts and names what it skipped"

# `sort -u` under LC_ALL=C is what fixes the two positions this case turns on: `../x.md` first, because
# `.` sorts below every letter, and `d01.md` 65th — one past the 64-attempt cap, with the 63 `b` names
# filling it. Under a locale that sorts punctuation away, `../x.md` itself lands past the cap and both
# asserts flip.
new_root
new_home
{
  printf '# Root\n\n'
  stale=1
  while [ "$stale" -le 63 ]; do printf '@b%02d.md\n' "$stale"; stale=$((stale + 1)); done
  printf '@../x.md\n@d01.md\n'
} >"$root/CLAUDE.md"
stale=1
while [ "$stale" -le 63 ]; do
  printf 'one\n' >"$check_home/.claude/b$(printf '%02d' "$stale").md"
  stale=$((stale + 1))
done
printf 'one\n' >"$check_home/.claude/d01.md"
assert_reports "not counted: ../x.md" "reports the probe-shaped name the resolver did look at"
assert_does_not_report "not counted: d01.md" "and carries no reason over to a name past the cap"

new_root
{
  flood=1
  while [ "$flood" -le 300 ]; do printf '[x](nope%s.md)\n' "$flood"; flood=$((flood + 1)); done
} >"$root/kk-flavor/standards/flood.md"
printf '#!/usr/bin/env bash\n' >"$root/skills/notexec.sh"
assert_reports "script not executable" "shows a tampered-check finding through a flood of link findings"

new_root
printf '#!/bin/sh\necho hi\n' >"$root/skills/notexec.sh"
chmod 644 "$root/skills/notexec.sh"
{
  promoted=1
  while [ "$promoted" -le 300 ]; do
    printf '[x](nope%03d flavor mounted elsewhere filler)\n' "$promoted"
    promoted=$((promoted + 1))
  done
} >"$root/kk-flavor/standards/flood.md"
# `filler` is what makes the needle flood-only: `flavor mounted elsewhere` alone also matches the
# genuine rank-1 finding this fixture's $HOME may raise, and the assertion would then compare the
# real finding against itself.
assert_ranks_above "script not executable" "flavor mounted elsewhere filler" \
  "ranks a real finding above a flood whose link targets forge a mount finding"

new_root_with_symlinked_flavor
{
  buried=1
  while [ "$buried" -le 300 ]; do printf '[x](nope%03d.md)\n' "$buried"; buried=$((buried + 1)); done
} >"$root/real-flavor/standards/flood.md"
assert_reports "$never_ran" "surfaces the did-not-run guard under a flood that sorts ahead of it"

new_root
forged="$root/kk-flavor/standards/a
syntax: FORGED.md"
new_file_with_newline_name "$forged" '[x](nowhere.md)' 'the forged-finding-line case'
run_check "newline in a committed path cannot forge a finding line"
if [ "$(grep -c '^syntax: FORGED' <<<"$check_output")" = 0 ]; then
  record_pass "a newline in a committed path cannot start a forged finding line"
else
  record_fail "a newline in a committed path cannot start a forged finding line"
fi

new_root
printf 'if then\n' >"$root/skills/broken.sh"
chmod +x "$root/skills/broken.sh"
buried_syntax=1
while [ "$buried_syntax" -le 300 ]; do
  printf '#!/bin/sh\necho ok\n' >"$root/skills/notexec$buried_syntax.sh"
  chmod 644 "$root/skills/notexec$buried_syntax.sh"
  buried_syntax=$((buried_syntax + 1))
done
assert_reports "syntax: " "shows a syntax error under a flood of its own priority tier"

new_root_with_symlinked_flavor
flooded=1
while [ "$flooded" -le 120 ]; do
  printf 'if then\n' >"$root/skills/broken$flooded.sh"
  chmod +x "$root/skills/broken$flooded.sh"
  flooded=$((flooded + 1))
done
assert_reports "$never_ran" "shows a rank-1 finding under a flood of the gravest class"
assert_reports "of this class, suppressed" "and says how many of the flooding class it withheld"

echo "check.sh — citations name their section in the delimited form"

undelimited="undelimited section citation"

# The cited name is a `%s` argument, never inline: the citation scan reads this file too, so a fixture
# line carrying the citation whole becomes a finding against the repo. Split this way, the path the
# scan extracts is `%s`, which is not a `.md`, and it is dropped before any finding.
new_root
printf '# Target\n\n## One home\n' >"$root/kk-flavor/standards/target.md"
printf 'see [%s](%s) → One home for the rule\n' target.md target.md >"$root/kk-flavor/standards/citer.md"
assert_reports "$undelimited" "fires on a citation whose section is not delimited"

new_root
printf '# Target\n\n## One home\n' >"$root/kk-flavor/standards/target.md"
printf 'see [%s](%s) → **One home** for the rule\n' target.md target.md >"$root/kk-flavor/standards/citer.md"
assert_does_not_report "$undelimited" "accepts the bolded form"

new_root
printf '# Target\n\n## One home\n' >"$root/kk-flavor/standards/target.md"
printf 'see [%s](%s) → `One home` for the rule\n' target.md target.md >"$root/kk-flavor/standards/citer.md"
assert_does_not_report "$undelimited" "accepts the backticked form, which the parser also reads exactly"

# A shell comment cites the same way a document does, and the scan reads both.
new_root
printf '# Target\n\n## One home\n' >"$root/kk-flavor/standards/target.md"
new_script "citer.sh" "$(printf '#!/usr/bin/env bash\n# untested: fixture\n# the rule is %s → One home\ntrue' target.md)"
assert_reports "$undelimited" "fires on an undelimited citation inside a shell comment"

# Each defect below makes a skill unreachable rather than merely mis-linked: the loader finds a skill by
# its directory, invokes it by its frontmatter `name`, and routes to it by its `description`.
echo "check.sh — the skill directory itself"

new_root
mkdir -p "$root/skills/orphan" "$root/skills/wrong-name" "$root/skills/no-desc"
printf -- '---\nname: misnamed\ndescription: does a thing\n---\n' >"$root/skills/wrong-name/SKILL.md"
printf -- '---\nname: no-desc\n---\n' >"$root/skills/no-desc/SKILL.md"
assert_reports "skill dir without SKILL.md" "fires on a skill directory holding no SKILL.md"
assert_reports "skill name/dir mismatch" "fires when the frontmatter name is not the directory name"
assert_reports "skill without a description" "fires on a SKILL.md carrying no description"

echo "check.sh — script test position"

missing_test="script names a missing test"
no_position="script declares no test position"

new_root
new_script "lonely.sh" '#!/usr/bin/env bash
# Does a thing.
true'
assert_reports "$no_position" "fires on a script naming neither a test nor an untested reason"

new_root
new_script "claims.sh" '#!/usr/bin/env bash
# A change here needs a case in claims-test.sh beside it.
true'
assert_reports "$missing_test" "fires on a header naming a -test.sh that is not in the tree"

new_root
new_script "covered.sh" '#!/usr/bin/env bash
# A change here needs a case in covered-test.sh beside it.
true'
new_script "covered-test.sh" '#!/usr/bin/env bash
true'
assert_does_not_report "$missing_test" "accepts a header whose named test exists"
assert_does_not_report "$no_position" "a named existing test is a declared position"

new_root
new_script "waived.sh" '#!/usr/bin/env bash
# untested: a four-line wrapper whose only failure mode is the exec bit.
true'
assert_does_not_report "$no_position" "accepts an explicit untested: declaration with a reason"

new_root
new_script "bare.sh" '#!/usr/bin/env bash
# untested:
true'
assert_reports "$no_position" "a bare untested: with no reason does not clear the check"

# The harness is exempt: asking a test file to name its own test makes every one of them a finding.
new_root
new_script "harness-test.sh" '#!/usr/bin/env bash
true'
new_script "harness-mutate.sh" '#!/usr/bin/env bash
true'
assert_does_not_report "$no_position" "asks nothing of -test.sh and -mutate.sh themselves"

# Header-scoped on purpose: a suite a script merely mentions in its body would read as coverage.
new_root
new_script "body.sh" '#!/usr/bin/env bash
# Does a thing.
set -u
# see also body-test.sh
true'
assert_reports "$no_position" "a -test.sh named below the header does not count as declared"

# The cap that keeps a crafted header from turning one scan into thousands of whole-tree walks. It has
# to *report*, never quietly read less than it looks like it read.
new_root
new_script "greedy.sh" '#!/usr/bin/env bash
# see n1-test.sh n2-test.sh n3-test.sh n4-test.sh n5-test.sh n6-test.sh
# and n7-test.sh n8-test.sh n9-test.sh n10-test.sh n11-test.sh n12-test.sh
true'
assert_reports "names more suites than the scan reads" "reports a header naming more suites than it reads"

# The bound on the header read. A declaration past 200 lines is not seen, which is correct, and it
# still has to be *reported* rather than pass as declared.
new_root
{
  printf '#!/usr/bin/env bash\n'
  line=1
  while [ "$line" -le 205 ]; do
    printf '# padding %s\n' "$line"
    line=$((line + 1))
  done
  printf '# untested: this reason sits past the 200-line bound and cannot clear the check\n'
  printf 'true\n'
} >"$root/skills/buried.sh"
chmod +x "$root/skills/buried.sh"
assert_reports "$no_position" "a declaration past the header bound does not clear the check"

# The suite list is built from filenames the reviewed tree chose. A newline in one splits a basename
# in two, the tail reads as a suite that exists, and a header naming an absent suite then passes. The
# control case comes first: without the hostile file, the finding must be there to lose.
new_root
new_script "tool.sh" '#!/usr/bin/env bash
# a change here needs a case in ghost-test.sh
true'
assert_reports "$missing_test" "reports a named suite that is absent (control for the case below)"
new_file_with_newline_name "$root/skills/$(printf 'x\nghost-test.sh')" 'not a suite' 'the forged-suite-name case'
assert_reports "$missing_test" "a newline in a filename cannot forge the suite that satisfies a header"

new_root
new_script "dash.sh" '#!/usr/bin/env bash
# a change here needs a case in --test.sh
true'
assert_reports "$missing_test" "a suite name starting with a dash is still checked"
assert_does_not_report "unrecognized option" "and grep never dumps its usage into the findings"
assert_does_not_report "Usage: grep" "nor its usage banner"

echo "$passed passed, $failed failed"
[ "$failed" = 0 ]
