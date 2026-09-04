#!/usr/bin/env bash
# Cases for resolve.sh. The ones that must not be weakened are the three ways a tool fails to be
# reached: no binary and no source, no binary and no Go, and a binary that cannot be executed. Each
# has to exit 2: these tools report findings, so a resolver that returned quietly would turn every
# unreachable tool into a clean bill of health.
#
# Every path below is built from a variable: a literal skill or tool path here would be read by the
# wiring check as a citation and reported against the real checkout.
set -u

# A fixture this suite could not build is a run that did not happen, never a resolver that is broken.
# `ai/run-tests.sh` reads any non-zero status that is not 2 as `FAIL <suite>`, so exiting 1 when
# `mktemp` fails reports resolve.sh as the thing at fault — the same misreading this suite's own
# subject exists to stop, one layer up. So: exit 2, and name the fixture.
# See testing.md → **What a suite reports**.
# No summary line follows it. The passes behind it are real, and the run they belong to is not a result.
nomeasure() { # <what could not be built>
  printf 'resolve-test.sh: %s — no verdict was reached, so this is not a result\n' "$1" >&2
  exit 2
}

here=$(CDPATH= cd -P "$(dirname "$0")" && pwd -P)
resolver="$here/resolve.sh"

# Most cases below build a real binary before they can assert anything about serving one, so an
# ambient `go` is a fixture like the temp directory is. Without it they fail one after another and
# the suite prints a summary full of FAILs — a machine missing a toolchain, reported as a broken
# resolver. The no-toolchain cases are unaffected: each strips `go` from PATH for itself, which is
# the state under test rather than the state of this machine.
command -v go >/dev/null 2>&1 ||
  nomeasure "there is no go on PATH, and the fixtures below have to build a binary before any case can assert on one"

base=$(mktemp -d) || nomeasure "could not create a temp directory to hold the fixtures"
trap 'rm -rf "$base"' EXIT

passed=0
failed=0

record_pass() {
  passed=$((passed + 1))
  echo "  pass  $1"
}

record_fail() {
  failed=$((failed + 1))
  echo "  FAIL  $1  — $2"
}

expect_status() {
  local name="$1" want="$2"
  [ "$status" -eq "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "exit $status, wanted $want — output: $out"
}

expect_out() {
  local name="$1" want="$2"
  case "$out" in
    *"$want"*) record_pass "$name" ;;
    *) record_fail "$name" "wanted '$want' in: $out" ;;
  esac
}

# Every refusal below goes through this rather than through expect_status. Each one of them exits 2,
# so the code says a refusal happened and never which — a case asserting only the code passes on
# whatever the fixture broke first, and reads as proof of the cause in its own name. The needle has to
# be wording no other refusal shares. `command not found` is a stripped PATH killing the script before
# it reaches any check at all, which the resolver itself never writes and no case ever means.
expect_refusal() { # <name> <the wording only this cause produces>, over $status and $out
  local name="$1" want="$2"
  if [ "$status" -ne 2 ]; then
    record_fail "$name" "exit $status, wanted 2 — output: $out"
    return
  fi
  case "$out" in
    *"command not found"* | *": not found"*)
      record_fail "$name" "a missing command produced this refusal, not '$want' — output: $out" ;;
    *"$want"*) record_pass "$name" ;;
    *) record_fail "$name" "wanted '$want' in: $out" ;;
  esac
}

# A tools directory shaped like the real one, built per case so nothing carries between them.
tool=widget
new_tools() {
  local dir="$1"
  mkdir -p "$dir/$tool" || nomeasure "could not build the tool directory in $dir"
  cp "$resolver" "$dir/resolve.sh" || nomeasure "could not copy the resolver under test into $dir"
  chmod 755 "$dir/resolve.sh"
  printf 'module fixture\n\ngo 1.24\n' >"$dir/go.mod"
  printf 'package main\n\nfunc main() {}\n' >"$dir/$tool/main.go"
}

