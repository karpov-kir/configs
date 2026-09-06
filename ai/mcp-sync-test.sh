#!/usr/bin/env bash
# Cases for mcp-sync.sh — the comment stripping, the @CONFIGS@ substitution, the argument guard, and
# the two refusals that stop a registration before it happens.
#   usage: mcp-sync-test.sh   # prints one line per case; exit 0 when all pass, 1 otherwise
#
# Sourcing mcp-sync.sh reaches its functions without running the sync, and the guard in that script
# is what makes sourcing safe. The cases past that fence drive a copy with a stubbed `claude`,
# because the real one writes the live MCP registry.
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
# gone both runs fall through and die at the first command this PATH lacks, exiting 1 with wording no
# arm here writes.
bare="$tmp/bare-path"
mkdir -p "$bare"
# `bash` because the runs below are launched through it, `sed` because the usage arm reads this
# script's own header with it. Nothing else is — `dirname` included — which is what keeps a
# regression contained: past the guard the run dies resolving its own directory, well before the jq
# probe and further still from the registry.
for needed in bash sed; do
  ln -s "$(command -v "$needed")" "$bare/$needed"
done
arg_config="$tmp/arg-guard-config"

run_guarded() { # <arg>, over $out and $status
  out="$(CLAUDE_CONFIG_DIR="$arg_config" PATH="$bare" bash "$script_dir/mcp-sync.sh" "$1" 2>&1)"
  status=$?
}

# --- shared:needle-tests ---
# Each reports its condition with a distinctive word, so a check that fails prints the whole output
# rather than a bare `no`. They are functions rather than inline `case`es because bash 3.2 — macOS's
# own, and one leg of the gates workflow — does not parse `$( )` so much as scan it for the first
# unbalanced `)`, and a `case` pattern supplies one: the substitution closes mid-pattern, holding an
# unfinished `case`. A function covers every construct with a bare `)`, not just `case`.
held() { # <needle> <text> — 'held' when the text holds the needle, the whole text when it does not
  case "$2" in
    *"$1"*) printf 'held' ;;
    *) printf '%s' "$2" ;;
  esac
}

lacked() { # <needle> <text> — 'lacked' when the text is free of the needle, the whole text otherwise
  case "$2" in
    *"$1"*) printf '%s' "$2" ;;
    *) printf 'lacked' ;;
  esac
}
# --- end shared:needle-tests ---

# Both doors past the guard, so the guard's refusal is told apart from a PATH that killed the script
# later. On the stripped PATH below the run dies at `dirname`, the first command it reaches for, so
# that is the `command not found`. The `jq is required` needle looks dead and stays: it is the door
# the run comes out of on any PATH that carries dirname, and dropping it would leave this control
# reading a deeper death as a clean refusal.
stopped_at_the_guard() { # <text> — 'stopped' when the text names no death past the guard, the text otherwise
  case "$1" in
    *'command not found'* | *': not found'* | *'jq is required'*) printf '%s' "$1" ;;
    *) printf 'stopped' ;;
  esac
}

run_guarded --not-an-argument
check "an unknown argument exits 2 rather than syncing" "2" "$status"
check "and names the argument it refused" "held" "$(held 'unknown argument --not-an-argument' "$out")"
check "and says nothing was synced" "held" "$(held 'Nothing was synced' "$out")"
# The control the status cannot give: the stripped PATH kills a script that reaches past the guard,
# and that refusal exits 1 rather than 2 — but only the wording says which door the run came out of.
check "control: and it is the guard refusing, not a command this PATH lacks" "stopped" \
  "$(stopped_at_the_guard "$out")"

run_guarded --help
check "--help exits 0" "0" "$status"
check "and prints usage rather than performing the sync" "held" "$(held 'usage: mcp-sync.sh' "$out")"
check "and says the script takes no arguments" "held" "$(held 'takes no arguments' "$out")"

# The two lines above come from a `printf`. The rest of that arm is a line range out of the script's
# own header — a claim about a file's content held by two line numbers, which a line added above the
# range or a paragraph moved inside it turns into the wrong lines, or into none, with nothing here
# failing. Repeating the numbers here would move the rot rather than remove it, so both ends are
# pinned by content: the header line the range has to start at, and the note past it that has to stay
# out.
#
# Both needles are read out of mcp-sync.sh, never written out here. A needle spelled literally is a
# claim about that file's prose, and prose gets reworded: rename the `tested by:` marker and the
# literal matches nothing, "the help lacks it" becomes true of every possible output, and the case
# passes forever without reaching its subject — no failure, and nothing saying it stopped measuring.
# Read from the file, a needle follows the rewording, and the controls below fail when it reads empty.
header_prose() { # <n> — the nth non-blank comment line of mcp-sync.sh's header, empty when there is none
  sed -n '2,$p' "$script_dir/mcp-sync.sh" | sed -n 's/^# \{0,1\}\(..*\)$/\1/p' | sed -n "${1}p"
}

