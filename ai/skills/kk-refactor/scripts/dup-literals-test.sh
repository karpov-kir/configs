#!/usr/bin/env bash
# Cases for dup-literals.sh. The one that must not be weakened is "a path argument is refused with
# exit 2, and says so differently from a scan git rejected": three skills read exit 2 as "did not run,
# never clean", and both of this script's exit-2 doors print "the scan did NOT run", so the only thing
# telling a live refusal apart from a dead tool is the half of the message that names which door it was.
# Each of those two cases therefore pins its own wording and asserts the other's is absent.
#
# The second is "duplicated lines that begin with a plus are still compared": in the diff they arrive
# wearing the shape of a `+++` header, and skipping that shape unanchored drops exactly the literals a
# copy-paste of diff-like text produces.
#
# Every literal is generated from a length here, never pasted in. This file is scanned by the tooling
# in this tree, so a long string written out twice literally would be a finding against the real
# checkout — reported against this suite, by the script this suite covers.
#
# Every fixture is a throwaway git repo under one mktemp root. Nothing here writes inside the checkout
# the suite lives in.
#
# DUP_LITERALS_UNDER_TEST names the script to drive, so a mutation run can point the whole suite at a
# deliberately broken copy and see which case goes red. That is how each case below is known to be able
# to fail rather than assumed to be.
set -u

here=$(CDPATH= cd -P "$(dirname "$0")" && pwd -P)
script="${DUP_LITERALS_UNDER_TEST:-$here/dup-literals.sh}"
[ -x "$script" ] || {
  echo "dup-literals-test: $script is not an executable file — nothing was tested" >&2
  exit 2
}

base=$(mktemp -d) || exit 2
trap 'rm -rf "$base"' EXIT

# Both the developer's git config and this script's own env knobs move every verdict below, so the
# suite pins them rather than inheriting them. A global core.excludesFile that happens to ignore the
# fixture extension would empty the untracked cases; an exported DUP_MIN_LEN would move every length
# boundary. In both cases the suite keeps passing while checking nothing.
export GIT_CONFIG_NOSYSTEM=1
export HOME="$base/home"
export XDG_CONFIG_HOME="$base/config"
mkdir -p "$HOME" "$XDG_CONFIG_HOME"
unset DUP_MIN_LEN DUP_MAX_FILE_BYTES

passed=0
failed=0
fixture_count=0
repo=""

record_pass() {
  passed=$((passed + 1))
  echo "  pass  $1"
}

record_fail() {
  failed=$((failed + 1))
  echo "  FAIL  $1  — $2"
}

# One seeded repository, built once and copied per fixture. Building it takes six processes — init,
# two configs, a write, an add, a commit — and a fixture needs none of them to be its own, so the
# copy is one process where the build is six. That matters here and not in most suites: a mutation
# run executes this whole file once per mutant, so a fixture's cost is paid a hundred times over.
seed_repo="$base/seed"
mkdir -p "$seed_repo" &&
  git -C "$seed_repo" init -q >/dev/null 2>&1 &&
  git -C "$seed_repo" config user.email t@t &&
  git -C "$seed_repo" config user.name t &&
  git -C "$seed_repo" config commit.gpgsign false &&
  printf 'seed\n' >"$seed_repo/seed.txt" &&
  git -C "$seed_repo" add seed.txt &&
  git -C "$seed_repo" commit -qm base >/dev/null &&
  git -C "$seed_repo" rev-parse --verify -q HEAD >/dev/null || {
  echo "dup-literals-test: could not build the seed repository in $seed_repo — stopping, since every fixture below is a copy of it" >&2
  exit 2
}

# A fresh repo per case, so no case inherits another's untracked files: with no arguments this script
# scans every untracked file in the tree, and one left behind would be counted into the next verdict.
# The copy is checked by its effect and not only by its exit status — with no HEAD to diff against,
# every case below would report git's error instead of the duplicates it is asserting.
new_repo() {
  fixture_count=$((fixture_count + 1))
  repo="$base/repo$fixture_count"
  cp -R "$seed_repo" "$repo" &&
    git -C "$repo" rev-parse --verify -q HEAD >/dev/null || {
    echo "dup-literals-test: could not build a fixture repo in $repo — stopping, since every case below reads one" >&2
    exit 2
  }
}

