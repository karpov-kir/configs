#!/usr/bin/env bash
# The pre-commit gate: every check this repo gates on, run only where the change could have moved it.
#   usage: gate.sh [--full] [--mutants] [--units] [--why <unit>]
#          (no flag)  the fast path — run what is stale, skip what is not, defer the mutation harnesses
#          --full     run everything from cold, ignoring and then refreshing every cached verdict
#          --mutants  settle the deferred mutation units, and nothing else
#          --units    print the unit table with each unit's freshness, and stop
#          --why      print the input files one unit is keyed on, and stop
#
# The full sweep runs about half an hour on a developer machine — 185s of `go test`, 160s of shell
# suites, and tens of minutes of mutation across the two harnesses, whose own headers carry their
# figures. Nobody runs that before committing, and a gate nobody runs is a gate whose failures nobody
# sees. It was already hiding one: under load a shell mutation run loses a mutant to its 120s
# watchdog, which suppresses the coverage check and leaves a sound case reading as unproven.
#
# Doing the same work faster does not reach the target. A process spawn on this class of machine costs
# 18-38ms and does not parallelise — 240 git spawns took 13.2s at one worker and 8.6s at twenty-four,
# so throughput saturates near 28-54 spawns a second whatever the concurrency. That ceiling predicts
# the sweep almost exactly: eco-report's 4310 git spawns are its whole 95s. The sweep is tens of
# thousands of spawns against the thousand or so that fit in half a minute, and no amount of caching,
# no wider -j and no cheaper fixture closes a gap that size. The only lever left is not running the
# work, so that is the one this script pulls.
#
# Skipping is sound, not a sample, because every check here is a pure function of a declared set of
# input files plus the toolchain: the same bytes through the same compiler give the same verdict. A
# unit whose inputs hash to what they hashed on the last green run has a verdict that is already
# known, so skipping it asserts nothing that was not measured. A unit whose inputs moved by a byte is
# run. Nothing here samples, times out or guesses.
#
# What it may never do, and how each is prevented:
#   - Report a pass for a unit it did not run and has no recorded verdict for. A cache miss runs.
#   - Resolve a unit to an empty input set. That is a rename or a typo silently narrowing the gate, so
#     it exits 2 and names the unit, the way run-tests.sh exits 2 when discovery finds no suites.
#   - Finish having resolved nothing at all. Also exit 2.
#   - Skip something quietly. Every run prints one line per unit, and the deferred mutation units get
#     their own block with the command that settles them.
#
# This is a fast path beside the full sweep, never instead of it: .github/workflows/gates.yml still
# runs every command from cold on every push, and `--full` is the same sweep on demand.
#
# tested by: gate-test.sh

set -uo pipefail
export LC_ALL=C

