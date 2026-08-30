#!/usr/bin/env bash
# Tests for mcp-sync.sh's strip_comments — the one piece deciding what jq parses, and the only part
# of that script reachable without calling `claude mcp` for real.
#   usage: mcp-sync-test.sh   # prints one line per case; exit 0 when all pass, 1 otherwise
#
# A change to strip_comments needs a case here. Two cases are the controls that make the rest mean
# something: a same-line `//` must survive so jq fails loudly rather than a comment quietly becoming
# config, and the repo's own mcp.jsonc must fail jq *without* the stripper.
# Sourcing mcp-sync.sh reaches the function without running the sync — the guard in that script is
# what makes sourcing safe.
set -uo pipefail
export LC_ALL=C

script_dir="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"

tmp="$(mktemp -d)" || exit 1
trap 'rm -rf "$tmp"' EXIT

# Sourcing happens before any case runs, so no case can guard what sourcing does. If the guard in
# mcp-sync.sh were ever inverted, the line below would run the sync itself — and on a developer
# machine that is the real MCP registry. Pointing the CLI at a throwaway config dir first makes that
# harmless by construction rather than by a case noticing afterwards.
export CLAUDE_CONFIG_DIR="$tmp/registry"

# shellcheck source=./mcp-sync.sh
. "$script_dir/mcp-sync.sh"

pass=0
fail=0

check() {
  local name="$1" expected="$2" actual="$3"
  if [ "$expected" = "$actual" ]; then
    echo "ok   — $name"
    pass=$((pass + 1))
  else
    echo "FAIL — $name"
    printf '       expected: %q\n' "$expected"
    printf '       actual:   %q\n' "$actual"
    fail=$((fail + 1))
  fi
}

printf '// a comment\n{"a": 1}\n' > "$tmp/lead.jsonc"
check "a line starting with // is blanked" \
  "$(printf '\n{"a": 1}')" "$(strip_comments "$tmp/lead.jsonc")"

printf '  \t// indented\n{"a": 1}\n' > "$tmp/indent.jsonc"
check "leading whitespace before // does not save it" \
  "$(printf '\n{"a": 1}')" "$(strip_comments "$tmp/indent.jsonc")"

printf '{"a": 1} // trailing\n' > "$tmp/trail.jsonc"
check "a // after JSON on the same line is left alone" \
  '{"a": 1} // trailing' "$(strip_comments "$tmp/trail.jsonc")"

if strip_comments "$tmp/trail.jsonc" | jq . >/dev/null 2>&1; then verdict=accepted; else verdict=rejected; fi
check "control: jq then rejects that file rather than guessing" "rejected" "$verdict"

printf '{\n  "url": "https://example.com/mcp"\n}\n' > "$tmp/url.jsonc"
check "https:// inside a value survives" \
  "https://example.com/mcp" "$(strip_comments "$tmp/url.jsonc" | jq -r .url)"

printf '// one\n{\n  // two\n  "a": 1\n}\n' > "$tmp/mixed.jsonc"
check "blanks rather than deletes, so jq's line numbers still point at the real line" \
  "$(wc -l < "$tmp/mixed.jsonc" | tr -d ' ')" "$(strip_comments "$tmp/mixed.jsonc" | wc -l | tr -d ' ')"

# The guard is what stops this file syncing for real, and no case can assert what sourcing already
# did — the fence above is what makes that safe. This checks the guard still returns, in a subshell
# with both tools off PATH and its own throwaway config dir, so the check itself cannot sync either.
guard="$(CLAUDE_CONFIG_DIR="$tmp/guard-config" PATH=/usr/bin:/bin bash -c '. "$1" && echo returned' _ "$script_dir/mcp-sync.sh" 2>/dev/null)"
check "control: sourcing stops at the guard, so running this file cannot sync" "returned" "$guard"

# The argument guard. Every run of this script writes the live MCP registry, so an argument it does
# not understand has to stop it rather than be ignored: `bash mcp-sync.sh --help`, run expecting
# usage text, performed a real registration instead. Driven on the same stripped PATH the guard case
# above uses, so neither arm can reach the `claude` CLI even if it regressed — and with the guard
# gone both runs fall through to the jq probe, which exits 1 with wording no arm here writes.
bare="$tmp/bare-path"
mkdir -p "$bare"
# `bash` because the runs below are launched through it, `sed` because the usage arm reads this
# script's own header with it. Neither `jq` nor `claude` is here, which is what keeps a regression
# contained: past the guard the run dies at the jq probe instead of reaching the registry.
for needed in bash sed; do
  ln -s "$(command -v "$needed")" "$bare/$needed"
done
arg_config="$tmp/arg-guard-config"

run_guarded() { # <arg>, over $out and $status
  out="$(CLAUDE_CONFIG_DIR="$arg_config" PATH="$bare" bash "$script_dir/mcp-sync.sh" "$1" 2>&1)"
  status=$?
}

held() { # <needle> — the needle back when $out holds it, the whole output when it does not
  case "$out" in
    *"$1"*) printf 'held' ;;
    *) printf '%s' "$out" ;;
  esac
}

run_guarded --not-an-argument
check "an unknown argument exits 2 rather than syncing" "2" "$status"
check "and names the argument it refused" "held" "$(held 'unknown argument --not-an-argument')"
check "and says nothing was synced" "held" "$(held 'Nothing was synced')"
# The control the status cannot give: the stripped PATH kills a script that reaches past the guard,
# and that refusal exits 1 rather than 2 — but only the wording says which door the run came out of.
check "control: and it is the guard refusing, not a command this PATH lacks" "clean" \
  "$(case "$out" in *'command not found'* | *': not found'* | *'jq is required'*) printf '%s' "$out" ;; *) printf 'clean' ;; esac)"

run_guarded --help
check "--help exits 0" "0" "$status"
check "and prints usage rather than performing the sync" "held" "$(held 'usage: mcp-sync.sh')"
check "and says the script takes no arguments" "held" "$(held 'takes no arguments')"

# The two lines above come from a `printf`. The rest of that arm is a line range out of the script's
# own header — a claim about a file's content held by two line numbers, which a line added above the
# range or a paragraph moved inside it turns into the wrong lines, or into none, with nothing here
# failing. Repeating the numbers here would move the rot rather than remove it, so both ends are
# pinned by content: the header line the range has to start at, and the note past it that has to stay
# out. The control is what stops the first of those comparing against an empty string, which every
# output contains — a pass over nothing, reported as a pass.
help_first="$(sed -n '2,$p' "$script_dir/mcp-sync.sh" | sed -n 's/^# \{0,1\}\(..*\)$/\1/p' | head -1)"
check "control: the header's opening line was found, so the case below compares something" "found" \
  "$([ -n "$help_first" ] && printf 'found' || printf 'empty')"
check "and prints the header's opening line, so the range still starts where it should" "held" "$(held "$help_first")"
check "and stops before the notes under it" "clean" \
  "$(case "$out" in *'tested by:'*) printf '%s' "$out" ;; *) printf 'clean' ;; esac)"

check "the repo's own mcp.jsonc parses after stripping" "parses" \
  "$(strip_comments "$script_dir/mcp.jsonc" | jq -e . >/dev/null 2>&1 && echo parses || echo broken)"

check "control: and does not parse without it" "broken" \
  "$(jq -e . "$script_dir/mcp.jsonc" >/dev/null 2>&1 && echo parses || echo broken)"

echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
