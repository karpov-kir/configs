#!/usr/bin/env bash
# Tests for report.sh's index isolation and its refusals: the paths where a defect is either
# unrecoverable or silently stamps an unreviewed tree as reviewed.
#   usage: report-test.sh   # prints one line per case; exit 0 when all pass, 1 otherwise
#
# The index group comes first because nothing undoes what it catches. `current_tree` delegates the recipe
# to `~/.kk-flavor/scripts/tree-fingerprint.sh`, whose own failure modes are pinned in
# `~/.kk-flavor/scripts/tree-fingerprint-test.sh`. What these cases pin is this side of the seam: that a
# subcommand which fingerprints leaves the human's staged-versus-unstaged split exactly as they left it,
# and that the fingerprint really follows the tree. Git records nothing about what was staged before, so
# no later refusal puts a wrecked split back
# (`~/.kk-flavor/standards/skill-protocol.md` → **Queue**). `init` fingerprints nothing, so the case
# beside them guards only that the subcommand stays out of the index.
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
  # Both checks matter, because this suite reaches `discard`, which `rm -rf`s `.idsd/` and deletes
  # intent files. A failed `git init` leaves no repo here, so `git rev-parse --show-toplevel` from
  # inside resolves to whatever repository encloses the temp dir. Every destructive case then runs
  # against that one, this checkout included if $TMPDIR ever sits inside it. Exiting beats carrying on:
  # no later assertion can make a fixture that is not its own repo safe.
  git -C "$repo" init -q || {
    echo "report-test: git init failed in $repo — stopping before any destructive case runs"
    exit 1
  }
  # Compared physically: on macOS $TMPDIR sits under /var, a symlink to /private/var, and git answers
  # with the resolved path. A literal comparison would fail on every fixture, making this guard look
  # like the very thing it exists to catch.
  resolved_root=$(git -C "$repo" rev-parse --show-toplevel 2>/dev/null)
  repo_physical=$(cd "$repo" 2>/dev/null && pwd -P)
  [ -n "$repo_physical" ] && [ "$resolved_root" = "$repo_physical" ] || {
    echo "report-test: $repo resolves to '$resolved_root', not itself — stopping before any destructive case runs"
    exit 1
  }
  # Checked by its effect, not just by exit status: `git add -A` needs a HEAD to compare against, so a
  # fixture whose commit never landed sends every case below down the unfingerprintable-tree path instead
  # of the one it meant to test.
  printf 'base\n' >"$repo/tracked.txt"
  git -C "$repo" add tracked.txt &&
    git -C "$repo" -c user.email=t@t -c user.name=t commit -qm base &&
    git -C "$repo" diff --quiet HEAD -- tracked.txt || {
    echo "report-test: could not commit the fixture in $repo — stopping before any destructive case runs"
    exit 1
  }
}

# Runs report.sh the way a skill does, from inside the repo, so `git rev-parse --show-toplevel`
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

# A standalone `review: …` has no slug and shares the one `review` stem, which is what most fixtures
# below use.
report_path() {
  printf '%s\n' "$repo/.idsd/qualify-reports/${1:-review}-qualify-report.md"
}

# A copy of the whole skill dir: scripts beside templates, the layout report.sh derives its template and
# todo-gate paths from. So a case can break either without touching this checkout's own, and one copy per
# case means no mutation carries.
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

# Drive a ship to a stamped, tree-fresh state. Unstamped, state_token answers `resume` without reading
# the tree at all, so a case that pins anything past the freshness checks has to come through here. $1 is
# the runner, so a case on a broken skill copy uses the same sequence as one on this checkout's own.
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
# restrict this user (root reads anything), so the case is skipped by name rather than failed. Restore
# the file with `chmod 644` afterwards, so the fixture teardown can remove it.
made_unreadable() {
  chmod 000 "$1"
  [ -r "$1" ] || return 0
  echo "  skip  chmod does not restrict this user (root?) — $2 cannot run"
  return 1
}

# An intent file for one slug. Its body is a fixed constant because no case asserts on it: what they
# care about is the file's presence, and whether `discard` takes it or leaves it.
new_intent_file() {
  mkdir -p "$repo/.idsd/intents"
  printf '# intent\n' >"$repo/.idsd/intents/$1.md"
}

# The human's own durable file: what keeps .idsd/ standing when a ship's scratch goes, and what
# `promote` needs something of. It creates .idsd/ first, so no caller can write the charter before the
# directory that holds it.
new_durable_charter() {
  mkdir -p "$repo/.idsd"
  printf '# durable\n' >"$repo/.idsd/charter.md"
}

# The other repo mode: .idsd/ tracked through a durable charter, with qualify-reports/ gitignored the way
# a shared idsd setup does it. Every plain `new_repo` fixture is a throwaway. qualify-reports/ is left
# absent, since `init` is what creates it.
new_committed_repo() {
  new_repo
  new_durable_charter
  printf '.idsd/qualify-reports/\n' >"$repo/.gitignore"
  git -C "$repo" add .gitignore .idsd/charter.md
  git -C "$repo" -c user.email=t@t -c user.name=t commit -qm "committed idsd"
  assert_fixture_is_committed
}

# Checked at the one place the state is built. A fixture whose commit did not land is a throwaway, and
# the committed-mode branches its cases test (discard's refusal, check-ignore's warning, init's
# acceptance) answer the same way in both modes. So every case above such a fixture passes while testing
# nothing, all at once.
assert_fixture_is_committed() {
  if [ "$(cd "$repo" && "$report_sh" repo-mode)" = committed ]; then
    # Named by fixture dir: this runs at four fixtures, and four identical pass lines could not be told
    # apart. A guard that prints nothing on success reads, in the output, exactly like one that never ran.
    record_pass "fixture ${repo##*/} is a committed repo"
  else
    out="git ls-files .idsd printed: '$(git -C "$repo" ls-files .idsd)'"
    record_fail "fixture did not establish a committed repo — .idsd/ is not tracked"
  fi
}

