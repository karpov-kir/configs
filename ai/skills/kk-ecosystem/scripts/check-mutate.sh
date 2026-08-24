#!/usr/bin/env bash
# Proves check-test.sh's cases can fail, by breaking one guard in check.sh at a time.
#   usage: check-mutate.sh   # one line per mutation; exit 0 only when every one proved something
#
# Slow on purpose — one full suite run per mutation. The suite is about 100 seconds, so the whole
# harness is a little over an hour. Run it when a guard or a case changes, and budget for that.
# It lives beside the code rather than in a scratch directory: nothing else keeps a mutation in step
# with the guard it aims at.
set -uo pipefail
export LC_ALL=C

# Physical, because the gate below compares it against a `pwd -P` of the mount: a logical `pwd` keeps
# the symlink that mount *is*, and the gate would refuse the very path its message names.
here="$(cd -P -- "$(dirname "$0")" && pwd -P)"
target="$here/check.sh"
suite="$here/check-test.sh"
for required in "$target" "$suite"; do
  [ -x "$required" ] || { echo "check-mutate: $required is not executable"; exit 1; }
done

# Only from the installed checkout. This executes the suite sitting beside it, and in a review worktree
# that suite is the branch's own file — which would break the read-never-execute property `check.sh`
# leans on. Mutating a branch's copy proves nothing about the guards that will actually run anyway.
installed="$(cd -P -- "${HOME:-}/.claude/skills/kk-ecosystem/scripts" 2>/dev/null && pwd -P)"
[ -n "$installed" ] && [ "$installed" = "$here" ] || {
  echo "check-mutate: run it as ~/.claude/skills/kk-ecosystem/scripts/check-mutate.sh — this copy is somewhere else" >&2
  echo "check-mutate: exit 2 — nothing was mutated. It runs the suite beside it, so where it runs from decides whose code executes." >&2
  exit 2
}

# --- shared:mutation-runner ---
# A mutation that no longer matches the code, or that breaks the script instead of its behaviour, reports
# a pass it never earned — the stale-gate defect one level up, inside the verification. So both halves
# are asserted: the edit must change the file, and it must turn at least one case red. Either failure
# makes the whole run exit non-zero.
mutants_run=0
mutants_bad=0

# judge_mutant <label> <verdict> <cases that failed>, where verdict is one of:
#   invalid   — the expression itself failed; nothing was mutated
#   inert     — the expression ran and changed nothing
#   broken    — the mutant no longer parses, so a red case says nothing about the guard
#   spread    — it changed more than one line, so it removed guards it was not aimed at
#   truncated — the suite stopped early, so its FAIL lines are not a verdict on this guard
#   applied   — a real one-line edit to a script that still runs
# The four failing verdicts all look like success from a distance: a mutant that cannot run, or one that
# killed a guard nobody aimed at, fails cases in bulk. `bash -n` sees *shell* syntax only, so it does not
# catch an edit that spreads across awk programs. The kill count is printed for the same reason — a
# number far above one is worth reading even when the verdict is clean.
judge_mutant() {
  mutants_run=$((mutants_run + 1))
  case "$2" in
    applied)
      if [ "$3" -eq 0 ]; then
        echo "  KILLED NOTHING  $1"
        mutants_bad=$((mutants_bad + 1))
      else
        echo "  killed $3        $1"
      fi ;;
    *)
      echo "  $(printf '%-14s' "${2}") $1"
      mutants_bad=$((mutants_bad + 1)) ;;
  esac
}

report_mutants() {
  echo "$mutants_run mutation(s), $mutants_bad that proved nothing"
  [ "$mutants_bad" -eq 0 ]
}
# The baseline, before any mutation. Every verdict below means "this edit turned a green case red",
# which says nothing if the case was already red.
assert_baseline_green() {
  local out
  out="$("$1" 2>&1)" || {
    echo "  BASELINE RED    $1"
    printf '%s\n' "$out" | grep -E '^  FAIL|passed,' | sed 's/^/      /'
    echo "  Fix the suite first: every mutation below would credit itself with failures that already exist."
    exit 2
  }
}
# --- end shared:mutation-runner ---

# The suite refuses to run anywhere but `$HOME/.claude/skills/kk-ecosystem/scripts`, because it executes
# the check.sh beside it and that gate is what keeps a branch's copy from being the code that runs. So the
# sandbox *is* an installation: a throwaway `$HOME` whose mount path holds the mutant and a copy of the
# suite, and nothing else. The gate holds unweakened and decides in favour of the mutant. Put the pair in
# a bare `mktemp -d` instead and every mutant exits 2 with no trailer, so each one reads as `truncated`
# and the whole harness proves nothing while reporting a full run.
sandbox_root=""
sandbox_mount=""
new_sandbox() {
  sandbox_root="$(mktemp -d)" || return 1
  sandbox_mount="$sandbox_root/home/.claude/skills/kk-ecosystem/scripts"
  mkdir -p "$sandbox_mount" || return 1
  cp "$suite" "$sandbox_mount/"
}

