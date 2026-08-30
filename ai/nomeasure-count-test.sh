#!/usr/bin/env bash
# Cases for nomeasure-count.sh — the counter that decides when `shell-mutate.sh` reporting "I did not
# measure" stops being weather and becomes a red job.
#   usage: nomeasure-count-test.sh   # prints one line per case; exit 0 when all pass, 1 otherwise,
#                                    # 2 when it could not reach the script at all
#
# Every scenario gets a directory of its own under mktemp, and the count file inside it is named to
# the script rather than found by it. Nothing here writes into the checkout.
#
# NOMEASURE_COUNT_UNDER_TEST names the file every case drives, so a mutation run can point the whole
# suite at a mutated copy without editing anything in the checkout.
set -uo pipefail
export LC_ALL=C

here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
script="${NOMEASURE_COUNT_UNDER_TEST:-$here/nomeasure-count.sh}"

# Exit 2 and no summary line: every case below is `$script ...`, so a script this suite cannot
# execute fails all of them for a reason that has nothing to do with the guard each one names.
[ -x "$script" ] || {
  echo "nomeasure-count-test.sh: $script is not an executable file — nothing was measured" >&2
  exit 2
}

work="$(mktemp -d)" || {
  echo "nomeasure-count-test.sh: could not make a temp directory — nothing was measured" >&2
  exit 2
}
trap 'rm -rf "$work"' EXIT

pass=0
fail=0

check() { # <name> <expected> <actual>
  local name="$1" expected="$2" actual="$3"
  if [ "$expected" = "$actual" ]; then
    echo "ok   — $name"
    pass=$((pass + 1))
  else
    echo "FAIL — $name"
    printf '       expected: %q\n' "$expected"
    printf '       actual:   %q\n' "$actual"
    fail=$((fail + 1))
  fi
}

# bash 3.2 — macOS's own, and one leg of the gates workflow — ends a `$( )` at the first `)` of a
# `case` pattern written inside it. A needle test that has to run inside one goes through a function.
held() { # <needle> <text> — 'held' when the text holds the needle, the whole text when it does not
  case "$2" in
    *"$1"*) printf 'held' ;;
    *) printf '%s' "$2" ;;
  esac
}

# Leaves the path in $fresh_path rather than printing it, and the counter is why: `x="$(fresh)"` runs
# the body in a subshell, so the increment is discarded and every scenario comes back with the same
# directory and the count file the last one left in it.
scenario=0
fresh_path=""
fresh() { # sets $fresh_path to a count file in a directory of its own, with nothing written yet
  scenario=$((scenario + 1))
  mkdir -p "$work/scenario-$scenario"
  fresh_path="$work/scenario-$scenario/count"
}

seed() { # <count-file> <contents> — a stored count left by an earlier run
  printf '%s' "$2" >|"$1"
}

stored() { # <count-file> — what the file holds, or 'absent'
  if [ -e "$1" ]; then
    cat "$1"
  else
    printf 'absent'
  fi
}

note=""
verdict=""
decide() { # <harness-status> <count-file> — leaves the sentence in $note and the exit code in $verdict
  note="$("$script" "$1" "$2" 2>/dev/null)"
  verdict=$?
}

# --- three in a row ---
#
# The behaviour the whole file exists for. Driven as one chain against one count file, because what
# escalates is the history and not any single run.

fresh
chain="$fresh_path"
check "control: the chain starts with no stored count at all" "absent" "$(stored "$chain")"

decide 2 "$chain"
check "the first did-not-measure warns rather than failing" "1" "$verdict"
check "and records one" "1" "$(stored "$chain")"
check "control: it said something, so the wording cases below compare against a real note" "yes" \
  "$([ -n "$note" ] && printf 'yes' || printf 'no')"
check "and the note counts the run" "held" "$(held "(1 consecutive)" "$note")"

decide 2 "$chain"
check "the second still warns" "1" "$verdict"
check "and records two" "2" "$(stored "$chain")"

decide 2 "$chain"
check "the third escalates" "3" "$verdict"
check "and records three" "3" "$(stored "$chain")"
check "and the note says the count is what changed the verdict" "held" \
  "$(held "3 consecutive runs" "$note")"
check "and that this is no longer machine load" "held" "$(held "no longer machine load" "$note")"

decide 2 "$chain"
check "a fourth keeps escalating rather than starting over" "3" "$verdict"
check "and the count keeps climbing" "4" "$(stored "$chain")"

# --- a measured run clears it ---
#
# Both of the harness's measured statuses, because the counter tracks whether it reached the guards
# at all. Exit 1 is a guard that did not redden, which is a finding about the code and proof that the
# harness got there.