# Whether .idsd/ is still hidden from the human's `git add -A`. That exclusion is the whole mechanism
# keeping a throwaway report out of their commits.
has_local_exclusion() {
  grep -qxF '.idsd/' "$repo/.git/info/exclude" 2>/dev/null
}

# Discard succeeded and took the whole scratch dir with it.
assert_idsd_removed() {
  if [ "$status" = 0 ] && [ ! -e "$repo/.idsd" ]; then
    record_pass "$1"
  else
    out="exit $status; left: $(find "$repo/.idsd" 2>/dev/null)"
    record_fail "$1"
  fi
}

# A refusal wrote no report. Asserted on the directory, so a report under any name counts.
assert_no_report_written() {
  if [ -z "$(ls -A "$repo/.idsd/qualify-reports" 2>/dev/null)" ]; then
    record_pass "$1"
  else
    record_fail "$1"
  fi
}

echo "report.sh — the human's index is never touched"

# The split: one path staged, one tracked path modified but not staged, one path untracked. A
# `git add -A` on the real index collapses all three into "staged".
new_repo
run_report check-ignore
printf 'staged\n' >"$repo/staged.txt"
git -C "$repo" add staged.txt
printf 'base\nmodified\n' >"$repo/tracked.txt"
printf 'untracked\n' >"$repo/untracked.txt"
before="$(index_state)"
# The three cases below compare this split before and after, and an empty split compares equal just as
# well. So a fixture that staged nothing would pass all three while the state they protect is not there.
if grep -q '^staged:staged.txt' <<<"$before" && grep -q '^unstaged:tracked.txt' <<<"$before"; then
  record_pass "fixture: the staged/unstaged split the three cases below compare"
else
  out="$before"
  record_fail "fixture did not establish a staged/unstaged split for the index cases"
fi
run_report init "review: index isolation"
after_init="$(index_state)"
if [ "$before" = "$after_init" ]; then
  record_pass "init stages nothing (a regression guard, not a fingerprint one)"
else
  out="before -> $before
after  -> $after_init"
  record_fail "init stages nothing (a regression guard, not a fingerprint one)"
fi

# `gate` reaches current_tree immediately after require_report, so it is the shortest path to the
# fingerprint. It is expected to block here; what matters is the index afterwards.
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

echo "report.sh — a missing fingerprint script refuses instead of recomputing"

# `current_tree` delegates to a sibling script, and the one thing it must not do when that script is gone
# is fingerprint the tree itself: a local copy of the recipe puts every untracked file's content in the
# human's own object store, recoverable for good. The fixture is the positive control — with the
# sibling reachable, `gate` gets past the fingerprint and blocks on freshness (exit 1), so the refusal
# below is the missing script and not some earlier guard.
new_repo
run_report check-ignore
run_report init "review: missing fingerprint"
run_report gate
if [ "$status" = 1 ]; then
  record_pass "fixture: gate reaches the fingerprint and blocks on freshness while the sibling is installed"
else
  record_fail "fixture did not reach the fingerprint with the sibling installed (gate exited $status, wanted 1)"
fi
# The path is resolved through $HOME, so an empty one is how the script goes missing without touching the
# real install.
empty_home="$base/nohome$case_number"
mkdir -p "$empty_home"
out="$(cd "$repo" && HOME="$empty_home" "$report_sh" gate 2>&1)"
status=$?
assert_refused "gate refuses when the fingerprint script is not installed"
assert_reports "tree-fingerprint.sh" "and names the path it wanted"
assert_reports "no local fallback" "and says it will not fall back to a recipe of its own"

echo "report.sh — init refuses rather than writing through a link"

new_repo
run_report check-ignore
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
run_report check-ignore
mkdir -p "$base/outside$case_number"
ln -s "$base/outside$case_number" "$repo/.idsd"
run_report init "review: symlinked idsd dir"
assert_refused "init refuses a symlinked .idsd directory"
if [ -e "$base/outside$case_number/qualify-reports" ]; then
  record_fail "init wrote the report outside the repo through .idsd"
else
  record_pass "nothing was written outside the repo through .idsd"
fi

# The second directory every write goes through, so it needs a case of its own: the `-L` check on the
# report file cannot see a link one level up.
new_repo
run_report check-ignore
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
run_report check-ignore
# Two forms, because two separate rules refuse them: the leading dot stops `../../escaped` on its own,
# so only a value that starts with a legal character reaches the charset.
for escaping_intent in "../../escaped" "ok/../../escaped"; do
  run_report init "$escaping_intent"
  assert_refused "init refuses the intent '$escaping_intent', whose path could escape the directory"
done
# The escape lands at $repo/escaped-qualify-report.md, two levels up from qualify-reports/, not at $base.
# Asserting only on $base, or on $repo/.. (the same directory), lets a widened charset through.
if [ -e "$repo/escaped-qualify-report.md" ] || [ -e "$repo/.idsd/escaped-qualify-report.md" ] ||
  [ -e "$base/escaped-qualify-report.md" ]; then
  out="$(find "$base" -name 'escaped*' 2>/dev/null)"
  record_fail "no report was written outside qualify-reports/"
else
  record_pass "no report was written outside qualify-reports/"
fi

# What the refusals prevent is invisibility rather than the write itself: `*` never matches a leading
# dot, so a dot-named report would stand open while `list` says "no reports". So `list` is asserted here
# as well as the three exits.
new_repo
run_report check-ignore
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
run_report check-ignore
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
# The listing comes from todo-gate.sh, and it is now the only record of what --force discarded: no copy
# is kept beside the report. A --force that replaces a report while printing none of its open items is
# how routed work silently disappears.
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
run_report check-ignore
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
run_report check-ignore
run_report init "review: stamp guard"
run_report stamp "code-review,security-review,tighten,refactor,retro"
assert_refused "stamp refuses before this pass has invalidated"
assert_reports "never invalidated" "and names invalidate as what is missing"