# Commits the named file empty, so everything a case writes into it afterwards is an ADDED line. The
# script compares added lines only, so a fixture written before its baseline exists compares nothing.
track_empty() {
  : >"$repo/$1" &&
    git -C "$repo" add -- "$1" >/dev/null &&
    git -C "$repo" commit -qm track >/dev/null || {
    echo "dup-literals-test: could not track $1 in $repo — stopping" >&2
    exit 2
  }
}

commit_all() {
  git -C "$repo" add -A >/dev/null &&
    git -C "$repo" commit -qm fixture >/dev/null || {
    echo "dup-literals-test: could not commit the fixture in $repo — stopping" >&2
    exit 2
  }
}

# A run of one character, of the length a case is about. Every literal below comes from here.
repeat_char() {
  printf "%${2}s" "" | tr ' ' "$1"
}

# Probed here, in the suite's own shell, and not inside the helper: every call site reads it through
# `$(...)`, so a stop in there would end that subshell alone. A run that came back empty would leave
# every boundary case comparing two empty literals — and the ones that expect a clean tree would pass
# over a fixture that is not the length they are named for.
probe=$(repeat_char x 100)
case "$probe" in
  *[!x]*) probe="" ;;
esac
[ "${#probe}" -eq 100 ] || {
  echo "dup-literals-test: 'printf | tr' builds no 100-character run on this machine — nothing was tested, since every literal below comes from it" >&2
  exit 2
}
unset probe

# `</dev/null` is a guard, not tidiness: a mutated copy that stops exiting where a case expects must
# fail the case rather than block on the terminal this suite inherited. A suite that hangs when a guard
# is removed cannot prove that guard fires.
run() {
  out=$(cd "$repo" && "$script" "$@" </dev/null 2>&1)
  status=$?
}

# stdout alone. Exit 2 means the scan did not run, so anything it leaves on stdout is read by a caller
# piping the report to a file as the report itself.
run_stdout() {
  out=$(cd "$repo" && "$script" "$@" </dev/null 2>/dev/null)
  status=$?
}

run_env() {
  local assignment="$1"
  shift
  out=$(cd "$repo" && env "$assignment" "$script" "$@" </dev/null 2>&1)
  status=$?
}

expect_status() {
  local name="$1" want="$2"
  [ "$status" -eq "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "exit $status, wanted $want — output: $out"
}

expect_out() {
  local name="$1" want="$2"
  case "$out" in
    *"$want"*) record_pass "$name" ;;
    *) record_fail "$name" "wanted '$want' in: $out" ;;
  esac
}

expect_not_out() {
  local name="$1" unwanted="$2"
  case "$out" in
    *"$unwanted"*) record_fail "$name" "'$unwanted' appears in: $out" ;;
    *) record_pass "$name" ;;
  esac
}

expect_no_out() {
  local name="$1"
  [ -z "$out" ] &&
    record_pass "$name" ||
    record_fail "$name" "expected no output, got: $out"
}

