#!/usr/bin/env bash
# Proves the shell suites' cases can fail, by breaking one guard at a time in a COPY of the script
# and requiring the case the break was aimed at to go red.
#   usage: shell-mutate.sh [-j <jobs>] [-p] [-k <keys>] [-m <substring>] [-l] [-v]
#          -j  mutants in flight at once (default: twice the cores)
#          -p  preflight only — every anchor matches exactly once, every named case exists
#          -k  run only these scripts' mutants, comma-separated (see -l). This one still GATES:
#              a script is either wholly in or wholly out, so the coverage tail over the scripts
#              that are in is complete, and every case of theirs is still accounted for.
#          -m  run only the mutants whose label holds this substring. This one does NOT gate:
#              it cuts across scripts, so almost every case is unreached by construction.
#          -l  print one line per script — key, script, suite, mutant count — and stop
#          -v  each mutant's diff, and every case it reddened with the suite's own reason
#
# A suite whose cases have never been seen to fail reads as coverage and measures nothing: they may
# all pass, and all pass for the wrong reason. This is what earns a script its tested-by header.
#
# A case counts as proven only when three things hold, and the two nobody checks are the first two.
# The mutation reached the file: asserted against the mutated bytes, never inferred from this
# harness's own log, because a replacement the shell ate before the tool saw it leaves a copy that
# still holds the original and a suite that stays green over unmutated code. The copy still parses:
# `bash -n` before the suite runs, because a syntax error reddens every case at once and proves
# nothing about any of them. And the red names the case the break was aimed at — the only one of the
# three that looks like the check.
#
# A whole run takes minutes, and that order of magnitude is the design: the mutants run in parallel, and one
# costs a copy of a single file rather than a copy of a checkout. Measured over the whole list, about
# eleven minutes on a quiet machine and twenty-two on a contended one, at `-j 4`. This is the only
# place that figure lives — `ai/gate.sh` cites this file rather than restating a number that would
# then rot alone. Under load, drop `-j` rather than reading the watchdog's kills as findings; the run
# reports those apart and exits 2, never 1.
#
# Each mutant names its script, a search string that must match exactly once, its replacement, and
# the case it must kill. Preflight refuses a stale anchor and a case name no suite holds — the two
# ways a mutation run credits itself with a failure it did not cause. Attribution is the deliverable
# and a count is not: the tail of the report names every case no mutant has yet been able to fail,
# and the run stays red while one of them is neither proven nor declared out of reach with a reason.
#
# untested: no suite covers this harness. It watches itself instead — the preflight and baseline
# checks below refuse every way a mutant could credit itself with a red it does not own, by name and
# before anything runs, and a run that measured nothing exits 2 rather than reporting a pass. A suite
# over it would have to build a script, a suite and a mutant per case, which is this file with the
# names changed.
#
# Nothing is written inside this checkout. The mutated copy lives under mktemp and the suite is
# pointed at it through the <SCRIPT>_UNDER_TEST variable each suite reads for exactly this purpose.
set -uo pipefail
export LC_ALL=C

here=$(CDPATH= cd -P "$(dirname "$0")" && pwd -P)

jobs=0
preflight_only=0
filter=""
key_filter=""
list_units=0
verbose=0
while getopts ":j:pk:m:lv" opt; do
  case "$opt" in
    j) jobs="$OPTARG" ;;
    p) preflight_only=1 ;;
    k) key_filter="$OPTARG" ;;
    m) filter="$OPTARG" ;;
    l) list_units=1 ;;
    v) verbose=1 ;;
    *)
      echo "usage: shell-mutate.sh [-j <jobs>] [-p] [-k <keys>] [-m <substring>] [-l] [-v]" >&2
      exit 2
      ;;
  esac
done

case "$jobs" in
  '' | *[!0-9]*) jobs=0 ;;
esac
# Twice the cores, not one short of them. A mutant is a shell suite forking git a few hundred times
# and waiting on each, so the machine sits idle at one job per core: measured here, ten at once held
# 1.9 cores busy and twenty-four held 2.6. Oversubscribing buys most of what is left on the table.
if [ "$jobs" -le 0 ]; then
  cores=$(sysctl -n hw.ncpu 2>/dev/null || nproc 2>/dev/null || echo 4)
  jobs=$((cores * 2))
  [ "$jobs" -ge 2 ] || jobs=2
fi

# A mutant that stops exiting where the suite expects must not hang the run. macOS ships no
# `timeout`, so the watchdog is a background sleep against the suite's own pid.
#
# Err high. A bound under what a suite honestly takes turns every one of that suite's mutants into a
# NO MEASURE, and the run exits 2 having proved nothing. That is a harness permanently declining to
# gate, which is worse than the hang the watchdog is here to stop.
suite_limit=120

# The keys, each one a script, the suite that covers it, and the variable that suite reads to be
# pointed at a copy. Three lookups rather than one table: bash 3.2 has no associative arrays.
#
# No key for `cadence` or `density`: each is a stub over a Go tool, so a key here would mutate the
# resolver rather than the logic. Their mutants live in go-mutate.
script_of() {
  case "$1" in
    dup) printf '%s\n' "$here/skills/kk-refactor/scripts/dup-literals.sh" ;;
  esac
}

