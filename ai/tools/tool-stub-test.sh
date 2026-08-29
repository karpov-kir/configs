#!/usr/bin/env bash
# Cases for the `# --- shared:tool-stub ---` region every skill script uses to reach its Go binary.
# It is one body copied into several files, so it is tested once here rather than per copy; the
# wiring check's shared-region scan is what proves the copies are still identical.
#
# Two cases must not be weakened. "reaches its tool from an unrelated cwd" is the defect the stub
# exists for, and it fails silently: a stub that resolved nothing prints nothing, and a check that
# printed nothing reads exactly like a clean tree. "argv[0] survives the exec" decides which skill
# directory the tool writes into.
#
# Every path is built from variables at runtime. A literal skill or tool path here would be read by
# the wiring check as a citation and reported against the real checkout.
set -u

here=$(CDPATH= cd -P "$(dirname "$0")" && pwd -P)
tools=$here
ai=$(CDPATH= cd -P "$here/.." && pwd -P)
skills="$ai/skills"
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

# A root holding one restatement, so ruleecho has something to find: without content to find, a stub
# that silently did nothing would pass the same assertion a working one does. cite-graph takes the
# same root — it needs only that some `.md` is there, since a root holding none exits 2 and that is
# the status the probe below reads as the tool never having run.
echo_root="$base/echo-root"
mkdir -p "$echo_root/a" "$echo_root/b"
rule='a shared rule stating several discriminating words plainly'
printf '# A\n\n**%s**\n' "$rule" >"$echo_root/a/one.md"
printf '# B\n\n**%s**\n' "$rule" >"$echo_root/b/two.md"

# report.sh reads the repository from its cwd, so its probe needs one; the other three take a root.
report_repo="$base/report-repo"
mkdir -p "$report_repo"
(CDPATH= cd "$report_repo" && git init -q .) || exit 1

# skill | script | cwd for the probe | args | a string the tool prints when it really ran
stubs() {
  cat <<TABLE
kk-ecosystem|check.sh|$base|$ai|always-loaded:
kk-reduce|stats.sh|$base|$ai|prose:
idsd-qualify|report.sh|$report_repo|list|no reports
kk-ecosystem|ruleecho.sh|$base|$echo_root|rule stated twice
kk-ecosystem|cite-graph.sh|$base|$echo_root|citation edge(s)
TABLE
}

echo "shared:tool-stub"

# The negative control, and the regression the stub exists for.
while IFS='|' read -r skill script cwd args marker; do
  [ -n "$skill" ] || continue
  stub="$skills/$skill/scripts/$script"
  out=$(CDPATH= cd "$cwd" && "$stub" "$args" 2>&1)
  status=$?
  expect_out "$script reaches its tool from an unrelated cwd" "$marker"
  if [ "$status" -eq 2 ]; then
    record_fail "$script did not exit 2 there" "exit 2 means the tool never ran — output: $out"
  else
    record_pass "$script did not exit 2 there"
  fi
done <<<"$(stubs)"

# Each way the stub cannot reach a resolver. Both exit 2 and name the fix, because a stub that
# returned quietly would hand the caller silence and the caller reads silence as clean.
while IFS='|' read -r skill script cwd args marker; do
  [ -n "$skill" ] || continue

  # A checkout that mounts the skill without shipping ai/tools/.
  orphan="$base/orphan-$script/skills/$skill/scripts"
  mkdir -p "$orphan"
  cp "$skills/$skill/scripts/$script" "$orphan/$script"
  chmod 755 "$orphan/$script"
  out=$("$orphan/$script" "$args" 2>&1)
  status=$?
  expect_status "$script exits 2 in a checkout with no tools directory" 2
  expect_out "$script names what that checkout is missing" "does not ship ai/tools/"
  expect_out "$script names the tool that did not run" "did NOT run"

  # A resolver that is there but lost its exec bit: a different fix, so a different message.
  noexec="$base/noexec-$script"
  mkdir -p "$noexec/skills/$skill/scripts" "$noexec/tools"
  cp "$skills/$skill/scripts/$script" "$noexec/skills/$skill/scripts/$script"
  chmod 755 "$noexec/skills/$skill/scripts/$script"
  cp "$tools/resolve.sh" "$noexec/tools/resolve.sh"
  chmod 644 "$noexec/tools/resolve.sh"
  out=$("$noexec/skills/$skill/scripts/$script" "$args" 2>&1)
  status=$?
  expect_status "$script exits 2 when the resolver is not executable" 2
  expect_out "$script says to chmod it" "chmod"