expect_line_count() {
  local name="$1" want="$2" got
  got=$(printf '%s\n' "$out" | grep -c '')
  [ "$got" -eq "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "$got lines, wanted $want"
}

echo "dup-literals.sh"

# A clean tree, both invocations. Without this the exit-0 half of the contract rests on cases that
# expect 0 for a reason — a length floor, a size cap — and a script that exited 0 unconditionally would
# satisfy every one of them.
new_repo
run HEAD
expect_status "an unchanged tree exits 0" 0
expect_no_out "and prints nothing"
run
expect_status "an unchanged tree with no arguments exits 0 too" 0

# --- the two exit-2 doors, and telling them apart ---

# A path is legal for `git diff` and diffs against the index, so scanning it would compare the wrong
# change set and exit 0. Refused instead, and the refusal has to be readable as a refusal.
new_repo
run .
expect_status "a path argument exits 2" 2
expect_out "and the refusal names it as a path" "'.' is a path, not a git-diff revision"
expect_out "and says the scan did not run" "the scan did NOT run"
expect_not_out "and is not the message a scan git rejected prints" "git rejected these arguments"
run_stdout .
expect_no_out "and a refused run leaves nothing on stdout"

run seed.txt
expect_status "a file path exits 2" 2
expect_out "and the refusal names the file" "'seed.txt' is a path, not a git-diff revision"

# `--output=` alone drains the pipe, so an option that reached git would exit 0 over a real duplicate.
run --output=/dev/null
expect_status "an option exits 2" 2
expect_out "and the refusal names it as an option" "is an option, not a git-diff revision"

# The other door: git ran and rejected the arguments. Same exit code, deliberately different message,
# because this one is the "did not run" a caller must never read as clean.
run no-such-revision-here
expect_status "a revision git cannot resolve exits 2" 2
expect_out "and says git rejected the arguments" "git rejected these arguments"
expect_out "and that it is not a clean result" "Not a clean result"
expect_not_out "and is not the path refusal" "is a path, not a git-diff revision"
run_stdout no-such-revision-here
expect_no_out "and a rejected run leaves nothing on stdout"

# An argument that is both a revision and a filename passes the path guard — it resolves as a revision
# — and git refuses it as ambiguous. Exit 2 through the second door, still never a clean 0.
new_repo
printf 'hello\n' >"$repo/dev"
commit_all
git -C "$repo" branch dev >/dev/null 2>&1
run dev
expect_status "an argument that is both a revision and a filename exits 2" 2
expect_out "and it is git that rejected it" "git rejected these arguments"

# Validation stops at `--`, so a pathspec after it is not read as a revision. Both halves: the pathspec
# reaches git, and it selects.
new_repo
track_empty dupes.ts
# Two runs rather than one, and this is load-bearing for the truncation case further down. A literal
# made of a single repeated character has the property that every 60-character window of it looks
# like every other, so `%.60s` and `%.200s` both produce output holding a 60-character run followed
# by the ellipsis — and a case asserting the truncated form passes either way, which is a case that
# cannot fail. The tail being a different character is what makes the cut visible.
literal="$(repeat_char x 60)$(repeat_char y 50)"
{
  printf '%s\n' "$literal"
  printf '%s\n' "$literal"
} >"$repo/dupes.ts"
run HEAD -- dupes.ts
expect_status "a pathspec after -- is scanned rather than refused" 1
expect_out "and the pathspec selected the file" "2x token (110 chars)"
run HEAD -- seed.txt
expect_status "a pathspec after -- that selects nothing exits 0" 0

# --- the 0/1 contract and the length floor ---

run HEAD
expect_status "a literal added twice exits 1" 1
expect_out "and the count and length are printed" "2x token (110 chars)"
# The 60-character bound on the echoed literal, built from the same generator: this output is read back
# by an agent, and one added line of a minified bundle would otherwise be printed whole.
truncated=$(repeat_char x 60)
expect_out "and the literal is truncated to its first 60 characters" "${truncated}…"

# One occurrence is not a duplicate. This is the case a scan that reported everything long would fail.
new_repo
track_empty single.ts
printf '%s\n' "$(repeat_char x 110)" >"$repo/single.ts"
run HEAD
expect_status "a literal added once is not a duplicate" 0
expect_no_out "and the single occurrence is not reported"

# DUP_MIN_LEN, both sides of the default. 99 is under the floor and 100 reaches it; an off-by-one here
# either floods the report with ordinary lines or silences the shortest real duplicates.
new_repo
track_empty floor.ts
under=$(repeat_char y 99)
{
  printf '%s\n' "$under"
  printf '%s\n' "$under"
} >"$repo/floor.ts"
run HEAD
expect_status "a 99-character duplicate is under the default floor" 0
at_floor=$(repeat_char y 100)
{
  printf '%s\n' "$at_floor"
  printf '%s\n' "$at_floor"
} >"$repo/floor.ts"
run HEAD
expect_status "a 100-character duplicate reaches it" 1
expect_out "and its length is reported" "2x token (100 chars)"

# The knob moves the verdict, not just the message. Without this the default could be read from the
# environment and ignored everywhere else.
new_repo
track_empty short.ts
{
  printf 'abcdefghij\n'
  printf 'abcdefghij\n'
} >"$repo/short.ts"
run HEAD
expect_status "a ten-character duplicate is clean at the default" 0
run_env DUP_MIN_LEN=10 HEAD
expect_status "and is a duplicate under a lowered floor" 1
expect_out "which reports the length it was measured at" "2x token (10 chars)"

# --- what counts as one literal ---

# A long token embedded in two otherwise different lines. Whole-line comparison alone would miss it,
# and this is the copy-pasted key or fixture id the script exists to find.
new_repo
track_empty tokens.ts
token=$(repeat_char z 100)
{
  printf 'let first = "%s";\n' "$token"
  printf 'let second = "%s";\n' "$token"
} >"$repo/tokens.ts"
run HEAD
expect_status "the same long token in two different lines is a duplicate" 1
expect_out "and is reported as a token" "2x token (100 chars)"

# A duplicated statement whose own tokens are all short: the whole-line half of the comparison, and the
# trim is what makes two differently indented copies the same literal.
new_repo
track_empty statement.ts
left=$(repeat_char a 60)
right=$(repeat_char b 60)
{
  printf '    %s %s\n' "$left" "$right"
  printf '%s %s   \n' "$left" "$right"
} >"$repo/statement.ts"
run HEAD
expect_status "two differently indented copies of one statement are a duplicate" 1
expect_out "and are reported as a line, at the trimmed length" "2x line  (121 chars)"

# The floor again, on the whole-line half of the comparison. The token half has its own floor and its
# own boundary case above; this line is 100 characters made of a 60- and a 39-character run, so
# neither token reaches the floor and only the line comparison can find it. Without a case here the
# line floor could be off by one in either direction with nothing to say so.
new_repo
track_empty short-tokens.ts
line_at_floor="$(repeat_char c 60) $(repeat_char d 39)"
{
  printf '%s\n' "$line_at_floor"
  printf '%s\n' "$line_at_floor"
} >"$repo/short-tokens.ts"
run HEAD
expect_status "a 100-character line of short tokens reaches the floor" 1
expect_out "and is reported as a line at that length" "2x line  (100 chars)"

# A duplicate that is both a whole line and a single token is one finding, not two. Reported twice it
# would double every count in the report an agent reads back.
new_repo
track_empty once.ts
{
  printf '%s\n' "$at_floor"
  printf '%s\n' "$at_floor"
} >"$repo/once.ts"
run HEAD
expect_status "a duplicate that is both a line and a token exits 1" 1
expect_line_count "and is reported once, not twice" 1
expect_out "as the token" "2x token"

# --- the diff-header anchor ---

# Duplicated content lines beginning `++ ` arrive in the diff as `+++ …`, the shape of a real header.
# Skipping that shape without requiring a `diff --git` first drops them, and a duplicated literal
# shaped like diff output is exactly what a copy-pasted patch or review comment produces. The prefix is
# built here from a variable so it is not a literal in this file.
new_repo
track_empty plus.ts
plus_prefix="++ "
{
  printf '%s%s\n' "$plus_prefix" "$at_floor"
  printf '%s%s\n' "$plus_prefix" "$at_floor"
} >"$repo/plus.ts"
run HEAD
expect_status "duplicated lines that begin with a plus are still compared" 1
expect_out "and the duplicate is reported" "2x token (100 chars)"

# --- untracked files ---

# The no-argument form scans untracked files too; passing a revision means "this change set", and an
# untracked file is in no change set. Both halves, from one fixture: the pair is what proves the branch
# is taken on the argument count rather than always or never.
new_repo
{
  printf '%s\n' "$at_floor"
  printf '%s\n' "$at_floor"
} >"$repo/fresh.ts"
run
expect_status "an untracked file is scanned when no revision is given" 1
expect_out "and the untracked file's duplicate is reported" "2x token (100 chars)"
run HEAD
expect_status "and is not scanned when one is" 0
expect_no_out "and nothing is reported then"

run_env DUP_MAX_FILE_BYTES=10
expect_status "an untracked file over the byte cap is skipped" 0

# A NUL in the first 8KB is the binary test. Without it a minified bundle or an image would be compared
# a line at a time.
new_repo
{
  printf '%s\n' "$at_floor"
  printf '%s\n' "$at_floor"
  printf 'tail\000byte\n'
} >"$repo/binary.ts"
run
expect_status "an untracked binary file is skipped" 0
expect_no_out "and the binary file holds no reported duplicate"

# An untracked file with no final newline, followed by another. `sed 's/^/+/'` passes the missing
# newline through, fusing that file's last line with the next file's first, so neither is compared as
# the literal it is — the two copies below become one 201-character line and the duplicate disappears.
# Two files, because the defect is invisible in a stream holding only one.
new_repo
# No trailing newline, which is the whole fixture: the file ends mid-line, the way an editor without a
# final-newline setting leaves it.
printf '%s' "$at_floor" >"$repo/afirst.ts"
printf '%s\n' "$at_floor" >"$repo/bsecond.ts"
run
expect_status "two untracked files are compared apart when the first has no final newline" 1
expect_out "and the literal they share is the duplicate" "2x token (100 chars)"

# A newline in an untracked name is not a hazard here, unlike in the density scan next door: this
# emitter writes no `+++ b/<path>` header, so there is no header for a second line to forge, and the
# file is scanned rather than skipped. Pinned because the divergence between the two scripts is
# deliberate, and a copy of the other one's guard into this one would be a silent loss of coverage.
new_repo
newline_name=$(printf 'weird\nsecond line')
{
  printf '%s\n' "$at_floor"
  printf '%s\n' "$at_floor"
} >"$repo/$newline_name"
run
expect_status "an untracked path whose name holds a newline is still scanned" 1
expect_out "and the newline-named file's duplicate is reported" "2x token (100 chars)"

# --- shape of the report ---

# No file is excluded by extension, unlike the density scan next door: a repeated literal in a document
# or a fixture is as much a copy-paste as one in code.
new_repo
track_empty notes.md
{
  printf '%s\n' "$at_floor"
  printf '%s\n' "$at_floor"
} >"$repo/notes.md"
run HEAD
expect_status "a duplicate in a markdown file is reported too" 1

# `.gitattributes` belongs to whoever wrote the branch. `* -diff` turns every diff body into "Binary
# files … differ", which compares nothing and exits 0 over a real duplicate.
new_repo
track_empty attrs.ts
printf '*.ts -diff\n' >"$repo/.gitattributes"
commit_all
{
  printf '%s\n' "$at_floor"
  printf '%s\n' "$at_floor"
} >"$repo/attrs.ts"
run HEAD
expect_status "a duplicate survives a -diff attribute" 1
expect_out "and the duplicate still arrives" "2x token (100 chars)"

# Two revisions, the form a pipeline orchestrator passes. `"${@:-HEAD}"` has to forward all of them.
new_repo
track_empty pair.ts
{
  printf '%s\n' "$at_floor"
  printf '%s\n' "$at_floor"
} >"$repo/pair.ts"
commit_all
# A third copy on top, uncommitted, so that the range is a real range. Without it `git diff HEAD~1`
# reaches the working tree and counts the same two, and the case would pass for a script that
# forwarded only its first argument.
printf '%s\n' "$at_floor" >>"$repo/pair.ts"
run HEAD~1 HEAD
expect_status "a two-revision range is scanned" 1
expect_out "and reports against that range" "2x token (100 chars)"

# The cap and the announcement are the same number, and a suppressed duplicate is announced rather than
# dropped. If the two numbers ever diverge, findings vanish with nothing said.
new_repo
track_empty many.ts
: >"$repo/many.ts"
for ((n = 1; n <= 201; n++)); do
  distinct="$(repeat_char q 100)$n"
  printf '%s\n%s\n' "$distinct" "$distinct" >>"$repo/many.ts"
done
run HEAD
expect_status "201 duplicates still exit 1" 1
expect_out "and the ones past the cap are announced, not dropped" "… and 1 further duplicate(s), not shown"
expect_line_count "and exactly the cap is printed above the announcement" 201

# The index is the caller's. This script is run mid-review over a tree someone is working in, so a
# staged change has to be in exactly the same state afterwards.
new_repo
track_empty staged.ts
{
  printf '%s\n' "$at_floor"
  printf '%s\n' "$at_floor"
} >"$repo/staged.ts"
git -C "$repo" add -- staged.ts >/dev/null || {
  echo "dup-literals-test: could not stage staged.ts in $repo — stopping, since the case below is about a staged change" >&2
  exit 2
}
before=$(cd "$repo" && git status --porcelain)
# The staged state is asserted, not assumed. Nothing here compares the run against a fixed string, so
# an unstaged fixture would leave the case comparing one untouched tree against another and passing
# without ever putting anything in the index for the script to disturb.
case "$before" in
  'M  staged.ts') ;;
  *)
    echo "dup-literals-test: staged.ts is not staged in $repo (git status said '$before') — stopping, since comparing two unstaged trees would pass without measuring" >&2
    exit 2
    ;;
esac
run
after=$(cd "$repo" && git status --porcelain)
expect_status "a run over a staged change still reports it" 1
[ "$before" = "$after" ] &&
  record_pass "and the caller's index is untouched" ||
  record_fail "and the caller's index is untouched" "status was '$before', became '$after'"

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]