new_repo
run_report check-ignore
run_report init "review: stage marker guard"
run_report invalidate
run_report stage-returned code-review
run_report stage-returned security-review
assert_refused "a second stage cannot be marked returned while the first's items are unrecorded"
assert_reports "has not moved since" "and says the report has not moved"

# The same guard, met by the one caller it must not stop: streaming resumes a stage and takes its return
# again with nothing recorded in between, so the outstanding stage is the one being marked. The other four
# are cleared first so the re-marked one is the only stage the stamp below can block on.
new_repo
run_report check-ignore
run_report init "review: resumed stage"
run_report invalidate
for cleared_stage in security-review tighten refactor retro; do
  run_report stage-returned "$cleared_stage"
  run_report no-items "$cleared_stage"
done
run_report stage-returned code-review
run_report stage-returned code-review
if [ "$status" = 0 ]; then
  record_pass "a resumed stage can be marked returned again"
else
  record_fail "a resumed stage can be marked returned again (exit $status)"
fi
# Exit 0 alone would also be satisfied by a re-mark that cleared the gate. What must survive the re-mark
# is the stamp's demand that the stage's items reach the report: it rewrites the same checksum, so the
# report still has not moved, and a stamp taken here would record a review whose findings were never
# written down.
run_report stamp "code-review,security-review,tighten,refactor,retro"
assert_refused "and the re-mark leaves the stamp still gated on that stage's unrecorded items"
assert_reports "unchanged since" "and names the unrecorded items as the reason"
if grep -q '^reviewed-tree: pending$' "$(report_path)"; then
  record_pass "and nothing was stamped over them"
else
  record_fail "and nothing was stamped over them"
fi
# The positive control for the refusal above: the same stamp lands once that one stage is accounted for,
# so what blocked it was the unrecorded items and nothing else on the way.
run_report no-items code-review
run_report stamp "code-review,security-review,tighten,refactor,retro"
if [ "$status" = 0 ] && ! grep -q '^reviewed-tree: pending$' "$(report_path)"; then
  record_pass "and stamps once that stage's items are accounted for"
else
  record_fail "and stamps once that stage's items are accounted for (exit $status)"
fi

# The stamp's other per-stage refusal: an entry recorded as having run for a stage that was never marked
# returned at all. `refactor,retro` is legally shaped whether or not either ran, so the grammar check
# above cannot see this and only the per-stage marker can. Four of the five are marked, leaving `retro`
# as the one thing between this pass and a stamp it never earned.
new_repo
run_report check-ignore
run_report init "review: unmarked stage"
run_report invalidate
for cleared_stage in code-review security-review tighten refactor; do
  run_report stage-returned "$cleared_stage"
  run_report no-items "$cleared_stage"
done
run_report stamp "code-review,security-review,tighten,refactor,retro"
assert_refused "stamp refuses a stage recorded as having run that was never marked returned"
assert_reports "never marked returned" "and names the stage-returned call that never happened"
if grep -q '^reviewed-tree: pending$' "$(report_path)"; then
  record_pass "and stamped nothing for the pass that skipped it"
else
  record_fail "and stamped nothing for the pass that skipped it"
fi
# And the same stamp lands once that stage is marked too, so the refusal above was this guard alone.
run_report stage-returned retro
run_report no-items retro
run_report stamp "code-review,security-review,tighten,refactor,retro"
if [ "$status" = 0 ] && ! grep -q '^reviewed-tree: pending$' "$(report_path)"; then
  record_pass "and stamps once every stage carries a marker"
else
  record_fail "and stamps once every stage carries a marker (exit $status)"
fi

echo "report.sh — two intents ship side by side"

# The whole point of the per-intent path: a second intent's init is not a collision, so neither ship has
# to be finished before the other starts. And `check-ignore` first, as a real pass does, or the report
# sits inside its own fingerprint and every state below reads `re-qualify`.
new_repo
run_report check-ignore
run_report init "001-first-intent"
run_report init "002-second-intent"
if [ "$status" = 0 ] && [ -f "$(report_path 001-first-intent)" ] && [ -f "$(report_path 002-second-intent)" ]; then
  record_pass "a second intent gets its own report rather than a refusal"
else
  record_fail "a second intent gets its own report rather than a refusal (exit $status)"
fi

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

run_report stage-returned code-review 001-first-intent
run_report invalidate 002-second-intent
run_report no-items code-review 001-first-intent
if [ "$status" = 0 ]; then
  record_pass "one intent's invalidate leaves the other's stage markers standing"
else
  record_fail "one intent's invalidate leaves the other's stage markers standing (exit $status)"
fi

# The state column is asserted by value, not by "a tab follows the name". The looser form is satisfied
# by a listing that emits an empty token for every ship, or `BOGUS` where `resume` belongs. `list`
# is the surface `idsd-ship continue` routes on with several ships in flight.
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
run_report check-ignore
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

# `--force` shares its charset with a legal slug, so read positionally it resolves as an intent name and
# closes a report that does not exist, reporting success while the real one stands.
new_repo
run_report check-ignore
run_report init "review: force alone"
run_report close --force
if [ "$status" = 0 ] && [ ! -f "$(report_path)" ]; then
  record_pass "close reads --force as a flag, not as the intent name"
else
  record_fail "close reads --force as a flag, not as the intent name (exit $status)"
fi

echo "report.sh — check-ignore holds before qualify-reports/ exists"

