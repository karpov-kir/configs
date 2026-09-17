#!/usr/bin/env bash
# go-tools: none — every case here passes --skip-tools, so ai/bootstrap.sh's installer step never runs
# and no Go binary is built or invoked. The tool paths ai/tools/gate reads are in the script this suite
# covers, on a branch this suite never takes. Without this line the unit keys on the whole tool tree.
set -u
here=$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
checkout=$(CDPATH= cd -P -- "$here/.." && pwd -P)
suite_name="ai/rtk-bootstrap-test.sh"
. "$checkout/lib/test-harness.sh" || exit 2
unset CODEX_HOME
mkdir -p "$tmp/bin"
cat >"$tmp/bin/rtk" <<'RTK'
#!/usr/bin/env bash
printf '%s\n' "$@" >"$HOME/rtk-args"
[ -z "${RTK_TEST_FAIL:-}" ] || exit 1
case " $* " in *" --codex "*) ;; *) exit 0 ;; esac
mkdir -p "$CODEX_HOME"
printf 'RTK usage\n' >"$CODEX_HOME/RTK.md"
printf '@RTK.md\n' >>"$CODEX_HOME/AGENTS.md"
RTK
chmod +x "$tmp/bin/rtk"
run_rtk_boot() {
  out=$(HOME="$home" CODEX_HOME="$home/profile" PATH="$tmp/bin:$PATH" bash "$here/bootstrap.sh" --agent=codex --owner --skip-brew --skip-tools --skip-mcp --skip-verify "$@" 2>&1)
  status=$?
}
# The path is a literal bootstrap.sh creates and the owner instructions send every session to. Copying
# it here would only catch bootstrap.sh drifting from the copy. Every fixture below derives it from the
# instruction source instead, which a case can read before any install has run, and one case holds the
# installed copy against that source. The drift that reaches an agent — a rule naming a store bootstrap
# never created — is then a red case rather than a silent one.
memory_named_by() { # <instruction file> <home>
  local named
  named=$(sed -n 's/.*`\(~\/[^`]*\/MEMORY\.md\)`.*/\1/p' "$1" 2>/dev/null | head -1)
  [ -n "$named" ] || return 1
  printf '%s' "$2${named#\~}"
}
store_in() { # <home>
  memory_named_by "$here/owner-instructions.md" "$1"
}
owner_source_before=$(cat "$here/owner-instructions.md")
fresh_home
run_rtk_boot --dry-run
expect_status "RTK dry run succeeds" 0
expect_absent "RTK dry run does not invoke the CLI" "$home/rtk-args"
run_rtk_boot
expect_status "Codex RTK install succeeds" 0
grep -q "rtk proxy" "$home/profile/AGENTS.md" && record_pass "Owner instructions describe RTK" || record_fail "Owner instructions describe RTK" "missing guidance"
[ -f "$home/profile/RTK.md" ] && record_pass "RTK respects the Codex profile" || record_fail "RTK respects the Codex profile" "missing RTK.md"
if [ -f "$home/rtk-args" ] && [ "$(cat "$home/rtk-args")" = "$(printf 'init\n--codex\n--global')" ]; then
  record_pass "RTK receives the native Codex global arguments"
else
  record_fail "RTK receives the native Codex global arguments" "wrong or missing arguments"
fi
before=$(cat "$home/profile/AGENTS.md")
run_rtk_boot
expect_status "Codex RTK reinstall succeeds" 0
[ "$(cat "$home/profile/AGENTS.md")" = "$before" ] && record_pass "RTK reinstall preserves instructions" || record_fail "RTK reinstall preserves instructions" "instructions changed"
run_rtk_boot --uninstall
expect_status "RTK bridge uninstall succeeds" 0
expect_absent "uninstall removes the owner instruction mount" "$home/profile/AGENTS.md"
expect_absent "uninstall removes the ecosystem bucket" "$home/.kk-flavor"
fresh_home
RTK_TEST_FAIL=1 run_rtk_boot
expect_status "a failed RTK init fails bootstrap" 1
expect_out "the RTK failure is named" "rtk init failed"
fresh_home
mkdir -p "$home/profile"
ln -s "$home/elsewhere" "$home/profile/RTK.md"
run_rtk_boot
expect_status "RTK refuses a symlinked instruction target" 1
expect_absent "RTK refuses before invoking the CLI" "$home/rtk-args"
expect_absent "RTK does not write through the symlink" "$home/elsewhere"
fresh_home
run_rtk_boot --skip-rtk
expect_status "RTK initialization can be skipped" 0
expect_absent "skip-rtk does not invoke the CLI" "$home/rtk-args"
fresh_home
out=$(HOME="$home" CODEX_HOME="$home/profile" PATH="$tmp/bin:$PATH" bash "$here/bootstrap.sh" --agent=claude --owner --skip-brew --skip-tools --skip-mcp --skip-verify 2>&1)
status=$?
expect_status "Claude owner initializes its native RTK hook" 0
if [ -f "$home/rtk-args" ] && [ "$(cat "$home/rtk-args")" = "$(printf 'init\n--agent\nclaude\n--global\n--hook-only\n--auto-patch')" ]; then
  record_pass "Claude RTK uses hook-only mode with automatic settings patching"
