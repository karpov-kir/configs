#!/usr/bin/env bash
# Runs every shell suite in this repository: the `*-test.sh` files beside the scripts they cover.
#   usage: run-tests.sh [-s <suite>] [<root>]   # <root> defaults to the repository this script lives in
#          -s  run just this one suite, by path, instead of discovering them all
#
# The suites run several at a time, half the machine's cores by default, and their output is buffered
# and printed in discovery order — so this reads exactly as it did when they ran one after another.
# `RUN_TESTS_JOBS=1` puts them back on one lane, which is the first thing to try when a suite fails
# here and passes on its own.
#
# `-s` gives a caller that already knows which suite a change could have moved (`ai/gate.sh` is one)
# this file's reading of the result: the exit-2 "did not measure", and the vacuity check that makes a
# suite exiting 0 having run no case a failure. `bash <suite>` gives neither.
#
# Discovery rather than a list, so a suite written tomorrow runs without anyone registering it. Its
# cost is a gate that finds nothing and reports success, so finding zero suites exits 2.
#
# tested by: run-tests-test.sh
set -uo pipefail
export LC_ALL=C

die() {
  printf '%s: %s\n' "${0##*/}" "$1" >&2
  exit 2
}

# `CDPATH=`: set in the environment, `cd` echoes the directory it landed on, so any path built from a
# bare `cd ... && pwd` comes back two lines long and the checks below refuse a directory that is
# really there.
real_dir() { # <directory>
  CDPATH= cd -P -- "$1" && pwd -P
}

named_suite=""
while getopts ":s:" opt; do
  case "$opt" in
    s) named_suite="$OPTARG" ;;
    *)
      echo "usage: run-tests.sh [-s <suite>] [<root>]" >&2
      exit 2
      ;;
  esac
done
shift $((OPTIND - 1))

root="${1:-$(real_dir "$(dirname -- "${BASH_SOURCE[0]}")/..")}"
[ -d "$root" ] || die "not a directory: $root"

