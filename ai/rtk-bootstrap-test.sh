#!/usr/bin/env bash
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
[ -f "$home/Document/AI/MEMORY.md" ] && record_pass "owner memory file exists" || record_fail "owner memory file exists" "missing"
printf '# Memory\n\nKeep this entry.\n' >"$home/Document/AI/MEMORY.md"
run_rtk_boot --skip-rtk
[ "$(cat "$home/Document/AI/MEMORY.md")" = "$(printf '# Memory\n\nKeep this entry.')" ] && record_pass "reinstall preserves memory" || record_fail "reinstall preserves memory" "overwritten"
out=$(HOME="$home" PATH="$tmp/bin:$PATH" bash "$here/bootstrap.sh" --agent=claude --owner --skip-rtk --skip-brew --skip-tools --skip-mcp --skip-verify 2>&1)
status=$?
expect_status "second owner provider installs" 0
cmp -s "$home/.claude/CLAUDE.md" "$home/profile/AGENTS.md" && record_pass "both owner instruction files are identical" || record_fail "both owner instruction files are identical" "different"
run_rtk_boot --uninstall
[ -f "$home/Document/AI/MEMORY.md" ] && record_pass "uninstall keeps memory" || record_fail "uninstall keeps memory" "removed"
[ "$(cat "$here/owner-instructions.md")" = "$owner_source_before" ] && record_pass "native RTK never changes shared owner source" || record_fail "native RTK never changes shared owner source" "source changed"
fresh_home
out=$(HOME="$home" bash "$here/bootstrap.sh" --agent=codex --skip-rtk --skip-brew --skip-tools --skip-mcp --skip-verify 2>&1)
status=$?
expect_status "ordinary install still works" 0
expect_absent "ordinary install creates no owner memory" "$home/Document/AI/MEMORY.md"
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
