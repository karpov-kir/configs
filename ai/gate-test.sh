#!/usr/bin/env bash
# Cases for gate.sh. What is under test is not whether the checks pass — that is what the checks are
# for — but whether the gate can ever report clean over something it did not look at. So most of these
# are about the refusals: an input set that resolves to nothing, two units under one id, a cached
# verdict answering for a command that changed, and a failed unit leaving no record behind.
#
# The one that must not be weakened is "a unit that failed is run again next time". A record written
# for a red unit would answer for it the moment an unrelated file moved back, and the gate would then
# report fresh over code it had last seen broken. It is asserted by running a red unit twice and
# requiring the second run to run it rather than skip it.
#
# Every fixture is a throwaway git repository under one mktemp root, reached through GATE_ROOT, with
# GATE_CACHE and GATE_UNITS_FILE pointing at the same root. No fixture is written inside the checkout
# the suite lives in, and nothing here reads or writes the developer's own gate cache — a suite that
# wrote into it would make their next real run skip a unit this suite invented.
#
# A few cases are the exception: discovery, the two harness-listing parsers and the self-key are all
# things the GATE_UNITS_FILE seam bypasses by design, so the only way to see them is to run the real
# thing. Those cases drive `gate.sh` against this checkout. It is read-only apart from the gitignored
# harness binary the gate builds on any run, and it writes no cache record.
set -u

here=$(CDPATH= cd -P "$(dirname "$0")" && pwd -P)
# The real checkout, for the three cases that have to drive the gate against it rather than against a
# fixture. Named apart from `repo`, which is always the throwaway repository of the case in hand.
checkout="$here/.."
gate="${GATE_UNDER_TEST:-$here/gate.sh}"
[ -x "$gate" ] || {
  echo "gate-test: $gate is not an executable file — nothing was tested" >&2
  exit 2
}

base=$(mktemp -d) || {
  echo "gate-test: could not create a temporary directory — nothing was tested" >&2
  exit 2
}
trap 'rm -rf "$base"' EXIT

# The developer's own git config moves what `git ls-files --others --exclude-standard` answers: a
# global core.excludesFile that happens to ignore the fixture extension would empty every input set
# and turn each case below into the NO INPUTS refusal it is trying to tell apart from a pass.
export GIT_CONFIG_NOSYSTEM=1
export HOME="$base/home"
export XDG_CONFIG_HOME="$base/config"
mkdir -p "$HOME" "$XDG_CONFIG_HOME"

passed=0
failed=0
skipped=0
repo=""
cache=""
units=""
out=""
status=0
fixture=0

record_pass() {
  passed=$((passed + 1))
  echo "  pass  $1"
}

record_fail() {
  failed=$((failed + 1))
  echo "  FAIL  $1  —  $2"
}

# A declined case is counted, not just printed. ai/run-tests.sh reads that count, and a skip that only
# prints leaves this suite's last line with two fields — which asserts that every case here runs
# everywhere (`~/.kk-flavor/standards/testing.md` → **7. What a suite reports**). One case does not,
# and on a machine that declines it nothing a caller reads would say so.
record_skip() { # <count of cases the guarded block holds> <case> <why>
  skipped=$((skipped + $1))
  echo "  skip  $2  —  $3"
}

# A fresh fixture repository, its own cache and its own unit table. Committed, because the gate reads
# `git rev-parse --git-common-dir` to find where its records live and needs a real repository to do it.
new_repo() {
  fixture=$((fixture + 1))
  repo="$base/r$fixture"
  cache="$base/c$fixture"
  units="$base/u$fixture"
  mkdir -p "$repo" "$cache" || exit 2
  git -C "$repo" init -q || exit 2
  : >"$units"
}

# The same repository holding the one input file most cases key a unit on. A case that needs its
# inputs to say something of their own writes them itself.
new_watched_repo() {
  new_repo
  echo x >"$repo/watched.txt"
}