suite_of() {
  case "$1" in
    dup) printf '%s\n' "$here/skills/kk-refactor/scripts/dup-literals-test.sh" ;;
  esac
}

var_of() {
  case "$1" in
    dup) printf '%s\n' "DUP_LITERALS_UNDER_TEST" ;;
  esac
}

# ai/gate.sh is deliberately absent. Nine mutants over its refusals were run: six killed, and the
# three that survived each found a real defect. It stays out because of the coverage tail below. A key
# in scope needs every case name proven or excused, and gate-test.sh holds close to ninety: roughly
# thirty-five more mutants at about 130s each.
keys="dup"

# Membership in a space-separated key list. `-k` narrows `$keys` in place, so every loop below asks
# this before doing anything with a mutant, and a run without `-k` answers yes for all of them.
has_key() { # <space-separated keys> <key>
  case " $1 " in
    *" $2 "*) return 0 ;;
  esac
  return 1
}

m_key=()
m_label=()
m_case=()
m_from=()
m_to=()

# mutant <key> <label> <the case it must kill> <search string> <replacement>
mutant() {
  m_key+=("$1")
  m_label+=("$2")
  m_case+=("$3")
  m_from+=("$4")
  m_to+=("$5")
}

u_key=()
u_case=()
u_why=()

# unreachable <key> <the case> <why no single break reddens it, and what did>
#
# A case one broken guard cannot turn red. Declaring it is what makes every other unproven case a
# failure of this run rather than a line in a report nobody acts on. Three things keep the list from
# becoming the place a hard case goes to be forgotten: the reason names the route the case WAS
# reddened by, preflight refuses a declaration naming a case no suite holds, and a mutant that does
# redden a declared case fails the run as a stale claim.
unreachable() {
  u_key+=("$1")
  u_case+=("$2")
  u_why+=("$3")
}

declared_reason() { # <key> <case>
  local i
  for ((i = 0; i < ${#u_case[@]}; i++)); do
    if [ "${u_key[$i]}" = "$1" ] && [ "${u_case[$i]}" = "$2" ]; then
      printf '%s\n' "${u_why[$i]}"
      return
    fi
  done
}

# Literal, never a pattern: an anchor holds regex metacharacters, and index() and substr() read it as
# bytes. Through the environment rather than `awk -v`, which processes escape sequences and would
# turn the `\+` inside an anchor into a plain `+` before the search ever ran.
anchor_count() { # <search string> <file>
  MUT_FROM="$1" awk '
    { body = body $0 "\n" }
    END {
      from = ENVIRON["MUT_FROM"]
      while ((at = index(body, from)) > 0) { n++; body = substr(body, at + length(from)) }
      print n + 0
    }
  ' "$2"
}

apply_mutation() { # <search string> <replacement> <file>, mutated text to stdout
  MUT_FROM="$1" MUT_TO="$2" awk '
    { body = body $0 "\n" }
    END {
      from = ENVIRON["MUT_FROM"]
      at = index(body, from)
      if (at == 0) exit 1
      printf "%s", substr(body, 1, at - 1) ENVIRON["MUT_TO"] substr(body, at + length(from))
    }
  ' "$3"
}

# The cases a suite reports, as `<name><tab><reason>`. The `pass` lines of a green baseline are the
# only list of case names there is; the `FAIL` lines of a mutant run are what that mutant killed. The
# name is cut at the first separator rather than by a greedy match, because the reason after it holds
# one too — and the separator is read from the environment so this file need not carry its bytes.
#
# The reason is kept, not discarded, because it is the only thing that tells a case reddening over
# the guard from a case reddening because the mutant broke the fixture it needed first. A verdict
# alone reads the same either way.
case_lines() { # <marker> <file>
  MUT_MARK="  $1  " MUT_SEP="$fail_separator" awk '
    BEGIN { mark = ENVIRON["MUT_MARK"]; width = length(mark); sep = ENVIRON["MUT_SEP"] }
    substr($0, 1, width) == mark {
      rest = substr($0, width + 1)
      at = index(rest, sep)
      if (at > 0) print substr(rest, 1, at - 1) "\t" substr(rest, at + length(sep))
      else print rest "\t"
    }
  ' "$2"
}

case_names() { # <marker> <file>
  case_lines "$1" "$2" | cut -f1 | sort -u
}

case_reason() { # <case name> <tab-separated file>
  MUT_WANT="$1" awk -F '\t' 'BEGIN { want = ENVIRON["MUT_WANT"] } $1 == want { print $2; exit }' "$2"
}

print_reasons() { # <tab-separated file> <indent>
  MUT_SEP="$fail_separator" MUT_PAD="$2" awk -F '\t' '
    { printf "%s%s%s%s\n", ENVIRON["MUT_PAD"], $1, ENVIRON["MUT_SEP"], $2 }
  ' "$1"
}

# A count out of a suite's last line, read by field name and never by the line's shape — the form
# `~/.kk-flavor/standards/testing.md` → **7. What a suite reports** fixes. A suite that grows a skipped
# count keeps parsing here, and a line holding no `failed` field at all is the suite stopping early
# rather than reporting.
summary_field() { # <field> <the suite's last line>
  MUT_FIELD="$1" MUT_LINE="$2" awk 'BEGIN {
    n = split(ENVIRON["MUT_LINE"], word, /[ ,]+/)
    for (i = 2; i <= n; i++) {
      if (word[i] == ENVIRON["MUT_FIELD"] && word[i - 1] ~ /^[0-9]+$/) { print word[i - 1]; exit }
    }
  }'
}

run_suite() { # <variable> <mutated copy> <suite> <output file>
  local pid watchdog status
  env "$1=$2" bash "$3" </dev/null >"$4" 2>&1 &
  pid=$!
  (
    sleep "$suite_limit"
    kill -9 "$pid" 2>/dev/null
  ) &
  watchdog=$!
  wait "$pid"
  status=$?
  { kill "$watchdog" && wait "$watchdog"; } 2>/dev/null
  return "$status"
}

# One line per suite in the baseline block, and one per mutant in the report. Two shapes, because the
# middle column differs: a suite name against a mutant label.
#
# Both hold the state column here rather than at each call site. There it was hand-typed padding that
# had to match that column's width exactly, with nothing checking that it did.
baseline_line() { # <state> <suite name> <detail>
  printf '  %-15s %-28s %s\n' "$1" "$2" "$3"
}

mutant_line() { # <verdict> <mutant label> <detail>
  printf '  %-15s %-62s %s\n' "$1" "$2" "$3"
}

# Both built rather than spelled out. This file is scanned by the same tooling these scripts belong
# to, so a path written literally here is one that tooling reads as a citation against the real
# checkout, and the suites' own fixtures are built from variables for that same reason.
intent_dir=".idsd"
fail_separator="  $(printf '\342\200\224') "

# --- the mutants ---
#
# Every one aims at a guard with a case behind it. A mutation that kills nothing means the guard is
# unobserved, and that is a finding about the suite rather than a reason to drop either side.

# dup-literals.sh. The same two doors, plus the comparison itself: what counts as one literal, and
# what the diff-header anchor keeps from being read as a header.

mutant dup "dup: an option is no longer refused as one" \
  "and the refusal names it as an option" \
  '    case "$arg" in
      -*)' \
  '    case "$arg" in
      --no-such-option)'

