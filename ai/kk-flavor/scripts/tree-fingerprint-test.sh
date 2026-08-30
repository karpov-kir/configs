#!/usr/bin/env bash
# Cases for tree-fingerprint.sh. The one that must not be weakened is "the caller's index is untouched":
# that failure is why the script exists.
set -u

# `CDPATH=`: set in the environment, `cd` echoes where it landed whenever the path it is given is
# relative and not dot-led, so `here` comes back two lines long and `$script` names nothing. A suite
# is not covered by the script it covers: this guard is the harness's own, and nothing under test
# reaches it.
here=$(CDPATH= cd "$(dirname "$0")" && pwd)
script="$here/tree-fingerprint.sh"
# Exit 2, and it says why: a fixture root that cannot be created is a suite that did not measure,
# which run-tests.sh counts apart from a failure. Exit 1 there would claim the script under test is
# broken — a different claim, and a false one (`~/.kk-flavor/standards/testing.md` -> **7. What a
# suite reports**).
base=$(mktemp -d) || {
  echo "tree-fingerprint-test: could not create a temporary directory — nothing was tested" >&2
  exit 2
}
trap 'rm -rf "$base"' EXIT

passed=0
failed=0
skipped=0
case_number=0

record_pass() {
  passed=$((passed + 1))
  echo "  pass  $1"
}

record_fail() {
  failed=$((failed + 1))
  echo "  FAIL  $1"
}

# Counted, not just printed. This suite declines cases on machines whose git cannot build the fixture
# they need, and an uncounted skip leaves that run reporting the same two numbers as one that checked
# them — `~/.kk-flavor/standards/testing.md` → **7. What a suite reports**.
record_skip() {
  skipped=$((skipped + 1))
  echo "  skip  $1  — $2"
}

# A fresh repo per case, so none inherits another's index or objects. The commit is checked by its effect
# and not just its exit status: staged but never committed, `tracked.txt` would still let "an unstaged edit
# to a tracked file changes the fingerprint" pass, against a file with no committed state to differ from.
new_repo() {
  case_number=$((case_number + 1))
  repo="$base/repo$case_number"
  mkdir -p "$repo" &&
    git -C "$repo" init -q &&
    git -C "$repo" config user.email t@t &&
    git -C "$repo" config user.name t &&
    printf 'one\n' >"$repo/tracked.txt" &&
    git -C "$repo" add tracked.txt &&
    git -C "$repo" commit -qm first &&
    git -C "$repo" diff --quiet HEAD -- tracked.txt || {
    echo "tree-fingerprint-test: could not build a fixture repo in $repo — stopping, since every case below would read a repo that is not there" >&2
    exit 2
  }
}

run() {
  out=$("$script" "$@" 2>&1)
  status=$?
}

# The bare form, run from inside the tree: how a skill calls it, and the form where a mistaken `add -A`
# would land on the caller's own index.
run_from_inside_repo() {
  out=$(cd "${1:-$repo}" && "$script" 2>&1)
  status=$?
}

echo "tree-fingerprint.sh"

new_repo
run "$repo"
if [ "$status" = 0 ] && [ ${#out} = 40 ]; then
  record_pass "prints a 40-character tree hash"
else
  record_fail "prints a 40-character tree hash (exit $status, got '$out')"
fi

# Both invocation forms, because the working directory decides which index a mistaken `add -A` would hit.
for invocation in path-argument no-argument; do
  new_repo
  printf 'staged\n' >"$repo/staged.txt"
  git -C "$repo" add staged.txt
  printf 'untracked\n' >"$repo/loose.txt"
  before=$(git -C "$repo" diff --name-only --cached)
  case "$invocation" in
    path-argument) run "$repo" ;;
    no-argument) run_from_inside_repo ;;
  esac
  after=$(git -C "$repo" diff --name-only --cached)
  if [ "$status" = 0 ] && [ "$before" = "$after" ] && [ "$after" = "staged.txt" ]; then
    record_pass "the caller's index is untouched ($invocation), staged file still the only staged file"
  else
    record_fail "the caller's index is untouched ($invocation) (before '$before', after '$after')"
  fi