done <<<"$(stubs)"

# argv[0] survives the exec. The tools derive the skill directory from argv[0], so a stub that let the
# binary's own path through would send every write to the tools directory instead. Proven with the
# ledger, because it is the one write whose destination is visible. Asserted from both ends, since a
# run that wrote nowhere would satisfy the first half on its own.
ledger_name=stats.md
real_ledger="$skills/kk-reduce/$ledger_name"
# Copied and compared with cmp rather than hashed, so the case needs no particular checksum tool.
before="$base/ledger-before"
cp "$real_ledger" "$before"

fake="$base/fake"
mkdir -p "$fake/tools/bin" "$fake/skills/kk-reduce/scripts"
cp "$tools/resolve.sh" "$fake/tools/resolve.sh"
chmod 755 "$fake/tools/resolve.sh"
cp "$tools/bin/eco-stats" "$fake/tools/bin/eco-stats"
cp "$skills/kk-reduce/scripts/stats.sh" "$fake/skills/kk-reduce/scripts/stats.sh"
chmod 755 "$fake/skills/kk-reduce/scripts/stats.sh"

out=$("$fake/skills/kk-reduce/scripts/stats.sh" --append "tool-stub-test fixture row" "$ai" 2>&1)
status=$?
expect_status "the ledger write lands under the invoking skill directory" 0
expect_out "and says where it appended" "$fake/skills/kk-reduce/$ledger_name"

if [ -s "$fake/skills/kk-reduce/$ledger_name" ]; then
  record_pass "and the fixture ledger now holds a row"
else
  record_fail "and the fixture ledger now holds a row" "nothing at $fake/skills/kk-reduce/$ledger_name"
fi

if cmp -s "$before" "$real_ledger"; then
  record_pass "and this checkout's own ledger is untouched"
else
  record_fail "and this checkout's own ledger is untouched" "$real_ledger changed"
fi

# Every stub documents itself in the lowercase `usage: <script>` form, because that string is the
# anchor two scans in eco-check's subcommands.go match on: `usageSubcommands` reads the stub's own
# grammar by it and `goDispatchLabels` finds the tool's dispatch switch by it. A stub writing `Usage:`
# is outside both while reading to a human as though it were documented, so the cross-check between
# grammar and dispatch goes silent rather than red — and a silent scan is the one failure this
# repository cannot see. Discovered from the shared region rather than listed, so the stub written
# tomorrow is covered without a row here, and finding no stub at all fails rather than passes empty.
region='--- shared:tool-stub ---'
stubs_scanned=0
while IFS= read -r stub_file; do
  [ -n "$stub_file" ] || continue
  grep -q -e "$region" "$stub_file" || continue
  stubs_scanned=$((stubs_scanned + 1))
  stub_base=${stub_file##*/}
  name="$stub_base documents itself in the lowercase form eco-check anchors on"
  grep -q "usage: $stub_base" "$stub_file" &&
    record_pass "$name" ||
    record_fail "$name" "no 'usage: $stub_base' line — a capitalised Usage: is outside both scans in subcommands.go"
done <<<"$(find "$skills" -name '*.sh' -type f -not -name '*-test.sh' | sort)"

if [ "$stubs_scanned" -gt 0 ]; then
  record_pass "the usage-form scan found $stubs_scanned stub(s) to read"
else
  record_fail "the usage-form scan found stub(s) to read" "nothing under $skills carries the shared region, so the form went unchecked"
fi

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]
