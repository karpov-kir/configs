#!/usr/bin/env bash
# Cases for source-stamp.sh. What must not be weakened is which source a tool's stamp covers: a stamp
# blind to a file the tool compiles is a binary reported as current after that file changed. The cases
# below drive one tool per shape — a main of its own, and a main under cmd/ with its library beside
# it — against every directory in the fixture.
#
# Every path is built from a variable: a literal skill or tool path here would be read by the wiring
# check as a citation and reported against the real checkout.
set -u

# A fixture this suite could not build is a run that did not happen, never a stamper that is broken.
# `ai/run-tests.sh` reads exit 2 as "unmeasured" and anything else non-zero as `FAIL <suite>`, so a
# bare `exit 2` reports the right outcome and never which fixture went missing.
# See testing.md -> **What a suite reports**.
# No summary line follows it. The passes behind it are real, and the run they belong to is not.
nomeasure() { # <what could not be built>
  printf 'source-stamp-test.sh: %s — no verdict was reached, so this is not a result\n' "$1" >&2
  exit 2
}

here=$(CDPATH= cd -P "$(dirname "$0")" && pwd -P)
stamper="$here/source-stamp.sh"
base=$(mktemp -d) || nomeasure "could not create a temp directory to hold the fixtures"
trap 'rm -rf "$base"' EXIT

passed=0
failed=0
skipped=0

record_pass() {
  passed=$((passed + 1))
  echo "  pass  $1"
}

record_fail() {
  failed=$((failed + 1))
  echo "  FAIL  $1  — $2"
}

record_skip() { # <name> <why this machine cannot run it>
  skipped=$((skipped + 1))
  echo "  skip  $1  — $2"
}

# A refusal asserted on what it says, never on its exit code alone: every refusal this script has
# exits 2, so the code says one happened and never which. `command not found` is a stripped PATH
# killing the script before it reaches any check, which no case here ever means.
# --- shared:expect-refusal ---
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
# --- end shared:expect-refusal ---

# The module shape this repository has: a tool with its own main, a tool whose main sits under cmd/
# with its library beside it, and a package both of them compile against.
own_main=widget
cmd_main=gadget
shared=common
fixture="$base/tools"
mkdir -p "$fixture/$own_main" "$fixture/$cmd_main" "$fixture/cmd/$cmd_main" "$fixture/$shared" ||
  nomeasure "could not build the module fixture under $fixture"
cp "$stamper" "$fixture/source-stamp.sh" ||
  nomeasure "could not copy the stamper under test into $fixture"
chmod 755 "$fixture/source-stamp.sh" ||
  nomeasure "could not make the stamper under test executable"
printf 'module fixture\n\ngo 1.24\n' >"$fixture/go.mod"
printf 'package main\n\nfunc main() {}\n' >"$fixture/$own_main/main.go"
printf 'package main\n\nfunc main() {}\n' >"$fixture/cmd/$cmd_main/main.go"
printf 'package %s\n\nvar Value = 1\n' "$cmd_main" >"$fixture/$cmd_main/$cmd_main.go"
printf 'package %s\n\nvar Value = 1\n' "$shared" >"$fixture/$shared/$shared.go"
printf 'package main\n\nfunc TestNothing() {}\n' >"$fixture/$own_main/${own_main}_test.go"

stamp_of() { # <tool>
  "$fixture/source-stamp.sh" "$1" 2>/dev/null
}