done

# An untracked file has to reach the fingerprint, or a ledger survives edits it should not.
new_repo
run "$repo"
first="$out"
printf 'new\n' >"$repo/untracked.txt"
run "$repo"
if [ "$status" = 0 ] && [ -n "$first" ] && [ "$first" != "$out" ]; then
  record_pass "an untracked file changes the fingerprint"
else
  record_fail "an untracked file changes the fingerprint (both '$first')"
fi

new_repo
run "$repo"
first="$out"
run "$repo"
if [ "$status" = 0 ] && [ -n "$first" ] && [ "$first" = "$out" ]; then
  record_pass "an unchanged tree fingerprints the same twice"
else
  record_fail "an unchanged tree fingerprints the same twice ('$first' then '$out')"
fi

# Editing tracked content must move it too: a fingerprint that only saw the index wouldn't.
new_repo
run "$repo"
first="$out"
printf 'two\n' >"$repo/tracked.txt"
run "$repo"
if [ "$status" = 0 ] && [ "$first" != "$out" ]; then
  record_pass "an unstaged edit to a tracked file changes the fingerprint"
else
  record_fail "an unstaged edit to a tracked file changes the fingerprint (both '$first')"
fi

# An ignored path staying out is what lets a pass write its report inside the repo without invalidating
# every ledger head it took. The un-ignored write is the positive control: a fingerprint that read no new
# file at all would satisfy the ignored half on its own.
new_repo
printf 'reports/\n' >"$repo/.gitignore"
git -C "$repo" add .gitignore && git -C "$repo" commit -qm ignore-reports
run "$repo"
before_report="$out"
mkdir -p "$repo/reports"
printf 'a report the pass wrote inside the repo\n' >"$repo/reports/qualify-report.md"
run "$repo"
after_ignored="$out"
printf 'not ignored\n' >"$repo/loose.txt"
run "$repo"
after_unignored="$out"
if [ "$status" = 0 ] && [ -n "$before_report" ] && [ "$after_ignored" = "$before_report" ] &&
  [ "$after_unignored" != "$before_report" ]; then
  record_pass "an ignored path stays out of the fingerprint, while an un-ignored one moves it"
else
  record_fail "an ignored path stays out of the fingerprint (clean '$before_report', ignored '$after_ignored', un-ignored '$after_unignored')"
fi