mutant dup "dup: a path argument is scanned instead of refused" \
  "a path argument exits 2" \
  'if [ -e "$arg" ] && ! git rev-parse --verify --quiet "$arg^{}" >/dev/null 2>&1; then' \
  'if false; then'

mutant dup "dup: validation does not stop at the double dash" \
  "a pathspec after -- is scanned rather than refused" \
  '[ "$arg" = "--" ] && break' \
  '[ "$arg" = "--" ] && true'

mutant dup "dup: the path refusal goes to stdout" \
  "and a refused run leaves nothing on stdout" \
  'is a path, not a git-diff revision — the scan did NOT run." >&2' \
  'is a path, not a git-diff revision — the scan did NOT run."'

mutant dup "dup: git rejecting the arguments reads as clean" \
  "a revision git cannot resolve exits 2" \
  '|| {
    echo "dup-literals.sh: git rejected these arguments' \
  '|| true
  true || {
    echo "dup-literals.sh: git rejected these arguments'

mutant dup "dup: the git-rejected message is swallowed by the report pipe" \
  "and says git rejected the arguments" \
  'the scan did NOT run. Not a clean result." >&2' \
  'the scan did NOT run. Not a clean result."'

mutant dup "dup: a -diff attribute suppresses the whole scan" \
  "a duplicate survives a -diff attribute" \
  '--no-color --text' \
  '--no-color'

mutant dup "dup: untracked files are scanned even for a named revision" \
  "and is not scanned when one is" \
  'if [ "$#" -eq 0 ]; then
    emit_untracked_as_added_lines' \
  'if true; then
    emit_untracked_as_added_lines'

mutant dup "dup: untracked files are never scanned" \
  "an untracked file is scanned when no revision is given" \
  'if [ "$#" -eq 0 ]; then
    emit_untracked_as_added_lines' \
  'if false; then
    emit_untracked_as_added_lines'

mutant dup "dup: the untracked byte cap is removed" \
  "an untracked file over the byte cap is skipped" \
  '[ "$bytes" -le "$max_file_bytes" ] &&' \
  '[ "$bytes" -le 99999999 ] &&'

mutant dup "dup: the byte cap stops reading its variable" \
  "an untracked file over the byte cap is skipped" \
  'max_file_bytes="${DUP_MAX_FILE_BYTES:-262144}"' \
  'max_file_bytes="262144"'

mutant dup "dup: the length floor stops reading its variable" \
  "and is a duplicate under a lowered floor" \
  'min_length="${DUP_MIN_LEN:-100}"' \
  'min_length="100"'

# Both floors at once, which is what the boundary case needs: an exclusive token floor alone leaves
# the line branch to find the same literal and report it under the other name.
mutant dup "dup: the length floor moves one past the boundary" \
  "a 100-character duplicate reaches it" \
  'min_length="${DUP_MIN_LEN:-100}"' \
  'min_length="101"'

mutant dup "dup: the binary test never fires" \
  "an untracked binary file is skipped" \
  ' | wc -c)" -eq 0 ]' \
  ' | wc -c)" -ge 0 ]'

mutant dup "dup: the binary test reads four bytes, not eight kilobytes" \
  "an untracked binary file is skipped" \
  '[ "$(head -c 8192 "./$file" 2>/dev/null | tr -cd' \
  '[ "$(head -c 4 "./$file" 2>/dev/null | tr -cd'

mutant dup "dup: the emitter fuses a file with no final newline into the next" \
  "two untracked files are compared apart when the first has no final newline" \
  'awk '"'"'{ print "+" $0 }'"'"' "./$file" 2>/dev/null || true' \
  'sed '"'"'s/^/+/'"'"' "./$file" 2>/dev/null || true'

mutant dup "dup: a plus-led added line is skipped as a header" \
  "duplicated lines that begin with a plus are still compared" \
  '/^\+\+\+ / { if (pending) { pending = 0; next } }' \
  '/^\+\+\+ / { next }'

mutant dup "dup: the token length floor becomes exclusive" \
  "and its length is reported" \
  'if (length(parts[i]) >= min_length) tokens[parts[i]]++' \
  'if (length(parts[i]) > min_length) tokens[parts[i]]++'

mutant dup "dup: the line length floor becomes exclusive" \
  "a 100-character line of short tokens reaches the floor" \
  'if (length(trimmed) >= min_length) lines[trimmed]++' \
  'if (length(trimmed) > min_length) lines[trimmed]++'

mutant dup "dup: the token length floor is removed" \
  "a 99-character duplicate is under the default floor" \
  'if (length(parts[i]) >= min_length) tokens[parts[i]]++' \
  'if (length(parts[i]) >= 0) tokens[parts[i]]++'

mutant dup "dup: trailing whitespace is not trimmed" \
  "two differently indented copies of one statement are a duplicate" \
  'gsub(/^[[:space:]]+|[[:space:]]+$/, "", trimmed)' \
  'gsub(/^[[:space:]]+/, "", trimmed)'

mutant dup "dup: the quote stops separating tokens" \
  "and is reported as a token" \
  'count = split(raw, parts, /[[:space:]"' \
  'count = split(raw, parts, /[[:space:]'

mutant dup "dup: a single occurrence counts as a duplicate" \
  "a literal added once is not a duplicate" \
  'for (token in tokens) if (tokens[token] >= 2) {' \
  'for (token in tokens) if (tokens[token] >= 1) {'

mutant dup "dup: a literal that is both a line and a token is reported twice" \
  "and is reported once, not twice" \
  'if (lines[line] >= 2 && !(line in tokens)) {' \
  'if (lines[line] >= 2) {'

mutant dup "dup: the echoed literal is no longer truncated" \
  "and the literal is truncated to its first 60 characters" \
  'printf "%dx token (%d chars): %.60s' \
  'printf "%dx token (%d chars): %.200s'

mutant dup "dup: a duplicate no longer exits 1" \
  "a literal added twice exits 1" \
  'exit (found > 0)' \
  'exit 0'

mutant dup "dup: the cap and its announcement diverge" \
  "and exactly the cap is printed above the announcement" \
  'if (++shown <= max_shown) printf "%dx token' \
  'if (++shown <= 199) printf "%dx token'

mutant dup "dup: a suppressed duplicate is dropped in silence" \
  "and the ones past the cap are announced, not dropped" \
  'if (found > max_shown) printf' \
  'if (found > 100000) printf'

mutant dup "dup: only the first revision is forwarded to git" \
  "and reports against that range" \
  '"${@:-HEAD}"' \
  '"${1:-HEAD}"'

mutant dup "dup: a clean tree exits 1" \
  "an unchanged tree exits 0" \
  'exit (found > 0)' \
  'exit 1'

mutant dup "dup: the path refusal wears the other door's wording" \
  "and is not the message a scan git rejected prints" \
  'dup-literals.sh: '"'"'$arg'"'"' is a path' \
  'dup-literals.sh: git rejected these arguments, and '"'"'$arg'"'"' is a path'

mutant dup "dup: the git-rejected message wears the path refusal's wording" \
  "and is not the path refusal" \
  'git rejected these arguments — exit 2' \
  'git rejected these arguments, and it is a path, not a git-diff revision — exit 2'

mutant dup "dup: the report stops naming what it counted" \
  "and the count and length are printed" \
  'printf "%dx token (%d chars): %.60s' \
  'printf "%dx tok %d: %.60s'

mutant dup "dup: the announcement fires over a clean tree" \
  "and prints nothing" \
  'if (found > max_shown) printf' \
  'if (found >= 0) printf'

mutant dup "dup: the scan unstages what the caller staged" \
  "and the caller's index is untouched" \
  'emit_untracked_as_added_lines() {' \
  'emit_untracked_as_added_lines() {
  git reset -q >/dev/null 2>&1'

# The denominator, as next door, plus the two things this scanner declines to read: a file whose name
# marks it secret-bearing, and an added line carrying the bytes of a binary file git pushed through
# `--text`. Both are files the scan never read, so both have to reach the tally.
mutant dup "dup: the summary stops naming how much was read" \
  "and the denominator says no file reached it" \
  'printf "dup-literals.sh: %d file(s) reached the scan, %d duplicate(s), %d file(s) skipped unread, %d binary line(s) ignored.\n",' \
  'printf "dup-literals.sh: scan finished.\n",'

mutant dup "dup: a run that read nothing stops saying so" \
  "and names that as saying nothing about the change set" \
  'if (reached + 0 == 0)' \
  'if (reached + 0 < 0)'

mutant dup "dup: files reaching the scan are never counted" \
  "so this run is not reported as having read nothing" \
  '/^diff --git / { pending = 1; reached++; next }' \
  '/^diff --git / { pending = 1; next }'

mutant dup "dup: a file declined by the untracked arm leaves no mark" \
  "and the skip is counted rather than silent" \
  '      printf '"'"'dup-skipped-untracked\n'"'"'
    fi' \
  '      :
    fi'

# The awk half of the same tally, so the mark being emitted and the mark being counted are proven
# apart. With only the mutant above, a counter that never ran would still look observed.
mutant dup "dup: the marks a declined file leaves are never counted" \
  "and the binary skip reaches the denominator too" \
  '/^dup-skipped-untracked$/ { skipped++; next }' \
  '/^dup-skipped-untracked$/ { next }'

mutant dup "dup: binary added lines are compared as literals again" \
  "and the ignored lines are counted rather than dropped in silence" \
  'if (raw ~ /[\001-\010\013\014\016-\037\177]/) { binary_lines++; next }' \
  'if (0) { binary_lines++; next }'

mutant dup "dup: the secret-bearing skip list stops firing" \
  "and no part of the token reaches the output" \
  'if [ -n "$secret_named" ]; then' \
  'if false; then'

# The other direction: a skip list matching everything protects every secret and finds nothing at
# all, which is the shape that reads as a clean scan.
mutant dup "dup: the skip list swallows every untracked file" \
  "and is reported" \
  '      *credential* | *secret*) secret_named=1 ;;' \
  '      *) secret_named=1 ;;'

# --- the run ---

total="${#m_label[@]}"
[ "$total" -gt 0 ] || {
  echo "shell-mutate.sh: the mutant list is empty — read this as the harness broken, never as a clean run" >&2
  exit 2
}

# One line per script: what a caller scopes `-k` on. Counted off the mutant list itself, so the
# mapping has one home and cannot drift from the mutants it describes.
if [ "$list_units" -eq 1 ]; then
  for key in $keys; do
    count=0
    for ((i = 0; i < total; i++)); do
      [ "${m_key[$i]}" = "$key" ] && count=$((count + 1))
    done
    printf '%s\t%s\t%s\t%s\n' "$key" "$(script_of "$key")" "$(suite_of "$key")" "$count"
  done
  exit 0
fi

# The scope. A key naming no script exits 2 rather than narrowing the run to nothing: a caller who
# misspells one would otherwise get a green over zero mutants, which is the verdict this harness is
# least entitled to produce. Whole scripts only — that is what lets `-k` gate where `-m` cannot: the
# coverage tail below is computed per script, so a script wholly in scope has its whole tail checked.
skipped=0
if [ -n "$key_filter" ]; then
  chosen=""
  saved_ifs="$IFS"
  IFS=','
  set -f
  for want in $key_filter; do
    IFS="$saved_ifs"
    set +f
    want="${want// /}"
    if [ -n "$want" ]; then
      has_key "$keys" "$want" || {
        echo "shell-mutate.sh: no script is keyed '$want' — run -l for the keys there are; nothing was mutated" >&2
        exit 2
      }
      has_key "$chosen" "$want" || chosen="$chosen $want"
    fi
    IFS=','
    set -f
  done
  IFS="$saved_ifs"
  set +f
  chosen="${chosen# }"
  [ -n "$chosen" ] || {
    echo "shell-mutate.sh: -k selected no script at all — nothing was mutated" >&2
    exit 2
  }
  keys="$chosen"
  for ((i = 0; i < total; i++)); do
    has_key "$keys" "${m_key[$i]}" || skipped=$((skipped + 1))
  done
fi

scratch=$(mktemp -d) || exit 1
trap 'rm -rf "$scratch"' EXIT
mkdir -p "$scratch/out" "$scratch/base" "$scratch/cover" || exit 1

started=$(date +%s)

# The baseline, over every suite a mutant names. Each verdict below means "this edit turned a green
# case red", which says nothing about a case that was already red — and the pass lines are also the
# only list of case names there is, which is what makes a stale one refusable.
echo "baseline"
for key in $keys; do
  for required in "$(script_of "$key")" "$(suite_of "$key")"; do
    [ -x "$required" ] || {
      echo "shell-mutate.sh: $required is not an executable file — nothing was mutated" >&2
      exit 2
    }
  done
  (
    bash "$(suite_of "$key")" </dev/null >"$scratch/base/$key.out" 2>&1
    echo "$?" >"$scratch/base/$key.status"
  ) &
done
wait

baseline_red=0
for key in $keys; do
  status=$(cat "$scratch/base/$key.status" 2>/dev/null || echo 1)
  suite_name=$(basename "$(suite_of "$key")")
  trailer=$(tail -n 1 "$scratch/base/$key.out")
  case_lines pass "$scratch/base/$key.out" | cut -f1 >"$scratch/base/$key.reported"
  sort -u "$scratch/base/$key.reported" >"$scratch/base/$key.cases"
  count=$(grep -c '' <"$scratch/base/$key.cases")
  reported=$(grep -c '' <"$scratch/base/$key.reported")
  # Exit 2 is not a red. The suite never measured — a fixture it could not build, a tool this machine
  # does not have — so nothing it printed is a statement about the script, and the fix is the fixture.
  # Held apart from the red because "BASELINE RED" sends the reader to the code instead.
  if [ "$status" -eq 2 ]; then
    baseline_line "DID NOT MEASURE" "$suite_name" "$trailer"
    baseline_red=1
    continue
  fi
  if [ "$status" -ne 0 ]; then
    baseline_line "BASELINE RED" "$suite_name" "$trailer"
    baseline_red=1
    continue
  fi
  # A suite reporting no passes leaves no case name for a kill to be attributed to, whatever it exited
  # with. Read by field name rather than by matching the line, so a suite that grows a skipped count is
  # not refused for its shape — and a skip count above 0 is a different fact from vacuity: the cases
  # are there and this machine declined them, so the fix is the machine rather than the suite.
  case "$(summary_field passed "$trailer")" in
    '' | 0)
      case "$(summary_field skipped "$trailer")" in
        '' | 0) baseline_line VACUOUS "$suite_name" "$trailer" ;;
        *) baseline_line "ALL DECLINED" "$suite_name" "$trailer" ;;
      esac
      baseline_red=1
      continue
      ;;
  esac
  # Attribution here is by case name. Two cases sharing one cannot be told apart: a mutant reddening
  # either credits both, and the name being proven able to fail proves it of only one of them.
  if sort "$scratch/base/$key.reported" | uniq -d | grep -q ''; then
    baseline_line AMBIGUOUS "$suite_name" "$reported reported case(s) under $count name(s)"
    sort "$scratch/base/$key.reported" | uniq -cd | sed 's/^/      /'
    baseline_red=1
    continue
  fi
  baseline_line green "$suite_name" "$trailer, $count case name(s), each reported once"
