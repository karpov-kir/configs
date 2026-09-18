#!/usr/bin/env bash
# Cases specific to voice-baseline: it holds a tree to a recorded count and refuses in both
# directions. Every case drives a fixture root with a stub checker, so each one tests the comparison
# and the refusals. A rise and a fall are both driven: a suite proving only that a clean tree passes
# goes green against a script whose comparison is absent.
set -u

# `CDPATH=`: with it set in the environment, `cd` echoes where it landed for a relative path that
# lacks a dot prefix. `here` then comes back two lines long and `$script` points at a path the shell
# cannot find.
here=$(CDPATH= cd "$(dirname "$0")" && pwd)
script="$here/voice-baseline.sh"

# Exit 2, and it says why: a fixture root that cannot be created is a suite that did not measure,
# which run-tests.sh counts apart from a failure. Exit 1 there would claim the script under test is
# broken, which is a different claim and a false one.
base=$(mktemp -d) || {
  echo "voice-baseline-test: could not create a temporary directory — nothing was tested" >&2
  exit 2
}
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

# A fixture root carrying two instruction files, a baseline and a checker that answers from a table
# the case writes. The stub prints its summary on stderr, where the real checker prints it.
build_root() {
  local root="$1" alpha="$2" beta="$3"
  mkdir -p "$root/ai/kk-flavor/standards" "$root/ai/kk-flavor/skills/kk-edit/scripts"
  printf '# alpha\n' >"$root/ai/kk-flavor/standards/alpha.md"
  printf '# beta\n' >"$root/ai/kk-flavor/standards/beta.md"
  cat >"$root/ai/kk-flavor/skills/kk-edit/scripts/voice-check.sh" <<STUB
#!/usr/bin/env bash
case "\$*" in
  *alpha.md*) echo "voice-check.sh: instruction profile: $alpha finding(s) over 1 file(s)." >&2 ;;
  *beta.md*)  echo "voice-check.sh: instruction profile: $beta finding(s) over 1 file(s)." >&2 ;;
esac
STUB
  chmod +x "$root/ai/kk-flavor/skills/kk-edit/scripts/voice-check.sh"
}

write_baseline() {
  local root="$1"
  shift
  { echo "# fixture"; printf '%s\n' "$@"; } >"$root/ai/kk-flavor/voice-baseline.txt"
}

expect() {
  local name="$1" want_status="$2" want_text="$3" root="$4"
  local out status
  out=$("$script" "$root" 2>&1)
  status=$?
  if [ "$status" -ne "$want_status" ]; then
    record_fail "$name" "exit $status, wanted $want_status: $out"
    return
  fi
  if [ -n "$want_text" ] && [[ "$out" != *"$want_text"* ]]; then
    record_fail "$name" "output does not name \"$want_text\": $out"
    return
  fi
  record_pass "$name"
}

echo "voice-baseline.sh"

root="$base/on-the-line"
build_root "$root" 3 1
write_baseline "$root" "3 ai/kk-flavor/standards/alpha.md" "1 ai/kk-flavor/standards/beta.md"
expect "every file on its line passes" 0 "every one on its line" "$root"

root="$base/risen"
build_root "$root" 5 1
write_baseline "$root" "3 ai/kk-flavor/standards/alpha.md" "1 ai/kk-flavor/standards/beta.md"
expect "a file over its line is refused" 1 "over its baseline of 3" "$root"

root="$base/slack"
build_root "$root" 1 1
write_baseline "$root" "3 ai/kk-flavor/standards/alpha.md" "1 ai/kk-flavor/standards/beta.md"
expect "a file under its line is refused" 1 "under its baseline of 3" "$root"

root="$base/new-dirty"
build_root "$root" 2 0
write_baseline "$root" "0 ai/kk-flavor/standards/beta.md"
expect "a new file with findings is refused" 1 "no baseline line" "$root"

root="$base/new-clean"
build_root "$root" 0 0
write_baseline "$root" "0 ai/kk-flavor/standards/beta.md"
expect "a new file measuring zero is allowed" 0 "every one on its line" "$root"

root="$base/no-baseline"
build_root "$root" 1 1
expect "a missing baseline did not run" 2 "is missing" "$root"

root="$base/no-checker"
build_root "$root" 1 1
write_baseline "$root" "1 ai/kk-flavor/standards/alpha.md" "1 ai/kk-flavor/standards/beta.md"
chmod -x "$root/ai/kk-flavor/skills/kk-edit/scripts/voice-check.sh"
expect "a checker it cannot run did not run" 2 "not executable" "$root"

root="$base/regenerate"
build_root "$root" 4 2
write_baseline "$root" "9 ai/kk-flavor/standards/alpha.md" "9 ai/kk-flavor/standards/beta.md"
if out=$("$script" --regenerate "$root" 2>&1) && "$script" "$root" >/dev/null 2>&1; then
  if grep -q '^4 ai/kk-flavor/standards/alpha.md$' "$root/ai/kk-flavor/voice-baseline.txt"; then
    record_pass "--regenerate rewrites the file from what the tree measures"
  else
    record_fail "--regenerate rewrites the file from what the tree measures" \
      "alpha.md is not recorded at 4: $(cat "$root/ai/kk-flavor/voice-baseline.txt")"
  fi
else
  record_fail "--regenerate rewrites the file from what the tree measures" "$out"
fi

root="$base/regenerate-keeps-header"
build_root "$root" 1 1
write_baseline "$root" "9 ai/kk-flavor/standards/alpha.md"
"$script" --regenerate "$root" >/dev/null 2>&1
if head -1 "$root/ai/kk-flavor/voice-baseline.txt" | grep -q '^# fixture$'; then
  record_pass "--regenerate keeps the file's header"
else
  record_fail "--regenerate keeps the file's header" "the header is gone"
fi

echo "voice-baseline-test.sh: $passed passed, $failed failed"
[ "$failed" -eq 0 ]