# Discovery asks git first, because every file it finds is then executed. `--cached --others
# --exclude-standard` is tracked files plus new untracked ones and nothing else, so a build artefact
# or a vendored tree never gets to execute as a suite. `find` stays as the fallback for a root that is
# legitimately not a checkout, and which one answered is reported: two file sets, never one line.
suites=()
absent=0
broken=0
if [ -n "$named_suite" ]; then
  discovery="named"
  case "$named_suite" in
    /*) ;;
    *) named_suite="$root/$named_suite" ;;
  esac
  [ -f "$named_suite" ] ||
    die "no suite at $named_suite — read this as discovery broken, never as a clean run"
  # -s has to earn the same two guarantees the discovery arm gives: inside the root, and not something
  # .gitignore excludes. Behind an `[ -f ]` alone, `-s ../../../x` executes a file outside the
  # repository, and `-s vendor/dropped-test.sh` executes what run-tests-test.sh proves discovery refuses.
  root_real="$(real_dir "$root")" || exit 2
  suite_real="$(real_dir "$(dirname "$named_suite")" 2>/dev/null)/$(basename "$named_suite")"
  case "$suite_real" in
    "$root_real"/*) ;;
    *)
      die "$named_suite resolves to $suite_real, outside $root_real — it is not this root's to run, and nothing was tested"
      ;;
  esac
  if git -C "$root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    if ! git -C "$root" ls-files --error-unmatch --cached --others --exclude-standard \
      -- "${suite_real#"$root_real"/}" >/dev/null 2>&1; then
      die "${suite_real#"$root_real"/} is ignored or unknown to git, and discovery would not have run it — nothing was tested"
    fi
  fi
  suites+=("$named_suite")
elif git -C "$root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  discovery="git"
  # `-z`, and a NUL-delimited read: without it `ls-files` C-quotes any path that is not plain ASCII
  # (`core.quotePath` is on by default) and the quoted text names no file. The absent arm below then
  # fires: a suite sitting right there is announced absent, the run exits 0, and the gate loses it.
  while IFS= read -r -d '' suite; do
    [ -n "$suite" ] || continue
    if [ -f "$root/$suite" ]; then
      suites+=("$root/$suite")
      continue
    fi
    # `-f` follows symlinks, so it is false for two things and only one is harmless. Gone is an
    # ordinary unstaged deletion. Present but not a runnable regular file — a dangling symlink, a
    # directory, a device — is a broken suite, and green over it means a suite plainly there never ran.
    if [ -e "$root/$suite" ] || [ -L "$root/$suite" ]; then
      printf 'BROKEN %-47s present but not a runnable file — NOT run\n' "$suite"
      broken=$((broken + 1))
      continue
    fi
    # `ls-files` answers what git knows about, and an unstaged deletion is still tracked. Without this
    # the runner reaches `bash <gone>`, gets 127, and an ordinary deletion reddens the whole sweep.
    printf 'ABSENT %-47s tracked by git, not in the working tree — NOT run\n' "$suite"
    absent=$((absent + 1))
  done < <(git -C "$root" ls-files -z --cached --others --exclude-standard -- '*-test.sh' | sort -z -u)
else
  discovery="find"
  # NUL-delimited for the same reason the git arm is: a newline in a path splits one file into two
  # names, and neither of them is a file.
  while IFS= read -r -d '' suite; do
    suites+=("$suite")
  done < <(find "$root" -name "*-test.sh" -type f -not -path "*/node_modules/*" -print0 | sort -z)
fi

[ "${#suites[@]}" -gt 0 ] ||
  die "no *-test.sh under $root — read this as discovery broken, never as a clean run"

# A suite's own count for a named field, read by name rather than position: run-tests-test.sh reports
# three fields where most report two, so counting words in would take the wrong number.
summary_field() { # <field> <the suite's last line>
  RT_FIELD="$1" RT_LINE="$2" awk 'BEGIN {
    n = split(ENVIRON["RT_LINE"], word, /[ ,]+/)
    for (i = 2; i <= n; i++) {
      if (word[i] == ENVIRON["RT_FIELD"] && word[i - 1] ~ /^[0-9]+$/) { print word[i - 1]; exit }
    }
  }'
}

# Keep the complete diagnostics when the console tail cannot hold the first failure. A failed
# write must print the full output instead: losing it would force another diagnostic run.
report_output() {
  local directory log
  printf '%s\n' "$output" | tail -15 | sed 's/^/     /'
  directory="$diagnostic_dir"
  if [ -z "$directory" ]; then
    directory="$(real_dir "${TMPDIR:-/tmp}" 2>/dev/null && printf .)" || directory=""
    directory="${directory%$'\n'.}"
    if [ -n "$directory" ]; then
      case "$directory/" in
        "$root_real/"*)
          directory="$(real_dir /tmp 2>/dev/null && printf .)" || directory=""
          directory="${directory%$'\n'.}"
          ;;
      esac
    fi
    case "$directory/" in
      "$root_real/"*) directory="" ;;
    esac
    if [ -n "$directory" ]; then
      diagnostic_dir="$(umask 077; mktemp -d "$directory/kk-suite-logs.XXXXXX")" || diagnostic_dir=""
    fi
    directory="$diagnostic_dir"
  fi
  if [ -n "$directory" ] &&
    log="$(umask 077; mktemp "$directory/suite.XXXXXX")" &&
    (umask 077; printf 'suite: %s\nstatus: %s\n\n%s\n' "$name" "$status" "$output" >| "$log"); then
    printf '     full log: %s\n' "$log"
  else
    diagnostic_unretained=$((diagnostic_unretained + 1))
    printf '     could not retain full log; complete output follows:\n' >&2
    printf '%s\n' "$output"
  fi
}

# The repository's own dirty set, so a suite that writes into the checkout is caught once here rather
# than in every suite. Empty output with a non-zero status means git could not answer, which is not
# the same as a clean tree — the caller distinguishes them.
tree_state() {
  git -C "$root" status --porcelain 2>/dev/null
}

# How many suites are in flight. Overlapping them is safe in the one respect anything here checks:
# none leaves a change behind in the checkout, and `tree_state` compares `git status` before against
# after. That is narrower than it sounds. It catches a write still sitting there at the end, and misses
# one a suite reverts before finishing, and one further edit to a file already dirty when the run
# started; where git cannot answer, the summary says `containment unchecked` and nothing is compared at
# all. Each suite is meant to build its own temp HOME, and none of this reaches that — `git status` over
# the checkout is blind to a write landing anywhere else, `$HOME` included.
#
# So `bootstrap.sh --verify` takes one lane by default: it calls this runner right after writing
# $HOME/.claude, $HOME/.kk-flavor and $HOME/.codex. One lane is no fix — a suite that escapes escapes
# alone too — it only keeps that from happening beside five peers while the config is half-written.
#
# Bounded, not all at once. Every suite in flight at once made the slowest one take 146s where it takes
# 80 alone — they compete for the cores the `go build` inside them already wants — and half the machine
# leaves that room.
resolve_jobs() {
  # Unset is held apart from every value a caller can spell, and only unset gets the default.
  # `${RUN_TESTS_JOBS:-0}` would collapse them, making a caller's own `0` indistinguishable from the
  # sentinel and handing it the default in silence. Zero is refused: no run is a run on no lanes.
  # Set-but-empty is refused with it — a variable that did not expand names no number.
  jobs="${RUN_TESTS_JOBS-}"
  if [ -n "${RUN_TESTS_JOBS+named}" ]; then
    case "$jobs" in
      "") die "RUN_TESTS_JOBS is set but empty, so it names no number of suites" ;;
      *[!0-9]*) die "RUN_TESTS_JOBS is '$jobs', which is not a whole number of suites" ;;
      # Refused before the arithmetic below, which would otherwise report a number `test` could not
      # read as one against the message about zero.
      [0-9][0-9][0-9][0-9][0-9]*) die "RUN_TESTS_JOBS is '$jobs', which is more lanes than a machine has" ;;
      *) [ "$jobs" -ge 1 ] || die "RUN_TESTS_JOBS is '$jobs', and a run needs at least one lane" ;;
    esac
  fi
  # A default, not a ceiling: a caller who has read the note above and wants the lanes on that path
  # asks for a count and gets it. Only the caller who named nothing is decided here.
  if [ -z "$jobs" ] && [ -n "${BOOTSTRAP_VERIFYING:-}" ]; then
    jobs=1
  fi
  if [ -z "$jobs" ]; then
    # Validated like a caller's value, because it is read from outside: getconf answering something
    # that is not a count would otherwise reach the arithmetic below and kill the runner with a
    # syntax error, which callers read as a suite failing rather than as the runner not measuring.
    cores="$(getconf _NPROCESSORS_ONLN 2>/dev/null || true)"
    case "$cores" in
      "" | *[!0-9]*) cores=2 ;;
    esac
    jobs=$(( cores / 2 ))
    [ "$jobs" -lt 1 ] && jobs=1
  fi
  # `wait -n` arrived in bash 4.3. Without it there is no way to free one slot at a time, so the suites
  # run one at a time — which is what this did before, and is never wrong, only slower.
  #
  # Asked of the version and not by trying it: `(wait -n)` with no children exits 127 on every bash that
  # has it, so a probe reads as "missing" everywhere and silently leaves the whole run sequential.
  #
  # `RUN_TESTS_NO_WAIT_N` is a seam: every machine that runs the suite HAS `wait -n`, so without it
  # nothing could reach the downgrade or its notice, and a regression in either would look exactly like
  # a pass. It forces the fallback; it never suppresses one.
  if [ -n "${RUN_TESTS_NO_WAIT_N:-}" ] ||
    [ "${BASH_VERSINFO[0]:-0}" -lt 4 ] ||
    { [ "${BASH_VERSINFO[0]}" -eq 4 ] && [ "${BASH_VERSINFO[1]:-0}" -lt 3 ]; }; then
    # Said, not done quietly. A caller who set RUN_TESTS_JOBS=6 and silently got one lane holds a
    # number they believe they set and did not, and would see it only as a run six times as long.
    if [ "$jobs" -gt 1 ]; then
      printf '%s: bash %s has no `wait -n`, so the suites run one at a time rather than %s at a time\n' \
        "${0##*/}" "${BASH_VERSINFO[0]:-?}.${BASH_VERSINFO[1]:-?}" "$jobs" >&2
    fi
    jobs=1
  fi
}