done
[ "$baseline_red" -eq 0 ] || {
  echo "a suite is not fit to mutate against — it never measured at all, fails unmutated, reports no passes at all, or names two cases the same, and every mutant below would credit itself with a red it cannot own" >&2
  exit 2
}

# Preflight. A stale anchor and a case name no suite holds are the same defect: both make a mutant
# report on something other than the guard it names, and both are refused before anything runs.
#
# Counted per mutant, never per defect: one mutant carrying both a dead anchor and a dead case name
# is one mutant that does not resolve, and adding a point for each would report two out of a total
# that counts mutants — a number that can exceed its own denominator. Declarations are counted on
# their own line for the same reason, since they are not mutants and never were part of that total.
stale=0
stale_declarations=0
checked=0
for ((i = 0; i < total; i++)); do
  has_key "$keys" "${m_key[$i]}" || continue
  checked=$((checked + 1))
  unresolved=0
  matches=$(anchor_count "${m_from[$i]}" "$(script_of "${m_key[$i]}")")
  if [ "$matches" -ne 1 ]; then
    printf '  anchor x%-7s %s\n' "$matches" "${m_label[$i]}"
    unresolved=1
  fi
  if ! grep -Fxq -- "${m_case[$i]}" "$scratch/base/${m_key[$i]}.cases"; then
    printf '  no such case    %s aims at "%s", which its suite does not hold\n' "${m_label[$i]}" "${m_case[$i]}"
    unresolved=1
  fi
  stale=$((stale + unresolved))