help_first="$(header_prose 1)"
check "control: the header's opening line was found, so the case below compares something" "found" \
  "$([ -n "$help_first" ] && printf 'found' || printf 'empty')"
check "and prints the header's opening line, so the range still starts where it should" "held" "$(held "$help_first" "$out")"

# The help prints four of those lines, so the fifth is the first one it must not reach.
help_past="$(header_prose 5)"
check "control: the line past the printed range was found, so the case below compares something" "found" \
  "$([ -n "$help_past" ] && printf 'found' || printf 'empty')"
check "and stops before the notes under it" "lacked" \
  "$(lacked "$help_past" "$out")"

check "the repo's own mcp.jsonc parses after stripping" "parses" \
  "$(strip_comments "$script_dir/mcp.jsonc" | jq -e . >/dev/null 2>&1 && echo parses || echo broken)"

check "control: and does not parse without it" "broken" \
  "$(jq -e . "$script_dir/mcp.jsonc" >/dev/null 2>&1 && echo parses || echo broken)"

# What the claude CLI is handed is a literal string and it expands nothing, so a `@CONFIGS@` that
# survives registration is a server whose command does not exist.
check "the token becomes the directory" \
  '{"command": "/opt/kk/mcp-env.sh"}' \
  "$(substitute_configs_dir '{"command": "@CONFIGS@/mcp-env.sh"}' /opt/kk)"

# Every occurrence, not the first: mcp.jsonc carries one per stdio server.
check "every occurrence, not just the first" \
  "/a/x /a/y" \
  "$(substitute_configs_dir '@CONFIGS@/x @CONFIGS@/y' /a)"

check "text without the token is untouched" \
  '{"url": "https://example.com/mcp"}' \
  "$(substitute_configs_dir '{"url": "https://example.com/mcp"}' /opt/kk)"

# The reason this is a bash replacement and not sed: every sed delimiter is a character a path may
# hold, and `&` is the replacement's own back-reference there. None of them is special here.
check "a path holding sed's own metacharacters lands literally" \
  '/tmp/a&b$c*d e/mcp-env.sh' \
  "$(substitute_configs_dir '@CONFIGS@/mcp-env.sh' '/tmp/a&b$c*d e')"

for good in "/Users/kk/configs/ai" "/tmp/with space/ai" '/tmp/a&b$c*d/ai' "/tmp/ünïcodé/ai"; do
  check "a directory that substitutes: $good" "yes" \
    "$(configs_dir_is_substitutable "$good" && printf 'yes' || printf 'no')"
done

for bad in '/tmp/a"b/ai' '/tmp/a\b/ai'; do
  check "a directory that does not: $bad" "no" \
    "$(configs_dir_is_substitutable "$bad" && printf 'yes' || printf 'no')"
done

# Why those two are refused rather than escaped. A backslash lands inside a JSON string as an escape,
# so the result parses and names a *different* path. A crafted quote closes the string and the rest
# of the directory becomes structure — here an `env` object, the one key this wrapper exists to keep
# out of a server's entry. Both are silent, and what they displace is the environment stripping.
entry='{"command": "@CONFIGS@/mcp-env.sh"}'
backslashed="$(substitute_configs_dir "$entry" '/tmp/a\b')"
check "a backslash yields JSON that parses" "parses" \
  "$(printf '%s' "$backslashed" | jq -e . >/dev/null 2>&1 && echo parses || echo broken)"
check "and names a path that is not the directory" "differs" \
  "$(test "$(printf '%s' "$backslashed" | jq -r .command)" = '/tmp/a\b/mcp-env.sh' && echo same || echo differs)"

injected="$(substitute_configs_dir "$entry" '/tmp/x", "env": {"LEAK": "1"}, "ignored": "')"
check "a crafted quote yields JSON that parses too" "parses" \
  "$(printf '%s' "$injected" | jq -e . >/dev/null 2>&1 && echo parses || echo broken)"
check "and it has grown the env key the wrapper exists to remove" "command,env,ignored" \
  "$(printf '%s' "$injected" | jq -r 'keys | join(",")' 2>/dev/null)"

# End to end over the repo's own file: what mcp-sync.sh hands the CLI has to name a command that is
# really there, or every stdio server registers and none of them starts.
synced_json="$(substitute_configs_dir "$(strip_comments "$script_dir/mcp.jsonc")" "$script_dir")"
check "the repo's own mcp.jsonc still parses after substitution" "parses" \
  "$(printf '%s' "$synced_json" | jq -e . >/dev/null 2>&1 && echo parses || echo broken)"

stdio_commands="$(printf '%s' "$synced_json" | jq -r '.mcpServers | to_entries[] | select(.value.type == "stdio") | .value.command')"
check "control: it declares at least one stdio server, so the check below reads something" "yes" \
  "$([ -n "$stdio_commands" ] && printf 'yes' || printf 'no')"

unresolved=""
while IFS= read -r command; do
  [ -n "$command" ] || continue
  [ -x "$command" ] || unresolved="$unresolved $command"