# unit <id> <kind> <inputs> <command>
add_unit() {
  printf '%s\t%s\t%s\t%s\n' "$1" "$2" "$3" "$4" >>"$units"
}

run_gate() {
  out=$(GATE_ROOT="$repo" GATE_CACHE="$cache" GATE_UNITS_FILE="$units" "$gate" "$@" 2>&1)
  status=$?
}

expect_status() { # <case> <wanted status>
  [ "$status" -eq "$2" ] &&
    record_pass "$1" ||
    record_fail "$1" "exited $status, want $2; output was: $(printf '%s' "$out" | tr '\n' '/')"
}

expect_out() { # <case> <substring>
  case "$out" in
    *"$2"*) record_pass "$1" ;;
    *) record_fail "$1" "no '$2' in: $(printf '%s' "$out" | tr '\n' '/')" ;;
  esac
}

expect_no_out() { # <case> <substring>
  case "$out" in
    *"$2"*) record_fail "$1" "'$2' is present and must not be: $(printf '%s' "$out" | tr '\n' '/')" ;;
    *) record_pass "$1" ;;
  esac
}

expect_file() { # <case> <path>
  [ -f "$2" ] &&
    record_pass "$1" ||
    record_fail "$1" "$2 is not there"
}

# A unit's recorded verdict, which is `<stem>.<key>` in the cache with a `.inputs` sidecar beside it.
# Asserted by its own name because exit 0 does not distinguish a unit that recorded from one that did
# not — deleting the write leaves this suite green everywhere except here.
expect_record() { # <case> <stem>
  if find "$cache" -maxdepth 1 -name "$2.*" ! -name '*.inputs' 2>/dev/null | grep -q .; then
    record_pass "$1"
  else
    record_fail "$1" "no record for $2 in $cache"
  fi
}

expect_no_file() { # <case> <path>
  [ -f "$2" ] &&
    record_fail "$1" "$2 is there and must not be" ||
    record_pass "$1"
}

# --- a unit runs, and is skipped once its inputs stop moving ---

new_repo
echo hello >"$repo/watched.txt"
# The command leaves a marker, which is how "it was skipped" is told from "it ran and printed the same
# thing". A verdict line alone reads identically either way.
add_unit ran check watched.txt "touch $repo/marker"
run_gate
expect_status "a unit with a real input runs, and the run is clean" 0
expect_out "and says it ran" "ran ok"
expect_file "and the command actually ran" "$repo/marker"

rm -f "$repo/marker"
run_gate
expect_status "a second run over unchanged inputs is still clean" 0
expect_out "and reports the unit fresh rather than run" "fresh"
expect_no_file "and does NOT run the command again" "$repo/marker"

echo moved >>"$repo/watched.txt"
run_gate
expect_out "an input that changed by a byte puts the unit back to run" "ran ok"
expect_file "and the command ran again" "$repo/marker"

# The key is over the command as well as the files. Without that, editing what a unit does would leave
# the old verdict answering for the new command.
rm -f "$repo/marker"
: >"$units"
add_unit ran check watched.txt "touch $repo/marker2"
run_gate
expect_out "a unit whose command changed is run again, though its inputs did not" "ran ok"
expect_file "and it is the new command that ran" "$repo/marker2"

# --- a red unit is never recorded ---

new_watched_repo
add_unit red check watched.txt "touch $repo/red-ran; false"
run_gate
expect_status "a failing unit fails the gate" 1
expect_out "and is named as failed" "FAILED"
expect_no_file "and no record is written for a unit that failed" "$cache/red.inputs"
rm -f "$repo/red-ran"
run_gate
expect_status "and a second run over the same tree fails again" 1
expect_file "having really run it, not answered out of a record it never earned" "$repo/red-ran"

