#!/usr/bin/env bash
# Proves stats-test.sh's cases can fail, by breaking one guard at a time in stats.sh — or in check.sh,
# for the cases that assert the two scripts agree.
#   usage: stats-mutate.sh   # one line per mutation; exit 0 only when every one proved something
#
# Slow on purpose: one full suite run per mutation. Run it when a guard or a case changes.
# The runner's judgement is `check-mutate.sh`'s, fenced as a shared region so the two cannot drift.
set -uo pipefail
export LC_ALL=C

# Physical, because the gate below compares it against a `pwd -P` of the mount. A logical `pwd` keeps the
# symlink that mount *is*, so the two never agree and the gate refuses the very path its message names.
here="$(cd -P -- "$(dirname "$0")" && pwd -P)"
stats="$here/stats.sh"
check="$here/../../kk-ecosystem/scripts/check.sh"
suite="$here/stats-test.sh"
for required in "$stats" "$check" "$suite"; do
  [ -x "$required" ] || { echo "stats-mutate: $required is not executable"; exit 1; }
done

# Only from the installed checkout. This executes the suite sitting beside it, once per mutation, and in a
# review worktree that suite is the branch's own file — which would break the read-never-execute property
# `check.sh` leans on to justify its shape. The safe run is also the only useful one: mutating a branch's
# copy proves nothing about the guards that will actually run.
installed="$(cd -P -- "${HOME:-}/.claude/skills/kk-reduce/scripts" 2>/dev/null && pwd -P)"
[ -n "$installed" ] && [ "$installed" = "$here" ] || {
  echo "stats-mutate: run it as ~/.claude/skills/kk-reduce/scripts/stats-mutate.sh — this copy is somewhere else" >&2
  echo "stats-mutate: exit 2 — nothing was mutated. It runs the suite beside it, so where it runs from decides whose code executes." >&2
  exit 2
}

# --- shared:mutation-runner ---
# A mutation that no longer matches the code, or that breaks the script instead of its behaviour, reports
# a pass it never earned — the stale-gate defect one level up, inside the verification. Both happened here,
# three times between them, so both halves are asserted: the edit must change the file, and it must turn
# at least one case red. Either failure makes the whole run exit non-zero.
mutants_run=0
mutants_bad=0

# judge_mutant <label> <verdict> <cases that failed>, where verdict is one of:
#   invalid   — the expression itself failed; nothing was mutated
#   inert     — the expression ran and changed nothing
#   broken    — the mutant no longer parses, so a red case says nothing about the guard
#   spread    — it changed more than one line, so it removed guards it was not aimed at
#   truncated — the suite stopped early, so its FAIL lines are not a verdict on this guard
#   applied   — a real one-line edit to a script that still runs
# `spread` exists because `bash -n` only sees *shell* syntax: `/gsub/d` once deleted four lines across three
# awk programs, one of them the drift comparator's `END {`, whose stderr check.sh sends to /dev/null. The
# mutant parsed, a guard nobody aimed at went silently dead, and the aimed guard's case failed by luck.
# `truncated` exists because a mutant that drives the script to exit 2 mid-suite leaves earlier FAIL lines
# behind, which count as a kill while most cases never ran.
# `broken` earns its own verdict because it is the trap that is easiest to mistake for success: a mutant
# that cannot run fails cases in bulk, which looks exactly like a guard being caught. The kill count is
# printed for the same reason — a number far above one is worth reading even when the verdict is clean.
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
# The baseline, before any mutation. Every verdict below means "this edit turned a green case red", which
# says nothing if the case was already red — a runner that skips this reports a clean sweep over a broken
# suite, which is the very failure it exists to catch, one level further up.
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