# The sentinel preserves newlines belonging to the path, which command substitution strips.
root_real="$(real_dir "$root" && printf .)" || exit 2
root_real="${root_real%$'\n'.}"
root_real="${root_real%/}"
diagnostic_dir=""
diagnostic_unretained=0

passed=0
failed=0
unmeasured=0
checkout_moved=0

before_tree="$(tree_state)"
tree_readable=$?
containment=""

resolve_jobs

work="$(mktemp -d)" || die "no temp directory to collect the suites' output in — nothing ran"
trap 'rm -rf "$work"' EXIT

# Buffered per suite rather than streamed, because concurrent suites writing to one stream interleave
# mid-line. The status goes to its own file: `wait` reports the status of whichever job it reaped, not
# of the suite this index names.
run_suite() { # <index> <suite>
  bash "$2" >"$work/$1.out" 2>&1
  printf '%s' "$?" >"$work/$1.status"
}

running=0
for index in "${!suites[@]}"; do
  if [ "$running" -ge "$jobs" ]; then
    wait -n
    running=$((running - 1))
  fi
  run_suite "$index" "${suites[$index]}" &
  running=$((running + 1))
done
wait

for index in "${!suites[@]}"; do
  suite="${suites[$index]}"
  name="${suite#"$root"/}"
  output="$(cat "$work/$index.out" 2>/dev/null)"
  # A missing status file is a suite whose subshell died before it could write one — unmeasured, and
  # never folded into a pass. 2 is this file's own word for that.
  status="$(cat "$work/$index.status" 2>/dev/null)"
  case "$status" in "" | *[!0-9]*) status=2 ;; esac
  last="$(printf '%s' "$output" | tail -1)"

  # Exit 2 is a suite saying it did not measure — a dependency missing, a machine too loaded to time
  # anything. Counted apart and never folded into failures: unproven is not disproven.
  if [ "$status" -eq 2 ]; then
    printf 'NOMEASURE %-47s %s\n' "$name" "$last"
    unmeasured=$((unmeasured + 1))
    report_output
    continue
  fi

  if [ "$status" -ne 0 ]; then
    printf 'FAIL %s\n' "$name"
    report_output
    failed=$((failed + 1))
    continue
  fi

  # Exiting 0 having passed nothing and skipped nothing means no case ran, and a green exit over no
  # cases is the one failure discovery cannot otherwise see. Zero passed *with* a skip count is a
  # different fact: the cases exist and this machine declined them by name.
  ran="$(summary_field passed "$last")"
  declined="$(summary_field skipped "$last")"
  if [ "${ran:-0}" -eq 0 ] && [ "${declined:-0}" -eq 0 ]; then
    printf 'VACUOUS %-49s %s\n' "$name" "$last"
    report_output
    failed=$((failed + 1))
    continue
  fi

  printf 'ok   %-52s %s\n' "$name" "$last"
  passed=$((passed + 1))
