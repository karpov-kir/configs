#!/usr/bin/env bash
# Cases for cadence.sh. The one that must not be weakened is "a stamp later than today is undetermined,
# not a 'not due'": exits 1 and 2 both end in "no offer made", so a wrong one there suppresses a
# periodic pass for as long as the bad stamp sits in the file and looks identical from the outside. The
# undetermined cases therefore assert the code and that no verdict line came back with it. The colon in
# the "not due:" those cases look for is load-bearing at both ends: every "not due:" message contains
# "due:" as a substring, so the due cases cannot assert "due:" and must assert the absence of the
# longer one instead — and the undetermined message deliberately ends "this is not a 'not due'", so
# searching for the bare phrase finds the disclaimer and fails a working script.
#
# Every case runs a copy of the script inside a throwaway skill directory, never the script in place.
# cadence.sh resolves its retro record from its own location, so running the real one would write a
# real offer date into idsd-ship's own directory and suppress the next seven days of retro offers for
# whoever works in this checkout — including the sessions running right now.
#
# The dates a case needs are produced by `date`, not by re-deriving the script's own day arithmetic:
# a fixture that reimplemented it would agree with itself rather than with the code. Each one is then
# cross-checked against the elapsed count the script prints, so a wrong fixture fails the case instead
# of hiding inside it.
#
# CADENCE_UNDER_TEST names the file those copies are made from, so a mutation run can point the whole
# suite at a deliberately broken copy and see which case goes red.
set -u

here=$(CDPATH= cd -P "$(dirname "$0")" && pwd -P)
source_script="${CADENCE_UNDER_TEST:-$here/cadence.sh}"
[ -x "$source_script" ] || {
  echo "cadence-test: $source_script is not an executable file — nothing was tested" >&2
  exit 2
}

# Exit 2, and it says why: a fixture root that cannot be created is a suite that did not measure,
# which run-tests.sh counts apart from a failure. Exit 1 there would claim the script under test is
# broken — a different claim, and a false one (`~/.kk-flavor/standards/testing.md` -> **7. What a
# suite reports**).
base=$(mktemp -d) || {
  echo "cadence-test: could not create a temporary directory — nothing was tested" >&2
  exit 2
}
trap 'rm -rf "$base"' EXIT

# The audit record is resolved through git, so the developer's own git config must not reach these
# fixtures: a global core.hooksPath or a template dir would change what `git init` produces underneath
# every audit case below.
export GIT_CONFIG_NOSYSTEM=1
export HOME="$base/home"
export XDG_CONFIG_HOME="$base/config"
mkdir -p "$HOME" "$XDG_CONFIG_HOME"

# Where the retro cases run: a directory inside no repository, which is also how they show the retro
# record needs no repository to resolve.
neutral="$base/neutral"
mkdir -p "$neutral"
run_dir="$neutral"

passed=0
failed=0
fixture_count=0
cadence=""
retro_state=""
repo=""

record_pass() {
  passed=$((passed + 1))
  echo "  pass  $1"
}

record_fail() {
  failed=$((failed + 1))
  echo "  FAIL  $1  — $2"
}

# A throwaway skill directory holding its own copy of the script. The layout is the part that matters:
# the script resolves the directory above its own, so the copy has to sit in a scripts/ subdirectory
# the way the real one does, and the record then lands beside it inside $base.
new_skill() {
  fixture_count=$((fixture_count + 1))
  local skill="$base/skill$fixture_count"
  mkdir -p "$skill/scripts" &&
    cp "$source_script" "$skill/scripts/cadence.sh" &&
    chmod +x "$skill/scripts/cadence.sh" || {
    echo "cadence-test: could not build a fixture skill in $skill — stopping, since every case below runs one" >&2
    exit 2
  }
  cadence="$skill/scripts/cadence.sh"
  retro_state="$skill/last-offer-retro.txt"
}

