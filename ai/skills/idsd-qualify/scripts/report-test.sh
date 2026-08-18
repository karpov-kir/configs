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

# One report per intent, under qualify-reports/, named after the intent slug. A standalone `review: …`
# has no slug and shares the one `review` stem, which is what most fixtures below use.
report_path() {
  printf '%s\n' "$repo/.idsd/qualify-reports/${1:-review}-qualify-report.md"
}

# A copy of the whole skill dir — scripts beside templates, the layout report.sh derives its template and
# todo-gate paths from — so a case can break either without editing this checkout's own. One copy per
# case, so no mutation carries.
new_skill_copy() {
  local copy="$base/skill$case_number"
  mkdir -p "$copy/scripts" "$copy/templates"
  cp "$here/report.sh" "$here/todo-gate.sh" "$copy/scripts/"
  cp "$here/../templates/qualify-report-template.md" "$copy/templates/"
  chmod +x "$copy/scripts/report.sh" "$copy/scripts/todo-gate.sh"
  copied_report_sh="$copy/scripts/report.sh"
  copied_template="$copy/templates/qualify-report-template.md"
  copied_todo_gate="$copy/scripts/todo-gate.sh"
}

# Drive a ship to a stamped, tree-fresh state — what state_token must reach before it reads anything past
# the freshness checks; unstamped, it answers `resume` and stops there. $1 is the runner, so a case on a
# broken skill copy uses the same sequence as one on this checkout's own script.
stamp_full_pass() {
  local runner="$1" ship="$2" stage
  "$runner" invalidate "$ship"
  for stage in code-review security-review tighten refactor retro; do
    "$runner" stage-returned "$stage" "$ship"
    "$runner" no-items "$stage" "$ship"
  done
  "$runner" stamp "code-review,security-review,tighten,refactor,retro" "$ship"
}

# run_report against the copy new_skill_copy made, so the template under test is the copied one.
run_copied_report() {
  out="$(cd "$repo" && "$copied_report_sh" "$@" 2>&1)"
  status=$?
}