# A PATH directory holding only the commands named, so a case can model a machine missing one of the
# others. Symlinks, so what runs is the real binary anywhere a case has not put a shim beside them.
new_path_dir() { # <directory> <commands the fixture keeps>
  local dir="$1" needed
  shift
  mkdir -p "$dir" || nomeasure "could not build the PATH fixture at $dir"
  for needed in "$@"; do
    ln -s "$(command -v "$needed")" "$dir/$needed"
  done
}

# A PATH holding what the resolver needs to reach its `go` check, and no `go`. An empty PATH breaks
# `#!/usr/bin/env bash` before the script starts and a missing `dirname` kills it at self-resolution;
# both still exit 2, which is why the cases that use it assert on the wording of the refusal.
bare="$base/bare-path"
# `find` is in here because this fixture models a machine with no Go toolchain, not one with no
# POSIX utilities: the staleness check asks `find` its whole question, and without it the resolver
# takes the "could not check" branch instead of the one these cases are about.
new_path_dir "$bare" bash dirname mkdir mv rm find

echo "resolve.sh"

# The negative control, first: with source and a toolchain the resolver does reach a binary. Without
# this every case below could pass on a resolver that only ever exits 2.
one="$base/one"
new_tools "$one"
out=$(CDPATH= cd "$base" && "$one/resolve.sh" "$tool" 2>&1)
status=$?
expect_status "builds from source when there is no binary yet" 0
if [ -x "$out" ]; then
  record_pass "and prints a path that is executable"
else
  record_fail "and prints a path that is executable" "not executable: $out"
fi
# Compared against the physically resolved directory, because that is what the resolver prints: on
# macOS the temp dir is reached through a symlinked /var, and following it is what lets a skill find
# its tools through its own mount symlink.
one_real=$(CDPATH= cd -P "$one" && pwd -P)
case "$out" in
  "$one_real/bin/$tool") record_pass "and puts it in bin/ under the tools directory" ;;
  *) record_fail "and puts it in bin/ under the tools directory" "got $out, wanted $one_real/bin/$tool" ;;
esac

# The binary the case above built is now present, so this one proves the first branch is taken with
# no toolchain at all: the whole reason a release install needs no Go.
out=$(PATH="$bare" "$one/resolve.sh" "$tool" 2>&1)
status=$?
expect_status "reuses an existing binary with no Go on PATH" 0

# And the override still refuses, so the branch above is a preference rather than the only path.
out=$(PATH="$bare" ECO_TOOLS_BUILD=1 "$one/resolve.sh" "$tool" 2>&1)
status=$?
expect_refusal "ECO_TOOLS_BUILD=1 skips that binary and needs Go" "go is not installed"
expect_out "and says the tool did not run" "did NOT run"

# A checkout that ships no source and no binary, driven from an unrelated cwd: the shape a skill
# mounted from an incomplete checkout has.
two="$base/two"
mkdir -p "$two"
cp "$resolver" "$two/resolve.sh"
chmod 755 "$two/resolve.sh"
out=$(CDPATH= cd "$base" && "$two/resolve.sh" "$tool" 2>&1)
status=$?
expect_refusal "no binary and no source exits 2 from an unrelated cwd" "ships neither"
# One glob over both halves, because a refusal naming only the binary reads as a bad install when the
# fix is a whole checkout.
case "$out" in
  *"no prebuilt binary at"*"no source at"*) record_pass "and names both of the things that were missing" ;;
  *) record_fail "and names both of the things that were missing" "output: $out" ;;
esac

# Source present, no toolchain. Distinct from the case above because the fix is different, and
# because this is the one that would otherwise read as clean on a machine without Go.
three="$base/three"
new_tools "$three"
out=$(PATH="$bare" "$three/resolve.sh" "$tool" 2>&1)
status=$?
expect_refusal "source but no Go exits 2" "go is not installed"
expect_out "and says so rather than reporting clean" "unchecked, not clean"