# `check-ignore` runs before the first write into `.idsd/`, and its exit 1 blocks that write. So it has to
# answer correctly while qualify-reports/ does not exist yet, and that is where the trailing slash in
# ignore_surface earns its keep: without it, `git check-ignore -q .idsd/qualify-reports` exits 1 on a
# directory that is not there.
new_committed_repo
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

# The unforced path, the one `idsd-ship done` invokes, on a report whose items are all cleared.
new_repo
run_report check-ignore
run_report init "001-landed"
run_report close 001-landed
if [ "$status" = 0 ] && [ ! -f "$(report_path 001-landed)" ]; then
  record_pass "close needs no --force once nothing is open"
else
  record_fail "close needs no --force once nothing is open (exit $status)"
fi

# `close` retires a landed ship's report, and the archived intent file is then the only record it landed.
# Read absence alone and `state` answers `no-report`, which `idsd-ship continue` routes to
# "start ship <intent>": rebuilding work already merged.
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

# And an intent that was never archived still reads no-report once closed. The archive is the fact being
# read, not the closing.
run_report init "002-closed-unbuilt"
run_report close 002-closed-unbuilt
run_report state 002-closed-unbuilt
if [ "$out" = no-report ]; then
  record_pass "and no-report still answers for a closed report with no archived intent"
else
  record_fail "and no-report still answers for a closed report with no archived intent (said '$out')"
fi

echo "report.sh — an unreadable report is not a state"

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

# The same invariant with the unreadable report ordered second, the only order that pins the buffering.
# Reached first, nothing is printed whether the listing is buffered or streamed.
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
  # keep, and an unreadable one prints none, which reads exactly like a report with nothing open.
  run_report carry 002-unreadable-second
  assert_refused "carry refuses a report it cannot read rather than reporting no open items"
  # The message carries this one: with require_report's readability refusal deleted, `carry` still
  # exits 2, because todo-gate.sh cannot read the file either and read_open_todos refuses in its place.
  # Only this assertion tells the two apart; the discard case below pins the same guard where nothing
  # else refuses for it.
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
run_report check-ignore
run_report init "001-discarding"
new_intent_file 001-discarding
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

# qualify-reports/ is ignored by the entry promote itself writes, and `git add` on a directory whose
# every file is ignored stages nothing and exits 0. With nothing else under .idsd/, reading success off
# that add leaves repo-mode still saying throwaway, and the next check-ignore re-excludes .idsd/,
# silently undoing the promotion.
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
if has_local_exclusion; then
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

# Committed mode takes the other check-ignore branch entirely: the one that asks git rather than writing
# an exclusion, and the only one that can confirm the entry instead of creating it.
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

# The filename and the frontmatter have to name the same ship. When they differ, one intent gets two
# reports so the ambiguity refusal never fires, `discard` deletes another ship's in-flight intent, and
# `state` answers `done` for an open one.
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

# Asserted on disk as well as on the exit: the harm is the scaffolded report, whose blank `intent:` every
# reader treats as a standalone review.
new_repo
run_report check-ignore
run_report init " "
assert_refused "init refuses a whitespace-only intent"
assert_no_report_written "and wrote no report for it"

echo "report.sh — discard reconciles the two names before deleting anything"

# The destructive path: discard is addressed by the filename, and deletes the intent file the frontmatter
# names. Disagreeing, it deletes another ship's in-flight intent, which throwaway mode keeps no copy of
# anywhere.
new_repo
run_report check-ignore
run_report init "001-mine"
new_intent_file 002-yours
new_intent_file 001-mine
sed -i.bak 's/^intent: 001-mine$/intent: 002-yours/' "$(report_path 001-mine)" && rm -f "$(report_path 001-mine).bak"
run_report discard 001-mine
assert_refused "discard refuses when the filename and the frontmatter name different ships"
if [ -f "$repo/.idsd/intents/002-yours.md" ] && [ -f "$repo/.idsd/intents/001-mine.md" ]; then
  record_pass "and deleted neither intent file"
else
  record_fail "and deleted neither intent file"
fi

echo "report.sh — list walks the tree once, and never streams a partial answer"

# Three stamped reports, so state_token reaches the fingerprint for each. state_token runs inside a
# command substitution, so a cache it filled there dies with that subshell: every ship would walk the
# tree again, and two ships could then be scored against different trees.
new_repo
run_report check-ignore
for ship in 001-a 002-b 003-c; do
  run_report init "$ship"
  stamp_full_pass run_report "$ship"
done
# Stamped is the load-bearing word. An unstamped ship answers `resume` without reading the tree at all,
# so the count below would be 1, the priming call alone, and would pass with no ship ever reaching the
# cache this pins. Asserted through `list`'s own output, the surface the case reads.
run_report list
if [ "$(grep -c '	ready$' <<<"$out")" = 3 ]; then
  record_pass "fixture: three stamped ships, each one reaching the fingerprint"
else
  record_fail "fixture did not establish three stamped ships for the tree-walk count"
fi
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
  # "unfingerprintable" is the state under test, and nothing else here establishes it: both ships are
  # unstamped, so they answer `resume` without the tree either way and the case passes on a perfectly
  # readable tree. `gate` is the shortest path that has to fingerprint, so its 2 is the failure to do so,
  # where a readable tree gives 1, the freshness block.
  run_report gate 001-unstamped
  if [ "$status" = 2 ]; then
    record_pass "fixture: a tree that cannot be fingerprinted"
  else
    record_fail "fixture did not establish an unfingerprintable tree (gate exited $status, wanted 2)"
  fi
  run_report list
  if [ "$status" = 0 ] && grep -q '^001-unstamped	resume$' <<<"$out" && grep -q '^002-unstamped	resume$' <<<"$out"; then
    record_pass "an unfingerprintable tree does not silence a listing of ships that never needed it"
  else
    record_fail "an unfingerprintable tree does not silence a listing of ships that never needed it (exit $status)"
  fi
