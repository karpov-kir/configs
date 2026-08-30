#!/usr/bin/env bash
# Cases for mcp-env.sh — the wrapper every stdio MCP server now launches through, and the only thing
# standing between an unpinned `npx` package and every credential exported in the shell that started
# Claude Code.
#   usage: mcp-env-test.sh   # prints one line per case; exit 0 when all pass, 1 otherwise, 2 when it
#                            # could not reach the script at all
#
# The control that makes the rest mean something is the sentinel one: "no secret reached the child"
# passes just as well when the child printed nothing at all, so the same sentinels are measured
# without the wrapper first, and that run has to find them.
#
# MCP_ENV_UNDER_TEST names the file every case drives, so a mutation run can point the whole suite at
# a mutated copy without editing anything in the checkout.
set -uo pipefail
export LC_ALL=C

here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
script="${MCP_ENV_UNDER_TEST:-$here/mcp-env.sh}"

# Exit 2 and no summary line: the cases below are all `$script ...`, so a script this suite cannot
# execute makes every one of them fail for a reason that has nothing to do with the guard it names.
[ -x "$script" ] || {
  echo "mcp-env-test.sh: $script is not an executable file — nothing was measured" >&2
  exit 2
}

pass=0
fail=0

check() { # <name> <expected> <actual>
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

# bash 3.2 — macOS's own, and one leg of the gates workflow — mis-parses a `case` written inline in a
# `$( )`, ending the substitution at the first pattern's `)`. A needle test that has to run inside one
# goes through a function instead.
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

# `env` as the command throughout, not a shell: bash would add PWD, SHLVL and _ of its own, and the
# set comparison below would then be measuring bash rather than the allow-list.
child_env() { # <variable assignments…> — the launching environment, one NAME=VALUE per argument
  env "$@" "$script" env
}

# --- the sentinels ---
#
# Three names no allow-list entry resembles, carrying values nothing else on this machine prints.
secrets="FAKE_API_KEY=sentinel-alpha GH_TOKEN=sentinel-bravo AWS_SECRET_ACCESS_KEY=sentinel-charlie"

# shellcheck disable=SC2086
leaked_without="$(env $secrets env | grep -c 'sentinel-')"
check "control: the sentinels really are in the launching environment" "3" "$leaked_without"

# shellcheck disable=SC2086
leaked_through="$(child_env $secrets | grep -c 'sentinel-')"
check "and none of them reaches the child" "0" "$leaked_through"

# shellcheck disable=SC2086
check "control: the child printed its environment, so the line above measured something" "yes" \
  "$([ -n "$(child_env $secrets)" ] && printf 'yes' || printf 'no')"

# --- the allow-list, every row of it ---
#
# The names the wrapper may pass, written out here rather than asked of it. Every other case derives
# its expectation from the script under test, so on their own they stay green while ALLOWED grows:
# adding GITHUB_TOKEN to it leaves all of them passing and hands the token to an unpinned `npx`
# package. This literal is what goes red on that edit, so widening the wrapper means editing this
# list in the same commit, where a reviewer reads the new name next to the old ones.
expected_allowed="$(printf '%s\n' \
  PATH HOME USER LOGNAME \
  TMPDIR TMP TEMP \
  LANG LC_ALL LC_CTYPE \
  XDG_CONFIG_HOME XDG_CACHE_HOME XDG_DATA_HOME XDG_STATE_HOME \
  MISE_DATA_DIR MISE_CONFIG_DIR MISE_CACHE_DIR \
  NODE_EXTRA_CA_CERTS SSL_CERT_FILE SSL_CERT_DIR \
  HTTP_PROXY HTTPS_PROXY NO_PROXY http_proxy https_proxy no_proxy |
  sort)"

# The names the help prints, set to markers and asked for back. Anything the child holds that the
# help does not name is a leak the sentinels above would have missed; anything named that does not
# arrive is a server that may not start. The help is `${ALLOWED[*]}` verbatim, so this list is
# ALLOWED itself, which is why it is held to the literal above before anything else trusts it.
allowed="$("$script" --help | sed -n 's/^passes only: //p' | tr ' ' '\n' | sed '/^$/d' | sort)"
check "control: the help names an allow-list at all" "yes" \
  "$([ -n "$allowed" ] && printf 'yes' || printf 'no')"
check "the allow-list is exactly the names pinned in this file" "$expected_allowed" "$allowed"

marked=()
while IFS= read -r name; do
  [ -n "$name" ] || continue
  case "$name" in
    # Left at their real values: the wrapper needs PATH to exec anything, and a marker HOME would
    # only be a marker.
    PATH | HOME) ;;
    # A locale name has to be a locale, or every child bash writes a setlocale warning to stderr and
    # this suite's output grows a line no case put there.
    LANG | LC_*) marked+=("$name=C") ;;
    *) marked+=("$name=marker-$name") ;;
  esac