done
for ((i = 0; i < ${#u_case[@]}; i++)); do
  has_key "$keys" "${u_key[$i]}" || continue
  if ! grep -Fxq -- "${u_case[$i]}" "$scratch/base/${u_key[$i]}.cases"; then
    printf '  no such case    "%s" is declared out of reach, and its suite does not hold it\n' "${u_case[$i]}"
    stale_declarations=$((stale_declarations + 1))
  fi
done
if [ "$stale" -gt 0 ] || [ "$stale_declarations" -gt 0 ]; then
  echo "preflight: $stale of $checked mutants in scope do not resolve, and $stale_declarations out-of-reach declaration(s) name a case no suite holds — nothing was mutated"
  exit 1
fi
echo "preflight: $checked anchors in scope ($skipped outside -k), all matching exactly once; every case a mutant names or a declaration excuses is held"
[ "$preflight_only" -eq 0 ] || exit 0

run_mutant() { # <index>
  local i="$1"
  local key="${m_key[$i]}" want="${m_case[$i]}"
  local script suite variable work mutated verdict killed at ran
  at=$(date +%s)
  script=$(script_of "$key")
  suite=$(suite_of "$key")
  variable=$(var_of "$key")
  work="$scratch/m$i"
  mkdir -p "$work" || {
    printf 'invalid|0|0\n' >"$scratch/out/$i"
    return
  }
  mutated="$work/$(basename "$script")"
  if ! apply_mutation "${m_from[$i]}" "${m_to[$i]}" "$script" >"$mutated"; then
    printf 'invalid|0|0\n' >"$scratch/out/$i"
    return
  fi
  chmod +x "$mutated"
  diff -U1 "$script" "$mutated" >"$work/edit.diff"
  # Four verdicts off the copy's own bytes before the suite is ever run, because every one of them
  # looks like a kill from a distance and none of them says anything about the guard aimed at.
  #
  # An edit that changed nothing. An edit whose replacement never landed — what a `$`-token eaten
  # before the tool saw it, or a tab written as spaces, leaves behind: a copy that still holds the
  # original, under a suite that stays green over unmutated code. An edit that landed and left the
  # anchor standing anyway, which is a second occurrence the preflight count did not see. And one
  # the shell will not parse, which reddens every case at once.
  if cmp -s "$script" "$mutated"; then
    printf 'inert|0|%s\n' "$(($(date +%s) - at))" >"$scratch/out/$i"
    return
  fi
  if [ "$(anchor_count "${m_to[$i]}" "$mutated")" -eq 0 ]; then
    printf 'NOT APPLIED|0|%s\n' "$(($(date +%s) - at))" >"$scratch/out/$i"
    return
  fi
  # Unless the replacement carries the anchor inside it, which is how a mutant wraps a guard rather
  # than removing it — there the anchor is meant to survive.
  case "${m_to[$i]}" in
    *"${m_from[$i]}"*) ;;
    *)
      if [ "$(anchor_count "${m_from[$i]}" "$mutated")" -ne 0 ]; then
        printf 'ANCHOR LEFT|0|%s\n' "$(($(date +%s) - at))" >"$scratch/out/$i"
        return
      fi
      ;;
  esac
  if ! bash -n "$mutated" 2>/dev/null; then
    printf 'broken|0|%s\n' "$(($(date +%s) - at))" >"$scratch/out/$i"
    return
  fi
  run_suite "$variable" "$mutated" "$suite" "$work/suite.out"
  ran=$?
  # The watchdog, told apart from every other way a suite can stop. A signalled exit is this machine
  # being too busy to finish inside `suite_limit`, and it says nothing about the guard. A suite that
  # exited on its own and stopped early says the mutation broke something before the branch.
  # Collapsing the two reports a loaded machine as an unobserved guard, which is a false claim about
  # the code and the one a run under load will make most often.
  if [ "$ran" -ge 128 ]; then
    printf 'NO MEASURE|0|%s\n' "$(($(date +%s) - at))" >"$scratch/out/$i"
    return
  fi
  # The suite's own last line, read for its `failed` field. Without one the suite stopped early — a
  # fixture it could not build — and its FAIL lines are not a verdict on this guard.
  if [ -z "$(summary_field failed "$(tail -n 1 "$work/suite.out")")" ]; then
    printf 'truncated|0|%s\n' "$(($(date +%s) - at))" >"$scratch/out/$i"
    return
  fi
  case_lines FAIL "$work/suite.out" >"$scratch/out/$i.reasons"
  cut -f1 <"$scratch/out/$i.reasons" | sort -u >"$scratch/out/$i.killed"
  killed=$(grep -c '' <"$scratch/out/$i.killed")
  if [ "$killed" -eq 0 ]; then
    verdict="KILLED NOTHING"
  elif grep -Fxq -- "$want" "$scratch/out/$i.killed"; then
    verdict="killed"
  else
    verdict="MISSED"
  fi
  printf '%s|%s|%s\n' "$verdict" "$killed" "$(($(date +%s) - at))" >"$scratch/out/$i"
}

selected=0
# The in-scope count, never the whole list: a `-k` run announcing all of them reads as a whole run,
# which is the narrowing the closing line below is written not to hide. Without -k the two are equal.
echo "$checked mutants, $jobs at once — one guard removed at a time"
for ((i = 0; i < total; i++)); do
  has_key "$keys" "${m_key[$i]}" || continue
  case "${m_label[$i]}" in
    *"$filter"*) ;;
    *) continue ;;
  esac
  selected=$((selected + 1))
  while [ "$(jobs -pr | grep -c '')" -ge "$jobs" ]; do
    sleep 0.1
  done
  run_mutant "$i" &