fi
chmod 644 "$repo/blocker.txt"

echo "report.sh — the pre-scoping path is reported, never passed over in silence"

new_repo
mkdir -p "$repo/.idsd"
printf -- '---\nintent: 002-live\n---\n\n# Decide\n\n- [ ] a live decision\n' >"$repo/.idsd/ship-report.md"
# The pre-rename directory is the second historical path, and the note must name it too: a repo mid-ship
# across the rename has its report there, which the `ship-report.md` entry does not cover.
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

# The staged `cp` writes to `$report.new`, one more path the symlink guard chain has to cover. A link
# planted there is committable, so it arrives through someone else's branch, and `cp` would overwrite
# its target while `init` reported success.
new_repo
run_report check-ignore
mkdir -p "$repo/.idsd/qualify-reports" "$base/victim$case_number"
printf 'PRECIOUS\n' >"$base/victim$case_number/keep.md"
ln -s "$base/victim$case_number/keep.md" "$(report_path review).new"
# The planted link is the fixture here, and unlike every other symlink case this one expects `init` to
# succeed. So a failed `ln -s` leaves PRECIOUS intact and the report written, and both cases below pass
# with nothing ever planted.
if [ -L "$(report_path review).new" ]; then
  record_pass "fixture: a link planted at the staged path"
else
  record_fail "fixture did not plant a link at the staged path"
fi
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

# The fixture first: an untouched copy must initialize. Without this, a copy report.sh cannot read at all
# would refuse every case below, and each refusal would pass for the wrong reason.
new_repo
run_report check-ignore
new_skill_copy
run_copied_report init "001-intact-copy"
if [ "$status" = 0 ] && [ -f "$(report_path 001-intact-copy)" ]; then
  record_pass "the copied skill dir initializes a report from its own template"
else
  record_fail "the copied skill dir initializes a report from its own template (exit $status)"
fi

# The drift that gates clean: a placeholder outside is_unstamped()'s set puts a real-looking
# reviewed-tree in every new report, which `gate` reads as a completed review of whatever tree matches.
new_repo
run_report check-ignore
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
  run_report check-ignore
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

# The template is read and never written, so what the symlink guard stops is content from outside the
# repo becoming a new report's frontmatter. A committed link arrives through someone else's branch, and a
# forged `reviewed-tree` is what it would carry in.
new_repo
run_report check-ignore
new_skill_copy
mkdir -p "$base/foreign$case_number"
# A plausible template, placeholders included, so every other check on it passes and the symlink guard is
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
# template. So this assertion names the harm, content from outside the repo reaching .idsd/, rather than
# the shape of the refusal.
if ! grep -rqF SMUGGLED "$repo/.idsd" 2>/dev/null; then
  record_pass "and no content from outside the repo reached .idsd/"
else
  out="$(grep -rlF SMUGGLED "$repo/.idsd" 2>/dev/null)"
  record_fail "and no content from outside the repo reached .idsd/"
fi

# A template that is gone. Without this refusal the `intent:` guard fires on grep's own failure to open
# the file, and the message stops naming the real cause, so the message is what pins this one.
new_repo
run_report check-ignore
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

# `state`, `carry` and `close` share one reader, and the point of sharing it is that they cannot drift
# apart, so all three are asserted here against the same broken gate. Broken means an exit above 1, which
# read as "nothing open" would let a report still holding unrouted `- [ ]` pass the merge gate.
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
# caller's own shell is not pinned here. Through `$( )` with the exit status unrelayed, a downstream
# guard refuses anyway, so the suite cannot tell the two shapes apart.
new_repo
run_report check-ignore
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

# `promote` drops the local exclusion first, so every refusal after that point owes a restore. Without
# one on the `git add` refusal, the exclusion is gone and `git status` lists .idsd/.
new_repo
run_report check-ignore
run_report init "001-promoting"
new_intent_file 001-promoting
: >"$repo/.git/index.lock"
run_report promote
assert_refused "promote refuses when it cannot stage"
if has_local_exclusion; then
  record_pass "and put the local exclusion back, so .idsd/ stays invisible to 'git add -A'"
else
  record_fail "and put the local exclusion back, so .idsd/ stays invisible to 'git add -A'"
fi
rm -f "$repo/.git/index.lock"

# promote needs one report as its evidence a ship happened here, which is why it does not go through
# require_report.
new_repo
run_report check-ignore
run_report promote
assert_refused "promote refuses with no report at all"
# Asserted on the message: without this refusal `promote` runs on and `git add .idsd` fails because the
# directory is not there, so it still exits 2, for a reason that is not this guard.
assert_reports "nothing to promote" "and names the missing report as the reason"

echo "report.sh — state never answers a token it cannot stand behind"

# The worst-token failure: with two ships open and no intent named, an unguarded `state` prints
# `no-report` and exits 0, because resolve_report returns 3 before set_report_paths and the arm falls
# into the absence branch. `idsd-ship continue` routes that to "start a fresh ship", over two live ones.
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

echo "report.sh — discard's destructive path"

# discard is the destructive path: `rm -rf` on .idsd/, plus the intent file, in the mode that keeps no
# copy of either anywhere.
new_repo
run_report check-ignore
run_report init "001-only-ship"
new_intent_file 001-only-ship
run_report invalidate 001-only-ship
run_report stage-returned code-review 001-only-ship
markers_dir="$repo/.git/idsd-stage-returns/001-only-ship"
run_report discard 001-only-ship
assert_idsd_removed "discard removes the whole .idsd/ when this ship was the only thing in it"
# The stage markers live in the git dir, so removing .idsd/ cannot reach them. They need their own
# removal, or the next ship for this intent inherits a completed stage record and stamps for free.
if [ ! -e "$markers_dir" ]; then
  record_pass "and the stage markers in the git dir, which removing .idsd/ never reaches"
