#!/usr/bin/env bash
# Runs every shell suite in this repository: the `*-test.sh` files beside the scripts they cover.
#   usage: run-tests.sh [-s <suite>] [<root>]   # <root> defaults to the repository this script lives in
#          -s  run just this one suite, by path, instead of discovering them all
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

# The repository's own dirty set, so a suite that writes into the checkout is caught once here rather
# than in every suite. Empty output with a non-zero status means git could not answer, which is not
# the same as a clean tree — the caller distinguishes them.
tree_state() {
  git -C "$root" status --porcelain 2>/dev/null
}

passed=0
failed=0
unmeasured=0
checkout_moved=0

before_tree="$(tree_state)"
tree_readable=$?
containment=""
for suite in "${suites[@]}"; do
  name="${suite#"$root"/}"
  output="$(bash "$suite" 2>&1)"
  status=$?
  last="$(printf '%s' "$output" | tail -1)"

  # Exit 2 is a suite saying it did not measure — a dependency missing, a machine too loaded to time
  # anything. Counted apart and never folded into failures: unproven is not disproven.
  if [ "$status" -eq 2 ]; then
    printf 'NOMEASURE %-47s %s\n' "$name" "$last"
    unmeasured=$((unmeasured + 1))
    continue
  fi

  if [ "$status" -ne 0 ]; then
    printf 'FAIL %s\n' "$name"
    printf '%s\n' "$output" | tail -15 | sed 's/^/     /'
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