# Every case below is one edit and one comparison, so the edit is undone before the next runs.
edit_then() { # <name> <file> <changed|unchanged> <tool>
  local name="$1" file="$2" want="$3" tool="$4" before after saved got
  before=$(stamp_of "$tool")
  saved=$(cat "$fixture/$file")
  printf '\nvar scratch = 2\n' >>"$fixture/$file"
  after=$(stamp_of "$tool")
  printf '%s\n' "$saved" >"$fixture/$file"
  if [ "$before" = "$after" ]; then got=unchanged; else got=changed; fi
  [ "$got" = "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "the stamp was $got, wanted $want"
}

echo "source-stamp.sh"

# The controls, first. Without a stamp that is computable and stable, every "unchanged" below passes
# on a script that always answers the same thing — including nothing at all.
out=$(stamp_of "$own_main")
if [ ${#out} -eq 64 ]; then
  record_pass "control: stamps a tool to 64 hex characters"
else
  record_fail "control: stamps a tool to 64 hex characters" "got '$out'"
fi
if [ "$(stamp_of "$own_main")" = "$out" ] && [ -n "$out" ]; then
  record_pass "control: the same source stamps the same twice"
else
  record_fail "control: the same source stamps the same twice" "two runs disagreed over '$out'"
fi
# The guard on the coverage set. A per-tool subset would make these two differ, and the way such a
# subset goes wrong is silent: it drops a directory the tool really imports and stops noticing edits
# there. Whatever narrows this again has to fail here first.
if [ "$(stamp_of "$cmd_main")" = "$out" ]; then
  record_pass "control: every tool in the module stamps the same source alike"
else
  record_fail "control: every tool in the module stamps the same source alike" \
    "$own_main answered '$out', $cmd_main answered '$(stamp_of "$cmd_main")'"
fi

# What the stamp has to see, or a binary built before the edit reports as current.
edit_then "an edit to the tool's own source changes its stamp" "$own_main/main.go" changed "$own_main"
edit_then "an edit to a shared package changes it" "$shared/$shared.go" changed "$own_main"
edit_then "an edit to go.mod changes it" "go.mod" changed "$own_main"
edit_then "a main under cmd/ is the tool's own source" "cmd/$cmd_main/main.go" changed "$cmd_main"
edit_then "and so is the library beside it" "$cmd_main/$cmd_main.go" changed "$cmd_main"

# The same edits seen from another tool. These moved the stamp only after the coverage set stopped
# guessing which directory belongs to whom: a library that cmd/ backs is still one this tool may
# import — eco-report imports tree-fingerprint, and cmd/tree-fingerprint backs it too — so a stamp
# that skipped it served the old binary and said nothing. Covering it costs a rebuild instead.
edit_then "a library another tool's cmd/ backs still moves it" "$cmd_main/$cmd_main.go" changed "$own_main"
edit_then "another tool's main under cmd/ moves it" "cmd/$cmd_main/main.go" changed "$own_main"
edit_then "another tool's own main moves it" "$own_main/main.go" changed "$cmd_main"
edit_then "a test file leaves it alone, since none reaches a binary" \
  "$own_main/${own_main}_test.go" unchanged "$own_main"

# A file added and a file removed. A digest over file contents alone would miss both, so the stamp
# folds in the names too.
before=$(stamp_of "$own_main")
printf 'package main\n\nvar Added = 1\n' >"$fixture/$own_main/added.go"
after=$(stamp_of "$own_main")
rm -f "$fixture/$own_main/added.go"
if [ "$before" != "$after" ]; then
  record_pass "a source file added to the tool changes its stamp"
else
  record_fail "a source file added to the tool changes its stamp" "the stamp stayed '$before'"
fi
if [ "$(stamp_of "$own_main")" = "$before" ]; then
  record_pass "control: and removing it again restores the stamp"
else
  record_fail "control: and removing it again restores the stamp" "wanted '$before', got '$(stamp_of "$own_main")'"
fi

# A release is stamped on Linux and read on macOS, on whichever of the two hashers that machine has.
# The stamp is a digest over their own output, so the whole scheme rests on the two writing a digest
# and a name identically. If that ever stops holding, every install warns about binaries that are
# perfectly current.
both_hashers="$base/both"
mkdir -p "$both_hashers" || nomeasure "could not build the PATH fixture at $both_hashers"
missing=""
# Exactly what source-stamp.sh calls, and nothing spare: an entry nothing uses would let a new
# dependency land silently, and it would also skip this case on a machine the stamper runs fine on.
for needed in bash dirname find sort cut; do
  ln -s "$(command -v "$needed")" "$both_hashers/$needed" 2>/dev/null || missing="$missing $needed"
done
for candidate in shasum sha256sum; do
  command -v "$candidate" >/dev/null 2>&1 || missing="$missing $candidate"
done
if [ -n "$missing" ]; then
  record_skip "shasum and sha256sum stamp the same source alike" \
    "this machine has no$missing, so the two cannot be compared here"
else
  only_shasum="$base/only-shasum"
  only_sha256sum="$base/only-sha256sum"
  cp -R "$both_hashers" "$only_shasum" || nomeasure "could not build the shasum-only PATH fixture"
  cp -R "$both_hashers" "$only_sha256sum" || nomeasure "could not build the sha256sum-only PATH fixture"
  ln -s "$(command -v shasum)" "$only_shasum/shasum"
  ln -s "$(command -v sha256sum)" "$only_sha256sum/sha256sum"
  with_shasum=$(PATH="$only_shasum" "$fixture/source-stamp.sh" "$own_main" 2>&1)
  with_sha256sum=$(PATH="$only_sha256sum" "$fixture/source-stamp.sh" "$own_main" 2>&1)
  if [ ${#with_shasum} -ne 64 ]; then
    record_fail "shasum and sha256sum stamp the same source alike" \
      "shasum alone did not stamp: $with_shasum"
  elif [ "$with_shasum" = "$with_sha256sum" ]; then
    record_pass "shasum and sha256sum stamp the same source alike"
  else
    record_fail "shasum and sha256sum stamp the same source alike" \
      "shasum said '$with_shasum', sha256sum said '$with_sha256sum'"
  fi
fi

# A machine with neither cannot stamp, and says so rather than answering with an empty stamp that
# every comparison would then read as a mismatch.
no_hasher="$base/no-hasher"
cp -R "$both_hashers" "$no_hasher" || nomeasure "could not build the no-hasher PATH fixture"
out=$(PATH="$no_hasher" "$fixture/source-stamp.sh" "$own_main" 2>&1)
status=$?
expect_refusal "a machine with no sha256 tool exits 2" "no shasum or sha256sum"

# The refusals. Each of these would otherwise answer with a stamp of something other than what was
# asked for, and a wrong stamp reads as a stale binary on every run afterwards.
out=$("$fixture/source-stamp.sh" absent 2>&1)
status=$?
expect_refusal "a tool with no source exits 2" "no source for absent"

for bad in "../$own_main" "sub/$own_main" "$own_main;true" ""; do
  out=$("$fixture/source-stamp.sh" "$bad" 2>&1)
  status=$?
  expect_refusal "refuses the tool name '$bad'" "is not a tool name"
done

out=$("$fixture/source-stamp.sh" 2>&1)
status=$?
expect_refusal "no tool name exits 2" "usage: source-stamp.sh"

out=$("$fixture/source-stamp.sh" "$own_main" extra 2>&1)
status=$?
expect_refusal "a second argument exits 2" "usage: source-stamp.sh"

# stdout carries the stamp and nothing else, or resolve.sh compares a hash with a log line stuck to it.
out=$("$fixture/source-stamp.sh" "$own_main" 2>/dev/null)
lines=$(printf '%s\n' "$out" | wc -l | tr -d ' ')
if [ "$lines" = 1 ] && [ ${#out} -eq 64 ]; then
  record_pass "prints one line of stamp on stdout and nothing else"
else
  record_fail "prints one line of stamp on stdout and nothing else" "$lines line(s): $out"
fi

echo
echo "$passed passed, $failed failed, $skipped skipped"
[ "$failed" -eq 0 ]