done

# A suite that passes while corrupting the checkout measured something, but not its whole effect.
# Don't name the suite: a concurrent editor looks identical from here, and naming one sends someone
# hunting through code that is fine. It goes out as its own result rather than a failure — `failed`
# counts suites that went red, and nothing here shows one did.
if [ "$tree_readable" -eq 0 ]; then
  after_tree="$(tree_state)"
  if [ "$before_tree" != "$after_tree" ]; then
    echo
    echo "the checkout changed while the suites ran, so this result is not trustworthy. Either a suite"
    echo "wrote into the repository it is testing, or something else edited the tree during the run."
    echo "With several sessions in one checkout the second is common, and nothing here tells them apart:"
    diff <(printf '%s\n' "$before_tree") <(printf '%s\n' "$after_tree") | sed 's/^/     /'
    containment=", the checkout moved"
    checkout_moved=1
  fi
else
  containment=", containment unchecked"
fi

absent_note=""
[ "$absent" -eq 0 ] || absent_note=", $absent tracked but absent from the working tree"
broken_note=""
[ "$broken" -eq 0 ] || broken_note=", $broken present but not runnable"
[ "$diagnostic_unretained" -eq 0 ] || printf 'warning: full logs unavailable for %s suite(s); complete diagnostics were printed above\n' "$diagnostic_unretained"
[ -z "$diagnostic_dir" ] || printf 'full logs: %s\n' "$diagnostic_dir"
printf '\n%s suite(s) found: %s passed, %s failed, %s unmeasured%s%s%s, discovered by %s\n' \
  "${#suites[@]}" "$passed" "$failed" "$unmeasured" "$absent_note" "$broken_note" "$containment" "$discovery"

# Order: a red outranks a non-measurement, and a moved checkout sits between them — it refuses every
# line of the result, not one of them. Exit 3 is the shared vocabulary's ran-and-refuses-the-result
# and 2 its did-not-measure; a caller that confuses them reads a live refusal as a dead tool. A
# broken suite counts as unmeasured, never failed: it never ran at all, and it must not be silent.
[ "$failed" -eq 0 ] || exit 1
[ "$checkout_moved" -eq 0 ] || exit 3
[ "$broken" -eq 0 ] || exit 2
[ "$unmeasured" -eq 0 ] || exit 2
