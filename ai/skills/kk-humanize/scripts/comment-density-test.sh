#!/usr/bin/env bash
# Cases for comment-density.sh. The one that must not be weakened is "a path argument is refused with
# exit 2, and says so differently from a scan git rejected": three skills read exit 2 as "did not run,
# never clean", and both of this script's exit-2 doors print "the scan did NOT run", so the only thing
# telling a live refusal apart from a dead tool is the half of the message that names which door it was.
# Each of those two cases therefore pins its own wording and asserts the other's is absent.
#
# The second is "an added line shaped like a diff header does not reassign the file": a source file's
# own content is the one input a branch author chooses freely, and reassigning the file to a prose path
# turns a real outlier into a clean run.
#
# Every fixture is a throwaway git repo under one mktemp root. Nothing here writes inside the checkout
# the suite lives in — a scan of the working tree is what several other sessions may be running at the
# same moment, and a stray fixture file would land in their results as a finding.
#
# COMMENT_DENSITY_UNDER_TEST names the script to drive, so a mutation run can point the whole suite at
# a deliberately broken copy and see which case goes red. That is how each case below is known to be
# able to fail rather than assumed to be.
set -u

here=$(CDPATH= cd -P "$(dirname "$0")" && pwd -P)
script="${COMMENT_DENSITY_UNDER_TEST:-$here/comment-density.sh}"
[ -x "$script" ] || {
  echo "comment-density-test: $script is not an executable file — nothing was tested" >&2
  exit 2
}

base=$(mktemp -d) || exit 2
trap 'rm -rf "$base"' EXIT

# Both the developer's git config and this script's own env knobs move every verdict below, so the
# suite pins them rather than inheriting them. A global core.excludesFile that happens to ignore the
# fixture extension would empty the untracked cases; an exported COMMENT_MAX_RATIO would move every
# boundary. In both cases the suite keeps passing while checking nothing.
export GIT_CONFIG_NOSYSTEM=1
export HOME="$base/home"
export XDG_CONFIG_HOME="$base/config"
mkdir -p "$HOME" "$XDG_CONFIG_HOME"
unset COMMENT_MAX_RATIO COMMENT_MIN_LINES DENSITY_MAX_FILE_BYTES

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
  echo "comment-density-test: could not build the seed repository in $seed_repo — stopping, since every fixture below is a copy of it" >&2
  exit 2
}

# A fresh repo per case, so no case inherits another's untracked files: with no arguments this script
# scans every untracked file in the tree, and one left behind would be counted into the next verdict.
# The copy is checked by its effect and not only by its exit status — with no HEAD to diff against,
# every case below would report git's error instead of the counts it is asserting.
new_repo() {
  fixture_count=$((fixture_count + 1))
  repo="$base/repo$fixture_count"
  cp -R "$seed_repo" "$repo" &&
    git -C "$repo" rev-parse --verify -q HEAD >/dev/null || {
    echo "comment-density-test: could not build a fixture repo in $repo — stopping, since every case below reads one" >&2
    exit 2
  }
}

# Commits the named file empty, so everything a case writes into it afterwards is an ADDED line. The
# script counts added lines only, so a fixture written before its baseline exists counts nothing.
track_empty() {
  : >"$repo/$1" &&
    git -C "$repo" add -- "$1" >/dev/null &&
    git -C "$repo" commit -qm track >/dev/null || {
    echo "comment-density-test: could not track $1 in $repo — stopping" >&2
    exit 2
  }
}

commit_all() {
  git -C "$repo" add -A >/dev/null &&
    git -C "$repo" commit -qm fixture >/dev/null || {
    echo "comment-density-test: could not commit the fixture in $repo — stopping" >&2
    exit 2
  }
}

# Content built from a count, never pasted in: this file is itself scanned by the tooling in this tree,
# and a block of comment lines written out literally here would be a finding against the real checkout.
comment_lines() {
  local total="$1" i
  for ((i = 1; i <= total; i++)); do printf '// comment %d\n' "$i"; done
}