# Exit 2 from a unit is this repo's "it did not run" and must not be collapsed into a failure: the
# mutation suites exit 2 when the watchdog kills one on a loaded machine, and reading that as a red
# guard names the code for something the machine did. It must also not be recorded, or the next run
# would call it fresh over a measurement that never happened.
new_watched_repo
add_unit unmeasured check watched.txt "echo did not run; exit 2"
run_gate
expect_status "a unit exiting 2 exits the gate 2, not 1 and not 0" 2
expect_out "and is named as never measured, not as failed" "NO MEASURE"
expect_out "and refuses to call a run holding an unmeasured unit a pass" "this is not a pass"
expect_no_file "and no record is written for a unit that never measured" "$cache/unmeasured.inputs"
run_gate
expect_out "and a second run measures it again rather than calling it fresh" "NO MEASURE"

# A real failure still outranks a machine one, so a run holding both exits 1.
new_watched_repo
add_unit red check watched.txt "false"
add_unit unmeasured check watched.txt "exit 2"
run_gate
expect_status "a failure beside an unmeasured unit exits 1, not 2" 1

# A unit id is package-qualified and holds `/` and `:`, and a record is a file. Written straight into
# the cache, the `/` named a directory that does not exist: every write for a go mutation unit failed,
# `--mutants` reported a pass having recorded nothing, and the deferred list could never shrink. The
# redirect's error went to stderr and the unit still exited 0, so nothing said so.
new_watched_repo
add_unit "mutants:go:eco-check/shell.go" mutation watched.txt "touch $repo/mutants-ran"
run_gate --mutants
expect_status "a unit whose id holds a slash settles cleanly" 0
expect_file "and its command runs" "$repo/mutants-ran"
run_gate
expect_no_out "and it is recorded, so the next fast run no longer defers it" "DEFERRED"
expect_out "and reports it fresh instead" "fresh"

# Two ids that differ but flatten to one filename would share a record, which the id guard cannot see.
new_watched_repo
add_unit "mutants:go:a/b.go" check watched.txt "true"
add_unit "mutants:go:a:b.go" check watched.txt "true"
run_gate
expect_status "two ids that name one cache record exit 2" 2
expect_out "and say the two ids differ" "different ids, one record name"

# Exit 3 is ai/run-tests.sh's "ran, and refuses its own result": the checkout moved under it, which a
# suite corrupting its own repository and a neighbouring session editing a file look identical from.
# Folded into failures it names the code for something another agent did — the false diagnosis that
# file's own comment exists to avoid — so it is counted with the non-measurements instead.
new_watched_repo
add_unit refused check watched.txt "echo the checkout changed while the suites ran; exit 3"
run_gate
expect_status "a unit exiting 3 exits the gate 2, not 1" 2
expect_out "and is named as refusing its result, not as failed" "REFUSED"
expect_no_out "and is not reported as a failure of the code" "FAILED"
expect_no_file "and no record is written for a unit that refused its result" "$cache/refused.inputs"

# A discovered suite path is interpolated into a command string the gate evals. A file named
# `ai/a;true;#-test.sh` is listed by `git ls-files`, and evaluated it becomes `ai/run-tests.sh -s
# ai/a` then `true` — the unit exits 0 and a green record is written for a suite that never ran. The
# file's contents are empty, so nothing reading contents would see it; the executable part is its
# name. Driven through --check-path rather than by writing such a file into this checkout.
check_path() { # <case> <path> <wanted status>
  local got
  "$gate" --check-path "$2" >/dev/null 2>&1
  got=$?
  [ "$got" -eq "$3" ] &&
    record_pass "$1" ||
    record_fail "$1" "--check-path '$2' exited $got, want $3"
}
check_path "an ordinary suite path is accepted" "ai/mcp-env-test.sh" 0
check_path "a path carrying a semicolon is refused" 'ai/a;true;#-test.sh' 2
check_path "a path carrying a space is refused" "ai/has space-test.sh" 2
check_path "a path carrying a backtick is refused" 'ai/a`id`-test.sh' 2
check_path "a path carrying a dollar is refused" 'ai/a$(id)-test.sh' 2
check_path "a path beginning with a dash is refused" "-evil-test.sh" 2
check_path "--check-path with no path is refused" "" 2