# One seeded repository, built once and copied per fixture. Building it takes six processes — init,
# two configs, a write, an add, a commit — and a fixture needs none of them to be its own, so the
# copy is one process where the build is six. That matters here and not in most suites: a mutation
# run executes this whole file once per mutant, so a fixture's cost is paid a hundred times over.
# It sits under $base beside $neutral, which stays outside every repository: git looks upward for a
# checkout and never down, so a repository inside $base puts none of its siblings in one.
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
  echo "cadence-test: could not build the seed repository in $seed_repo — stopping, since every audit fixture is a copy of it" >&2
  exit 2
}

# A repository for the audit cases. Checked by its effect: with no commit there is no HEAD, and
# `git worktree add` below needs one.
new_repo() {
  fixture_count=$((fixture_count + 1))
  repo="$base/repo$fixture_count"
  cp -R "$seed_repo" "$repo" &&
    git -C "$repo" rev-parse --verify -q HEAD >/dev/null || {
    echo "cadence-test: could not build a fixture repo in $repo — stopping, since the audit cases read one" >&2
    exit 2
  }
}

# A date offset from today, in the script's own format. Two dialects because both machines this runs on
# exist: BSD `date -v` on macOS, GNU `date -d` elsewhere. Neither working is a stop, never a skip — a
# suite that quietly dropped its boundary cases would report a pass over nothing.
#
# The dialect is chosen here, in the suite's own shell, because every call site reads the helper
# through `$(...)` and an `exit` inside one of those ends that subshell alone. With the stop in the
# helper the suite printed its refusal five times, carried on over five empty stamps and reported
# "64 passed, 9 failed" — a `failed` count where the truth was that five fixtures were never built,
# and 64 passes over a machine whose fixture layer does not work.
date_dialect=""
if [ -n "$(date -v-1d +%Y-%m-%d 2>/dev/null)" ]; then
  date_dialect=bsd
elif [ -n "$(date -d '-1 days' +%Y-%m-%d 2>/dev/null)" ]; then
  date_dialect=gnu
else
  echo "cadence-test: neither 'date -v' nor 'date -d' shifts a date on this machine — nothing was tested, since every boundary case below needs one" >&2
  exit 2
fi

shift_date() {
  local offset="$1" shifted=""
  case "$date_dialect" in
    bsd) shifted=$(date -v"${offset}d" +%Y-%m-%d 2>/dev/null) ;;
    gnu) shifted=$(date -d "$offset days" +%Y-%m-%d 2>/dev/null) ;;
  esac
  # A shift that came back empty after the probe passed cannot stop the suite from in here, so it is
  # returned as a string no case can read as a date. The case then reddens naming it, where an empty
  # stamp would have left a `due` case passing on a record that says nothing.
  [ -n "$shifted" ] || shifted="date-shift-failed"
  printf '%s\n' "$shifted"
}

# Writes a recorded offer date into the current fixture's retro record. Read back, because a write
# that produced nothing leaves the script on its never-offered path — which is a `due`, and the due
# cases below would pass without the record they are named for.
record_offer() {
  printf '%s\n' "$1" >"$retro_state" &&
    [ "$(cat "$retro_state" 2>/dev/null)" = "$1" ] || {
    echo "cadence-test: could not record '$1' in $retro_state — stopping, since the cases below read it" >&2
    exit 2
  }
}

# `</dev/null` is load-bearing: a mutated copy that stops exiting where a case expects must
# fail the case rather than block on the terminal this suite inherited. A suite that hangs when a guard
# is removed cannot prove that guard fires.
run() {
  out=$(cd "$run_dir" && "$cadence" "$@" </dev/null 2>&1)
  status=$?
}