# This script's own absolute path, resolved BEFORE the cd below: `$0` may be relative and does not
# survive it. Invoked as `cd ai && bash gate.sh`, `$0` is `gate.sh`, which reads as nothing from the
# repository root. Two things need it — `--help` prints its usage out of it, and the key material
# below hashes it. The second is the dangerous one: an empty digest never changes, so the key would
# stop moving when this file does, and the gate would answer out of a cache its own verdict code no
# longer matches. That is the failure the hash was added to prevent.
case "$0" in
  /*) self="$0" ;;
  *) self="$(CDPATH= cd -P "$(dirname "$0")" && pwd -P)/$(basename "$0")" ;;
esac

# GATE_ROOT so a suite can drive this against a throwaway repository under mktemp. Every path below
# is relative to the root, and the input paths are resolved with `git ls-files` from it, so without
# this seam a suite could only test the gate by writing fixtures inside the checkout it gates.
here=$(CDPATH= cd -P "${GATE_ROOT:-$(dirname "$0")/..}" && pwd -P) || exit 2
cd "$here" || exit 2

# Here, not at the run loop: building the unit table builds go-mutate, lists both harnesses' units and
# hashes every input, which a driver measured at 5-10s. Started later, the closing line reported 8s
# where the human had waited 18.
started=$(date +%s)
mode=fast
why_unit=""
check_path=""
while [ $# -gt 0 ]; do
  case "$1" in
    --full) mode=full ;;
    --mutants) mode=mutants ;;
    --units) mode=units ;;
    --check-path)
      shift
      [ $# -gt 0 ] || {
        echo "gate.sh: --check-path needs a path" >&2
        exit 2
      }
      check_path="$1"
      mode=checkpath
      ;;
    --why)
      shift
      [ $# -gt 0 ] || {
        echo "gate.sh: --why needs a unit id — run --units for the list" >&2
        exit 2
      }
      why_unit="$1"
      mode=why
      ;;
    -h | --help)
      sed -n '2,8p' "$self" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "gate.sh: unknown argument '$1'" >&2
      exit 2
      ;;
  esac
  shift
done

# --- the machine ---

# `/usr/bin/git` on macOS is an xcrun shim in front of the git under the active developer directory,
# and going through it costs 2.06x the binary it resolves to — 37.9ms against 18.4ms, measured over
# three interleaved rounds of sixty spawns. Everything below is spawn-bound, so putting the real
# binary first on PATH is the single cheapest thing in this file. Only when it resolves to something
# else that is executable; anywhere without xcrun this is a no-op and PATH is left alone.
git_note="git: /usr/bin/git as found"
if resolved=$(xcrun -f git 2>/dev/null) && [ -x "$resolved" ]; then
  resolved_dir=$(dirname "$resolved")
  case ":$PATH:" in
    *":$resolved_dir:"*) ;;
    *)
      PATH="$resolved_dir:$PATH"
      export PATH
      git_note="git: $resolved, ahead of the xcrun shim (measured 2.06x cheaper per spawn)"
      ;;
  esac
fi

# A path or key that goes into a command string this file later evals. Anything outside this set —
# a space, a semicolon, a quote, a leading dash — stops being a filename and starts being syntax:
# a zero-byte `ai/a;true;#-test.sh` evals as `ai/run-tests.sh -s ai/a` then `true`, so the unit exits
# 0 and the gate writes a green record for a suite that never ran. The file's contents are empty, so
# nothing reviewing contents would see it; the executable part is the name.
#
# Refused rather than escaped, and refused at discovery rather than at use, so the gate fails closed
# the way its other refusals do and says which name it cannot handle. The interpolations are quoted
# where they are built too, so an injection needs both guards to fail.
safe_token() { # <what it is, for the message> <value>, exit 2 naming it if it cannot be safely named
  # An empty value matches neither pattern below and would sail through, having named no file at all
  # — the command it builds would then carry a bare flag with nothing after it.
  [ -n "$2" ] || {
    echo "gate.sh: an empty $1 names no file, so the gate refuses to build a command from it — nothing ran" >&2
    exit 2
  }
  case "$2" in
    -*)
      echo "gate.sh: $1 '$2' begins with a dash, which the command it goes into would read as an option — nothing ran" >&2
      exit 2
      ;;
  esac
  case "$2" in
    *[!A-Za-z0-9._/-]*)
      echo "gate.sh: $1 '$2' holds a byte the gate cannot safely put in a command, so it refuses to build one — nothing ran" >&2
      exit 2
      ;;
  esac
}

# Driven on its own so a suite can exercise the refusal without writing a hostile filename into this
# checkout — which is the only other way to reach it, and not a thing to leave lying in a repository.
if [ "$mode" = checkpath ]; then
  safe_token "path" "$check_path"
  echo "gate.sh: '$check_path' is a name the gate can safely build a command from"
  exit 0
fi

hasher=""
for candidate in "shasum -a 256" "sha256sum"; do
  if command -v "${candidate%% *}" >/dev/null 2>&1; then
    hasher="$candidate"
    break
  fi
done
[ -n "$hasher" ] || {
  echo "gate.sh: no shasum and no sha256sum on this machine, so no verdict can be keyed — nothing ran" >&2
  exit 2
}

command -v go >/dev/null 2>&1 || {
  echo "gate.sh: no go on this machine, so the Go half cannot be built or run — nothing ran" >&2
  exit 2
}

git_common=$(git rev-parse --git-common-dir 2>/dev/null) || {
  echo "gate.sh: this is not a git repository, so there is nothing to scope a change against — nothing ran" >&2
  exit 2
}
case "$git_common" in
  /*) ;;
  *) git_common="$here/$git_common" ;;
esac
# GATE_CACHE so a suite can point the whole run at its own fixture cache: these records decide what
# is skipped, and a suite writing into the developer's would make their next run skip a unit it never
# measured.
cache="${GATE_CACHE:-$git_common/eco-gate}"
mkdir -p "$cache" || exit 2

# The toolchain goes into every key. A verdict is a statement about these bytes under this compiler,
# and a Go release that changes what vet reports must not be answered out of the previous one's cache.
#
# And this file itself, because this file decides verdicts: run_gofmt is the gofmt check, run_gotest
# decides which packages are forced, and the exit-code reading below turns 0, 2 and 3 into passed,
# never-measured and refused. Left out, editing any of that leaves all 62 records fresh — the gate
# answering out of a cache its own verdict code no longer matches, which is the one thing its header
# swears it will not do. The price is deliberate: any edit here, a comment included, invalidates every
# record, so the next run re-runs the checks and re-defers the mutation units.
self_digest=$($hasher <"$self" 2>/dev/null | cut -d' ' -f1)
# Refused rather than defaulted: an empty digest is a key component that never changes, and every
# verdict keyed on it would then survive any edit to this file.
[ -n "$self_digest" ] || {
  echo "gate.sh: could not hash myself at '$self', so a verdict could not be keyed to the code deciding it — nothing ran" >&2
  exit 2
}
stamp="$(go version) | $(bash --version | head -n 1) | $(git --version) | gate $self_digest"

scratch=$(mktemp -d) || exit 2
trap 'rm -rf "$scratch"' EXIT

# --- the unit table ---

u_id=()
u_inputs=()
u_cmd=()
u_kind=()

# unit <id> <kind: check|mutation> <inputs, space-separated paths> <command>
unit() {
  u_id+=("$1")
  u_kind+=("$2")
  u_inputs+=("$3")
  u_cmd+=("$4")
}

# GATE_UNITS_FILE replaces the table below with one read from a file — id, kind, inputs, command,
# tab-separated. It is how gate-test.sh reaches the run loop, the cache and every refusal in seconds
# instead of by running the real suites, which are the very thing this script exists not to run.
units_from_file=""
if [ -n "${GATE_UNITS_FILE:-}" ]; then
  [ -f "$GATE_UNITS_FILE" ] || {
    echo "gate.sh: GATE_UNITS_FILE names $GATE_UNITS_FILE, which is not a file — nothing ran" >&2
    exit 2
  }
  units_from_file=1
  while IFS="$(printf '\t')" read -r fid fkind finputs fcmd; do
    [ -n "$fid" ] || continue
    unit "$fid" "$fkind" "$finputs" "$fcmd"
  done <"$GATE_UNITS_FILE"
fi

go_tree="ai/tools"
# The files the Go suites read from outside their own module. Go's test cache is keyed on the module
# and cannot see these, so a plain `go test` reports a cached pass over a changed template. The gate
# therefore keys on them itself and forces the packages that read them.
#
# Named file by file, from the literal `../../` constants in the suites, rather than by taking the
# whole tree each one sits in. eco-report's harness copies in exactly this one script out of
# kk-flavor, so keying on all of kk-flavor made editing score.sh force eco-report — 233s for a package
# that cannot read it. cite-graph looks like it reads kk-flavor too and does not: its
# `../../kk-flavor` is a string inside a fixture it writes, never a path it opens.
ext_flavor="ai/kk-flavor/scripts/tree-fingerprint.sh"
# The two directories the harness copies from: scripts/ for todo-gate.sh, templates/ for the report
# template. Directories rather than the two files, so a third thing copied in later is still keyed on
# — and not the whole skill, whose SKILL.md is prose no fixture reads.
ext_qualify="ai/skills/idsd-qualify/scripts ai/skills/idsd-qualify/templates"
ext_reduce="ai/skills/kk-reduce/stats.md"
ext_workflows=".github/workflows"
# workflows_test.go reads this file to pull `gotest_timeout` out of it and hold both workflows'
# -timeout to that number. Declared like the rest, or the guard against those three drifting apart is
# the one thing a `gotest_timeout` edit does not re-run: hashing this file into every key re-runs the
# unit, but the re-run still reaches Go's own cache, which cannot see a file outside the module and
# answers `ok (cached)`.
ext_gate="ai/gate.sh"

# The real unit table, as against the one GATE_UNITS_FILE supplies: the four Go checks, one unit per
# discovered shell suite, and one per mutated script from each harness's own listing.
discover_units() {
  unit gofmt check "$go_tree" 'run_gofmt'
  unit vet check "$go_tree" 'cd ai/tools && go vet ./...'
  unit gotest check "$go_tree $ext_flavor $ext_qualify $ext_reduce $ext_workflows $ext_gate" 'run_gotest'
  # --gate, because this unit's verdict has to be about the commit and nothing else. Without it the
  # check walks whatever sits on disk, gitignored files included, and two checkouts of one commit
  # disagree: a scratch record nobody staged turned this red in the install while every worktree of
  # the same commit was green.
  #
  # `.gitignore` is an input *because* of the flag. The rules decide which files the check judges, so
  # editing them moves this unit's verdict — and a verdict that moves without its key is the stale
  # green this whole script exists not to serve. ai/tools/.gitignore already rides the tools tree; the
  # root one is its own entry.
  unit wiring check "ai/skills ai/kk-flavor ai/tools .gitignore" 'ECO_TOOLS_BUILD=1 ai/skills/kk-ecosystem/scripts/check.sh --gate'

  # Every shell suite, discovered rather than listed — the rule ai/run-tests.sh lives by, so a suite
  # added later is gated without anyone remembering to register it here.
  #
  # A suite's inputs are itself, the script it covers, and ai/run-tests.sh. That last one for the same
  # reason the mutation units below take their harness: it decides what the suite's exit status and
  # summary line mean, so a change to it can flip this unit's verdict with neither the suite nor its
  # script moving a byte. The suites that drive a Go tool also take the tool tree, since a change there
  # moves what they observe.
  #
  # --cached --others --exclude-standard, the same query ai/run-tests.sh discovers by. Tracked files
  # alone would leave a suite someone just wrote ungated until they remembered to `git add` it — the
  # gate quietly narrowing itself, which is the failure this whole script is built not to have.
  suites=$(git ls-files --cached --others --exclude-standard -- '*-test.sh' | sort -u)
  [ -n "$suites" ] || {
    echo "gate.sh: discovery found no *-test.sh at all — read this as the gate broken, never as a clean run" >&2
    exit 2
  }
  for suite in $suites; do
    safe_token "suite" "$suite"
    sibling="${suite%-test.sh}.sh"
    inputs="$suite ai/run-tests.sh"
    [ -f "$sibling" ] && inputs="$inputs $sibling"
    if grep -qE 'tools/|resolve\.sh|eco-check|eco-report|eco-stats|cite-graph|rule-echo|ECO_TOOLS' "$suite" 2>/dev/null; then
      inputs="$inputs $go_tree"
    fi
    # Through run-tests.sh, never `bash $suite`: that file owns the reading of a suite's result — exit
    # 2 is "did not measure", and a suite exiting 0 having run no case at all is VACUOUS and a failure.
    # Run directly, a suite emptied to zero bytes exits 0 silently and the gate reported `ran ok`.
    unit "shell:$(basename "${suite%-test.sh}")" check "$inputs" "ai/run-tests.sh -s $(printf '%q' "$suite")"
  done

  # The mutation units, one per mutated script, each harness reporting its own. Nothing here restates
  # which mutants live where: that mapping has one home, in the harness's own list, and a copy of it
  # kept in this file is a copy that goes stale the first time a mutant moves.
  go_mutate_bin="ai/tools/go-mutate/go-mutate"
  (cd ai/tools && go build -o go-mutate/go-mutate ./go-mutate) 2>"$scratch/build.err" || {
    echo "gate.sh: go-mutate does not build, so its units cannot be listed and its verdicts cannot be trusted — nothing ran" >&2
    cat "$scratch/build.err" >&2
    exit 2
  }
  "$go_mutate_bin" -units >"$scratch/go-units" 2>"$scratch/go-units.err" || {
    echo "gate.sh: go-mutate could not list its units — nothing ran" >&2
    cat "$scratch/go-units.err" >&2
    exit 2
  }
  [ -s "$scratch/go-units" ] || {
    echo "gate.sh: go-mutate listed no units — read this as the harness broken, never as nothing to check" >&2
    exit 2
  }
  while IFS="$(printf '\t')" read -r mfile msuites mcount mpath; do
    [ -n "$mfile" ] || continue
    safe_token "mutant file" "$mfile"
    # The resolved path comes from the harness's own listing rather than being rebuilt here. The base a
    # mutant's `file` is relative to is the harness's to know, and a second copy of it here is a rename
    # away from a gate that resolves nothing.
    [ -n "$mpath" ] || {
      echo "gate.sh: the mutation harness listed $mfile with no resolved path, so the gate cannot say which file the unit is keyed on — nothing ran" >&2
      exit 2
    }
    target="${mpath#"$here"/}"
    inputs="$target ai/tools/go-mutate/mutants.go ai/tools/go-mutate/main.go"
    saved_ifs="$IFS"
    IFS=','
    for msuite in $msuites; do
      IFS="$saved_ifs"
      suite_dir="ai/tools/${msuite#./}"
      inputs="$inputs ${suite_dir%/}"
      IFS=','
    done
    IFS="$saved_ifs"
    # Keyed on the package-qualified path, never the basename: eco-check and eco-report both hold a
    # shell.go, and two units under one id would share one cache record — so running one would report
    # the other fresh over a file nothing had looked at.
    unit "mutants:go:${target#ai/tools/}" mutation "$inputs" "$go_mutate_bin -file $(printf '%q' "$mfile")"
  done <"$scratch/go-units"

  ai/shell-mutate.sh -l >"$scratch/sh-units" 2>"$scratch/sh-units.err" || {
    echo "gate.sh: shell-mutate.sh could not list its units — nothing ran" >&2
    cat "$scratch/sh-units.err" >&2
    exit 2
  }
  [ -s "$scratch/sh-units" ] || {
    echo "gate.sh: shell-mutate.sh listed no units — read this as the harness broken, never as nothing to check" >&2
    exit 2
  }
  while IFS="$(printf '\t')" read -r skey sscript ssuite scount; do
    [ -n "$skey" ] || continue
    safe_token "mutant key" "$skey"
    unit "mutants:shell:$skey" mutation \
      "${sscript#"$here"/} ${ssuite#"$here"/} ai/shell-mutate.sh" \
      "ai/shell-mutate.sh -k $(printf '%q' "$skey")"
  done <"$scratch/sh-units"
}

if [ -z "$units_from_file" ]; then
  discover_units
fi

total=${#u_id[@]}
[ "$total" -gt 0 ] || {
  echo "gate.sh: no units resolved at all — read this as the gate broken, never as a clean run" >&2
  exit 2
}

# The record's filename, which is not the id. Ids are package-qualified — `mutants:go:eco-check/
# shell.go` — and a `/` in one names a directory the cache does not have, so every write for a go
# mutation unit failed and `--mutants` reported a pass having recorded nothing. Nothing said so: the
# redirect's error went to stderr and the unit's exit status was still 0.
#
# Parameter expansion, not `tr`: this runs once per unit and there are 62 of them, so a pipeline here
# is 62 processes spent on string munging. At 18-38ms a spawn, that measured as a doubling of the fast
# path.
record_stem() { # <id>, sets record_stem_out
  record_stem_out="${1//[!A-Za-z0-9._-]/-}"
}
# The stem-and-id table the guard below reads falls out of the same pass. One redirect for the whole
# loop, rather than an append per unit.
u_stem=()
for ((i = 0; i < total; i++)); do
  record_stem "${u_id[$i]}"
  u_stem+=("$record_stem_out")
  printf '%s\t%s\n' "$record_stem_out" "${u_id[$i]}"
done >"$scratch/stems"
sort -o "$scratch/stems" "$scratch/stems"

# Two units that share a cache record share a verdict: running either would report the other fresh
# over inputs nothing had read. Asked about stems rather than ids, because the stem is the record's
# name. Identical ids always flatten to one stem, so an id check could never fire on its own.
duplicate_stems=$(cut -f1 "$scratch/stems" | uniq -d)
[ -z "$duplicate_stems" ] || {
  echo "gate.sh: these units share one cache record, so a verdict could not say which of them it belongs to — nothing ran" >&2
  for stem in $duplicate_stems; do
    ids=$(awk -F '\t' -v want="$stem" '$1 == want { print $2 }' "$scratch/stems" | sort -u | tr '\n' ' ')
    case "$(printf '%s\n' $ids | grep -c '')" in
      1) printf '    %s — carried by two units under one id\n' "$ids" >&2 ;;
      *) printf '    %s — different ids, one record name\n' "$ids" >&2 ;;
    esac
  done
  exit 2
}
# --- the commands the units run ---

run_gofmt() {
  local unformatted
  unformatted=$(cd ai/tools && gofmt -l .) || return 1
  [ -z "$unformatted" ] && return 0
  echo "gofmt would rewrite:" >&2
  echo "$unformatted" >&2
  return 1
}

# `go test` defaults to 10m, and eco-report runs past it on a loaded machine: 682s wall clock at
# the worst measured. The flag bounds each test binary, never the whole `./...` run. So the number
# is set against that one package, and nothing here caps the unit's total. 30m sits far above it on
# purpose: a timeout prints a goroutine dump that reads as a deadlock in os/exec, never as a slow
# test. A bound tight enough to fire under load would turn a busy machine into a false bug report.
gotest_timeout=30m

# Every `go test` this file runs goes through here, so the bound above cannot be tuned at one call
# site and left standing at the others.
bounded_go_test() { # <go test arguments>
  (cd ai/tools && go test -timeout "$gotest_timeout" "$@")
}

# Go's own cache is content-keyed over the module and is exactly right there: a warm `go test ./...`
# is 2.4s against 185s cold, and it does not cache failures — verified by appending a failing case and
# watching two consecutive runs both re-run and re-fail. What it cannot see is the files outside the
# module that the fixtures copy in, so those packages are forced with -count=1 whenever one moved.
run_gotest() {
  local groups="" status=0 module rest dir
  changed_since_green "$ext_flavor" && groups="$groups eco-report"
  changed_since_green "$ext_qualify" && groups="$groups eco-report"
  changed_since_green "$ext_reduce" && groups="$groups eco-stats"
  changed_since_green "$ext_workflows" && groups="$groups ."
  changed_since_green "$ext_gate" && groups="$groups ."
  if [ -z "$groups" ]; then
    bounded_go_test ./...
    return $?
  fi
  module=$(cd ai/tools && go list -m) || return 1
  for dir in $groups; do
    if [ "$dir" = "." ]; then printf '%s\n' "$module"; else printf '%s\n' "$module/$dir"; fi
  done | sort -u >"$scratch/forced"
  (cd ai/tools && go list ./...) | sort >"$scratch/allpkgs"
  # The forced packages come OUT of the cached run rather than being run in both. Left in, the whole
  # of eco-report is measured twice — 177s and then 194s on the cold run that found this.
  rest=$(comm -23 "$scratch/allpkgs" "$scratch/forced" | tr '\n' ' ')
  echo "forcing $(grep -c '' <"$scratch/forced") package(s) with -count=1: an input outside the module moved, and the Go cache cannot see those" >"$scratch/note"
  if [ -n "$(printf '%s' "$rest" | tr -d ' ')" ]; then
    bounded_go_test $rest || status=1
  fi
  bounded_go_test -count=1 $(tr '\n' ' ' <"$scratch/forced") || status=1
  return "$status"
}

# --- keys ---

# Every input file of every unit, hashed once. One pass over the union rather than one per unit: the
# units overlap heavily, and hashing ai/tools once instead of forty times is the difference between a
# key computation that is free and one that is the slowest thing here.
all_inputs=""
for ((i = 0; i < total; i++)); do
  all_inputs="$all_inputs ${u_inputs[$i]}"
done
# --cached and --others: a new file that is not yet added still changes what the suites see, and a
# gate that keys only on tracked files reports a cached pass over a test someone just wrote.
git ls-files -z --cached --others --exclude-standard -- $all_inputs 2>/dev/null |
  sort -zu >"$scratch/paths.z"
if [ ! -s "$scratch/paths.z" ]; then
  echo "gate.sh: not one input path resolved to a file — nothing ran" >&2
  exit 2
fi
# Existing files only, since ls-files still lists one that was deleted but not yet staged. The path
# list itself goes into every key below, so a deletion still moves the key it should move.
: >"$scratch/manifest"
tr '\0' '\n' <"$scratch/paths.z" >"$scratch/paths"
while IFS= read -r path; do
  [ -f "$path" ] && printf '%s\n' "$path"
done <"$scratch/paths" >"$scratch/present"
if [ -s "$scratch/present" ]; then
  # stderr kept, and the count checked: a file the hasher could not read used to vanish from the
  # manifest silently, which takes it out of every key that declared it — its edits then stop
  # invalidating anything, and the gate narrows itself exactly as its header swears it will not.
  tr '\n' '\0' <"$scratch/present" | xargs -0 $hasher >"$scratch/manifest" 2>"$scratch/hash.err"
  if [ -s "$scratch/hash.err" ]; then
    echo "gate.sh: the hasher could not read every declared input, so some file's changes would stop invalidating its unit — nothing ran" >&2
    sed 's/^/    /' "$scratch/hash.err" >&2
    exit 2
  fi
  hashed=$(grep -c '' <"$scratch/manifest")
  present=$(grep -c '' <"$scratch/present")
  [ "$hashed" -eq "$present" ] || {
    echo "gate.sh: $present input file(s) exist but only $hashed were hashed, so a key is short of what it declares — nothing ran" >&2
    exit 2
  }
fi

# The manifest-shaped lines under one of <paths>, each `<hash>  <path>`, sorted so a key does not move
# with the order the hasher happened to answer in. Both callers go through this one comparison now:
# writing it twice is what let `changed_since_green` take a single path while its caller passed two
# — it then matched nothing, read as "always changed", and forced eco-report on every stale gotest.
lines_under() { # <manifest file> <paths>
  local path
  for path in $2; do
    printf '%s\n' "${path%/}"
  done >"$scratch/want"
  awk 'NR == FNR { want[$0] = 1; next }
       { line = $0; sub(/^[0-9a-f]+[ \t]+/, "", line)
         for (w in want) {
           if (line == w || index(line, w "/") == 1) { print $0; break }
         }
       }' "$scratch/want" "$1" | sort
}

# The input lines one unit is keyed on.
unit_lines() { # <inputs>
  lines_under "$scratch/manifest" "$1"
}

# One unit's key file: three header lines, then the sorted `<hash>  <path>` line per input file.
# Built once per unit and reused, because every caller below wants some part of it and each rebuild
# is another awk over the whole manifest — four execs per unit, and on a spawn-bound machine that is
# the gate's own largest cost.
#
# Sets three globals rather than printing: a command substitution is a fork, and there are enough
# units here that forking three times each is measurable against the run it is trying to shorten.
keyfile=""
keyfile_has_inputs=0
keyfile_key=""
ensure_keyfile() { # <index>
  keyfile="$scratch/key.$1"
  if [ ! -f "$keyfile" ]; then
    {
      printf '%s\n%s\n%s\n' "${u_id[$1]}" "${u_cmd[$1]}" "$stamp"
      unit_lines "${u_inputs[$1]}"
    } >"$keyfile"
    $hasher <"$keyfile" | cut -d' ' -f1 >"$keyfile.key"
  fi
  keyfile_key=""
  IFS= read -r keyfile_key <"$keyfile.key"
  # An empty key is a record name that says nothing about the inputs, so every run afterwards would
  # answer out of it whatever the tree did. There is no safe way to continue past it.
  [ -n "$keyfile_key" ] || {
    echo "gate.sh: could not hash the inputs of ${u_id[$1]}, so no verdict can be keyed to them — nothing ran" >&2
    exit 2
  }
  # Past the three header lines there is one line per input file that exists. None means the unit
  # resolved to nothing — read with the shell's own read, so asking costs no process at all.
  keyfile_has_inputs=0
  {
    read -r _
    read -r _
    read -r _
    read -r _ && keyfile_has_inputs=1
  } <"$keyfile"
}

# Whether one path's contents differ from what the last green `gotest` was keyed on. Read from that
# unit's own recorded input lines, so it answers about the same bytes the verdict was recorded over.
changed_since_green() { # <paths>
  local recorded="$cache/gotest.inputs"
  # Nothing recorded means nothing to compare against, so every group counts as moved. Conservative
  # in the only safe direction: it over-runs, it never skips.
  [ -f "$recorded" ] || return 0
  local now then
  now=$(lines_under "$scratch/manifest" "$1" | $hasher | cut -d' ' -f1)
  then=$(lines_under "$recorded" "$1" | $hasher | cut -d' ' -f1)
  [ "$now" != "$then" ]
}

# --- --units and --why stop here ---

if [ "$mode" = units ]; then
  printf '%-34s %-9s %-8s %s\n' UNIT KIND STATE INPUTS
  for ((i = 0; i < total; i++)); do
    state=stale
    ensure_keyfile "$i"
    [ -f "$cache/${u_stem[$i]}.$keyfile_key" ] && state=fresh
    printf '%-34s %-9s %-8s %s\n' "${u_id[$i]}" "${u_kind[$i]}" "$state" "${u_inputs[$i]}"
  done
  exit 0
fi

if [ "$mode" = why ]; then
  for ((i = 0; i < total; i++)); do
    if [ "${u_id[$i]}" = "$why_unit" ]; then
      echo "${u_id[$i]}  (${u_kind[$i]})"
      ensure_keyfile "$i"
      echo "  command: ${u_cmd[$i]}"
      echo "  key:     $keyfile_key"
      echo "  inputs:"
      unit_lines "${u_inputs[$i]}" | sed 's/^/    /'
      exit 0
    fi
  done
  echo "gate.sh: no unit is called '$why_unit' — run --units for the list" >&2
  exit 2
fi

# --- the run ---

# One unit's line in the run report. The column widths live here rather than at each of the eight
# states that print one. The duration rides in the detail rather than in a column of its own, because
# only some states have one.
unit_line() { # <state> <id> <detail>
  printf '  %-11s %-32s %s\n' "$1" "$2" "$3"
}

echo "$git_note"
echo "$total unit(s): $mode path"
echo

ran=0
fresh=0
deferred=0
failed=0
unmeasured=0
empty=0
deferred_ids=""

for ((i = 0; i < total; i++)); do
  id="${u_id[$i]}"
  kind="${u_kind[$i]}"
  # An input set that resolves to no file is a rename or a typo quietly narrowing the gate. It is the
  # one state this script treats as worse than a failure, because it looks exactly like a pass.
  ensure_keyfile "$i"
  if [ "$keyfile_has_inputs" -eq 0 ]; then
    unit_line "NO INPUTS" "$id" "declared: ${u_inputs[$i]}"
    empty=$((empty + 1))
    continue
  fi
  key="$keyfile_key"
  stem="${u_stem[$i]}"
  record="$cache/$stem.$key"

  if [ "$mode" != full ] && [ -f "$record" ]; then
    # A fresh hit says these exact inputs are green, so it can also repair the sidecar the forcing
    # above reads. Without this, one failed run deletes the sidecar and every later run over-forces:
    # a change to eco-check was measured re-running the whole of eco-report for 180s because the
    # sidecar was missing and every external group therefore read as moved.
    [ -f "$cache/$stem.inputs" ] || tail -n +4 "$keyfile" >"$cache/$stem.inputs"
    unit_line "fresh" "$id" "${key:0:12} — inputs unchanged since it last passed"
    fresh=$((fresh + 1))
    continue
  fi
  if [ "$kind" = mutation ] && [ "$mode" = fast ]; then
    unit_line "DEFERRED" "$id" "inputs moved — not run on the fast path"
    deferred=$((deferred + 1))
    deferred_ids="$deferred_ids $id"
    continue
  fi
  if [ "$kind" = check ] && [ "$mode" = mutants ]; then
    unit_line "not asked" "$id" "--mutants settles the mutation units only"
    continue
  fi

  at=$(date +%s)
  # A unit's own output is held back and shown only if it fails, so a unit that needs to explain a
  # cost while it is still passing writes a line here instead. Without it, gotest can spend two extra
  # minutes forcing packages and the run says only "ran ok".
  note="$scratch/note"
  rm -f "$note"
  # A subshell, because a unit's command may cd and several do: run in this shell, the first `cd
  # ai/tools` would silently relocate every unit after it and they would gate on the wrong tree.
  (eval "${u_cmd[$i]}") >"$scratch/out.$i" 2>&1
  unit_status=$?
  took=$(($(date +%s) - at))
  [ -s "$note" ] && sed 's/^/             /' "$note"
  ran=$((ran + 1))
  if [ "$unit_status" -eq 0 ]; then
    # Written only for a verdict this run actually observed, and the inputs it was observed over are
    # written beside it — that second file is what `changed_since_green` reads, and without it the
    # forcing above would have nothing to compare against.
    : >"$record"
    tail -n +4 "$keyfile" >"$cache/$stem.inputs"
    unit_line "ran ok" "$id" "${took}s"
    continue
  fi
  # Neither a record nor a pass, whichever way it went: a verdict recorded before someone broke the
  # code would answer for the broken tree the moment they reverted an unrelated file.
  rm -f "$record" "$cache/$stem.inputs"
  # Exit 2 is this repo's "it did not run" — a mutation suite the watchdog killed on a loaded machine,
  # a fixture that could not be built. Held apart from a failure: calling it one names the code for
  # something the machine did, and a gate whose exit code cannot say "I measured nothing here" is
  # exactly what this script exists not to be.
  if [ "$unit_status" -eq 2 ]; then
    unit_line "NO MEASURE" "$id" "${took}s  it exited 2 — it did not run, so nothing is known"
    sed 's/^/                  /' "$scratch/out.$i" | tail -n 10
    unmeasured=$((unmeasured + 1))
    continue
  fi
  # Exit 3 is `ai/run-tests.sh`'s "ran, and refuses its own result" — the checkout moved underneath
  # it. A suite writing into its own repository and another session editing a file look identical from
  # here. Read as a failure, it names the code for something a neighbouring agent did, which that
  # file's own comment calls the false diagnosis it exists to avoid. So it counts with the
  # non-measurements: not a pass, not a verdict on the code, and no record written either way.
  if [ "$unit_status" -eq 3 ]; then
    unit_line "REFUSED" "$id" "${took}s  it exited 3 — the checkout moved while it ran, so it refuses its own result"
    sed 's/^/                  /' "$scratch/out.$i" | tail -n 10
    unmeasured=$((unmeasured + 1))
    continue
  fi
  unit_line "FAILED" "$id" "${took}s"
  sed 's/^/                  /' "$scratch/out.$i" | tail -n 40
  failed=$((failed + 1))
done

# --- the report ---

if [ -n "$deferred_ids" ]; then
  echo
  echo "DEFERRED — these have inputs that moved, and the fast path did not run them:"
  for id in $deferred_ids; do
    echo "    $id"
  done
  echo "  Mutation is a statement about whether the suites can fail, not about this change, and on this"
  echo "  machine it costs minutes per script. CI runs the full sweep on every push. To settle them here:"
  echo "      ai/gate.sh --mutants"
fi

echo
printf '%s unit(s): %s ran, %s fresh from cache, %s deferred, %s failed, %s that never measured, %s with no inputs, %ss wall clock\n' \
  "$total" "$ran" "$fresh" "$deferred" "$failed" "$unmeasured" "$empty" "$(($(date +%s) - started))"

# A unit that resolved to nothing outranks everything: the gate does not know what it did not look at,
# so it may not call the run clean, and it may not call it a failure of the code either.
if [ "$empty" -gt 0 ]; then
  echo "$empty unit(s) resolved to no input file — the gate narrowed itself and cannot report on them. Exit 2, and this is not a pass." >&2
  exit 2
fi
if [ "$ran" -eq 0 ] && [ "$fresh" -eq 0 ]; then
  echo "nothing was measured and nothing was answered from cache — exit 2, and this is not a pass." >&2
  exit 2
fi
# A finding about the code outranks one about the machine — the reading shell-mutate.sh's own last
# lines take. Exit 2 alone means nothing was found wrong and something never ran, which a caller may
# never read as a pass.
[ "$failed" -eq 0 ] || exit 1
[ "$unmeasured" -eq 0 ] || {
  echo "$unmeasured unit(s) exited 2 without measuring — nothing is known about them, and this is not a pass." >&2
  exit 2
}
exit 0