code_lines() {
  local total="$1" i
  for ((i = 1; i <= total; i++)); do printf 'let value%d = %d;\n' "$i" "$i"; done
}

blank_lines() {
  local total="$1" i
  for ((i = 1; i <= total; i++)); do printf '\n'; done
}

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

echo "comment-density.sh"

# A clean tree, both invocations. Without this the exit-0 half of the contract rests on cases that
# expect 0 for a reason — an exclusion, a floor — and a script that exited 0 unconditionally would
# satisfy every one of them.
new_repo
run HEAD
expect_status "an unchanged tree exits 0" 0
expect_no_out "and prints nothing"
run
expect_status "an unchanged tree with no arguments exits 0 too" 0

# --- the two exit-2 doors, and telling them apart ---

# A path is legal for `git diff` and diffs against the index, so scanning it would report the wrong
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

# `--output=` alone drains the pipe, so an option that reached git would exit 0 over a real outlier.
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
track_empty outlier.ts
{
  comment_lines 6
  code_lines 1
} >"$repo/outlier.ts"
run HEAD -- outlier.ts
expect_status "a pathspec after -- is scanned rather than refused" 1
expect_out "and the pathspec selected the file" "outlier.ts: 6 comment / 1 code added lines (0.86)"
run HEAD -- seed.txt
expect_status "a pathspec after -- that selects nothing exits 0" 0

# --- the 0/1 contract and its two thresholds ---

run HEAD
expect_status "a comment-heavy file exits 1" 1
expect_out "and prints its counts and ratio" "outlier.ts: 6 comment / 1 code added lines (0.86)"

# COMMENT_MIN_LINES, both sides. The floor is what keeps a two-line comment out of the report, so an
# off-by-one here either floods it or silences it.
new_repo
track_empty floor.ts
comment_lines 4 >"$repo/floor.ts"
run HEAD
expect_status "four added comment lines are under the floor" 0
expect_no_out "and nothing is reported"
comment_lines 5 >"$repo/floor.ts"
run HEAD
expect_status "five added comment lines reach the floor" 1
expect_out "and are reported" "floor.ts: 5 comment / 0 code added lines (1.00)"

# COMMENT_MAX_RATIO, both sides of a bar that is strictly greater-than. 6/20 is exactly 0.30 and must
# not be flagged; 7/20 is 0.35 and must be. A `>=` here would flag every file that merely meets the bar.
new_repo
track_empty ratio.ts
{
  comment_lines 6
  code_lines 14
} >"$repo/ratio.ts"
run HEAD
expect_status "a ratio exactly at the bar is not an outlier" 0
{
  comment_lines 7
  code_lines 13
} >"$repo/ratio.ts"
run HEAD
expect_status "a ratio above the bar is" 1
expect_out "and the ratio is printed" "ratio.ts: 7 comment / 13 code added lines (0.35)"

# Both knobs move the verdict, not just the message. Without these two the defaults could be read from
# the environment and ignored everywhere else.
run_env COMMENT_MAX_RATIO=0.9 HEAD
expect_status "raising the ratio clears the outlier" 0
run_env COMMENT_MIN_LINES=8 HEAD
expect_status "raising the floor clears it too" 0

# A blank added line is neither a comment nor code. Counted as code it would dilute every ratio: five
# comments among twenty blanks would read as 0.20 and pass.
new_repo
track_empty blanks.ts
{
  comment_lines 5
  blank_lines 20
} >"$repo/blanks.ts"
run HEAD
expect_status "blank added lines do not dilute the ratio" 1
expect_out "and they are counted as neither comment nor code" "blanks.ts: 5 comment / 0 code added lines (1.00)"