# stdout alone. Exit 2 means nothing was determined, so anything left on stdout is what a caller
# capturing the verdict reads as one.
run_stdout() {
  out=$(cd "$run_dir" && "$cadence" "$@" </dev/null 2>/dev/null)
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

expect_file_holds() {
  local name="$1" path="$2" want="$3" got
  got=$(cat "$path" 2>/dev/null)
  [ "$got" = "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "$path holds '$got', wanted '$want'"
}

expect_exists() {
  local name="$1" path="$2"
  [ -f "$path" ] &&
    record_pass "$name" ||
    record_fail "$name" "$path is not there"
}

expect_absent() {
  local name="$1" path="$2"
  [ ! -e "$path" ] &&
    record_pass "$name" ||
    record_fail "$name" "$path exists and should not"
}

echo "cadence.sh"

# --- the four ways it cannot run ---

# All four exit 2, and 2 is "undetermined", never "not due". A caller that reached the usage path and
# read it as a verdict would skip the pass it was asking about.
new_skill
run
expect_status "no arguments exit 2" 2
expect_out "and prints the usage naming the topics and actions" "usage: cadence.sh {retro|audit} {due|asked}"
# The grammar alone presents `due` and `asked` as two spellings of one query, and a caller probing it
# rewrites the record it was asking about. The warning is the only thing at the point of the mistake
# that says so, so it is part of the contract rather than decoration.
expect_out "and warns that asked writes where due only reads" "'asked' OVERWRITES it with today's date"

run bogus due
expect_status "an unknown topic exits 2" 2
expect_out "and an unknown topic prints the usage" "usage:"

run retro
expect_status "a topic with no action exits 2" 2
expect_out "a topic with no action prints the usage" "usage:"

run retro bogus
expect_status "an unknown action exits 2" 2

run_stdout
expect_no_out "the usage leaves nothing on stdout for a caller to read as a verdict"

# --- retro: the record's own three answers ---

# Never offered is due. This is the first run in a fresh checkout, and it has to be an offer rather
# than a silence.
new_skill
run retro due
expect_status "a retro never offered is due" 0
expect_out "and says why it is due" "no retro has ever been offered"
expect_not_out "and a never-offered retro does not read as a 'not due'" "not due:"

# Recording, and the record's effect. The write is asserted through the next `due` as well as through
# the file, because a record written somewhere the reader does not look would satisfy neither.
today=$(date +%Y-%m-%d)
run retro asked
expect_status "recording the offer exits 0" 0
expect_out "and names the date it recorded" "recorded the retro offer on $today"
expect_file_holds "and the record holds that date" "$retro_state" "$today"
run retro due
expect_status "the offer just recorded is not due" 1
expect_out "and says it is not due" "not due:"
expect_out "and reports no days elapsed" "0 days ago"

# The interval, both sides of it. `-ge` is what makes the seventh day an offer; `-gt` would move every
# cadence in the tree out by one day, invisibly.
new_skill
record_offer "$(shift_date -6)"
run retro due
expect_status "six days after the last offer is not due" 1
expect_out "and the elapsed count is the script's arithmetic, not this suite's" "6 days ago"
expect_out "and names the interval it was measured against" "interval 7 days"

new_skill
record_offer "$(shift_date -7)"
run retro due
expect_status "the seventh day is due" 0
expect_out "and reports the elapsed count" "7 days ago"
expect_not_out "and the seventh day does not read as a 'not due'" "not due:"

new_skill
record_offer "$(shift_date -8)"
run retro due
expect_status "past the interval is due" 0
expect_not_out "and past the interval does not read as a 'not due'" "not due:"

# The stamp is the first line. A record that grew a second line — an editor, a merge — must still
# resolve rather than fall through to undetermined.
new_skill
printf '%s\nleftover second line\n' "$(shift_date -30)" >"$retro_state"
run retro due
expect_status "a record with a trailing line still resolves from its first" 0
expect_out "and measures from that first line" "30 days ago"

# --- retro: everything that is undetermined, and never a 'not due' ---

# A stamp later than today: a clock change, a bad edit, a merge from a machine ahead. Reading it as a
# small negative elapsed would print "not due" and hold the offer off indefinitely.
new_skill
record_offer "$(shift_date +1)"
run retro due
expect_status "a stamp later than today is undetermined" 2
expect_out "and says which way it is wrong" "later than today"
expect_not_out "and a future stamp does not read as a 'not due'" "not due:"
# The message says outright which of the two it is. That sentence is the only thing standing between an
# undetermined run and a reader who has just seen two exits that both mean "no offer made".
expect_out "and says outright that it is not one" "this is not a 'not due'"
run_stdout retro due
expect_no_out "and a future stamp leaves nothing on stdout"

new_skill
record_offer "not-a-date"
run retro due
expect_status "a record holding a non-date is undetermined" 2
expect_out "and quotes what it found" "which is no YYYY-MM-DD date"
expect_not_out "and a non-date record does not read as a 'not due'" "not due:"

new_skill
: >"$retro_state"
run retro due
expect_status "an empty record is undetermined" 2
expect_out "and an empty record is not read as a date" "no YYYY-MM-DD date"

# Shaped like a date and impossible. The pattern alone accepts it, so the range check is the only thing
# between it and a day number computed from a month that does not exist.
new_skill
record_offer "2026-13-01"
run retro due
expect_status "a date shaped but out of range is undetermined" 2
expect_out "and the out-of-range month is not read as a date" "no YYYY-MM-DD date"

# The same defect in the day rather than the month, dated to last year so that reading it as
# arithmetic lands in the *past*. A month of 13 rolls into the following January, which the
# future-stamp guard below refuses on its own — so the case above cannot show the range check firing
# and this one can. The year is taken from the clock rather than written down: a fixed one drifts into
# the future as the years pass and the case would then pass through the wrong door.
new_skill
last_year=$(($(date +%Y) - 1))
record_offer "$last_year-01-32"
run retro due
expect_status "a day out of range is undetermined" 2
expect_out "and the out-of-range day is not read as a date" "no YYYY-MM-DD date"

# Right fields, wrong widths. Everything downstream of the pattern reads this as the first of January:
# a one-character month field parses as 1 and the truncated day field as 5, both inside the ranges
# checked above. The pattern is the only thing between it and a day number computed from a string that
# is not the date it looks like.
new_skill
record_offer "$last_year-1-15"
run retro due
expect_status "a date with unpadded fields is undetermined" 2
expect_out "and the unpadded date is not read as a date" "no YYYY-MM-DD date"

# A record that exists and cannot be read. A directory in its place is the portable way to reach that
# branch; the alternative, an unreadable file, is readable anyway when the suite runs as root.
new_skill
mkdir "$retro_state"
run retro due
expect_status "a record that cannot be read is undetermined" 2
expect_out "and says that is what happened" "could not be read"
expect_not_out "and an unreadable record does not read as a 'not due'" "not due:"

# The write failing is the other half: the offer was NOT recorded, so the next run must offer again,
# and the caller has to be told rather than left believing the date is on disk.
run retro asked
expect_status "a record that cannot be written exits 2" 2
expect_out "and says the offer was not recorded" "was NOT recorded"
expect_out "and says what follows from that" "the next run will offer again"

# --- the caller's environment, which decides where the record is ---

# `cd` consults CDPATH whenever the path it is given is relative and does not begin with a dot, and
# echoes where it landed when it used one — so the directory above the script comes back two lines
# long and every path built from it names somewhere nobody looks. Driven by a relative invocation from
# the directory above, the only shape that reaches CDPATH at all, and the shape a hand-run
# `bash ai/skills/idsd-ship/scripts/cadence.sh retro due` from a checkout root has.
#
# Both directions, because the read and the write fail differently and neither one says so: an
# unguarded `due` reports a record written today as never offered, and an unguarded `asked` prints
# "recorded the retro offer" while creating a directory named after the corruption and leaving the
# real record untouched.
new_skill
record_offer "$today"
skill_root=$(dirname "$(dirname "$cadence")")
out=$(cd "$skill_root" && CDPATH=. bash scripts/cadence.sh retro due </dev/null 2>&1)
status=$?
expect_status "CDPATH in the environment does not hide the record from a relative run" 1
expect_out "and the elapsed count is measured from that record, not from nothing" "0 days ago"

new_skill
record_offer "$(shift_date -30)"
skill_root=$(dirname "$(dirname "$cadence")")
out=$(cd "$skill_root" && CDPATH=. bash scripts/cadence.sh retro asked </dev/null 2>&1)
status=$?
expect_file_holds "and a relative run under CDPATH records into the file the reader looks in" "$retro_state" "$today"

# --- audit: the record that belongs to the repository ---

# The precondition every audit case rests on, and it is asserted: inside a repository the
# "not inside a git repository" case below would pass for the wrong reason.
if (cd "$neutral" && git rev-parse --show-toplevel >/dev/null 2>&1); then
  record_fail "the fixture root is outside any repository" "$neutral resolves to a repository, so the audit cases would read the wrong one"
else
  record_pass "the fixture root is outside any repository"
fi

new_skill
run_dir="$neutral"
run audit due
expect_status "an audit outside any repository is undetermined" 2
expect_out "and says there is no per-repo record" "not inside a git repository"
expect_not_out "and no repository does not read as a 'not due'" "not due:"
run_stdout audit due
expect_no_out "and an audit outside a repository leaves nothing on stdout"

new_repo
new_skill
run_dir="$repo"
run audit due
expect_status "an audit never offered is due" 0
expect_out "and says why" "no audit has ever been offered"

run audit asked
expect_status "recording the audit offer exits 0" 0
expect_file_holds "and the record lands in the repository's git dir" "$repo/.git/idsd-audit-offer" "$(date +%Y-%m-%d)"
# `report.sh discard` wipes a throwaway .idsd/, so a cadence kept there could never come due.
expect_absent "and nothing was written under .idsd" "$repo/.idsd"
run audit due
expect_status "the audit offer just recorded is not due" 1
expect_out "and says the audit is not due" "not due:"

# Run from a subdirectory. `--git-common-dir` answers relative to the caller in an ordinary repo, so
# without absolutising it the record is written to a .git the subdirectory does not have — created on
# the spot, invisible to every other caller, and the offer repeats forever.
new_repo
mkdir -p "$repo/deep/deeper"
new_skill
run_dir="$repo/deep/deeper"
run audit asked
expect_status "recording from a subdirectory exits 0" 0
expect_exists "and the record still lands in the repository's git dir" "$repo/.git/idsd-audit-offer"
expect_absent "and no git dir is invented beside the caller" "$repo/deep/deeper/.git"
run audit due
expect_status "and the record is found again from there" 1

# A linked worktree shares the repository, so it shares the record. `--git-path` would answer this
# worktree's own git dir, the date written from the main tree would be invisible here, and the offer
# would repeat in every worktree.
new_repo
linked="$base/linked$fixture_count"
if git -C "$repo" worktree add "$linked" -b other >/dev/null 2>&1; then
  new_skill
  run_dir="$repo"
  run audit asked
  expect_status "recording the audit offer from the main tree exits 0" 0
  run_dir="$linked"
  run audit due
  expect_status "and a linked worktree sees that same record" 1
  expect_out "and reports it as the repository's" "not due:"
else
  record_fail "a linked worktree sees the main tree's record" "git worktree add failed, so the case could not run"
fi

# The record is per repository, not per machine. One shared date would silence the audit everywhere
# after the first offer anywhere.
new_repo
new_skill
run_dir="$repo"
run audit due
expect_status "a second repository has its own record" 0
expect_out "and has never been offered" "no audit has ever been offered"

# The two topics are two records. One file for both would let a retro offer cancel an audit.
new_skill
run_dir="$neutral"
run retro asked
expect_status "recording a retro offer exits 0" 0
run_dir="$repo"
run audit due
expect_status "and leaves the audit cadence alone" 0
expect_out "which has still never been offered" "no audit has ever been offered"

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]