# A half-finished install: the file arrived without its exec bit, and is reported, not built over.
four="$base/four"
new_tools "$four"
mkdir -p "$four/bin"
: >"$four/bin/$tool"
chmod 644 "$four/bin/$tool"
out=$("$four/resolve.sh" "$tool" 2>&1)
status=$?
expect_refusal "a non-executable binary exits 2" "is not executable"
expect_out "and says the install did not complete" "did not complete"

# A directory left where the binary goes. `-x` alone is true for a directory, so without the
# regular-file test this would be handed to the caller to exec.
five="$base/five"
new_tools "$five"
mkdir -p "$five/bin/$tool"
out=$("$five/resolve.sh" "$tool" 2>&1)
status=$?
expect_refusal "a directory where the binary goes exits 2" "not a regular file"

# A build that cannot succeed. The source is there and so is Go, so this is the branch that would
# otherwise print a path to a binary that was never written.
six="$base/six"
new_tools "$six"
printf 'package main\n\nfunc main() { this is not Go }\n' >"$six/$tool/main.go"
out=$("$six/resolve.sh" "$tool" 2>&1)
status=$?
expect_refusal "a build failure exits 2" "did not build"
expect_out "and says the tool did not run" "did NOT run"
if [ -e "$six/bin/$tool" ]; then
  record_fail "and leaves no binary behind" "$six/bin/$tool exists after a failed build"
else
  record_pass "and leaves no binary behind"
fi

# Names that must never become a path, refused before they are joined to the tools directory. Each of
# these also fails to be a tool that exists, so the refusal has to be the name check's own: with that
# check deleted the resolver still exits 2 on all four, saying it ships neither source nor binary.
seven="$base/seven"
new_tools "$seven"
for bad in "../$tool" "sub/$tool" "$tool;true" ""; do
  out=$("$seven/resolve.sh" "$bad" 2>&1)
  status=$?
  expect_refusal "refuses the tool name '$bad'" "is not a tool name"
done

out=$("$seven/resolve.sh" 2>&1)
status=$?
expect_refusal "no tool name exits 2" "usage: resolve.sh"

out=$("$seven/resolve.sh" "$tool" extra 2>&1)
status=$?
expect_refusal "a second argument exits 2" "usage: resolve.sh"

# stdout carries the path and nothing else, or the caller execs a string with a build log in it.
eight="$base/eight"
new_tools "$eight"
out=$("$eight/resolve.sh" "$tool" 2>/dev/null)
status=$?
lines=$(printf '%s\n' "$out" | wc -l | tr -d ' ')
if [ "$status" -eq 0 ] && [ "$lines" = 1 ]; then
  record_pass "prints one line on stdout and sends the build log to stderr"
else
  record_fail "prints one line on stdout and sends the build log to stderr" "exit $status, $lines line(s): $out"
fi

# The staleness branch. What these assert is that the second resolve *rewrote* the binary: a marker
# dropped between the two runs, and the binary newer than it afterwards. Asserting the path came
# back would pass on the stale-serving resolver too.
rebuilt_since() {
  [ -n "$(find "$1" -newer "$2" -print 2>/dev/null)" ]
}

# The source edit three of the cases below turn on, written once so what differs between them is the
# fixture and nothing else.
edit_main() { # <fixture tools directory>
  printf 'package main\n\nfunc main() { _ = "edited" }\n' >|"$1/$tool/main.go"
}

# Edited in the tool's own package.
nine="$base/nine"
new_tools "$nine"
out=$("$nine/resolve.sh" "$tool" 2>/dev/null)
status=$?
expect_status "builds a first binary before the staleness cases" 0
nine_binary="$out"
touch "$base/nine-marker"
edit_main "$nine"
out=$("$nine/resolve.sh" "$tool" 2>/dev/null)
status=$?
expect_status "an edited source still resolves" 0
rebuilt_since "$nine_binary" "$base/nine-marker" &&
  record_pass "rebuilds rather than serving a binary older than its own package" ||
  record_fail "rebuilds rather than serving a binary older than its own package" "$nine_binary was not rewritten"