done <<<"$stdio_commands"
check "and every stdio command it registers is an executable on this machine" "" "$unresolved"

# The control that makes the line above a measurement rather than a tautology: without the
# substitution the same commands are `@CONFIGS@/...`, which resolves nowhere.
unsubstituted=""
while IFS= read -r command; do
  [ -n "$command" ] || continue
  [ -x "$command" ] || unsubstituted="$unsubstituted $command"
done <<<"$(strip_comments "$script_dir/mcp.jsonc" | jq -r '.mcpServers | to_entries[] | select(.value.type == "stdio") | .value.command')"
check "control: and without it none of them resolves" "$stdio_commands" \
  "$(printf '%s' "$unsubstituted" | tr ' ' '\n' | sed '/^$/d' | sed "s|^@CONFIGS@|$script_dir|")"

# The two refusals that guard registration itself. Both sit past the sourcing fence, so they are
# driven as real runs of a copy — with a `claude` that records what it was asked to register and
# registers nothing, because the real one writes the live MCP registry and is not on a CI runner
# either. jq is not stubbed: what it accepts is the thing under test.
stub_dir="$tmp/stub"
mkdir -p "$stub_dir"
cat >"$stub_dir/claude" <<'STUB'
#!/usr/bin/env bash
printf '%s %s\n' "$1" "$2" >>"$CLAUDE_CALL_LOG"
if [ "$2" = "add-json" ]; then
  printf '%s' "$7" >"$CLAUDE_JSON_DIR/$6"
fi
exit 0
STUB
chmod 755 "$stub_dir/claude"

sync_run() { # <label> <directory to run the copy from> <'with-env'|'no-env'|'dead-env'>
  local label="$1" dir="$2" env_state="$3"
  call_log="$tmp/claude-calls-$label"
  json_dir="$tmp/claude-json-$label"
  mkdir -p "$dir" "$json_dir"
  cp "$script_dir/mcp-sync.sh" "$dir/mcp-sync.sh"
  cp "$script_dir/mcp.jsonc" "$dir/mcp.jsonc"
  case "$env_state" in
    with-env) cp "$script_dir/mcp-env.sh" "$dir/mcp-env.sh" ;;
    dead-env) cp "$script_dir/mcp-env.sh" "$dir/mcp-env.sh" && chmod 644 "$dir/mcp-env.sh" ;;
  esac
  out="$(CLAUDE_CONFIG_DIR="$tmp/sync-config-$label" CLAUDE_CALL_LOG="$call_log" \
    CLAUDE_JSON_DIR="$json_dir" PATH="$stub_dir:$PATH" bash "$dir/mcp-sync.sh" 2>&1)"
  status=$?
}

# The control that makes the two refusals below mean something: on a checkout that is fine, the same
# script registers, and what it hands the CLI names a command that is really there. Without this,
# both refusals pass on any fixture broken enough to stop the run early.
sync_run good "$tmp/good/ai" with-env
check "a checkout that substitutes and has the wrapper syncs" "0" "$status"
registered_commands="$(cat "$json_dir"/* 2>/dev/null | jq -r 'select(.type == "stdio") | .command')"
check "control: it registered at least one stdio server" "yes" \
  "$([ -n "$registered_commands" ] && printf 'yes' || printf 'no')"
missing_commands=""
while IFS= read -r command; do
  [ -n "$command" ] || continue
  [ -x "$command" ] || missing_commands="$missing_commands $command"
done <<<"$registered_commands"
check "and every command it handed the CLI is executable" "" "$missing_commands"

# The injection above, driven as a real run this time, to show the refusal lands before the CLI is
# reached at all.
sync_run quoted "$tmp/qu\"oted/ai" with-env
check "a directory holding a quote exits 1 rather than registering" "1" "$status"
check "and says why the substitution cannot be made" "held" "$(held 'contains a quote or a backslash' "$out")"
check "and says nothing was synced" "held" "$(held 'Nothing was synced' "$out")"
check "control: nothing reached the CLI" "no calls" \
  "$([ -e "$call_log" ] && cat "$call_log" || printf 'no calls')"

# Without the wrapper, every stdio server registers pointing at a command that is not there. Nothing
# would say so until the next session started them.
sync_run wrapperless "$tmp/wrapperless/ai" no-env
check "a checkout with no mcp-env.sh exits 1 rather than registering" "1" "$status"
check "and names the wrapper it could not find" "held" "$(held 'mcp-env.sh is missing or not executable' "$out")"
check "control: that one did not reach the CLI either" "no calls" \
  "$([ -e "$call_log" ] && cat "$call_log" || printf 'no calls')"

# Present but not executable is the same refusal: `claude mcp add-json` would take it happily and the
# server would fail at launch.
sync_run unexecutable "$tmp/unexecutable/ai" dead-env
check "an mcp-env.sh that is not executable is refused too" "1" "$status"
check "and names it as well" "held" "$(held 'mcp-env.sh is missing or not executable' "$out")"

echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