done
wait

# --- the report ---

bad=0
unmeasured=0
for ((i = 0; i < total; i++)); do
  [ -f "$scratch/out/$i" ] || continue
  IFS='|' read -r verdict killed seconds <"$scratch/out/$i"
  collateral=""
  [ "${killed:-0}" -gt 1 ] && collateral="  +$((killed - 1)) more"
  if [ "$verdict" = "killed" ]; then
    # The suite's own reason for the red, not just that one happened. A case can redden because the
    # mutant broke a fixture it needed before the branch under test was ever reached, and the reason
    # is the only part of the output that tells the two apart.
    mutant_line killed "${m_label[$i]}" \
      "$(printf '"%s" %.72s%s' "${m_case[$i]}" "$(case_reason "${m_case[$i]}" "$scratch/out/$i.reasons")" "$collateral")"
  elif [ "$verdict" = "NO MEASURE" ]; then
    # Counted apart from `bad`, because this one is a finding about the machine. Rolling it in would
    # read as "this guard is unobserved", which is the claim the run is least entitled to make.
    mutant_line "NO MEASURE" "${m_label[$i]}" \
      "the ${suite_limit}s watchdog killed its suite, so nothing was measured about \"${m_case[$i]}\""
    unmeasured=$((unmeasured + 1))
  else
    mutant_line "$verdict" "${m_label[$i]}" "aimed at \"${m_case[$i]}\""
    [ -s "$scratch/out/$i.reasons" ] && print_reasons "$scratch/out/$i.reasons" "                  red instead: "
    bad=$((bad + 1))
  fi
  if [ "$verbose" -eq 1 ]; then
    sed -n '3,$p' "$scratch/m$i/edit.diff" 2>/dev/null | sed 's/^/                  /'
    [ -s "$scratch/out/$i.reasons" ] && print_reasons "$scratch/out/$i.reasons" "                  red: "
  fi
  [ -f "$scratch/out/$i.killed" ] && cat "$scratch/out/$i.killed" >>"$scratch/cover/${m_key[$i]}"