# Take read permission off a fixture file for the cases that need one. Non-zero means chmod did not
# restrict this user — root reads anything — so the case is skipped by name rather than failed. Restore
# the file with `chmod 644` afterwards, so the fixture teardown can remove it.
made_unreadable() {
  chmod 000 "$1"
  [ -r "$1" ] || return 0
  echo "  skip  chmod does not restrict this user (root?) — $2 cannot run"
  return 1
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
mkdir -p "$repo/.idsd/qualify-reports" "$base/elsewhere$case_number"
ln -s "$base/elsewhere$case_number/stolen.md" "$(report_path)"
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
if [ -e "$base/outside$case_number/qualify-reports" ]; then
  record_fail "init wrote the report outside the repo through .idsd"
else
  record_pass "nothing was written outside the repo through .idsd"
fi

# qualify-reports/ is the second directory every write goes through, and `-L` on the report file tests
# only its own final component — so the dir needs its own check or every write lands wherever it points.
new_repo
mkdir -p "$repo/.idsd" "$base/outside-reports$case_number"
ln -s "$base/outside-reports$case_number" "$repo/.idsd/qualify-reports"
run_report init "review: symlinked reports dir"
assert_refused "init refuses a symlinked qualify-reports directory"
if [ -e "$base/outside-reports$case_number/review-qualify-report.md" ]; then
  record_fail "init wrote the report outside the repo through qualify-reports"
else
  record_pass "nothing was written outside the repo through qualify-reports"
fi

echo "report.sh — an intent value cannot name a file outside qualify-reports/"

# The report's filename is built from the intent, and an intent can be seeded from a fetched ticket.
new_repo
run_report init "../../escaped"
assert_refused "init refuses an intent whose charset could escape the directory"
# The escape lands at $repo/escaped-qualify-report.md — two levels up from qualify-reports/, not at $base.
# Asserting on $base and $repo/.. (the same directory) let a widened charset pass this case.
if [ -e "$repo/escaped-qualify-report.md" ] || [ -e "$repo/.idsd/escaped-qualify-report.md" ] ||
  [ -e "$base/escaped-qualify-report.md" ]; then
  out="$(find "$base" -name 'escaped*' 2>/dev/null)"
  record_fail "no report was written outside qualify-reports/"
else
  record_pass "no report was written outside qualify-reports/"
fi

# `*` never matches a leading dot, so a dot-named report is created, addressable by name, and invisible
# to report_names — `list` says "no reports" and `state` says "no-report" while it stands open.
new_repo
for dot_intent in .. . .hidden; do
  run_report init "$dot_intent"
  assert_refused "init refuses the intent '$dot_intent', which no listing could ever see"
done
run_report list
if grep -q '^no reports$' <<<"$out" && [ -z "$(ls -A "$repo/.idsd/qualify-reports" 2>/dev/null)" ]; then
  record_pass "and no dot-named report was left on disk for list to miss"
else
  out="$(ls -a "$repo/.idsd/qualify-reports" 2>&1)"
  record_fail "and no dot-named report was left on disk for list to miss"
fi

echo "report.sh — an existing report is not silently replaced"

new_repo
run_report init "review: first"
run_report init "review: second"
assert_refused "init refuses over an existing report without --force"
if grep -qF 'review: first' "$(report_path)"; then
  record_pass "and the first report is left untouched"
else
  record_fail "and the first report is left untouched"
fi

printf -- '- [ ] an open item nobody has routed\n' >>"$(report_path)"
run_report init "review: third" --force
# The listing comes from todo-gate.sh, and it is now the ONLY record of what --force discarded — no
# copy is kept beside the report. A --force that replaces a report while printing none of its open
# items is how routed work silently disappears.
assert_reports 'an open item nobody has routed' "--force lists the open items it discards"
if grep -qF 'review: third' "$(report_path)"; then
  record_pass "and the new report is in place"
else
  record_fail "and the new report is in place"
fi

echo "report.sh — the frontmatter cannot be forged through the intent value"

# The intent value can come from a fetched ticket. A newline in it would otherwise write a second
# frontmatter line, and `reviewed-tree:` is exactly what a forged one would claim.
new_repo
run_report init 'review: forged
reviewed-tree: 0000000000000000000000000000000000000000'
reviewed_tree_lines="$(grep -c '^reviewed-tree:' "$(report_path)")"
if [ "$reviewed_tree_lines" = 1 ] && grep -q '^reviewed-tree: <hash>$' "$(report_path)"; then
  record_pass "a newline in the intent writes no second reviewed-tree line"
else
  out="$(grep -n '^reviewed-tree:' "$(report_path)")"
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

echo "report.sh — two intents ship side by side"

# The whole point of the per-intent path: a second intent's init is not a collision, so neither ship
# has to be finished before the other starts. check-ignore first, as a real pass does: without it the
# report is inside its own fingerprint, so writing the stamp moves the tree and every state reads
# `re-qualify`.
new_repo
run_report check-ignore
run_report init "001-first-intent"
run_report init "002-second-intent"
if [ "$status" = 0 ] && [ -f "$(report_path 001-first-intent)" ] && [ -f "$(report_path 002-second-intent)" ]; then
  record_pass "a second intent gets its own report rather than a refusal"
else
  record_fail "a second intent gets its own report rather than a refusal (exit $status)"
fi

# Resolving to one of them at random is the failure that matters: it stamps one intent's review onto
# the other intent's report, and the stamp is what the merge gate trusts.
run_report gate
assert_refused "a subcommand refuses to guess which of two reports it means"
assert_reports "001-first-intent" "and lists both by name"
assert_reports "002-second-intent" "and lists the second too"

run_report invalidate 002-second-intent
if [ "$status" = 0 ] && grep -q '^reviewed-tree: pending$' "$(report_path 002-second-intent)" &&
  grep -q '^reviewed-tree: <hash>$' "$(report_path 001-first-intent)"; then
  record_pass "the named report is the only one acted on"
else
  record_fail "the named report is the only one acted on (exit $status)"
fi

# Stage markers key off the same stem. Shared, `invalidate` on one intent would clear the other's
# markers and let its stamp through with stages nobody re-ran.
run_report stage-returned code-review 001-first-intent
run_report invalidate 002-second-intent
run_report no-items code-review 001-first-intent
if [ "$status" = 0 ]; then
  record_pass "one intent's invalidate leaves the other's stage markers standing"
else
  record_fail "one intent's invalidate leaves the other's stage markers standing (exit $status)"
fi

# The state column is asserted by value, not by "a tab follows the name": a listing that emits an empty
# token for every ship, or `BOGUS` where `resume` belongs, satisfied the looser form — and `list` is the
# surface `idsd-ship continue` routes on with several ships in flight.
run_report list
first_state="$(grep '^001-first-intent[[:space:]]' <<<"$out" | cut -f2)"
second_state="$(grep '^002-second-intent[[:space:]]' <<<"$out" | cut -f2)"
if [ "$first_state" = resume ] && [ "$second_state" = resume ]; then
  record_pass "list prints one line per open ship, each with its state"
else
  out="001 -> '${first_state:-<empty>}', 002 -> '${second_state:-<empty>}'"
  record_fail "list prints one line per open ship, each with its state"
fi

# And the states must be the ships' own, not one ship's answer repeated: stamping only 001 must move
# only 001's column.
stamp_full_pass run_report 001-first-intent
run_report list
if [ "$(grep '^001-first-intent[[:space:]]' <<<"$out" | cut -f2)" = ready ] &&
  [ "$(grep '^002-second-intent[[:space:]]' <<<"$out" | cut -f2)" = resume ]; then
  record_pass "and each state is that ship's own, not one answer repeated"
else
  record_fail "and each state is that ship's own, not one answer repeated"
fi

echo "report.sh — close retires one ship's scratch and nothing else"

new_repo
run_report init "001-closing"
run_report init "002-staying"
printf -- '- [ ] a decision nobody routed\n' >>"$(report_path 001-closing)"
run_report close 001-closing
assert_refused "close refuses while an open '- [ ]' stands"
assert_reports 'a decision nobody routed' "and names the item it would have discarded"
if [ -f "$(report_path 001-closing)" ]; then
  record_pass "and the report is still there"
else
  record_fail "and the report is still there"
fi

run_report close 001-closing --force
if [ "$status" = 0 ] && [ ! -f "$(report_path 001-closing)" ] && [ -f "$(report_path 002-staying)" ]; then
  record_pass "--force closes the named report and leaves the sibling ship alone"
else
  record_fail "--force closes the named report and leaves the sibling ship alone (exit $status)"
fi

# `--force` shares its charset with a legal slug, so read positionally it resolves as an intent name
# and closes a report that does not exist — reporting success while the real one stands.
new_repo
run_report init "review: force alone"
run_report close --force
if [ "$status" = 0 ] && [ ! -f "$(report_path)" ]; then
  record_pass "close reads --force as a flag, not as the intent name"
else
  record_fail "close reads --force as a flag, not as the intent name (exit $status)"
fi

echo "report.sh — check-ignore holds before qualify-reports/ exists"

# The documented first step: `check-ignore` runs before the first write into `.idsd/`, and its exit 1
# blocks that write. So it has to answer correctly while qualify-reports/ does not exist yet — measured,
# `git check-ignore -q .idsd/qualify-reports` exits 1 without the trailing slash and 0 with it, so the
# slash in ignore_surface is load-bearing exactly here. The suite otherwise only reached this branch
# after promote, when the directory exists and both entry forms match.
new_repo
printf '.idsd/qualify-reports/\n' >"$repo/.gitignore"
mkdir -p "$repo/.idsd"
printf '# durable\n' >"$repo/.idsd/charter.md"
git -C "$repo" add .gitignore .idsd/charter.md
git -C "$repo" -c user.email=t@t -c user.name=t commit -qm "committed idsd"
if [ ! -d "$repo/.idsd/qualify-reports" ] && [ "$(cd "$repo" && "$report_sh" repo-mode)" = committed ]; then
  run_report check-ignore
  if [ "$status" = 0 ]; then
    record_pass "check-ignore passes in committed mode before qualify-reports/ is created"
  else
    record_fail "check-ignore passes in committed mode before qualify-reports/ is created (exit $status)"
  fi
else
  record_fail "fixture is not a committed repo with qualify-reports/ absent"
fi

echo "report.sh — close on a clean report, the path 'done' runs"

# Both close success cases passed --force, so the unforced path — the one idsd-ship done actually
# invokes, on a report whose items are all cleared — was never exercised.
new_repo
run_report init "001-landed"
run_report close 001-landed
if [ "$status" = 0 ] && [ ! -f "$(report_path 001-landed)" ]; then
  record_pass "close needs no --force once nothing is open"
else
  record_fail "close needs no --force once nothing is open (exit $status)"
fi

# `close` retires a landed ship's report, and the archived intent file is then the only record it
# landed. Read absence alone and `state` answers `no-report`, which `idsd-ship continue` routes to
# "start ship <intent>" — rebuilding work already merged.
new_repo
run_report check-ignore
run_report init "001-landed-and-archived"
mkdir -p "$repo/.idsd/archive"
printf '# built and archived\n' >"$repo/.idsd/archive/001-landed-and-archived.md"
run_report close 001-landed-and-archived
run_report state 001-landed-and-archived
if [ "$out" = done ]; then
  record_pass "state answers done for a closed report whose intent is archived"
else
  record_fail "state answers done for a closed report whose intent is archived (said '$out')"
fi

# And an intent that was never archived still reads no-report once closed — the archive is the fact
# being read, not the closing.
run_report init "002-closed-unbuilt"
run_report close 002-closed-unbuilt
run_report state 002-closed-unbuilt
if [ "$out" = no-report ]; then
  record_pass "and no-report still answers for a closed report with no archived intent"
else
  record_fail "and no-report still answers for a closed report with no archived intent (said '$out')"
fi

echo "report.sh — an unreadable report is not a state"

# Measured, not assumed: every frontmatter reader greps with `2>/dev/null`, and an empty answer is in
# unstamped()'s set — so before the readability check, `state` answered `resume` for a report it never
# opened, and `idsd-ship continue` would rebuild the intent on that.
new_repo
run_report check-ignore
run_report init "001-unreadable"
run_report init "002-readable"
if made_unreadable "$(report_path 001-unreadable)" "the unreadable-report cases"; then
  run_report state 001-unreadable
  assert_refused "state refuses a report it cannot read rather than answering resume"

  # And the listing is all-or-nothing: half a listing reads exactly like a complete one.
  run_report list
  if [ "$status" != 0 ] && ! grep -q '002-readable' <<<"$out"; then
    record_pass "list prints no ship's state when one report cannot be read"
  else
    record_fail "list prints no ship's state when one report cannot be read (exit $status)"
  fi
fi
chmod 644 "$(report_path 001-unreadable)"

# The same invariant with the unreadable report ordered SECOND — which is the only order that pins the
# buffering. Reached first, nothing is printed whether the listing is buffered or streamed.
new_repo
run_report check-ignore
run_report init "001-readable-first"
run_report init "002-unreadable-second"
if made_unreadable "$(report_path 002-unreadable-second)" "the buffering case"; then
  run_report list
  if [ "$status" != 0 ] && ! grep -q '001-readable-first' <<<"$out"; then
    record_pass "list buffers, so a ship reached before the refusal is not printed either"
  else
    record_fail "list buffers, so a ship reached before the refusal is not printed either (exit $status)"
  fi

  # `carry` is where an unread report loses work silently: it prints the open items a re-qualify must
  # keep, and an unreadable one prints none — which reads exactly like a report with nothing open. This
  # refuses either way, though: with require_report's readability check gone, the scan's own failure to
  # read the report refuses in its place. The discard case below is what pins that check.
  run_report carry 002-unreadable-second
  assert_refused "carry refuses a report it cannot read rather than reporting no open items"
  # The message, not just the exit: with require_report's readability refusal deleted, `carry` still
  # exits 2 — read_open_todos refuses in its place, because todo-gate.sh cannot read the file either.
  # Only this assertion tells the two refusals apart, so only this one pins the earlier guard.
  assert_reports "its state is unknown" "and it is that guard refusing, not the scan failing behind it"
  run_report carry 001-readable-first
  if [ "$status" = 0 ]; then
    record_pass "and a readable sibling still carries normally"
  else
    record_fail "and a readable sibling still carries normally (exit $status)"
  fi
fi
chmod 644 "$(report_path 002-unreadable-second)"

echo "report.sh — discard removes nothing it could not read"

# `discard` rm -rf's .idsd/ and deletes the intent file, and it reads the slug to decide which. An
# unreadable report yields no slug, so without the readability refusal it deletes the report and
# orphans the intent: `state` then answers no-report for work that was never built.
new_repo
run_report init "001-discarding"
mkdir -p "$repo/.idsd/intents"
printf '# intent\n' >"$repo/.idsd/intents/001-discarding.md"
if made_unreadable "$(report_path 001-discarding)" "the discard case"; then
  run_report discard 001-discarding
  assert_refused "discard refuses a report it cannot read"
  if [ -f "$(report_path 001-discarding)" ] && [ -f "$repo/.idsd/intents/001-discarding.md" ]; then
    record_pass "and removed neither the report nor the intent file"
  else
    record_fail "and removed neither the report nor the intent file"
  fi
fi
chmod 644 "$(report_path 001-discarding)"

echo "report.sh — promote reports the mode, not the add"

# Measured: qualify-reports/ is ignored by the entry promote itself writes, and `git add` on a directory
# whose every file is ignored stages nothing and exits 0. With nothing else under .idsd/, promote used
# to print success while repo-mode still said throwaway — and the next check-ignore re-excluded .idsd/,
# silently undoing it.
new_repo
run_report check-ignore
run_report init "001-nothing-durable"
run_report promote
assert_refused "promote refuses when everything under .idsd/ is ignored"
if [ "$(cd "$repo" && "$report_sh" repo-mode)" = throwaway ]; then
  record_pass "and the repo is still a throwaway, as the refusal says"
else
  record_fail "and the repo is still a throwaway, as the refusal says"
fi
# Read straight from the exclude file: routing this through check-ignore would pass either way, since
# check-ignore re-adds the entry itself in throwaway mode.
if grep -qxF '.idsd/' "$repo/.git/info/exclude" 2>/dev/null; then
  record_pass "and promote put the local exclusion back, so nothing scratch can be staged"
else
  record_fail "and promote put the local exclusion back, so nothing scratch can be staged"
fi

# The same repo with one durable file: promote now has something to stage, and the mode flips.
printf '# intent\n' >"$repo/.idsd/intents-placeholder.md"
run_report promote
if [ "$status" = 0 ] && [ "$(cd "$repo" && "$report_sh" repo-mode)" = committed ]; then
  record_pass "promote succeeds once .idsd/ holds something that is not ignored"
else
  record_fail "promote succeeds once .idsd/ holds something that is not ignored (exit $status)"
fi

# Committed mode takes the other check-ignore branch entirely — the one that asks git rather than
# writing an exclusion, and the only branch that can report the report as committable.
run_report check-ignore
if [ "$status" = 0 ] && grep -q 'gitignored' <<<"$out"; then
  record_pass "committed mode confirms qualify-reports/ is gitignored"
else
  record_fail "committed mode confirms qualify-reports/ is gitignored (exit $status)"
fi

# And the warning fires when it is not: the entry is what keeps a report out of `git add -A`.
printf '' >"$repo/.gitignore"
run_report check-ignore
if [ "$status" = 1 ] && grep -q 'NOT gitignored' <<<"$out"; then
  record_pass "and warns when the entry is missing"
else
  record_fail "and warns when the entry is missing (exit $status)"
fi

echo "report.sh — the filename and the frontmatter name the same ship"

# A value beginning with whitespace truncated to nothing and took the `review` arm, while intent_slug
# sed-stripped the same whitespace and recovered the real slug. Measured consequences: one intent with
# two reports so the ambiguity refusal never fires, `discard` deleting another ship's in-flight intent,
# and `state` answering `done` for an open one.
new_repo
run_report check-ignore
run_report init "  002-spaced"
if [ -f "$(report_path 002-spaced)" ] && [ ! -f "$(report_path review)" ] &&
  grep -qx 'intent: 002-spaced' "$(report_path 002-spaced)"; then
  record_pass "a whitespace-led intent is filed and recorded under the same slug"
else
  out="$(ls "$repo/.idsd/qualify-reports"; grep -h '^intent:' "$repo"/.idsd/qualify-reports/*.md)"
  record_fail "a whitespace-led intent is filed and recorded under the same slug"
fi
run_report state 002-spaced
if [ "$out" = resume ]; then
  record_pass "and it is addressable by the slug it recorded"
else
  record_fail "and it is addressable by the slug it recorded (said '$out')"
fi

# Whitespace-only never reaches the filesystem: it would pass the emptiness guard and scaffold a report
# whose blank `intent:` every reader treats as a standalone review.
new_repo
run_report init " "
assert_refused "init refuses a whitespace-only intent"
if [ -z "$(ls -A "$repo/.idsd/qualify-reports" 2>/dev/null)" ]; then
  record_pass "and wrote no report for it"
else
  record_fail "and wrote no report for it"
fi

echo "report.sh — discard reconciles the two names before deleting anything"

# The destructive path: discard deletes the intent file the FRONTMATTER names, having been addressed by
# the FILENAME. Disagreeing, it deletes another ship's in-flight intent, which throwaway mode keeps no
# copy of anywhere.
new_repo
run_report check-ignore
run_report init "001-mine"
mkdir -p "$repo/.idsd/intents"
printf '# in flight\n' >"$repo/.idsd/intents/002-yours.md"
printf '# mine\n' >"$repo/.idsd/intents/001-mine.md"
sed -i.bak 's/^intent: 001-mine$/intent: 002-yours/' "$(report_path 001-mine)" && rm -f "$(report_path 001-mine).bak"
run_report discard 001-mine
assert_refused "discard refuses when the filename and the frontmatter name different ships"
if [ -f "$repo/.idsd/intents/002-yours.md" ] && [ -f "$repo/.idsd/intents/001-mine.md" ]; then
  record_pass "and deleted neither intent file"
else
  record_fail "and deleted neither intent file"
fi

echo "report.sh — list walks the tree once, and never streams a partial answer"

# Three STAMPED reports, so state_token reaches the fingerprint for each. state_token runs inside a
# command substitution, so a cache it filled there dies with that subshell: every ship would walk the
# tree again, and two ships could then be scored against different trees.
new_repo
run_report check-ignore
for ship in 001-a 002-b 003-c; do
  run_report init "$ship"
  stamp_full_pass run_report "$ship"
done
# Counted through a `git` shim on PATH, not from a `bash -x` trace: the priming call redirects its own
# stderr, which is where `set -x` writes, so a trace cannot see the very call this pins.
shim_dir="$base/gitshim$case_number"
mkdir -p "$shim_dir"
real_git="$(command -v git)"
{
  echo '#!/bin/sh'
  echo "printf '%s\\n' \"\$*\" >>\"$shim_dir/calls.log\""
  echo "exec \"$real_git\" \"\$@\""
} >"$shim_dir/git"
chmod +x "$shim_dir/git"
(cd "$repo" && PATH="$shim_dir:$PATH" "$report_sh" list >/dev/null 2>&1)
tree_walks="$(grep -c 'write-tree' "$shim_dir/calls.log" 2>/dev/null || echo 0)"
if [ "$tree_walks" = 1 ]; then
  record_pass "list fingerprints the tree once for every ship it lists"
else
  out="write-tree calls: $tree_walks (wanted 1)"
  record_fail "list fingerprints the tree once for every ship it lists"
fi

# Priming must not be fatal on its own: an unstamped ship answers without the tree at all, so a tree
# that cannot be fingerprinted must not silence a listing that never needed it.
new_repo
run_report check-ignore
run_report init "001-unstamped"
run_report init "002-unstamped"
printf 'unreadable\n' >"$repo/blocker.txt"
if made_unreadable "$repo/blocker.txt" "the priming case"; then
  run_report list
  if [ "$status" = 0 ] && grep -q '^001-unstamped	resume$' <<<"$out" && grep -q '^002-unstamped	resume$' <<<"$out"; then
    record_pass "an unfingerprintable tree does not silence a listing of ships that never needed it"
  else
    record_fail "an unfingerprintable tree does not silence a listing of ships that never needed it (exit $status)"
  fi
fi
chmod 644 "$repo/blocker.txt"

echo "report.sh — the pre-scoping path is reported, never passed over in silence"

# The path move's one real harm is silence: a repo mid-ship when this script changed has its report at
# the old root path, where nothing looks, and `state` answering no-report routes a fresh ship over it.
new_repo
mkdir -p "$repo/.idsd"
printf -- '---\nintent: 002-live\n---\n\n# Decide\n\n- [ ] a live decision\n' >"$repo/.idsd/ship-report.md"
# The pre-rename directory is the second historical path, and it must be named too — a repo mid-ship
# across the rename has its report there, and it is not `ship-report.md`.
mkdir -p "$repo/.idsd/ship-reports"
printf -- '---\nintent: 003-live\n---\n' >"$repo/.idsd/ship-reports/003-live-ship-report.md"
run_report state
if [ "$out" = "no-report" ]; then
  record_fail "state names the pre-scoping report rather than answering a bare no-report"
else
  record_pass "state names the pre-scoping report rather than answering a bare no-report"
fi
run_report list
assert_reports "ship-report.md" "list names the pre-scoping report it cannot see"
assert_reports "ship-reports" "and names the pre-rename directory too"
run_report gate 002-live
assert_reports "ship-report.md" "and so does a refusal for a named report that is not there"

echo "report.sh — init's staged write is not a way out of the repo"

# The staged `cp` writes to `$report.new`, a path the symlink guard chain did not cover — a link planted
# there (committable, so it arrives through someone else's branch) made `cp` overwrite the target and
# `init` report success.
new_repo
mkdir -p "$repo/.idsd/qualify-reports" "$base/victim$case_number"
printf 'PRECIOUS\n' >"$base/victim$case_number/keep.md"
ln -s "$base/victim$case_number/keep.md" "$(report_path review).new"
run_report init "review: staged write"
if [ "$(head -1 "$base/victim$case_number/keep.md")" = PRECIOUS ]; then
  record_pass "init writes no template through a link planted at the staged path"
else
  record_fail "init writes no template through a link planted at the staged path"
fi
if [ -f "$(report_path)" ] && grep -qF 'review: staged write' "$(report_path)"; then
  record_pass "and still initialized the report itself"
else
  record_fail "and still initialized the report itself"
fi

echo "report.sh — a drifted template is refused before any report is scaffolded"

# gate and state never read the template, so init's checks on it are the only thing between a drifted
# template and a new report whose frontmatter already reads as reviewed.

# The fixture first: an untouched copy must initialize. Without this, a copy report.sh cannot read at all
# would refuse every case below, and each refusal would pass for the wrong reason.
new_repo
new_skill_copy
run_copied_report init "001-intact-copy"
if [ "$status" = 0 ] && [ -f "$(report_path 001-intact-copy)" ]; then
  record_pass "the copied skill dir initializes a report from its own template"
else
  record_fail "the copied skill dir initializes a report from its own template (exit $status)"
fi

# The drift that gates clean: a placeholder outside unstamped()'s set puts a real-looking reviewed-tree in
# every new report, and `gate` reads that as a completed review of whatever tree happens to match.
new_repo
new_skill_copy
sed -i.bak 's/^reviewed-tree:.*/reviewed-tree: 1111111111111111111111111111111111111111/' "$copied_template" &&
  rm -f "$copied_template.bak"
run_copied_report init "001-drifted-placeholder"
assert_refused "init refuses a template whose reviewed-tree placeholder reads as a completed review"
assert_reports "would gate clean" "and says every new report would gate clean"
if [ ! -e "$(report_path 001-drifted-placeholder)" ]; then
  record_pass "and scaffolded no report from the drifted template"
else
  record_fail "and scaffolded no report from the drifted template"
fi

# A field the template stopped carrying: init stamps `intent:`, and gate and state read the other two, so
# a report scaffolded without one answers from a line that is not there.
for missing_field in intent reviewed-tree reviewed-stages; do
  new_repo
  new_skill_copy
  grep -v "^$missing_field:" "$copied_template" >"$copied_template.stripped" &&
    mv "$copied_template.stripped" "$copied_template"
  run_copied_report init "001-missing-$missing_field"
  assert_refused "init refuses a template with no '$missing_field:' line"
  assert_reports "has no '$missing_field:' line" "and names the missing '$missing_field:' line"
  if [ ! -e "$(report_path "001-missing-$missing_field")" ]; then
    record_pass "and scaffolded no report while '$missing_field:' is missing"
  else
    record_fail "and scaffolded no report while '$missing_field:' is missing"
  fi
done

# The template is read and never written, so what the symlink guard stops is content from outside the repo
# becoming a new report's frontmatter — a committed link arrives through someone else's branch, and a
# forged `reviewed-tree` is what it would carry in.
new_repo
new_skill_copy
mkdir -p "$base/foreign$case_number"
# A plausible template, placeholders included, so every OTHER check on it passes and the symlink guard is
# the only thing left between the link and a scaffolded report. A stub here would be refused for its
# missing `intent:` line instead, and three of the four assertions below could no longer fail.
printf -- '---\nintent: 002-attacker\nreviewed-tree: <hash>\nreviewed-stages: <stages>\n---\n\n# Decide\n\nSMUGGLED\n' \
  >"$base/foreign$case_number/outside.md"
rm -f "$copied_template"
ln -s "$base/foreign$case_number/outside.md" "$copied_template"
run_copied_report init "001-linked-template"
assert_refused "init refuses a template that is a symlink"
assert_reports "is a symlink" "and says it will not read the template through one"
if [ ! -e "$(report_path 001-linked-template)" ]; then
  record_pass "and scaffolded no report from the linked template"
else
  record_fail "and scaffolded no report from the linked template"
fi
# The write that could already have landed is the report, not the link's target: init only ever reads the
# template. So this assertion is the one that names the harm — content from outside the repo reaching
# .idsd/ — rather than the shape of the refusal.
if ! grep -rqF SMUGGLED "$repo/.idsd" 2>/dev/null; then
  record_pass "and no content from outside the repo reached .idsd/"
else
  out="$(grep -rlF SMUGGLED "$repo/.idsd" 2>/dev/null)"
  record_fail "and no content from outside the repo reached .idsd/"
fi

# A template that is gone. Without this refusal the `intent:` guard fires on grep's own failure to open the
# file, and the message stops naming the real cause — so the message is what pins this one.
new_repo
new_skill_copy
rm -f "$copied_template"
run_copied_report init "001-absent-template"
assert_refused "init refuses when the template is missing"
assert_reports "template not found" "and names the missing template as the cause"
if [ ! -e "$(report_path 001-absent-template)" ]; then
  record_pass "and scaffolded no report without a template"
else
  record_fail "and scaffolded no report without a template"
fi

echo "report.sh — a scan that did not run is never read as 'nothing open'"

# todo-gate.sh exits 0 with nothing open and 1 with the items printed, so anything above that is the scan
# failing to run. Read as "nothing open", it lets a report still holding unrouted `- [ ]` pass the merge
# gate. `state`, `carry` and `close` share one reader, and the point of sharing it is that they cannot
# drift apart, so all three are asserted here against the same broken gate.
new_repo
new_skill_copy
run_copied_report check-ignore
run_copied_report init "001-scan-fails"
stamp_full_pass run_copied_report 001-scan-fails
# The positive control, while the scan still works: this fixture reaches the open-item scan and answers
# `ready`. Without it, every refusal below could belong to an earlier guard and pin nothing.
run_copied_report state 001-scan-fails
if [ "$out" = ready ]; then
  record_pass "the fixture reaches the open-item scan while the scan still works"
else
  record_fail "the fixture reaches the open-item scan while the scan still works (said '$out')"
fi
printf '#!/bin/sh\nexit 3\n' >"$copied_todo_gate"
chmod +x "$copied_todo_gate"
for scan_reader in state carry close; do
  run_copied_report "$scan_reader" 001-scan-fails
  assert_refused "$scan_reader refuses when the open-item scan exits 3"
  assert_reports "todo-gate.sh exited 3" "and $scan_reader names the exit rather than reporting nothing open"
done
# close is the destructive one: a scan it could not run must leave the report where it is.
if [ -f "$(report_path 001-scan-fails)" ]; then
  record_pass "and close retired nothing on a scan it could not run"
else
  record_fail "and close retired nothing on a scan it could not run"
fi

echo "report.sh — a stage name that is not a stage is refused"

# What these pin is the refusal itself: the usage line, and no marker written. That it runs in the
# caller's own shell is NOT pinned here — through `$( )` with the exit status unrelayed, a downstream
# guard refuses anyway, so the suite cannot tell the two shapes apart.
new_repo
run_report init "001-stage-names"
stage_markers="$repo/.git/idsd-stage-returns/001-stage-names"
# An omitted stage name and an empty one reach `${2:-}` by the same path, so the empty form covers both; a
# separate case for the omitted one could not fail differently.
for bad_stage in bogus ""; do
  for stage_subcommand in stage-returned no-items; do
    run_report "$stage_subcommand" "$bad_stage"
    assert_refused "$stage_subcommand refuses the stage name '$bad_stage'"
    assert_reports "usage: report.sh $stage_subcommand" "and prints the stage vocabulary for '$bad_stage'"
  done
done
if [ -z "$(ls -A "$stage_markers" 2>/dev/null)" ]; then
  record_pass "and no stage marker was written for any of them"
else
  out="$(ls -A "$stage_markers")"
  record_fail "and no stage marker was written for any of them"
fi
# Last, so the marker check above cannot pass on a fixture where nothing was markable in the first place.
run_report stage-returned code-review
if [ "$status" = 0 ] && [ -f "$stage_markers/code-review" ]; then
  record_pass "while a real stage name on the same fixture is recorded"
else
  record_fail "while a real stage name on the same fixture is recorded (exit $status)"
fi

echo "report.sh — no refusal leaves .idsd/ exposed to 'git add -A'"

# `promote` drops the local exclusion first, so every refusal after that point owes a restore. The
# `git add` one did not: reproduced with a stale index.lock, the exclusion was gone and `git status`
# listed .idsd/. That is the whole mechanism keeping a throwaway report out of the human's commits.
new_repo
run_report check-ignore
run_report init "001-promoting"
mkdir -p "$repo/.idsd/intents"
printf '# durable\n' >"$repo/.idsd/intents/001-promoting.md"
: >"$repo/.git/index.lock"
run_report promote
assert_refused "promote refuses when it cannot stage"
if grep -qxF '.idsd/' "$repo/.git/info/exclude" 2>/dev/null; then
  record_pass "and put the local exclusion back, so .idsd/ stays invisible to 'git add -A'"
else
  record_fail "and put the local exclusion back, so .idsd/ stays invisible to 'git add -A'"
fi
rm -f "$repo/.git/index.lock"

# promote needs one report as its evidence a ship happened here; it dropped require_report for that.
new_repo
run_report check-ignore
run_report promote
assert_refused "promote refuses with no report at all"
# The message, not just the exit: without this refusal `promote` runs on and `git add .idsd` fails
# because the directory is not there, so it still exits 2 for a reason that is not this guard.
assert_reports "nothing to promote" "and names the missing report as the reason"

echo "report.sh — state never answers a token it cannot stand behind"

# The worst-token failure: with two ships open and no intent named, an unguarded `state` prints
# `no-report` and exits 0, because resolve_report returns 3 before set_report_paths and the arm falls
# into the absence branch. `idsd-ship continue` routes that to "start a fresh ship" — over two live ones.
new_repo
run_report check-ignore
run_report init "001-live"
run_report init "002-live"
run_report state
assert_refused "state refuses rather than answering no-report with two ships open"
if ! grep -q 'no-report' <<<"$out"; then
  record_pass "and no-report appears nowhere in what it printed"
else
  record_fail "and no-report appears nowhere in what it printed"
fi

# `state`'s stdout is parsed as exactly one token, so every note it emits must go to stderr.
new_repo
mkdir -p "$repo/.idsd"
printf -- '---\nintent: 002-old\n---\n\n# Decide\n' >"$repo/.idsd/ship-report.md"
state_stdout="$(cd "$repo" && "$report_sh" state 2>/dev/null)"
if [ "$state_stdout" = "no-report" ]; then
  record_pass "state's stdout is one token even while it notes the pre-scoping report on stderr"
else
  out="stdout was: $state_stdout"
  record_fail "state's stdout is one token even while it notes the pre-scoping report on stderr"
fi

echo "report.sh — discard's destructive path, which nothing has ever covered"

# `report.sh`'s own header has said since this file was written that no case reaches here: discard
# `rm -rf`s .idsd/ and deletes the intent file. The fixture that was supposedly missing is just a
# throwaway repo, which new_repo already builds — the gap was never blocked, only unwritten. It has
# since blocked two real improvements, which is the argument for closing it.
new_repo
run_report check-ignore
run_report init "001-only-ship"
mkdir -p "$repo/.idsd/intents"
printf '# the intent\n' >"$repo/.idsd/intents/001-only-ship.md"
run_report invalidate 001-only-ship
run_report stage-returned code-review 001-only-ship
markers_dir="$repo/.git/idsd-stage-returns/001-only-ship"
run_report discard 001-only-ship
if [ "$status" = 0 ] && [ ! -e "$repo/.idsd" ]; then
  record_pass "discard removes the whole .idsd/ when this ship was the only thing in it"
else
  out="exit $status; .idsd/ holds: $(ls -A "$repo/.idsd" 2>/dev/null)"
  record_fail "discard removes the whole .idsd/ when this ship was the only thing in it"
fi
# The stage markers live in the git dir, so removing .idsd/ cannot reach them — they need their own
# removal, or the next ship for this intent inherits a completed stage record and stamps for free.
if [ ! -e "$markers_dir" ]; then
  record_pass "and the stage markers in the git dir, which removing .idsd/ never reaches"
else
  record_fail "and the stage markers in the git dir, which removing .idsd/ never reaches"
fi
# Zero traces means the local exclusion too, or .git/info/exclude keeps a line for a dir that is gone.
if ! grep -qxF '.idsd/' "$repo/.git/info/exclude" 2>/dev/null; then
  record_pass "and the local exclusion, so the throwaway leaves nothing at all"
else
  record_fail "and the local exclusion, so the throwaway leaves nothing at all"
fi

# A durable file is the human's, never this ship's scratch, so it keeps .idsd/ standing.
new_repo
run_report check-ignore
run_report init "001-with-charter"
printf '# charter\n' >"$repo/.idsd/charter.md"
run_report discard 001-with-charter
if [ "$status" = 0 ] && [ -f "$repo/.idsd/charter.md" ] && [ ! -f "$(report_path 001-with-charter)" ]; then
  record_pass "discard keeps .idsd/ for a durable file while removing this ship's report"
else
  record_fail "discard keeps .idsd/ for a durable file while removing this ship's report (exit $status)"
fi
assert_reports "charter.md" "and names what kept it standing"
if grep -qxF '.idsd/' "$repo/.git/info/exclude" 2>/dev/null; then
  record_pass "and leaves the exclusion in place, since .idsd/ is still there to hide"
else
  record_fail "and leaves the exclusion in place, since .idsd/ is still there to hide"
fi

# A parallel ship is another human's work in flight. Deleting its report is the unrecoverable case.
new_repo
run_report check-ignore
run_report init "001-going"
run_report init "002-staying"
mkdir -p "$repo/.idsd/intents"
printf '# going\n' >"$repo/.idsd/intents/001-going.md"
printf '# staying\n' >"$repo/.idsd/intents/002-staying.md"
run_report discard 001-going
if [ "$status" = 0 ] && [ -f "$(report_path 002-staying)" ] &&
  [ -f "$repo/.idsd/intents/002-staying.md" ] && [ ! -f "$repo/.idsd/intents/001-going.md" ]; then
  record_pass "discard removes only the named ship, leaving a parallel one whole"
else
  out="exit $status; reports: $(ls "$repo/.idsd/qualify-reports" 2>/dev/null); intents: $(ls "$repo/.idsd/intents" 2>/dev/null)"
  record_fail "discard removes only the named ship, leaving a parallel one whole"
fi
assert_reports "other qualify report" "and names the parallel ship as what kept .idsd/ alive"

# An intent that already built has its file in archive/, not intents/ — both are this ship's.
new_repo
run_report check-ignore
run_report init "001-archived"
mkdir -p "$repo/.idsd/archive"
printf '# built\n' >"$repo/.idsd/archive/001-archived.md"
run_report discard 001-archived
if [ "$status" = 0 ] && [ ! -e "$repo/.idsd" ]; then
  record_pass "discard removes the intent file from archive/ as well as intents/"
else
  out="exit $status; left: $(find "$repo/.idsd" 2>/dev/null)"
  record_fail "discard removes the intent file from archive/ as well as intents/"
fi

# Committed mode: .idsd/ is the durable record, so there is nothing here to discard and the refusal
# is the only thing standing between this subcommand and the human's tracked files.
new_repo
mkdir -p "$repo/.idsd/qualify-reports"
printf '# durable\n' >"$repo/.idsd/charter.md"
printf '.idsd/qualify-reports/\n' >"$repo/.gitignore"
git -C "$repo" add .gitignore .idsd/charter.md
git -C "$repo" -c user.email=t@t -c user.name=t commit -qm "committed idsd"
run_report init "001-committed"
run_report discard 001-committed
assert_refused "discard refuses in committed mode, where .idsd/ is the durable record"
if [ -f "$repo/.idsd/charter.md" ] && [ -f "$(report_path 001-committed)" ]; then
  record_pass "and deleted nothing"
else
  record_fail "and deleted nothing"
fi

# `close` and `discard` compose in either order. Before, `close` deleted the report `discard` reads,
# so `discard` refused and the .idsd/ it was to clear stayed standing — an ordering that had to be
# carried in prose. The intent is the argument, so the report was never the only way to name the ship.
new_repo
run_report check-ignore
run_report init "001-closed-then-discarded"
mkdir -p "$repo/.idsd/intents"
printf '# the intent\n' >"$repo/.idsd/intents/001-closed-then-discarded.md"
run_report close 001-closed-then-discarded
run_report discard 001-closed-then-discarded
if [ "$status" = 0 ] && [ ! -e "$repo/.idsd" ]; then
  record_pass "discard runs after close, with no report left to read"
else
  out="exit $status; left: $(find "$repo/.idsd" 2>/dev/null)"
  record_fail "discard runs after close, with no report left to read"
fi

# Unnamed and with no report, there is nothing to identify — that refuses, and says naming the intent
# is the way through, or a closed ship could never be discarded at all.
new_repo
run_report check-ignore
run_report init "001-unnamed"
run_report close 001-unnamed
run_report discard
assert_refused "discard refuses when no report is left and no intent is named"
assert_reports "Name the intent" "and says naming the intent is what gets past it"

echo "$passed passed, $failed failed"
[ "$failed" = 0 ]
