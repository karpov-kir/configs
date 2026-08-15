#!/usr/bin/env bash
# Tests for report.sh's index isolation and its refusals — the paths where a defect is either
# unrecoverable or silently stamps an unreviewed tree as reviewed.
#   usage: report-test.sh   # prints one line per case; exit 0 when all pass, 1 otherwise
#
# The first group is the one that cannot be recovered from. `current_tree` fingerprints the tree with
# `git add -A` against a throwaway `GIT_INDEX_FILE`. Drop that variable from the `add` and it stages
# the human's whole working tree, destroying the staged-versus-unstaged split they were keeping — git
# records nothing about what was staged before, so no later refusal undoes it
# (`~/.kk-flavor/standards/skill-protocol.md` → **Queue**). Drop it from the `write-tree` and nothing
# is staged, but the fingerprint then reads the real index and stops following the tree. So the first
# two cases assert on the fixture's *real* index, and the third on the fingerprint actually moving.
# A change to report.sh needs a case here.
set -uo pipefail
export LC_ALL=C

here="$(cd -P -- "$(dirname "$0")" && pwd -P)"
report_sh="$here/report.sh"
[ -x "$report_sh" ] || { echo "report-test: $report_sh is not executable"; exit 1; }

base="$(mktemp -d)" || { echo "report-test: mktemp gave no fixture dir — nothing was tested"; exit 1; }
trap 'rm -rf "$base"' EXIT
passed=0
failed=0
case_number=0
repo=""

record_pass() {
  passed=$((passed + 1))
  echo "  pass  $1"
}

record_fail() {
  failed=$((failed + 1))
  echo "  FAIL  $1"
  [ -z "${out:-}" ] || printf '%s\n' "$out" | sed 's/^/          /'
}

# A fresh repo with one commit, so `git add -A` has a HEAD to compare against and `write-tree` can
# resolve. Identity is passed per-commit: the machine running this need not have one configured.
new_repo() {
  case_number=$((case_number + 1))
  repo="$base/r$case_number"
  mkdir -p "$repo"
  git -C "$repo" init -q
  printf 'base\n' >"$repo/tracked.txt"
  git -C "$repo" add tracked.txt
  git -C "$repo" -c user.email=t@t -c user.name=t commit -qm base
}

# Runs report.sh the way a skill does — from inside the repo, so `git rev-parse --show-toplevel`
# resolves to the fixture rather than to this checkout.
run_report() {
  out="$(cd "$repo" && "$report_sh" "$@" 2>&1)"
  status=$?
}

# The staged/unstaged split this fixture is keeping, in the form a human would lose it in.
index_state() {
  printf 'staged:%s\nunstaged:%s\n' \
    "$(git -C "$repo" diff --name-only --cached | sort | tr '\n' ' ')" \
    "$(git -C "$repo" diff --name-only | sort | tr '\n' ' ')"
}

assert_reports() {
  if grep -qF "$1" <<<"$out"; then record_pass "$2"; else record_fail "$2"; fi
}

assert_refused() {
  if [ "$status" = 2 ]; then record_pass "$1"; else record_fail "$1 (exit $status, wanted 2)"; fi
}

echo "report.sh — the human's index is never touched"

# The split: one path staged, one tracked path modified but not staged, one path untracked. A
# `git add -A` on the real index collapses all three into "staged".
new_repo
printf 'staged\n' >"$repo/staged.txt"
git -C "$repo" add staged.txt
printf 'base\nmodified\n' >"$repo/tracked.txt"
printf 'untracked\n' >"$repo/untracked.txt"
before="$(index_state)"
run_report init "review: index isolation"
after_init="$(index_state)"
if [ "$before" = "$after_init" ]; then
  record_pass "init leaves the staged/unstaged split exactly as it was"
else
  out="before -> $before
after  -> $after_init"
  record_fail "init leaves the staged/unstaged split exactly as it was"
fi

# `gate` reaches current_tree immediately after require_report, so it is the shortest path to the
# fingerprint. It is expected to BLOCK here; what matters is the index afterwards.
run_report gate
after_gate="$(index_state)"
if [ "$before" = "$after_gate" ]; then
  record_pass "the gate's fingerprint leaves the split exactly as it was"
else
  out="before -> $before