# Nothing else goes in that `$HOME` — no `.kk-flavor`. Every case that needs check.sh to see a flavor
# mount builds its own fake HOME per fixture and overrides this one.
run_suite_in_sandbox() {
  HOME="$sandbox_root/home" "$sandbox_mount/check-test.sh" 2>&1
}

# `sed` to a new file rather than `sed -i`, whose in-place flag spells differently on BSD and GNU.
run_mutant() {
  local label="$1" sed_expression="$2" verdict touched out fails
  new_sandbox || { echo "  mktemp failed   $label"; mutants_bad=$((mutants_bad + 1)); return; }
  if ! sed "$sed_expression" "$target" >"$sandbox_mount/check.sh" 2>/dev/null; then
    verdict=invalid
  elif cmp -s "$target" "$sandbox_mount/check.sh"; then
    verdict=inert
  elif ! bash -n "$sandbox_mount/check.sh" 2>/dev/null; then
    verdict=broken
  else
    # Exactly one original line, or it removed a guard it was not aimed at.
    touched=$(diff "$target" "$sandbox_mount/check.sh" | grep -c '^<' || true)
    if [ "$touched" = 1 ]; then verdict=applied; else verdict=spread; fi
  fi
  chmod +x "$sandbox_mount/check.sh" 2>/dev/null
  fails=0
  if [ "$verdict" = applied ]; then
    out="$(run_suite_in_sandbox)"
    # The trailer, not just the FAIL lines: a mutant that drives the suite to exit 2 part-way leaves
    # earlier FAILs behind, and counting those alone reads a truncated run as a kill.
    if printf '%s' "$out" | grep -qE '^[0-9]+ passed, [0-9]+ failed$'; then
      fails=$(printf '%s' "$out" | grep -c '^  FAIL' || true)
    else
      verdict=truncated
    fi
  fi
  judge_mutant "$label" "$verdict" "$fails"
  rm -rf "$sandbox_root"
}

# The sandbox itself, asserted green on an *unmutated* copy before any mutation runs. `assert_baseline_green`
# below proves the suite passes where it is installed; this proves the sandbox did not change what it
# proves. Without it a sandbox that fails cases on its own credits every mutation with a kill it did not
# earn — the same defect one level up that the verdicts above exist to catch.
assert_sandbox_green() {
  local out
  new_sandbox || { echo "  SANDBOX BROKEN  mktemp gave no fixture dir"; exit 2; }
  cp "$target" "$sandbox_mount/check.sh"
  chmod +x "$sandbox_mount/check.sh"
  out="$(run_suite_in_sandbox)"
  rm -rf "$sandbox_root"
  printf '%s' "$out" | grep -qE '^[0-9]+ passed, 0 failed$' || {
    echo "  SANDBOX RED     the suite is not green on an unmutated copy of check.sh"
    printf '%s\n' "$out" | grep -E '^  FAIL|^check-test:|passed,' | sed 's/^/      /'
    echo "  Fix the sandbox first: every mutation below would credit itself with failures it did not cause."
    exit 2
  }
}

assert_baseline_green "$suite"
assert_sandbox_green

echo "check.sh — one guard removed at a time"