else
  record_fail "and the stage markers in the git dir, which removing .idsd/ never reaches"
fi
# Zero traces means the local exclusion too, or .git/info/exclude keeps a line for a dir that is gone.
if ! has_local_exclusion; then
  record_pass "and the local exclusion, so the throwaway leaves nothing at all"
else
  record_fail "and the local exclusion, so the throwaway leaves nothing at all"
fi

# A durable file is the human's, never this ship's scratch, so it keeps .idsd/ standing.
new_repo
run_report check-ignore
run_report init "001-with-charter"
new_durable_charter
run_report discard 001-with-charter
if [ "$status" = 0 ] && [ -f "$repo/.idsd/charter.md" ] && [ ! -f "$(report_path 001-with-charter)" ]; then
  record_pass "discard keeps .idsd/ for a durable file while removing this ship's report"
else
  record_fail "discard keeps .idsd/ for a durable file while removing this ship's report (exit $status)"
fi
assert_reports "charter.md" "and names what kept it standing"
if has_local_exclusion; then
  record_pass "and leaves the exclusion in place, since .idsd/ is still there to hide"
else
  record_fail "and leaves the exclusion in place, since .idsd/ is still there to hide"
fi

# A parallel ship is another human's work in flight. Deleting its report is the unrecoverable case.
new_repo
run_report check-ignore
run_report init "001-going"
run_report init "002-staying"
new_intent_file 001-going
new_intent_file 002-staying
run_report discard 001-going
if [ "$status" = 0 ] && [ -f "$(report_path 002-staying)" ] &&
  [ -f "$repo/.idsd/intents/002-staying.md" ] && [ ! -f "$repo/.idsd/intents/001-going.md" ]; then
  record_pass "discard removes only the named ship, leaving a parallel one whole"
else
  out="exit $status; reports: $(ls "$repo/.idsd/qualify-reports" 2>/dev/null); intents: $(ls "$repo/.idsd/intents" 2>/dev/null)"
  record_fail "discard removes only the named ship, leaving a parallel one whole"
fi
assert_reports "other qualify report" "and names the parallel ship as what kept .idsd/ alive"

# An intent that already built has its file in archive/ rather than intents/, and both are this ship's.
new_repo
run_report check-ignore
run_report init "001-archived"
mkdir -p "$repo/.idsd/archive"
printf '# built\n' >"$repo/.idsd/archive/001-archived.md"
run_report discard 001-archived
assert_idsd_removed "discard removes the intent file from archive/ as well as intents/"

# Committed mode: .idsd/ is the durable record, so there is nothing here to discard and the refusal
# is the only thing standing between this subcommand and the human's tracked files.
new_committed_repo
run_report init "001-committed"
run_report discard 001-committed
assert_refused "discard refuses in committed mode, where .idsd/ is the durable record"
if [ -f "$repo/.idsd/charter.md" ] && [ -f "$(report_path 001-committed)" ]; then
  record_pass "and deleted nothing"
else
  record_fail "and deleted nothing"
fi

# `discard` runs after `close`, the order `idsd-ship done` uses; reversed, `close` finds no report and
# refuses. `close` deletes the report `discard` reads, and a `discard` that refuses on that leaves the
# .idsd/ it was to clear standing.
new_repo
run_report check-ignore
run_report init "001-closed-then-discarded"
new_intent_file 001-closed-then-discarded
run_report close 001-closed-then-discarded
run_report discard 001-closed-then-discarded
assert_idsd_removed "discard runs after close, with no report left to read"

# Unnamed and with no report, there is nothing to identify. That refuses, and says naming the intent is
# the way through, or a closed ship could never be discarded at all.
new_repo
run_report check-ignore
run_report init "001-unnamed"
run_report close 001-unnamed
run_report discard
assert_refused "discard refuses when no report is left and no intent is named"
assert_reports "Name the intent" "and says naming the intent is what gets past it"

echo "report.sh — init will not write a report into its own fingerprint"

# Skipping `check-ignore`, the documented first step, is silent. The report lands inside the tree it
# fingerprints, so `state` answers `re-qualify` straight after a complete five-stage stamp and `gate`
# blocks on freshness with nothing that can clear it.
new_repo
run_report init "001-unignored"
assert_refused "init refuses when git does not ignore the reports directory"
assert_reports "report.sh check-ignore" "and names the step that was skipped"
assert_no_report_written "and wrote no report that could never gate clean"

# Committed mode reaches the same precondition through a `.gitignore` entry rather than a local
# exclusion, so the assertion has to accept it, or it blocks every committed idsd repo.
new_committed_repo
run_report init "001-committed-ok"
if [ "$status" = 0 ] && [ -f "$(report_path 001-committed-ok)" ]; then
  record_pass "and accepts a committed repo, where .gitignore is what ignores it"
else
  record_fail "and accepts a committed repo, where .gitignore is what ignores it (exit $status)"
fi

echo "report.sh — init is where a legacy report gets mentioned"

# init is the first command a pass runs, so it is the moment a report at an older path is worth knowing
# about. Every other path emits the note only when it finds no report, and init finds none by
# definition, so without this it says nothing at all.
new_repo
run_report check-ignore
mkdir -p "$repo/.idsd"
printf -- '---\nintent: 009-live\n---\n\n# Decide\n\n- [ ] unrouted\n' >"$repo/.idsd/ship-report.md"
run_report init "001-fresh"
if [ "$status" = 0 ]; then
  record_pass "init still succeeds with a legacy report sitting beside it"
else
  record_fail "init still succeeds with a legacy report sitting beside it (exit $status)"