# The gate hashes itself into every key, so that editing the code which decides verdicts invalidates
# them. `$0` is relative and does not survive the cd to the repository root, and an empty digest is a
# constant — a key that stops moving when this file does. Every invocation shape must agree on one
# non-empty key, and the shape that breaks it is `cd ai && bash gate.sh`.
key_here=$(GATE_ROOT="$checkout" "$gate" --why wiring 2>/dev/null | awk '/key:/{print $2}')
key_there=$(cd "$(dirname "$gate")" && GATE_ROOT="$checkout" bash "$(basename "$gate")" --why wiring 2>/dev/null | awk '/key:/{print $2}')
[ -n "$key_here" ] &&
  record_pass "the gate's own key is not empty" ||
  record_fail "the gate's own key is not empty" "it came back blank"
[ "$key_here" = "$key_there" ] &&
  record_pass "and is the same however the script was invoked" ||
  record_fail "and is the same however the script was invoked" "'$key_here' from the root, '$key_there' from ai/"

# An input the hasher cannot read used to disappear from the manifest, which takes its path out of
# every key that declared it — the file's edits then stop invalidating anything and the gate narrows
# itself silently. Skipped by name rather than failed where chmod does not restrict this user, since
# root reads anything.
#
# Two inputs, and only one of them unreadable, because that is the shape the silent narrowing needs. A
# unit declaring the unreadable file alone resolves to nothing once the file is dropped, and the gate
# exits 2 on its NO INPUTS refusal instead — so the status assertion below would hold over exactly the
# bug it is here to catch, and only the message would tell them apart.
new_watched_repo
echo y >"$repo/read-me-fine.txt"
add_unit ran check "watched.txt read-me-fine.txt" "true"
chmod 000 "$repo/watched.txt"
if [ -r "$repo/watched.txt" ]; then
  record_skip 2 "an unreadable input is refused rather than dropped" "chmod does not restrict this user"
else
  run_gate
  expect_status "an input the hasher cannot read exits 2" 2
  expect_out "and says a unit's changes would stop invalidating it" "stop invalidating"
fi
chmod 644 "$repo/watched.txt"

# Removing the record of a unit that just failed is reachable only in --full mode: on the fast path a
# unit runs only on a cache miss, so there is no record to remove. Without a case here, a mutation
# over that removal killed nothing. It guards a unit that was green and then broke from reading fresh.
new_watched_repo
add_unit flip check watched.txt "touch $repo/ran; [ -f $repo/ok ]"
touch "$repo/ok"
run_gate
expect_status "a unit that passes exits clean" 0
expect_record "and its verdict is written to the cache" flip

rm -f "$repo/ok" "$repo/ran"
run_gate --full
expect_status "and --full runs it again over the same inputs, where it now fails" 1
expect_file "having really run it rather than read its old record" "$repo/ran"

touch "$repo/ok"
rm -f "$repo/ran"
run_gate
expect_file "the failed full run dropped that record, so the next fast run measures again" "$repo/ran"

# --- the refusals ---

new_repo
echo x >"$repo/real.txt"
add_unit ghost check no-such-file.txt "touch $repo/ghost-ran"
run_gate
expect_status "a table whose every input is missing exits 2" 2
expect_out "and says nothing ran" "nothing ran"
expect_no_file "and nothing did" "$repo/ghost-ran"

new_repo
echo x >"$repo/real.txt"
add_unit solid check real.txt "true"
add_unit ghost check no-such-file.txt "touch $repo/ghost-ran"
run_gate
expect_status "one unit among others resolving to no file exits 2, not 0" 2
expect_out "and says which unit narrowed the gate" "NO INPUTS"
expect_out "and refuses to call a run with an unresolved unit a pass" "this is not a pass"
expect_no_file "and that unit's command is not run" "$repo/ghost-ran"