# A linked worktree's objects live in the main repo rather than beside the working tree, so the redirect
# has to hold there too: the walk runs from inside the worktree, and the main repo's store is where its
# untracked files would otherwise land for good.
new_repo
worktree="$base/worktree$case_number"
if git -C "$repo" worktree add -q "$worktree" -b fingerprint-worktree 2>/dev/null && [ -d "$worktree" ]; then
  main_objects_before=$(find "$repo/.git/objects" -type f | wc -l | tr -d ' ')
  run_from_inside_repo "$worktree"
  first="$out"
  printf 'written inside the worktree\n' >"$worktree/untracked.txt"
  run_from_inside_repo "$worktree"
  main_objects_after=$(find "$repo/.git/objects" -type f | wc -l | tr -d ' ')
  if [ "$status" = 0 ] && [ ${#first} = 40 ] && [ ${#out} = 40 ] && [ "$first" != "$out" ] &&
    [ "$main_objects_before" = "$main_objects_after" ]; then
    record_pass "a linked worktree fingerprints without touching the main repo's object store"
  else
    record_fail "a linked worktree fingerprints without touching the main repo's object store (exit $status, '$first' -> '$out', objects $main_objects_before -> $main_objects_after)"
  fi
  git -C "$repo" worktree remove --force "$worktree" 2>/dev/null
else
  record_skip "a linked worktree fingerprints without touching the main repo's object store" \
    "git worktree add is unavailable here"
fi

case_number=$((case_number + 1))
mkdir -p "$base/plain$case_number"
run "$base/plain$case_number"
if [ "$status" = 2 ] && printf '%s' "$out" | grep -q 'not a git repository'; then
  record_pass "a directory outside any repository refuses, and says so"
else
  record_fail "a directory outside any repository refuses (exit $status, got '$out')"
fi

new_repo
run_from_inside_repo
if [ "$status" = 0 ] && [ ${#out} = 40 ]; then
  record_pass "no argument fingerprints the working directory"
else
  record_fail "no argument fingerprints the working directory (exit $status, got '$out')"
fi

# The fingerprint hashes every untracked, un-ignored file, and no ref points at those blobs, so nothing
# collects them. Land them in the human's own store and a working file's content stays recoverable for
# good, live credentials included. The moved fingerprint is the positive control: it proves the file was
# hashed, so a store that didn't grow means the objects went elsewhere rather than the file never being read.
new_repo
run "$repo"
clean_tree="$out"
printf 'a working file the human never meant to commit\n' >"$repo/untracked.txt"
objects_before=$(find "$repo/.git/objects" -type f | wc -l)
run "$repo"
objects_after=$(find "$repo/.git/objects" -type f | wc -l)
if [ "$status" = 0 ] && [ "$out" != "$clean_tree" ] && [ "$objects_before" = "$objects_after" ]; then
  record_pass "an untracked file's content never reaches the repository's object store"
else
  record_fail "an untracked file's content never reaches the repository's object store (objects $objects_before -> $objects_after, tree '$clean_tree' -> '$out')"
fi

# A git warning on the success path must not reach the channel the hash is read from: `run` captures
# 2>&1, exactly as a skill invoking this from a prompt does.
new_repo
# The embedded repo needs a commit of its own. Without one, `add -A` fails ("does not have a commit
# checked out") rather than warning, which is the case below, not this one.
git -C "$repo" init -q embedded
printf 'inner\n' >"$repo/embedded/inner.txt"
git -C "$repo/embedded" add inner.txt
git -C "$repo/embedded" -c user.email=t@t -c user.name=t commit -qm inner
# The fixture's own control: git has to actually warn on this tree, or the assertion below passes on a
# tree that never provoked a warning at all. Probed against a throwaway index, so the fixture's own is
# untouched.
probe_index="$base/warning-probe$case_number"
probe=$(GIT_INDEX_FILE="$probe_index" git -C "$repo" add -A 2>&1 >/dev/null)
rm -f "$probe_index"
if printf '%s' "$probe" | grep -q 'embedded git repository'; then
  record_pass "fixture: git warns on a tree holding an embedded repository"
else
  record_fail "fixture did not provoke a git warning (git said '$probe')"
fi
run "$repo"
if [ "$status" = 0 ] && [ ${#out} = 40 ]; then
  record_pass "a git warning stays off the channel the hash is read from"
else
  record_fail "a git warning stays off the channel the hash is read from (exit $status, ${#out} chars)"
fi

# The other half of the same choice: when the walk fails, git's own account has to reach the caller, or a
# skill reports "could not fingerprint" with nothing to act on. An embedded repo with no commit is the
# portable way to fail `add -A`, with no chmod, so this runs as any user, root included.
new_repo
git -C "$repo" init -q uncommitted-embedded
run "$repo"
if [ "$status" = 2 ] && printf '%s' "$out" | grep -q 'does not have a commit checked out' &&
  printf '%s' "$out" | grep -q 'could not fingerprint the tree'; then
  record_pass "a failed fingerprint carries git's own reason as well as this script's"
else
  record_fail "a failed fingerprint carries git's own reason (exit $status, got '$out')"
fi

# Git applies ignore rules only to paths the index does not already hold, so an index built from
# nothing reads a tracked file that matches one as untracked and drops it. The file could then be
# rewritten between two runs with the hash unmoved, which is a stale ledger passing as a resume point.
# The un-ignored edit is the positive control: without it a fingerprint that read nothing at all would
# satisfy the first half on its own.
new_repo
printf '*.log\n' >"$repo/.gitignore"
printf 'one\n' >"$repo/kept.log"
git -C "$repo" add .gitignore && git -C "$repo" add -f kept.log && git -C "$repo" commit -qm ignore-logs
run "$repo"
before_tracked_ignored="$out"
printf 'two\n' >"$repo/kept.log"
run "$repo"
after_tracked_ignored="$out"
printf 'three\n' >"$repo/tracked.txt"
run "$repo"
after_control="$out"
if [ "$status" = 0 ] && [ -n "$before_tracked_ignored" ] &&
  [ "$after_tracked_ignored" != "$before_tracked_ignored" ] &&
  [ "$after_control" != "$after_tracked_ignored" ]; then
  record_pass "a tracked file matching an ignore rule still changes the fingerprint"
else
  record_fail "a tracked file matching an ignore rule still changes the fingerprint (clean '$before_tracked_ignored', edited '$after_tracked_ignored', control '$after_control')"
fi

# A repository with no commit has nothing tracked and so nothing to seed the index from. Refusing there
# would make the fingerprint unavailable on a tree whose files are all untracked, which is every repo on
# its first run.
case_number=$((case_number + 1))
unborn="$base/unborn$case_number"
mkdir -p "$unborn" && git -C "$unborn" init -q
printf 'only file\n' >"$unborn/loose.txt"
run "$unborn"
if [ "$status" = 0 ] && [ ${#out} = 40 ]; then
  record_pass "a repository with no commit still fingerprints its untracked files"
else
  record_fail "a repository with no commit still fingerprints (exit $status, got '$out')"
fi

# The other half: HEAD resolves and its tree cannot be read. Ignoring that would drop every committed
# file from the walk while the run still printed a hash and exited 0 — the same silent miss as above,
# arriving by a broken object store instead of an ignore rule.
new_repo
head_tree=$(git -C "$repo" rev-parse HEAD^{tree} 2>/dev/null)
loose_tree="$repo/.git/objects/${head_tree:0:2}/${head_tree:2}"
if [ -n "$head_tree" ] && [ -f "$loose_tree" ] && rm -f "$loose_tree"; then
  run "$repo"
  if [ "$status" = 2 ] && printf '%s' "$out" | grep -q 'could not read HEAD'; then
    record_pass "a HEAD that resolves but cannot be read refuses rather than fingerprinting without it"
  else
    record_fail "a HEAD that resolves but cannot be read refuses (exit $status, got '$out')"
  fi
else
  record_skip "a HEAD that resolves but cannot be read refuses rather than fingerprinting without it" \
    "HEAD's tree object is not loose here, so it cannot be removed to build the case"
fi


# The same property for this suite's own root, which no case about the script under test reaches.
# A corrupt `here` does not announce itself as one: it reports having measured something else.
#
# The resolve line is extracted from this file rather than written out again, so this reddens when
# the guard leaves the real line and not when the line is reworded. Driven by a relative invocation
# from the directory above, the only shape that consults CDPATH at all. The control is what stops an
# extraction that has stopped matching from letting the case below pass over an empty probe.
cdpath_line=$(grep -m1 '^here=' "$0")
if [ -n "$cdpath_line" ]; then
  record_pass "control: this suite's own root resolution was found, so the case below drives something"
else
  record_fail "control: this suite's own root resolution was found, so the case below drives something (no 'here=' line in $0)"
fi
mkdir -p "$base/cdpath-probe/scripts"
printf '#!/usr/bin/env bash\n%s\necho "$here"\n' "$cdpath_line" >"$base/cdpath-probe/scripts/probe.sh"
cdpath_lines=$( (cd "$base/cdpath-probe" && CDPATH=. bash scripts/probe.sh 2>/dev/null) | grep -c '')
if [ "$cdpath_lines" = "1" ]; then
  record_pass "CDPATH in the environment does not corrupt this suite's own root"
else
  record_fail "CDPATH in the environment does not corrupt this suite's own root (it came back $cdpath_lines line(s) long)"
fi

# Skips are counted in the line rather than folded into passed: a run that declined a case did not check
# it, and two machines reporting the same two numbers must not mean they checked different sets.
echo "$passed passed, $failed failed, $skipped skipped"
[ "$failed" = 0 ]
