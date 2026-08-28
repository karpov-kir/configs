#!/usr/bin/env bash
# Cases for resolve.sh. The ones that must not be weakened are the three ways a tool fails to be
# reached: no binary and no source, no binary and no Go, and a binary that cannot be executed. Each
# has to exit 2, because these tools report findings and exit 0 with none is what clean looks like —
# a resolver that returned quietly would turn every unreachable tool into a clean bill of health.
#
# Every path below is built from a variable. A literal skill or tool path in this file would be read
# by the wiring check as a citation and reported against the real checkout, which is not what this
# suite is about.
set -u

here=$(cd "$(dirname "$0")" && pwd)
resolver="$here/resolve.sh"
base=$(mktemp -d) || exit 1
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

# A tools directory of the same shape as the real one: the resolver beside a go.mod, a package
# directory it can build, and a bin/ it writes into. Built per case so nothing carries.
tool=widget
new_tools() {
  local dir="$1"
  mkdir -p "$dir/$tool" || exit 1
  cp "$resolver" "$dir/resolve.sh" || exit 1
  chmod 755 "$dir/resolve.sh"
  printf 'module fixture\n\ngo 1.24\n' >"$dir/go.mod"
  printf 'package main\n\nfunc main() {}\n' >"$dir/$tool/main.go"
}

# A PATH holding what the resolver needs to reach its `go` check, and no `go`. Emptying PATH outright
# breaks `#!/usr/bin/env bash` before the script starts, and leaving out `dirname` kills it at
# self-resolution — either way it still exits 2, so the exit assertion would pass while proving
# nothing about `go`.
bare="$base/bare-path"
mkdir -p "$bare" || exit 1
for needed in bash dirname mkdir mv rm; do
  ln -s "$(command -v "$needed")" "$bare/$needed"
done

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
# macOS the temp directory is reached through a symlinked /var, and the resolver following it is the
# behaviour that lets a skill find its tools through the symlink it is mounted by.
one_real=$(CDPATH= cd -P "$one" && pwd -P)
case "$out" in
  "$one_real/bin/$tool") record_pass "and puts it in bin/ under the tools directory" ;;
  *) record_fail "and puts it in bin/ under the tools directory" "got $out, wanted $one_real/bin/$tool" ;;
esac

# The binary the case above built is now present, so this one proves the first branch is taken
# without a toolchain at all — which is the whole reason a release install needs no Go.
out=$(PATH="$bare" "$one/resolve.sh" "$tool" 2>&1)
status=$?
expect_status "reuses an existing binary with no Go on PATH" 0

# And the override still refuses, so the branch above is a preference rather than the only path.
out=$(PATH="$bare" ECO_TOOLS_BUILD=1 "$one/resolve.sh" "$tool" 2>&1)
status=$?
expect_status "ECO_TOOLS_BUILD=1 skips that binary and needs Go" 2
expect_out "and says the tool did not run" "did NOT run"

# Driven from a directory that is neither the repo nor any root under scan, against a checkout that
# ships no source and no binary. This is the shape a skill mounted from an incomplete checkout has.
two="$base/two"
mkdir -p "$two"
cp "$resolver" "$two/resolve.sh"
chmod 755 "$two/resolve.sh"
out=$(CDPATH= cd "$base" && "$two/resolve.sh" "$tool" 2>&1)
status=$?
expect_status "no binary and no source exits 2 from an unrelated cwd" 2
expect_out "and names both of the things that were missing" "ships neither"

# Source present, no toolchain. Distinct from the case above because the fix is different, and
# because this is the one that would otherwise read as clean on a machine without Go.
three="$base/three"
new_tools "$three"
out=$(PATH="$bare" "$three/resolve.sh" "$tool" 2>&1)
status=$?
expect_status "source but no Go exits 2" 2
expect_out "and says so rather than reporting clean" "unchecked, not clean"

# A half-finished install: the file arrived without its exec bit. Reported rather than built over,
# because building needs Go and would hide the broken install on the machine it was meant for.
four="$base/four"
new_tools "$four"
mkdir -p "$four/bin"
: >"$four/bin/$tool"
chmod 644 "$four/bin/$tool"
out=$("$four/resolve.sh" "$tool" 2>&1)
status=$?
expect_status "a non-executable binary exits 2" 2
expect_out "and says the install did not complete" "did not complete"

# A directory left where the binary goes. `-x` alone is true for a directory, so without the
# regular-file test this would be handed to the caller to exec.
five="$base/five"
new_tools "$five"
mkdir -p "$five/bin/$tool"
out=$("$five/resolve.sh" "$tool" 2>&1)
status=$?
expect_status "a directory where the binary goes exits 2" 2
expect_out "and says it is not a regular file" "not a regular file"

# A build that cannot succeed. The source is there and so is Go, so this is the branch that would
# otherwise print a path to a binary that was never written.
six="$base/six"
new_tools "$six"
printf 'package main\n\nfunc main() { this is not Go }\n' >"$six/$tool/main.go"
out=$("$six/resolve.sh" "$tool" 2>&1)
status=$?
expect_status "a build failure exits 2" 2
expect_out "and says the tool did not run" "did NOT run"
if [ -e "$six/bin/$tool" ]; then
  record_fail "and leaves no binary behind" "$six/bin/$tool exists after a failed build"
else
  record_pass "and leaves no binary behind"
fi

# Names that must never become a path. A tool name is a directory entry here, so anything that could
# climb out of the tools directory is refused before it is joined to one.
seven="$base/seven"
new_tools "$seven"
for bad in "../$tool" "sub/$tool" "$tool;true" ""; do
  out=$("$seven/resolve.sh" "$bad" 2>&1)
  status=$?
  expect_status "refuses the tool name '$bad'" 2
done

out=$("$seven/resolve.sh" 2>&1)
status=$?
expect_status "no tool name exits 2" 2

out=$("$seven/resolve.sh" "$tool" extra 2>&1)
status=$?
expect_status "a second argument exits 2" 2

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

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]
