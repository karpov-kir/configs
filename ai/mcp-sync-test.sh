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

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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

check "the repo's own mcp.jsonc parses after stripping" "parses" \
  "$(strip_comments "$script_dir/mcp.jsonc" | jq -e . >/dev/null 2>&1 && echo parses || echo broken)"

check "control: and does not parse without it" "broken" \
  "$(jq -e . "$script_dir/mcp.jsonc" >/dev/null 2>&1 && echo parses || echo broken)"

echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