# The fixture mirrors the repo's skills/ layout, because stats-test.sh reaches check.sh by relative path.
# `sed` to a new file rather than `sed -i`, whose in-place flag spells differently on BSD and GNU.
run_mutant() {
  local label="$1" which="$2" expr="$3" dir target verdict touched out fails
  dir="$(mktemp -d)" || { echo "  mktemp failed   $label"; mutants_bad=$((mutants_bad + 1)); return; }
  mkdir -p "$dir/skills/kk-reduce/scripts" "$dir/skills/kk-ecosystem/scripts"
  cp "$stats" "$dir/skills/kk-reduce/scripts/stats.sh"
  cp "$suite" "$dir/skills/kk-reduce/scripts/stats-test.sh"
  cp "$check" "$dir/skills/kk-ecosystem/scripts/check.sh"
  case "$which" in
    stats) target="$dir/skills/kk-reduce/scripts/stats.sh" ;;
    check) target="$dir/skills/kk-ecosystem/scripts/check.sh" ;;
    *) echo "stats-mutate: bad target '$which'"; exit 1 ;;
  esac
  if ! sed "$expr" "$target" >"$target.mutant" 2>/dev/null; then
    verdict=invalid
  elif cmp -s "$target" "$target.mutant"; then
    verdict=inert
  elif ! bash -n "$target.mutant" 2>/dev/null; then
    verdict=broken
  else
    # Exactly one original line, or it removed a guard it was not aimed at. `/gsub/d` matched three awk
    # programs; `/fence { next }/d` matched `in_fence { next }` as a substring too.
    touched=$(diff "$target" "$target.mutant" | grep -c '^<' || true)
    if [ "$touched" = 1 ]; then verdict=applied; else verdict=spread; fi
  fi
  mv "$target.mutant" "$target" 2>/dev/null
  chmod +x "$target" 2>/dev/null
  fails=0
  if [ "$verdict" = applied ]; then
    out="$("$dir/skills/kk-reduce/scripts/stats-test.sh" 2>&1)"
    # The trailer, not just the FAIL lines: a mutant that drives the suite to exit 2 part-way leaves the
    # FAILs it already printed behind, and counting those alone reads a truncated run as a kill.
    if printf '%s' "$out" | grep -qE '^[0-9]+ passed, [0-9]+ failed$'; then
      fails=$(printf '%s' "$out" | grep -c '^  FAIL' || true)
    else
      verdict=truncated
    fi
  fi
  judge_mutant "$label" "$verdict" "$fails"
  rm -rf "$dir"
}

assert_baseline_green "$suite"

echo "stats.sh — one guard removed at a time"

run_mutant "no -r in contained_in_root"        stats '/\[ -r "\$1" \] || return 1/d'
run_mutant "no -r in contained_in_root (check)" check '/\[ -r "\$1" \] || return 1/d'
run_mutant "check.sh counts no budget words"   check '/budget_words=$((budget_words +/d'
run_mutant "resolved import contributes none"  stats '/always_loaded_words=$((always_loaded_words + words))/d'
run_mutant "no newline collapse in the note"   stats "/note=\${note\/\/\[\$'\\\\n\\\\r'\]\/ }/d"
run_mutant "no pipe escaping in the note"      stats '/note=${note\/\/|\/\\\\|}/d'
run_mutant "no note-length bar"                stats 's|-gt 40|-gt 100000|'
run_mutant "refusals unreported"               stats 's#^  echo "stats.sh: import refused#  : "#'
run_mutant "ledger not taken out of prose"     stats '/prose=$((prose - ledger_words))/d'
run_mutant "ledger figure unreported"          stats "/^printf 'ledger:/d"
run_mutant "mounted-outside unreported"        stats "/^  printf 'mounted outside:/d"

run_mutant "mounted-outside gate removed"      stats 's|^\[ "$import_mount_is_installed" -eq 1 \] && for mounted|for mounted|'
run_mutant "in-tree mounts not excluded"       stats 's|"$root_canon"/\*) continue ;;|"$root_canon"/*) ;;|'

run_mutant "ledger symlink followed on write"   stats 's|^\[ -L "$history" \] && {|[ -L "" ] \&\& {|'

report_mutants