run_mutant "direction scan: no redirect to findings"   '/^done >>"\$findings" < <(find "\${direction_targets/s/>>"\$findings" //'
run_mutant "direction scan: guard never fires"         's|^\[ "$was_flavor_scanned" = 1 \]|[ 1 = 1 ]|'
run_mutant "direction scan: flag set from every file"  's/case "\$file" in "\$flavor"\/\*) was_flavor_scanned=1 ;; esac/was_flavor_scanned=1/'
run_mutant "direction scan: no leading-char anchor"    's/\[A-Za-z0-9._~-\]\[A-Za-z0-9._\/~-\]\*\/SKILL/[A-Za-z0-9._\/~-]*\/SKILL/'
# An `assert_does_not_report` case is only proven by a mutation that makes the scan fire where it must
# stay quiet, so some of these mutate toward over-reporting rather than under-reporting.
run_mutant "direction scan: trailing-hyphen strip removed" 's#${named%-}#${named}#'
run_mutant "direction scan: cites budget reset per file" 's#  safe_file="$(oneline "$file")"#  safe_file="$(oneline "$file")"; cites_shown=0#'
run_mutant "direction scan: cites budget uncapped"     's#cites_shown" -le 40#cites_shown" -le 100000#'
run_mutant "direction scan: names budget uncapped"     's#names_shown" -le 40#names_shown" -le 100000#'
run_mutant "direction scan: cites grep drops -a"       's#-a -noE '"'"'\[A-Za-z0-9#-noE '"'"'[A-Za-z0-9#'
run_mutant "direction scan: names grep drops -a"       's#-a -noE '"'"'\\b(kk#-noE '"'"'\\b(kk#'
run_mutant "direction scan: kk-flavor not excluded"    's#\[ "$named" = "kk-flavor" \] && continue#true#'
run_mutant "direction scan: unmounted name still reported" '\#\[ -f "$skills/$named/SKILL.md" \] \|\| continue#d'
run_mutant "direction scan: symlinked target walked"   's#-type f -print0)#-print0)#'
run_mutant "skill dir: SKILL.md never required"        's#\[ -f "$dir/SKILL.md" \] \|\|#[ -d "$dir" ] \|\|#'
run_mutant "import: no installed-checkout gate"        '/import_mount_is_installed" -eq 1 \] || return 1/d'
run_mutant "import: no symlinked-kk-flavor term"       's|\[ ! -L "\$root/kk-flavor" \] && ||'
run_mutant "import: any carrier's import resolves"     's|^    \*) return 1 ;;$|    *) ;;|'
run_mutant "import: no target-symlink refusal"         '/\[ -L "\${HOME}\/\.claude\/\$1" \] &&/d'
run_mutant "import: no readability refusal"            '/\[ -r "\${HOME}\/\.claude\/\$1" \] ||/d'
run_mutant "import: target never set"                  '/import_target="\${HOME}\/\.claude\/\$1"/d'
run_mutant "import: scan reads inside fences"          '/^       fence { next }$/d'
run_mutant "import: scan reads inside code spans"      '/^         gsub(\/`\[^`\]\*`\//d'
run_mutant "import: unreadable refusal unreported"     's|import_refusal="unreadable at the mount"; ||'
run_mutant "import: symlink refusal unreported"        's|import_refusal="a symlink at the mount"; ||'
run_mutant "import: traversal refusal unreported"      's|import_refusal="a traversal, not a bare filename"; ||'
run_mutant "import: traversal passes the guard"        '/a traversal, not a bare filename"; return 1 ;;/d'
run_mutant "import: subdirectory reported as a probe"  's|^    \*/\*) return 1 ;;$|    */*) import_refusal="a path"; return 1 ;;|'
run_mutant "import: subdirectory name resolves"        's|^    \*/\*) return 1 ;;$|    */*) ;;|'
run_mutant "import: no attempt cap"                    's|-lt 64|-lt 100000|'
# `#` as the delimiter wherever the expression holds a pipe: with `|` there, sed reads a malformed
# script and its bulk failures read as a spectacular kill.
run_mutant "findings: tier not ranked within itself"   's#(line ~ /\^syntax: /) return 0#(line ~ /^syntax: /) return 3#'
run_mutant "findings: mounted pattern unanchored"      's|/\^flavor mounted elsewhere/|/flavor mounted elsewhere/|'
run_mutant "findings: did-not-run guard unranked"      '/(line ~ \/\^direction scan read no files\/) return 1/d'
run_mutant "findings: link emitter not one-lined"      's|echo "dangling link: $(oneline "$file") -> $(oneline "$link")"|echo "dangling link: $file -> $link"|'

run_mutant "findings: no per-class cap"                's|shown\[r\] <= 40|shown[r] <= 100000|'
run_mutant "findings: control bytes not stripped"      "s|LC_ALL=C tr '\[:cntrl:\]' ' '|cat|"

# One mutant per guard in the script-test-position scan. Anchor each on the shortest stable fragment,
# never on a whole line: a bound added to that scan leaves a whole-line anchor matching nothing, which
# lands as `inert` — a failing verdict, so the run goes red until the anchor is repaired.
run_mutant "test position: harness not exempt"         's#\*-test.sh | \*-mutate.sh) continue ;;#*-test.sh | *-mutate.sh) ;;#'
run_mutant "test position: named test never resolved"  's|grep -qxF -- "$named_test"|true|'
run_mutant "test position: reason not required"        's|untested:\[\[:space:\]\]\*\[^\[:space:\]\]|untested:|'
run_mutant "test position: header not bounded"         's|^                 { exit }|                 { next }|'
run_mutant "test position: header read unbounded"      's|NR > 200|NR > 100000000|'
run_mutant "test position: name cap not announced"     's|"$named_count" -gt 8|"$named_count" -gt 100000|'
run_mutant "test position: suite charset unfiltered"   's#\*\[!A-Za-z0-9_.-\]\*) continue ;;#*[!A-Za-z0-9_.-]*) ;;#'
run_mutant "test position: name not option-guarded"    's|grep -qxF -- "$named_test"|grep -qxF "$named_test"|'

# Both directions off the delimited-citation guard's single decision point. Mutating it one way only
# leaves the two `assert_does_not_report` cases unproven: a guard that reported *every* citation would
# satisfy them without ever being wrong in the direction they test.
run_mutant "citations: delimited form not required"    's|delimited = (sec != "")|delimited = 1|'
run_mutant "citations: guard fires on every citation"  's|delimited = (sec != "")|delimited = 0|'

report_mutants