else
  record_fail "Claude RTK uses hook-only mode with automatic settings patching" "wrong or missing arguments"
fi
fresh_home
mkdir -p "$home/profile"
printf 'Personal RTK instructions\n' >"$home/profile/RTK.md"
run_rtk_boot
expect_status "an existing RTK document is supported" 0
[ "$(cat "$home/profile/RTK.md")" = 'Personal RTK instructions' ] && record_pass "existing RTK instructions are preserved" || record_fail "existing RTK instructions are preserved" "document overwritten"
expect_absent "existing RTK instructions bypass native initialization" "$home/rtk-args"
fresh_home
run_rtk_boot --skip-rtk
expect_status "owner creates its shared memory store" 0
owner_memory=$(store_in "$home") || owner_memory=
# No fallback literal here on purpose: one would let the case below pass on a run whose instructions
# named nothing, which is the failure this derivation exists to catch. The dependent cases go red too,
# and the first failure names the cause.
[ -n "$owner_memory" ] && record_pass "the owner instruction source names a memory store" || record_fail "the owner instruction source names a memory store" "no backticked MEMORY.md path in $here/owner-instructions.md"
installed_memory=$(memory_named_by "$home/profile/AGENTS.md" "$home") || installed_memory=
[ -n "$installed_memory" ] && [ "$installed_memory" = "$owner_memory" ] && record_pass "the installed instructions name the store their source does" || record_fail "the installed instructions name the store their source does" "installed names ${installed_memory:-nothing}, source names ${owner_memory:-nothing}"
[ -n "$owner_memory" ] && [ -f "$owner_memory" ] && record_pass "bootstrap created the store its instructions name" || record_fail "bootstrap created the store its instructions name" "instructions name ${owner_memory:-nothing}, which does not exist"
printf '# Memory\n\nKeep this entry.\n' >"$owner_memory"
run_rtk_boot --skip-rtk
[ "$(cat "$owner_memory")" = "$(printf '# Memory\n\nKeep this entry.')" ] && record_pass "reinstall preserves memory" || record_fail "reinstall preserves memory" "overwritten"
out=$(HOME="$home" PATH="$tmp/bin:$PATH" bash "$here/bootstrap.sh" --agent=claude --owner --skip-rtk --skip-brew --skip-tools --skip-mcp --skip-verify 2>&1)
status=$?
expect_status "second owner provider installs" 0
cmp -s "$home/.claude/CLAUDE.md" "$home/profile/AGENTS.md" && record_pass "both owner instruction files are identical" || record_fail "both owner instruction files are identical" "different"
run_rtk_boot --uninstall
[ -f "$owner_memory" ] && record_pass "uninstall keeps memory" || record_fail "uninstall keeps memory" "removed"
[ "$(cat "$here/owner-instructions.md")" = "$owner_source_before" ] && record_pass "native RTK never changes shared owner source" || record_fail "native RTK never changes shared owner source" "source changed"

# A machine that ran the older owner bootstrap holds the entries at the singular spelling. Creating an
# empty store beside them would strand every entry at a path no session reads.
fresh_home
migrated=$(store_in "$home")
mkdir -p "$home/Document/AI"
printf '# Memory\n\nEntry from the old path.\n' >"$home/Document/AI/MEMORY.md"
run_rtk_boot --skip-rtk
expect_status "owner install migrates a legacy memory store" 0
[ "$(cat "$migrated" 2>/dev/null)" = "$(printf '# Memory\n\nEntry from the old path.')" ] && record_pass "migration carries the entries over" || record_fail "migration carries the entries over" "content did not survive the move"
expect_absent "migration leaves nothing at the legacy path" "$home/Document/AI/MEMORY.md"
expect_absent "migration removes the emptied legacy directory" "$home/Document"

# bootstrap.sh's early return is what keeps a dry run to one claim about the store; this pins the
# regression where it also prints "would create" after "would move".
fresh_home
mkdir -p "$home/Document/AI"
printf '# Memory\n\nEntry from the old path.\n' >"$home/Document/AI/MEMORY.md"
run_rtk_boot --skip-rtk --dry-run
expect_status "a dry run with a legacy store succeeds" 0
expect_out "the dry run says it would move the store" "would move"
case "$out" in *"would create"*) record_fail "the dry run does not also claim it would create one" "said both" ;; *) record_pass "the dry run does not also claim it would create one" ;; esac
[ -f "$home/Document/AI/MEMORY.md" ] && record_pass "a dry run moves nothing" || record_fail "a dry run moves nothing" "the store left the legacy path"