fi
assert_reports "ship-report.md" "and names the legacy report it is not reading"

# The note names only the paths that exist, so its wording cannot claim a second one that is absent.
# Asserted on the absent path's own name: looking for a word like `either`, which no wording
# legacy_note emits, passes whether or not the note names both.
if ! grep -q 'ship-reports' <<<"$out"; then
  record_pass "and does not speak of two paths when only one is there"
else
  out="$out"
  record_fail "and does not speak of two paths when only one is there"
fi

echo "report.sh — discard deletes nothing for a ship that is not here"

# What the guard reaches is a slug naming nothing. A slug that names a real ship is still discardable,
# report closed or never made, and has to be: that is the close-then-discard case. So the residual
# exposure is a wrong slug that happens to name a live ship, which no guard can tell from an intended
# one.
new_repo
run_report check-ignore
run_report init "001-real"
new_intent_file 001-real
run_report discard 002-nothing-of-mine
assert_refused "discard refuses a slug that names no report and no intent file"
# Asserted on the refusal's own effect, not on the sibling surviving: the sibling's report keeps .idsd/
# alive either way, so "it is untouched" holds with the guard gone.
if [ ! -e "$repo/.idsd/intents/002-nothing-of-mine.md" ] && [ -d "$repo/.idsd/qualify-reports" ] &&
  [ -f "$repo/.idsd/intents/001-real.md" ]; then
  record_pass "and the reports directory it would have torn down still stands"
else
  record_fail "and the reports directory it would have torn down still stands"
fi
assert_reports "Looked for" "and names every path it looked in"

# A typo must not tear down a directory. `decisions.md` alone does not keep .idsd/ alive by design, so
# without the guard this reports "zero traces" for a ship that never existed.
new_repo
run_report check-ignore
mkdir -p "$repo/.idsd"
printf '# decisions\n' >"$repo/.idsd/decisions.md"
run_report discard 999-typo
assert_refused "discard refuses a typo rather than removing the directory around it"
if [ -f "$repo/.idsd/decisions.md" ]; then
  record_pass "and the decision log survives"
else
  record_fail "and the decision log survives"
fi

# A repo that never used idsd has no .idsd/ and no exclusion to lose; the guard is what stops discard
# reaching drop_local_exclusion there.
new_repo
run_report check-ignore
run_report discard 001-never-existed
assert_refused "discard refuses in a repo that holds no ship at all"
if has_local_exclusion; then
  record_pass "and leaves the local exclusion alone"
else
  record_fail "and leaves the local exclusion alone"
fi

# The guard must not break the case it was built around: a landed ship whose report `close` retired is
# still identified by its intent file, so close-then-discard survives.
new_repo
run_report check-ignore
run_report init "001-landed"
new_intent_file 001-landed
run_report close 001-landed
run_report discard 001-landed
assert_idsd_removed "and a closed ship is still discardable, identified by its intent file"

echo "report.sh — the destructive path carries the guards the write path has"

# An unreadable index fails `git ls-files`, which reads as "nothing tracked", so a committed repo
# reports throwaway and discard's committed-mode refusal never fires.
new_repo
new_durable_charter
new_intent_file 002-tracked
printf '.idsd/qualify-reports/\n' >"$repo/.gitignore"
git -C "$repo" add .gitignore .idsd/charter.md .idsd/intents/002-tracked.md
git -C "$repo" -c user.email=t@t -c user.name=t commit -qm "committed idsd"
# Built by hand rather than through new_committed_repo, because this one needs the intent file tracked,
# so it owes the same assertion the helper carries. Checked before the index is made unreadable, since
# `repo-mode` cannot answer afterwards.
assert_fixture_is_committed
if made_unreadable "$repo/.git/index" "the unreadable-index case"; then
  run_report discard 002-tracked
  assert_refused "discard refuses when the repo mode cannot be read, rather than assuming throwaway"
  if [ -f "$repo/.idsd/intents/002-tracked.md" ]; then
    record_pass "and the tracked intent file survives"
  else
    record_fail "and the tracked intent file survives"
  fi
fi
chmod 644 "$repo/.git/index"

# Without init's symlink guard here, an untracked .idsd link lets discard delete through to a target
# outside the repo.
new_repo
run_report check-ignore
mkdir -p "$base/outside-discard$case_number/intents"
printf '# not ours to delete\n' >"$base/outside-discard$case_number/intents/003-elsewhere.md"
ln -s "$base/outside-discard$case_number" "$repo/.idsd"
run_report discard 003-elsewhere
assert_refused "discard refuses a symlinked .idsd rather than deleting through it"
if [ -f "$base/outside-discard$case_number/intents/003-elsewhere.md" ]; then
  record_pass "and the file outside the repo is still there"
else
  record_fail "and the file outside the repo is still there"
fi
rm -f "$repo/.idsd"

echo "report.sh — a global exclude does not count as ignoring the report"

# `check-ignore -q` is satisfied by a `core.excludesFile`, which belongs to one machine: a clone
# without it stages the report on the next `git add -A`. The report
# carries a pass's security findings, so it is exactly the file that must not reach a commit.
new_repo
global_exclude="$base/global-exclude$case_number"
printf '.idsd/\n' >"$global_exclude"
git -C "$repo" config core.excludesFile "$global_exclude"
if git -C "$repo" check-ignore -q ".idsd/qualify-reports/" 2>/dev/null; then
  run_report init "001-global-only"
  assert_refused "init refuses when only a global core.excludesFile ignores the reports directory"
  assert_reports "core.excludesFile" "and names the global exclude as what does not count"
  assert_no_report_written "and wrote no report a clone would commit"
else
  record_fail "fixture did not establish a global-exclude-only state"
fi

echo "report.sh — ignored means ignored for everyone, not just this machine"