new_watched_repo
add_unit twice check watched.txt "true"
add_unit twice check watched.txt "true"
run_gate
expect_status "two units under one id exit 2" 2
expect_out "and name the id that is carried twice" "carried by two units under one id"
expect_out "and nothing is run" "nothing ran"

new_repo
run_gate
expect_status "a unit table holding no units at all exits 2" 2
expect_out "and reads as the gate broken, not as a clean tree" "never as a clean run"

new_repo
out=$(GATE_ROOT="$repo" GATE_CACHE="$base/c0" GATE_UNITS_FILE="$base/not-a-file" "$gate" 2>&1)
status=$?
expect_status "a unit table file that is not there exits 2" 2

new_watched_repo
add_unit ran check watched.txt "true"
run_gate --no-such-flag
expect_status "an unknown flag exits 2" 2

# --- mutation is deferred on the fast path and settled on demand ---

new_watched_repo
add_unit check-one check watched.txt "touch $repo/check-ran"
add_unit mutants:fake mutation watched.txt "touch $repo/mutants-ran"
run_gate
expect_status "a deferred mutation unit does not fail the gate" 0
expect_out "and is named as deferred" "DEFERRED"
expect_no_file "and is not run on the fast path" "$repo/mutants-ran"
expect_file "while the check units beside it do run" "$repo/check-ran"
expect_out "and the block says how to settle it" "gate.sh --mutants"

rm -f "$repo/check-ran"
run_gate --mutants
expect_status "--mutants settles it" 0
expect_file "and the mutation unit runs" "$repo/mutants-ran"
expect_no_file "and the check units are left alone" "$repo/check-ran"
run_gate
expect_no_out "a settled mutation unit is no longer deferred" "DEFERRED"

# --- --full ignores the cache ---

new_watched_repo
add_unit ran check watched.txt "touch $repo/marker"
# A mutation unit in the fixture, because the last assertion below is about mutation units and the
# case that stood here had none — it asserted the check unit's own marker twice, so a --full that
# went on deferring mutation units would have passed it.
add_unit mutants:fake mutation watched.txt "touch $repo/mutants-marker"
run_gate
rm -f "$repo/marker" "$repo/mutants-marker"
run_gate --full
expect_status "--full is clean over a green tree" 0
# The count, not a "ran ok" line: the mutation unit added above has no record either, so it prints one
# of those on any --full and a check unit answered out of the cache would sit behind it unseen.
expect_out "and runs a unit the fast path would have called fresh" "2 ran, 0 fresh from cache"
expect_file "and the command really ran" "$repo/marker"
expect_file "and mutation units run too rather than deferring" "$repo/mutants-marker"
expect_no_out "and nothing is deferred on the full path" "DEFERRED"

# --- the read-only modes ---

new_watched_repo
add_unit ran check watched.txt "true"
run_gate --units
expect_status "--units is clean" 0
expect_out "and reports the unit stale before anything has run" "stale"
run_gate
run_gate --units
expect_out "and fresh once it has passed" "fresh"

run_gate --why ran
expect_status "--why on a real unit is clean" 0
expect_out "and prints the file the unit is keyed on" "watched.txt"
run_gate --why nosuchunit
expect_status "--why on a unit that does not exist exits 2" 2

# The two harness listings, which the seam above cannot reach either — GATE_UNITS_FILE replaces the
# table, so every `mutants:` id in the fixture cases is fabricated and neither parser is ever run. The
# fourth column especially: it carries the resolved path a go mutation unit is keyed on, and a gate
# that misread it would key the unit on nothing and skip a file it never hashed.
sh_units=$(GATE_ROOT="$checkout" "$here/shell-mutate.sh" -l 2>/dev/null)
if [ -z "$sh_units" ]; then
  record_fail "shell-mutate.sh -l lists its scripts" "it printed nothing"
else
  bad=$(printf '%s\n' "$sh_units" | awk -F '\t' 'NF != 4 || $1 == "" || $2 == "" || $3 == "" || $4 == "" { n++ } END { print n + 0 }')
  [ "$bad" -eq 0 ] &&
    record_pass "shell-mutate.sh -l gives four non-empty fields per script" ||
    record_fail "shell-mutate.sh -l gives four non-empty fields per script" "$bad malformed line(s)"