done

# Which cases have been seen to fail at all. A case no mutant reaches is not proven able to fail, and
# a suite full of them is the thing this harness exists to make visible rather than to hide.
echo
unproven=0
stale_claim=0
for key in $keys; do
  : >>"$scratch/cover/$key"
  sort -u "$scratch/cover/$key" >"$scratch/cover/$key.reddened"
  : >"$scratch/cover/$key.declared"
done
for ((i = 0; i < ${#u_case[@]}; i++)); do
  has_key "$keys" "${u_key[$i]}" || continue
  printf '%s\n' "${u_case[$i]}" >>"$scratch/cover/${u_key[$i]}.declared"
done
for key in $keys; do
  sort -u "$scratch/cover/$key.declared" >"$scratch/cover/$key.excused"
  proven=$(comm -12 "$scratch/base/$key.cases" "$scratch/cover/$key.reddened" | grep -c '')
  held=$(grep -c '' <"$scratch/base/$key.cases")
  printf '%s: %s of %s case name(s) proven able to fail\n' "$(basename "$(suite_of "$key")")" "$proven" "$held"
  # A declaration claims no single break reddens the case. One that did is the claim gone stale, and
  # the alternative to saying so is a case counted as excused and as proven at the same time.
  while IFS= read -r name; do
    [ -n "$name" ] || continue
    printf '  STALE CLAIM     %s — declared out of reach, and a mutant reddened it\n' "$name"
    stale_claim=$((stale_claim + 1))
  done < <(comm -12 "$scratch/cover/$key.excused" "$scratch/cover/$key.reddened")
  while IFS= read -r name; do
    [ -n "$name" ] || continue
    if grep -Fxq -- "$name" "$scratch/cover/$key.excused"; then
      printf '  out of reach    %s — %s\n' "$name" "$(declared_reason "$key" "$name")"
    else
      printf '  NOT YET PROVEN  %s\n' "$name"
      unproven=$((unproven + 1))
    fi
  done < <(comm -23 "$scratch/base/$key.cases" "$scratch/cover/$key.reddened")
done

# A filtered run selects a few mutants, so almost every case is unreached by construction and the
# coverage above is a note rather than a verdict. Only a whole run can gate on it.
gated_unproven="$unproven"
if [ -n "$filter" ]; then
  gated_unproven=0
  echo
  echo "-m was given, so the coverage above is over the selected mutants only and does not gate"
fi
# The same reading a mutant killed by the watchdog earns: the cases it would have reddened are
# unproven for want of a measurement, not for want of a guard, and gating on them would name the code
# for a machine that ran out of time.
if [ "$unmeasured" -gt 0 ]; then
  gated_unproven=0
  echo
  echo "$unmeasured mutant(s) never measured, so the coverage above is short by whatever they would have proven and does not gate — rerun with a smaller -j on a quieter machine"
fi

# The run's own last line, read the way this file asks a suite's to be read —
# `~/.kk-flavor/standards/testing.md` → **7. What a suite reports**.
echo
printf '%s mutant(s) run, %s not run (outside -k), %s that proved nothing, %s that never measured, %s case name(s) neither proven nor excused, %ss wall clock\n' \
  "$selected" "$skipped" "$bad" "$unmeasured" "$unproven" "$(($(date +%s) - started))"
# A finding about the code outranks one about the machine. Exit 2 alone means nothing was found wrong
# and something never ran, which a caller may never read as a pass.
[ "$bad" -eq 0 ] && [ "$gated_unproven" -eq 0 ] && [ "$stale_claim" -eq 0 ] || exit 1
[ "$unmeasured" -eq 0 ] || exit 2