# The common global setup: `git config --global core.excludesFile ~/.gitignore`. A `*/.gitignore` arm
# matches it by name, so the guard against a machine-local exclude would pass for the very configuration
# most people have.
new_repo
home_gitignore="$base/home$case_number/.gitignore"
mkdir -p "$(dirname "$home_gitignore")"
printf '.idsd/\n' >"$home_gitignore"
git -C "$repo" config core.excludesFile "$home_gitignore"
if git -C "$repo" check-ignore -q ".idsd/qualify-reports/" 2>/dev/null; then
  run_report init "001-global-gitignore"
  assert_refused "init refuses a global core.excludesFile even when it is named .gitignore"
  assert_no_report_written "and wrote no report a clone would commit"
else
  record_fail "fixture did not establish a global-excludesFile state"
fi

# The remedy `init` names has to agree with `init`, or the human is sent between the two forever. This
# needs committed mode: throwaway `check-ignore` writes an exclusion and never asks the question, so a
# throwaway fixture passes whatever the committed branch does.
new_repo
new_durable_charter
git -C "$repo" add .idsd/charter.md
git -C "$repo" -c user.email=t@t -c user.name=t commit -qm "committed idsd, no gitignore"
home_gitignore2="$base/home2$case_number/.gitignore"
mkdir -p "$(dirname "$home_gitignore2")"
printf '.idsd/qualify-reports/\n' >"$home_gitignore2"
git -C "$repo" config core.excludesFile "$home_gitignore2"
if [ "$(cd "$repo" && "$report_sh" repo-mode)" = committed ] &&
  git -C "$repo" check-ignore -q ".idsd/qualify-reports/" 2>/dev/null; then
  run_report check-ignore
  if [ "$status" = 1 ] && grep -q 'NOT gitignored' <<<"$out"; then
    record_pass "check-ignore warns where init refuses, rather than reporting ok"
  else
    out="exit $status; said: $out"
    record_fail "check-ignore warns where init refuses, rather than reporting ok"
  fi
else
  record_fail "fixture did not establish a committed repo ignored only globally"
fi

# A linked worktree's info/exclude is absolute, so rejecting absolutes alone would break every worktree.
new_repo
run_report check-ignore
worktree_dir="$base/wt$case_number"
git -C "$repo" worktree add -q "$worktree_dir" -b wt-branch 2>/dev/null
if [ -d "$worktree_dir" ]; then
  out="$(cd "$worktree_dir" && "$report_sh" init "001-in-a-worktree" 2>&1)"
  status=$?
  if [ "$status" = 0 ] && [ -f "$worktree_dir/.idsd/qualify-reports/001-in-a-worktree-qualify-report.md" ]; then
    record_pass "init works in a linked worktree, whose info/exclude is an absolute path"
  else
    record_fail "init works in a linked worktree, whose info/exclude is an absolute path (exit $status)"
  fi
  git -C "$repo" worktree remove --force "$worktree_dir" 2>/dev/null
else
  echo "  skip  git worktree add unavailable — the absolute-info/exclude case cannot run"
fi

echo "report.sh — a standalone review can still be torn down after it is closed"

# `review` is the one stem with no intent file, and idsd-qualify's SKILL.md tells the agent to run
# `close review`, so this sequence is the documented one. Without the `review` exception it ends in a
# permanent refusal, leaving an empty .idsd/ and its exclusion standing in the mode whose whole contract
# is zero traces.
new_repo
run_report check-ignore
run_report init "review: a standalone pass"
run_report close review
run_report discard review
if [ "$status" = 0 ] && [ ! -e "$repo/.idsd" ] &&
  ! has_local_exclusion; then
  record_pass "discard tears down a closed standalone review, exclusion included"
else
  out="exit $status; .idsd: $([ -e "$repo/.idsd" ] && echo present || echo gone)"
  record_fail "discard tears down a closed standalone review, exclusion included"
fi

echo "report.sh — the teardown reports the exclusion from the result, not the attempt"

# With `drop_local_exclusion`'s return discarded, "zero traces" prints at exit 0 over a surviving
# entry. That is the one claim here a human acts on without checking.
new_repo
run_report check-ignore
run_report init "001-teardown"
new_intent_file 001-teardown
chmod 500 "$repo/.git/info"
if [ -w "$repo/.git/info" ]; then
  echo "  skip  chmod does not restrict this user (root?) — the exclusion-failure case cannot run"
  chmod 755 "$repo/.git/info"
else
  run_report discard 001-teardown
  chmod 755 "$repo/.git/info"
  if [ "$status" != 0 ] && ! grep -q 'zero traces' <<<"$out"; then
    record_pass "discard does not claim zero traces when the exclusion could not be removed"
  else
    out="exit $status; said: $out"
    record_fail "discard does not claim zero traces when the exclusion could not be removed"
  fi
fi

echo "report.sh — promote and check-ignore also refuse an unreadable index"

# The mode decides whether .idsd/ is durable, so every caller that acts on it owes the check.
new_repo
run_report check-ignore
run_report init "001-modes"
if made_unreadable "$repo/.git/index" "the unreadable-index cases"; then
  # Again the message rather than the exit: without the assertion both subcommands still exit 2, because
  # a later `git` call fails on the same unreadable index. Only the message tells the two apart, and only
  # the assertion stops the mode being read as "throwaway", the answer that deletes.
  run_report promote
  assert_refused "promote refuses when the repo mode cannot be read"
  assert_reports "repo mode is unknown" "and names the unreadable mode as the reason"
  run_report check-ignore
  assert_refused "check-ignore refuses when the repo mode cannot be read"
  assert_reports "repo mode is unknown" "and names it there too"
fi
chmod 644 "$repo/.git/index"

echo "$passed passed, $failed failed"
[ "$failed" = 0 ]