fi

go_bin="$checkout/ai/tools/go-mutate/go-mutate"
(cd "$checkout/ai/tools" && go build -o go-mutate/go-mutate ./go-mutate) >/dev/null 2>&1
go_units=$([ -x "$go_bin" ] && "$go_bin" -units 2>/dev/null)
if [ -z "$go_units" ]; then
  record_fail "go-mutate -units lists its mutated files" "it printed nothing, or the harness would not build"
else
  bad=$(printf '%s\n' "$go_units" | awk -F '\t' 'NF != 4 || $1 == "" || $4 !~ /^\// { n++ } END { print n + 0 }')
  [ "$bad" -eq 0 ] &&
    record_pass "go-mutate -units gives four fields with an absolute resolved path" ||
    record_fail "go-mutate -units gives four fields with an absolute resolved path" "$bad malformed line(s)"
  absent=$(printf '%s\n' "$go_units" | cut -f4 | while IFS= read -r f; do [ -f "$f" ] || printf 'x'; done)
  [ -z "$absent" ] &&
    record_pass "and every resolved path it names is a file that exists" ||
    record_fail "and every resolved path it names is a file that exists" "some column-four path is not a file"
fi

# And the parsers themselves: every script and every mutated file the harnesses list must come back as
# a unit. A parser that dropped a line would leave that file ungated with nothing saying so.
gate_units=$(GATE_ROOT="$checkout" "$gate" --units 2>/dev/null)
gone=$(printf '%s\n' "$sh_units" | cut -f1 | while IFS= read -r k; do
  [ -n "$k" ] || continue
  printf '%s\n' "$gate_units" | grep -q "^mutants:shell:$k " || printf '%s ' "$k"
done)
[ -z "$gone" ] &&
  record_pass "every script shell-mutate.sh lists becomes a mutation unit" ||
  record_fail "every script shell-mutate.sh lists becomes a mutation unit" "no unit for:$gone"

listed_go=$(printf '%s\n' "$go_units" | grep -c '')
built_go=$(printf '%s\n' "$gate_units" | grep -c '^mutants:go:')
[ "$listed_go" -eq "$built_go" ] &&
  record_pass "and every file go-mutate lists becomes one too" ||
  record_fail "and every file go-mutate lists becomes one too" "$listed_go listed, $built_go units built"

# The discovery query, which the seam above cannot reach. A suite this repo holds and the gate does
# not is the gate narrowed to less than the repo gates on, and nothing else here would see it.
discovered=$(git -C "$checkout" ls-files --cached --others --exclude-standard -- '*-test.sh' | sort -u)
if [ -z "$discovered" ]; then
  echo "gate-test: no *-test.sh in this checkout at all — read this as discovery broken, never as a clean run" >&2
  exit 2
fi
# Reusing the listing the harness-parser cases already took. Each `--units` builds go-mutate and
# hashes every input, about fifteen seconds, and this suite is the slowest thing the gate runs.
listed=$(printf '%s\n' "$gate_units" | awk '$1 ~ /^shell:/ { print $1 }' | sort -u)
if [ -z "$listed" ]; then
  echo "gate-test: gate.sh --units named no shell unit at all — nothing was compared" >&2
  exit 2
fi
missing=""
for suite in $discovered; do
  want="shell:$(basename "${suite%-test.sh}")"
  case "
$listed" in
    *"
$want"*) ;;
    *) missing="$missing $want" ;;
  esac
done
[ -z "$missing" ] &&
  record_pass "every *-test.sh this repo holds has a unit in the gate" ||
  record_fail "every *-test.sh this repo holds has a unit in the gate" "no unit for:$missing"

echo
echo "$passed passed, $failed failed, $skipped skipped"
[ "$failed" -eq 0 ]