done <<<"$allowed"

observed="$(child_env "${marked[@]}" | sed 's/=.*//' | sort)"
check "every name the help promises arrives, and nothing else does" "$allowed" "$observed"

# --- unset, set-empty, set: three different things ---
#
# Why the difference is worth three cases: an empty TMPDIR forwarded as `TMPDIR=` makes npx unpack
# into a path that is the empty string instead of falling back to /tmp.
check "a set variable arrives with its value" "LC_CTYPE=en_US.UTF-8" \
  "$(child_env LC_CTYPE=en_US.UTF-8 | grep '^LC_CTYPE=')"

check "a set-but-empty variable arrives, still empty" "LC_CTYPE=" \
  "$(child_env LC_CTYPE= | grep '^LC_CTYPE=')"

# Counted off one captured run, with a control on the same run: a wrapper that died before exec'ing
# anything also prints no LC_CTYPE, and "0" would read as the guard working.
unset_probe="$(env -u LC_CTYPE "$script" env)"
check "control: the child ran on that one, so a zero below means absence and not silence" "1" \
  "$(printf '%s\n' "$unset_probe" | grep -c '^HOME=')"
check "an unset variable stays unset rather than arriving empty" "0" \
  "$(printf '%s\n' "$unset_probe" | grep -c '^LC_CTYPE=')"

check "control: and that is not the child dropping every LC_CTYPE it is given" "1" \
  "$(child_env LC_CTYPE=whatever | grep -c '^LC_CTYPE=')"

# A value from outside the system, round-tripped. The list passes names, so the value is whatever the
# launching environment holds and this script must not touch it.
check "a non-ASCII value survives untouched" "LC_CTYPE=ünïcodé-Ω-Ω" \
  "$(child_env LC_CTYPE=ünïcodé-Ω-Ω | grep '^LC_CTYPE=')"

# --- the exec ---

check "the arguments reach the command intact" "a b|c\$d|e'f|\"g\"|" \
  "$("$script" printf '%s|' 'a b' 'c$d' "e'f" '"g"')"

"$script" sh -c 'exit 7'
check "the command's exit status is the wrapper's" "7" "$?"

"$script" sh -c 'exit 0'
check "control: and a command that succeeds is not reported as a failure" "0" "$?"

# --- the two arms that launch nothing ---
#
# Asserted on wording as well as status. Every refusal here exits 2, so the code says one happened and
# never which, and `command not found` is this suite's PATH failing rather than any arm of the script.
no_args_out="$("$script" 2>&1)"
no_args_status=$?
check "no command exits 2 rather than launching something" "2" "$no_args_status"
check "and says nothing was launched" "held" \
  "$(held "nothing was launched" "$no_args_out")"

help_out="$("$script" --help 2>&1)"
help_status=$?
check "--help exits 0" "0" "$help_status"
check "-h is the same arm" "$help_out" "$("$script" -h 2>&1)"

# The help arm prints a line range out of the wrapper's own header, so two line numbers stand in for a
# claim about that file's content, and a line added above the range silently makes them the wrong
# lines. Both ends are pinned by content instead: the header line the range has to start at, and the
# first line past it, which has to stay out.
#
# Both needles are read out of the wrapper, never written out here. A needle spelled literally is a
# claim about prose, and prose gets reworded: the moment the wrapper's wording moves, the literal
# matches nothing, "the help lacks it" becomes true of every possible output, and the case passes
# forever without reaching its subject — no failure, and nothing saying it stopped measuring. Read
# from the file, a needle follows the rewording, and the controls below fail when it reads as empty.
header_prose() { # <n> — the nth non-blank comment line of the wrapper's header, empty when there is none
  sed -n '2,$p' "$script" | sed -n 's/^# \{0,1\}\(..*\)$/\1/p' | sed -n "${1}p"
}

help_first="$(header_prose 1)"
check "control: the header's opening line was found, so the case below compares something" "found" \
  "$([ -n "$help_first" ] && printf 'found' || printf 'empty')"
check "and the help starts at that line" "held" \
  "$(held "$help_first" "$help_out")"

# The help prints two of those lines, so the third is the first one it must not reach.
help_past="$(header_prose 3)"
check "control: the line past the printed range was found, so the case below compares something" "found" \
  "$([ -n "$help_past" ] && printf 'found' || printf 'empty')"
check "and stops before the paragraph under it" "lacked" \
  "$(lacked "$help_past" "$help_out")"

echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
