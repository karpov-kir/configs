#!/usr/bin/env bash
# Proves check-test.sh's cases can fail, by breaking one guard in check.sh at a time.
#   usage: check-mutate.sh   # one line per mutation; exit 0 only when every one proved something
#
# Slow on purpose — one full suite run per mutation, and one fixture builds 65 files. Run it when a guard
# or a case changes, not on every check.
# It lives here rather than in a scratch directory because that is where it used to live, and three
# mutations silently stopped matching while nothing kept them in step with the code.
set -uo pipefail
export LC_ALL=C

# Physical, because the gate below compares it against a `pwd -P` of the mount. A logical `pwd` keeps the
# symlink that mount *is*, so the two never agree and the gate refuses the very path its message names.
here="$(cd -P -- "$(dirname "$0")" && pwd -P)"
target="$here/check.sh"
suite="$here/check-test.sh"
for required in "$target" "$suite"; do
  [ -x "$required" ] || { echo "check-mutate: $required is not executable"; exit 1; }
done

# Only from the installed checkout. This executes the suite sitting beside it, once per mutation, and in a
# review worktree that suite is the branch's own file — which would break the read-never-execute property
# `check.sh` leans on to justify its shape. The safe run is also the only useful one: mutating a branch's
# copy proves nothing about the guards that will actually run.
installed="$(cd -P -- "${HOME:-}/.claude/skills/kk-ecosystem/scripts" 2>/dev/null && pwd -P)"
[ -n "$installed" ] && [ "$installed" = "$here" ] || {
  echo "check-mutate: run it as ~/.claude/skills/kk-ecosystem/scripts/check-mutate.sh — this copy is somewhere else" >&2
  echo "check-mutate: exit 2 — nothing was mutated. It runs the suite beside it, so where it runs from decides whose code executes." >&2
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

# `sed` to a new file rather than `sed -i`, whose in-place flag spells differently on BSD and GNU.
run_mutant() {
  local label="$1" expr="$2" dir verdict touched out fails
  dir="$(mktemp -d)" || { echo "  mktemp failed   $label"; mutants_bad=$((mutants_bad + 1)); return; }
  cp "$suite" "$dir/"
  if ! sed "$expr" "$target" >"$dir/check.sh" 2>/dev/null; then
    verdict=invalid
  elif cmp -s "$target" "$dir/check.sh"; then
    verdict=inert
  elif ! bash -n "$dir/check.sh" 2>/dev/null; then
    verdict=broken
  else
    # Exactly one original line, or it removed a guard it was not aimed at. `/gsub/d` matched three awk
    # programs; `/fence { next }/d` matched `in_fence { next }` as a substring too.
    touched=$(diff "$target" "$dir/check.sh" | grep -c '^<' || true)
    if [ "$touched" = 1 ]; then verdict=applied; else verdict=spread; fi
  fi
  chmod +x "$dir/check.sh" 2>/dev/null
  fails=0
  if [ "$verdict" = applied ]; then
    out="$("$dir/check-test.sh" 2>&1)"
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

echo "check.sh — one guard removed at a time"

run_mutant "direction scan: no redirect to findings"   '/^done >>"\$findings" < <(find "\${direction_targets/s/>>"\$findings" //'
run_mutant "direction scan: guard never fires"         's|^\[ "$was_flavor_scanned" = 1 \]|[ 1 = 1 ]|'
run_mutant "direction scan: flag set from every file"  's/case "\$file" in "\$flavor"\/\*) was_flavor_scanned=1 ;; esac/was_flavor_scanned=1/'
run_mutant "direction scan: no leading-char anchor"    's/\[A-Za-z0-9._~-\]\[A-Za-z0-9._\/~-\]\*\/SKILL/[A-Za-z0-9._\/~-]*\/SKILL/'
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
# `#` as the delimiter wherever the expression holds a pipe. Using `|` there made sed read a malformed
# script once: the mutant failed 19 cases at a stroke and read as a spectacular kill.
run_mutant "findings: tier not ranked within itself"   's#(line ~ /\^syntax: /) return 0#(line ~ /^syntax: /) return 3#'
run_mutant "findings: mounted pattern unanchored"      's|/\^flavor mounted elsewhere/|/flavor mounted elsewhere/|'
run_mutant "findings: did-not-run guard unranked"      '/(line ~ \/\^direction scan read no files\/) return 1/d'
run_mutant "findings: link emitter not one-lined"      's|echo "dangling link: $(oneline "$file") -> $(oneline "$link")"|echo "dangling link: $file -> $link"|'

run_mutant "findings: no per-class cap"                's|shown\[r\] <= 40|shown[r] <= 100000|'
run_mutant "findings: control bytes not stripped"      "s|LC_ALL=C tr '\[:cntrl:\]' ' '|cat|"

report_mutants