fresh
measured="$fresh_path"
seed "$measured" "2"
decide 0 "$measured"
check "a measured run reports measured" "0" "$verdict"
check "and clears the count" "0" "$(stored "$measured")"
check "and says the count was cleared" "held" "$(held "back to 0" "$note")"

decide 2 "$measured"
check "so the next did-not-measure starts the chain again" "1" "$verdict"
check "control: from one, not from where it left off" "1" "$(stored "$measured")"

fresh
failing="$fresh_path"
seed "$failing" "2"
decide 1 "$failing"
check "a measured-but-failing run also reports measured" "0" "$verdict"
check "and also clears the count" "0" "$(stored "$failing")"

# --- a count this script cannot parse is no history ---
#
# Reading garbage as a number is how a corrupt cache entry holds the count under the threshold
# forever, which is the escalation quietly switched off. A row per shape: a word, a number with a
# tail, a negative, a padded one, an empty file, and two that are all digits and still not counts.

for garbage in "banana" "12abc" "-1" "  2  " "" "99999999999999999999" "9999999999999999999"; do
  fresh
  spoiled="$fresh_path"
  seed "$spoiled" "$garbage"
  check "control: '$garbage' really is what the file held" "$garbage" "$(stored "$spoiled")"
  decide 2 "$spoiled"
  check "a stored '$garbage' reads as no history, so this run is the first" "1" "$verdict"
  check "and '$garbage' leaves the file holding a count of one" "1" "$(stored "$spoiled")"
done

fresh
poisoned="$fresh_path"
seed "$poisoned" "banana"
decide 2 "$poisoned"
decide 2 "$poisoned"
decide 2 "$poisoned"
check "and a garbage value cannot suppress the escalation behind it" "3" "$verdict"

# The row that needs a chain of its own, because a single run of it looks correct. Nineteen digits is
# refused by its length; without that, bash wraps the increment at 64 bits to a negative, and the
# count stays under the threshold for good while every run goes on warning.
fresh
overflowing="$fresh_path"
seed "$overflowing" "9999999999999999999"
decide 2 "$overflowing"
decide 2 "$overflowing"
decide 2 "$overflowing"
check "a count too long to be one cannot suppress the escalation either" "3" "$verdict"

# --- the threshold itself ---

fresh
past="$fresh_path"
seed "$past" "9"
decide 2 "$past"
check "a stored count already past the threshold escalates" "3" "$verdict"
check "and still counts up" "10" "$(stored "$past")"

# --- the path comes from outside this script ---

mkdir -p "$work/día d'été/counts"
outside="$work/día d'été/counts/count"
decide 2 "$outside"
check "a count file path with a space and non-ASCII in it is written, not mangled" "1" \
  "$(stored "$outside")"
check "and the run reports on it" "1" "$verdict"

# --- nothing but the file it was handed ---

fresh
contained="$fresh_path"
decide 2 "$contained"
check "the run writes the named file and nothing else beside it" "count" \
  "$(ls "$(dirname "$contained")")"

# --- the arms that decide nothing ---
#
# Each is asserted on the count file as well as the status: a refusal that writes has already done
# the thing it refused, and the exit code alone would not show it.

no_args_out="$("$script" 2>&1)"
no_args_status=$?
check "no arguments exits 2" "2" "$no_args_status"
check "and names what it needed" "held" "$(held "needs the harness's exit status" "$no_args_out")"

one_arg_status=0
"$script" 2 >/dev/null 2>&1 || one_arg_status=$?
check "a status with no count file exits 2 rather than picking a path" "2" "$one_arg_status"

fresh
bad="$fresh_path"
seed "$bad" "1"
bad_out="$("$script" banana "$bad" 2>&1)"
bad_status=$?
check "a status that is not a number exits 2" "2" "$bad_status"
check "and says so" "held" "$(held "is no exit status" "$bad_out")"
check "and leaves the stored count untouched" "1" "$(stored "$bad")"

# A directory that does not exist, not a permission bit: CI runs as root often enough that chmod 000
# denies nothing there, and the case would then assert against a file the process writes happily.
unwritable="$work/no-such-directory/count"
unwritable_out="$("$script" 2 "$unwritable" 2>&1)"
unwritable_status=$?
check "a count file that will not take the write exits 2, not a warn" "2" "$unwritable_status"
check "and says the count is unchanged" "held" "$(held "the count is unchanged" "$unwritable_out")"
check "control: and it really did not create anything" "absent" "$(stored "$unwritable")"

measured_unwritable_status=0
"$script" 0 "$unwritable" >/dev/null 2>&1 || measured_unwritable_status=$?
check "and a reset that cannot be written is not reported as measured" "2" \
  "$measured_unwritable_status"

echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