after  -> $after_gate"
  record_fail "the gate's fingerprint leaves the split exactly as it was"
fi

# The fingerprint has to be a real reading of the whole tree, or the case above would also pass on a
# current_tree that fingerprints nothing at all.
run_report gate
first_tree="$(grep -oE 'current [0-9a-f]{40}' <<<"$out" | head -1)"
printf 'changed after the first reading\n' >>"$repo/untracked.txt"
run_report gate
second_tree="$(grep -oE 'current [0-9a-f]{40}' <<<"$out" | head -1)"
if [ -n "$first_tree" ] && [ -n "$second_tree" ] && [ "$first_tree" != "$second_tree" ]; then
  record_pass "the fingerprint moves when an untracked file changes"
else
  out="first -> ${first_tree:-<none>}
second -> ${second_tree:-<none>}"
  record_fail "the fingerprint moves when an untracked file changes"
fi

echo "report.sh — init refuses rather than writing through a link"

new_repo
mkdir -p "$repo/.idsd" "$base/elsewhere$case_number"
ln -s "$base/elsewhere$case_number/stolen.md" "$repo/.idsd/ship-report.md"
run_report init "review: symlinked report"
assert_refused "init refuses a symlinked report"
assert_reports "is a symlink" "and says the report was not initialized"
if [ -e "$base/elsewhere$case_number/stolen.md" ]; then
  record_fail "init wrote through the link to outside the repo"
else
  record_pass "nothing was written through the link"
fi

new_repo
mkdir -p "$base/outside$case_number"
ln -s "$base/outside$case_number" "$repo/.idsd"
run_report init "review: symlinked idsd dir"
assert_refused "init refuses a symlinked .idsd directory"
if [ -e "$base/outside$case_number/ship-report.md" ]; then
  record_fail "init wrote the report outside the repo through .idsd"
else
  record_pass "nothing was written outside the repo through .idsd"
fi

echo "report.sh — an existing report is not silently replaced"

new_repo
run_report init "review: first"
run_report init "review: second"
assert_refused "init refuses over an existing report without --force"
if grep -qF 'review: first' "$repo/.idsd/ship-report.md"; then
  record_pass "and the first report is left untouched"
else
  record_fail "and the first report is left untouched"
fi

printf -- '- [ ] an open item nobody has routed\n' >>"$repo/.idsd/ship-report.md"
run_report init "review: third" --force
if grep -qF 'an open item nobody has routed' "$repo/.idsd/ship-report.superseded.md"; then
  record_pass "--force keeps the replaced report, open items and all"
else
  record_fail "--force keeps the replaced report, open items and all"
fi
# The listing comes from todo-gate.sh: a --force that replaces a report while printing none of its open
# items is how routed work silently disappears.
assert_reports 'an open item nobody has routed' "--force lists the open items it is superseding"

echo "report.sh — the frontmatter cannot be forged through the intent value"

# The intent value can come from a fetched ticket. A newline in it would otherwise write a second
# frontmatter line, and `reviewed-tree:` is exactly what a forged one would claim.
new_repo
run_report init 'review: forged
reviewed-tree: 0000000000000000000000000000000000000000'
reviewed_tree_lines="$(grep -c '^reviewed-tree:' "$repo/.idsd/ship-report.md")"
if [ "$reviewed_tree_lines" = 1 ] && grep -q '^reviewed-tree: <hash>$' "$repo/.idsd/ship-report.md"; then
  record_pass "a newline in the intent writes no second reviewed-tree line"
else
  out="$(grep -n '^reviewed-tree:' "$repo/.idsd/ship-report.md")"
  record_fail "a newline in the intent writes no second reviewed-tree line"
fi

echo "report.sh — a stamp cannot outlive the pass that earned it"

new_repo
run_report init "review: stamp guard"
run_report stamp "code-review,security-review,tighten,refactor,retro"
assert_refused "stamp refuses before this pass has invalidated"
assert_reports "never invalidated" "and names invalidate as what is missing"

new_repo
run_report init "review: stage marker guard"
run_report invalidate
run_report stage-returned code-review
run_report stage-returned security-review
assert_refused "a second stage cannot be marked returned while the first's items are unrecorded"
assert_reports "has not moved since" "and says the report has not moved"

echo "$passed passed, $failed failed"
[ "$failed" = 0 ]