# Two stores is the one case where guessing loses an entry, so it refuses instead of picking.
fresh_home
kept=$(store_in "$home")
mkdir -p "$home/Document/AI" "${kept%/*}"
printf 'older store\n' >"$home/Document/AI/MEMORY.md"
printf 'newer store\n' >"$kept"
run_rtk_boot --skip-rtk
expect_status "a store at both paths refuses rather than picking one" 1
# Without this the case passes on any refusal the run happens to raise, including one about something else.
expect_out "the refusal names both stores" "owner memory exists at both"
[ "$(cat "$home/Document/AI/MEMORY.md")" = 'older store' ] && [ "$(cat "$kept")" = 'newer store' ] && record_pass "a refused migration leaves both stores untouched" || record_fail "a refused migration leaves both stores untouched" "a store changed"
fresh_home
out=$(HOME="$home" bash "$here/bootstrap.sh" --agent=codex --skip-rtk --skip-brew --skip-tools --skip-mcp --skip-verify 2>&1)
status=$?
expect_status "ordinary install still works" 0
# A derivation that yields nothing would hand expect_absent an empty path, and `[ ! -e "" ]` is true —
# the case would pass having checked nothing. The guard is what keeps this dependent case honest.
ordinary_store=$(store_in "$home") || ordinary_store=
[ -n "$ordinary_store" ] && expect_absent "ordinary install creates no owner memory" "$ordinary_store" || record_fail "ordinary install creates no owner memory" "the instruction source named no store to check for"
if grep -q 'MEMORY.md' "$home/.codex/AGENTS.md"; then record_fail "ordinary instructions contain no owner memory rule" "leaked"; else record_pass "ordinary instructions contain no owner memory rule"; fi
fresh_home
mkdir -p "$home/profile"
( . "$checkout/lib/flavor-region.sh"; printf '%s\n%s\n%s\n' "$flavor_region_open" "$(flavor_region_body)" "$flavor_region_close" ) >"$home/profile/AGENTS.md"
run_rtk_boot --skip-rtk --dry-run
expect_status "generated owner migration supports dry-run" 0
[ ! -L "$home/profile/AGENTS.md" ] && record_pass "migration dry-run preserves the generated file" || record_fail "migration dry-run preserves the generated file" "replaced"
run_rtk_boot --skip-rtk
expect_status "prior generated instructions migrate to owner" 0
[ ! -L "$home/profile/AGENTS.md" ] && cmp -s "$home/profile/AGENTS.md" "$here/owner-instructions.md" && record_pass "generated file becomes regular owner copy" || record_fail "generated file becomes regular owner copy" "wrong content or symlink"
compgen -G "$home/profile/AGENTS.md.backup.*" >/dev/null && record_pass "migration preserves a backup" || record_fail "migration preserves a backup" "missing"
fresh_home
mkdir -p "$home/profile"
printf 'My custom instructions\n' >"$home/profile/AGENTS.md"
run_rtk_boot --skip-rtk
expect_status "owner refuses to overwrite custom instructions" 1
[ "$(cat "$home/profile/AGENTS.md")" = 'My custom instructions' ] && record_pass "custom instructions survive" || record_fail "custom instructions survive" "modified"
fresh_home
mkdir -p "$home/profile"
ln -s "$here/CLAUDE.md" "$home/profile/AGENTS.md"
run_rtk_boot --skip-rtk
expect_status "legacy owner symlink migrates" 0
[ ! -L "$home/profile/AGENTS.md" ] && cmp -s "$home/profile/AGENTS.md" "$here/owner-instructions.md" && record_pass "legacy symlink becomes independent Codex copy" || record_fail "legacy symlink becomes independent Codex copy" "not a copy"
printf '\nLocal addition.\n' >>"$home/profile/AGENTS.md"
run_rtk_boot --skip-rtk
expect_status "reinstall refuses local instruction edits" 1
grep -q 'Local addition.' "$home/profile/AGENTS.md" && record_pass "local additions survive reinstall" || record_fail "local additions survive reinstall" "lost"
run_rtk_boot --uninstall
expect_status "uninstall refuses modified owner instructions" 1
grep -q 'Local addition.' "$home/profile/AGENTS.md" && record_pass "local additions survive uninstall" || record_fail "local additions survive uninstall" "lost"
[ -f "$here/owner-instructions.md" ] && [ ! -e "$here/CLAUDE.md" ] && [ ! -e "$here/AGENTS.md" ] && record_pass "owner template is separate from repo instruction entry points" || record_fail "owner template is separate from repo instruction entry points" "wrong source layout"

report_suite