# Edited in a package the tool's own directory does not contain. This is the shape the module-wide
# scan exists for: a check scoped to `$tools/$tool` would serve the stale binary here.
ten="$base/ten"
new_tools "$ten"
mkdir -p "$ten/shell" || nomeasure "could not build the sibling package the module-wide scan is checked against"
printf 'package shell\n' >|"$ten/shell/shell.go"
out=$("$ten/resolve.sh" "$tool" 2>/dev/null)
status=$?
expect_status "builds a first binary with a sibling package present" 0
ten_binary="$out"
touch "$base/ten-marker"
printf 'package shell\n\nconst Edited = true\n' >|"$ten/shell/shell.go"
out=$("$ten/resolve.sh" "$tool" 2>/dev/null)
status=$?
expect_status "an edited sibling package still resolves" 0
rebuilt_since "$ten_binary" "$base/ten-marker" &&
  record_pass "rebuilds when a package outside the tool's own directory moved" ||
  record_fail "rebuilds when a package outside the tool's own directory moved" "$ten_binary was not rewritten"

# Stale with nothing to rebuild with. It is still served, so what these assert is that the doubt is
# audible on stderr.
eleven="$base/eleven"
new_tools "$eleven"
out=$("$eleven/resolve.sh" "$tool" 2>/dev/null)
status=$?
expect_status "builds a first binary before the no-toolchain case" 0
edit_main "$eleven"
out=$(PATH="$bare" "$eleven/resolve.sh" "$tool" 2>&1)
status=$?
expect_status "serves a stale binary when there is no Go to rebuild with" 0
expect_out "and says on stderr that it may not be the code you are reading" "may not be the code you are reading"
out=$(PATH="$bare" "$eleven/resolve.sh" "$tool" 2>/dev/null)
status=$?
lines=$(printf '%s\n' "$out" | wc -l | tr -d ' ')
# The status and the emptiness are checked alongside the count because `printf '%s\n' ""` is itself one
# line: on the count alone, a resolver that exited 2 having printed nothing would record a pass here.
[ "$status" -eq 0 ] && [ -n "$out" ] && [ "$lines" = 1 ] &&
  record_pass "and keeps that warning off stdout" ||
  record_fail "and keeps that warning off stdout" "exit $status, $lines line(s) on stdout: $out"

# And the question itself failing to be askable. A missing `find` is the one gap the staleness check
# cannot answer, so it has to say it did not check.
nofind="$base/nofind-path"
new_path_dir "$nofind" bash dirname mkdir mv rm
twelve="$base/twelve"
new_tools "$twelve"
out=$("$twelve/resolve.sh" "$tool" 2>/dev/null)
status=$?
expect_status "builds a first binary before the no-find case" 0
edit_main "$twelve"
out=$(PATH="$nofind" "$twelve/resolve.sh" "$tool" 2>&1)
status=$?
expect_status "serves the binary when find is missing" 0
expect_out "and says the staleness question was NOT checked" "was NOT checked"

# And the question asked but not answered. A `find` that runs and fails prints nothing, which is
# byte-for-byte what a clean tree looks like, so its exit status is the only thing that separates
# them. Without that read the resolver serves the stale binary in silence — measured on the resolver
# before the status was read: exit 0, empty stderr, over a source edited after the build.
#
# A shim rather than a broken tree: what has to be exercised is find answering badly, and a fixture
# that made a real find fail would be denying the suite's own directory to itself.
badfind="$base/badfind-path"
new_path_dir "$badfind" bash dirname mkdir mv rm
printf '#!/bin/sh\nexit 1\n' >|"$badfind/find"
chmod 755 "$badfind/find" || nomeasure "could not make the failing find shim executable"
thirteen="$base/thirteen"
new_tools "$thirteen"
out=$("$thirteen/resolve.sh" "$tool" 2>/dev/null)
status=$?
expect_status "builds a first binary before the failing-find case" 0
edit_main "$thirteen"
out=$(PATH="$badfind" "$thirteen/resolve.sh" "$tool" 2>&1)
status=$?
expect_status "serves the binary when find runs and fails" 0
expect_out "and says so rather than answering the question clean" "find failed here"

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]