# Every comment form the counter recognises, including the continuation and closing lines of a block
# and an indented one. A form that stopped being recognised would silently move comments into the code
# count, which lowers the ratio — the direction that hides outliers.
new_repo
track_empty forms.c
{
  printf '/* opening\n'
  printf ' * middle\n'
  printf ' */\n'
  printf '# hash style\n'
  printf '    // indented\n'
  printf '/*tight\n'
  code_lines 1
} >"$repo/forms.c"
run HEAD
expect_status "block, star, closing, hash and indented forms all count as comments" 1
expect_out "and the code line is the only one counted as code" "forms.c: 6 comment / 1 code added lines (0.86)"

# Prose and data files are not source, and a pass over a document would otherwise report every heading
# it added. The lockfile-by-name shape is its own pattern, so it gets its own file.
new_repo
track_empty notes.md
track_empty notes.txt
track_empty data.json
track_empty pnpm-lock.yaml
for prose_file in notes.md notes.txt data.json pnpm-lock.yaml; do
  comment_lines 8 >"$repo/$prose_file"
done
run HEAD
expect_status "prose, data and lockfiles are not counted" 0
expect_no_out "and none of them is reported"

# --- the diff-header anchor ---

# The forged header. A source file whose own added content holds a line beginning `++ ` arrives in the
# diff as `+++ …`, which is the shape that reassigns the file when the `diff --git` anchor is not
# required first. Every added line after it is then counted against a path the change never touched.
# The marker is built here from a variable for the same reason the fixtures are: written out
# literally it would be a finding against this checkout.
#
# The forged path is a scanned extension, deliberately. A prose one is excluded before it can be
# reported, so a reassignment to it prints nothing and the last assertion below is one no defect can
# violate — a case that reads as cover for the anchor while measuring nothing.
new_repo
track_empty forge.ts
forged_header="++ b/elsewhere.ts"
{
  printf '%s\n' "$forged_header"
  comment_lines 6
} >"$repo/forge.ts"
run HEAD
expect_status "an added line shaped like a diff header does not reassign the file" 1
expect_out "and the outlier is still attributed to the file it is in" "forge.ts: 6 comment"
expect_not_out "and never to the path the added line names" "elsewhere.ts:"

# A non-ASCII path arrives C-quoted unless git is told otherwise, and a quoted path fails the `b/`
# test, so the file is never assigned and every one of its added lines is dropped. The counts are the
# assertion rather than the name: macOS normalises the filename on the way to disk, so the exact bytes
# git prints are the filesystem's business, while the counts appear only if the path resolved at all.
new_repo
non_ascii_name=$(printf 'caf\303\251.ts')
track_empty "$non_ascii_name"
{
  comment_lines 6
  code_lines 1
} >"$repo/$non_ascii_name"
run HEAD
expect_status "a non-ASCII path is still assigned" 1
expect_out "and its added lines are counted" "6 comment / 1 code added lines (0.86)"

# `.gitattributes` belongs to whoever wrote the branch. `* -diff` turns every diff body into "Binary
# files … differ", which counts nothing and exits 0 over a real outlier.
new_repo
track_empty attrs.ts
printf '*.ts -diff\n' >"$repo/.gitattributes"
commit_all
{
  comment_lines 6
  code_lines 1
} >"$repo/attrs.ts"
run HEAD
expect_status "a -diff attribute does not suppress the scan" 1
expect_out "and the counts still arrive" "attrs.ts: 6 comment / 1 code added lines (0.86)"

# --- untracked files ---

# The no-argument form scans untracked files too; passing a revision means "this change set", and an
# untracked file is in no change set. Both halves, from one fixture: the pair is what proves the branch
# is taken on the argument count rather than always or never.
new_repo
{
  comment_lines 6
  code_lines 1
} >"$repo/fresh.ts"
run
expect_status "an untracked file is scanned when no revision is given" 1
expect_out "and is reported by name" "fresh.ts: 6 comment / 1 code added lines (0.86)"
run HEAD
expect_status "and is not scanned when one is" 0
expect_no_out "and is not reported then"

run_env DENSITY_MAX_FILE_BYTES=10
expect_status "an untracked file over the byte cap is skipped" 0

# A NUL in the first 8KB is the binary test. Without it a minified bundle or an image would be counted
# a line at a time.
new_repo
{
  comment_lines 6
  code_lines 1
  printf 'tail\000byte\n'
} >"$repo/binary.ts"
run
expect_status "an untracked binary file is skipped" 0
expect_no_out "and is not reported"

# An untracked file with no final newline, followed by another. `sed 's/^/+/'` passes the missing
# newline through, fusing that file's last line with the next file's `diff --git` header: the header
# never anchors, the second file is never assigned, and both files' added lines land on the first one's
# counts. Two files, because the defect is invisible in a stream holding only one.
new_repo
{
  comment_lines 6
  code_lines 1
} >"$repo/afirst.ts"
# No trailing newline, which is the whole fixture. The command substitution drops the one code_lines
# writes, so the file ends mid-line the way an editor without a final-newline setting leaves it.
printf '%s' "$(code_lines 1)" >>"$repo/afirst.ts"
{
  comment_lines 6
  code_lines 1
} >"$repo/bsecond.ts"
run
expect_status "two untracked files are counted apart when the first has no final newline" 1
expect_out "and the first is reported on its own counts" "afirst.ts: 6 comment / 2 code added lines (0.75)"
expect_out "and the second is reported on its own" "bsecond.ts: 6 comment / 1 code added lines (0.86)"

# A newline in an untracked name writes a second line into the stream this script builds by hand — the
# one forged header a tracked branch cannot produce. Skipped and announced, never scanned silently.
new_repo
newline_name=$(printf 'weird\nsecond line')
{
  comment_lines 6
  code_lines 1
} >"$repo/$newline_name"
run
expect_status "an untracked path whose name holds a newline is skipped" 0
expect_out "and the skip is announced" "skipping an untracked path whose name contains a newline"
expect_not_out "and the file is not reported" "comment /"

# --- shape of the report ---

# Two revisions, the form a pipeline orchestrator passes. `"${@:-HEAD}"` has to forward all of them.
new_repo
track_empty pair.ts
comment_lines 6 >"$repo/pair.ts"
commit_all
# An uncommitted line on top of the committed six, so that the range is a real range. Without it
# `git diff HEAD~1` reaches the working tree and reports the same six, and the case would pass for a
# script that forwarded only its first argument.
code_lines 1 >>"$repo/pair.ts"
run HEAD~1 HEAD
expect_status "a two-revision range is scanned" 1
expect_out "and reports against that range" "pair.ts: 6 comment / 0 code added lines (1.00)"

# The cap and the announcement are the same number, and a suppressed outlier is announced rather than
# dropped. If the two numbers ever diverge, findings vanish with nothing said.
new_repo
for ((n = 1; n <= 201; n++)); do
  : >"$repo/many$n.ts"
done
commit_all
for ((n = 1; n <= 201; n++)); do
  comment_lines 5 >"$repo/many$n.ts"
done
run HEAD
expect_status "201 outliers still exit 1" 1
expect_out "and the ones past the cap are announced, not dropped" "… and 1 further outlier(s), not shown"
expect_line_count "and exactly the cap is printed above the announcement" 201

# The index is the caller's. This script is run mid-review over a tree someone is working in, so a
# staged change has to be in exactly the same state afterwards.
new_repo
track_empty staged.ts
{
  comment_lines 6
  code_lines 1
} >"$repo/staged.ts"
git -C "$repo" add -- staged.ts >/dev/null || {
  echo "comment-density-test: could not stage staged.ts in $repo — stopping, since the case below is about a staged change" >&2
  exit 2
}
before=$(cd "$repo" && git status --porcelain)
# The staged state is asserted, not assumed. Nothing here compares the run against a fixed string, so
# an unstaged fixture would leave the case comparing one untouched tree against another and passing
# without ever putting anything in the index for the script to disturb.
case "$before" in
  'M  staged.ts') ;;
  *)
    echo "comment-density-test: staged.ts is not staged in $repo (git status said '$before') — stopping, since comparing two unstaged trees would pass without measuring" >&2
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
