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
repo=$(CDPATH= cd -P "$ai/.." && pwd -P)
skills="$ai/skills"
# Exit 2, not 1, at every fixture site below. `run-tests.sh` reads 1 as FAIL and prints the last
# fifteen lines of output under it — which, for a fixture that died before the first case, is
# nothing. A red suite with an empty block sends a reader hunting a defect in the stubs when the
# truth is that this harness could not be built and NOTHING was tested. 2 is that fact in this
# repo's vocabulary, and run-tests.sh counts it apart as NOMEASURE.
base=$(mktemp -d) || {
  echo "tool-stub-test: mktemp -d refused, so no fixture root exists and nothing was tested" >&2
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
(CDPATH= cd "$report_repo" && git init -q .) || {
  echo "tool-stub-test: could not init the report.sh fixture repository, so nothing was tested" >&2
  exit 2
}

# comment-density.sh scans a diff, so its probe needs a repository with a COMMIT — `git diff HEAD` on an
# unborn HEAD exits 2, which is indistinguishable here from the stub failing to reach its tool.
density_repo="$base/density-repo"
mkdir -p "$density_repo"
(
  CDPATH= cd "$density_repo" &&
    git init -q . &&
    git config user.email t@t &&
    git config user.name t &&
    printf 'x := 1\n' >seed.go &&
    git add seed.go &&
    git commit -qm seed
) >/dev/null 2>&1 || {
  echo "tool-stub-test: could not init the comment-density fixture repository, so nothing was tested" >&2
  exit 2
}

# skill | script | cwd for the probe | args | a string the tool prints when it really ran
stubs() {
  cat <<TABLE
kk-ecosystem|check.sh|$base|$ai|always-loaded:
kk-reduce|stats.sh|$base|$ai|prose:
idsd-qualify|report.sh|$report_repo|list|no reports
kk-ecosystem|ruleecho.sh|$base|$echo_root|rule stated twice
kk-ecosystem|cite-graph.sh|$base|$echo_root|citation edge(s)
kk-humanize|comment-density.sh|$density_repo|HEAD|reached the scan
TABLE
}

echo "shared:tool-stub"

# bloat-judge.sh in situ, and the reason every stub declares its own offset instead of the region holding one
# depth. Most stubs sit at `skills/<skill>/scripts/`, three below the tools directory; this one and
# `tree-fingerprint.sh` are at `kk-flavor/scripts/`, two below, and `gate.sh` sits beside `tools/`
# itself. The copied fixtures further down are all built at the three-below depth, and the offset scan
# near the bottom is what reaches the other two; this drives the real file at the real depth against
# its real tool. The probe table cannot hold it because that harness passes exactly one argument, and
# the judge with none prints its usage and exits 2.
judge_stub="$ai/kk-flavor/scripts/bloat-judge.sh"
name="bloat-judge.sh reaches its tool from two levels below tools/, in the tree not a fixture"
if [ -x "$judge_stub" ]; then
  judge_err="$base/judge-probe.err"
  (CDPATH= cd "$base" && "$judge_stub" >/dev/null 2>"$judge_err")
  status=$?
  if [ "$status" -eq 2 ] && grep -q "usage:" "$judge_err"; then
    record_pass "$name"
  else
    record_fail "$name" "exit $status, stderr: $(cat "$judge_err" 2>/dev/null)"
  fi
else
  record_fail "$name" "$judge_stub is not executable, so the one stub at that depth went unchecked"
fi

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

# The region resolves the one path its stub declares — `$here/$tools_offset/tools/resolve.sh` — and
# consults nothing else. These two cases are the halves of that: a stub at the shallower depth really
# reaches its own tools directory, and no stub reaches one outside its checkout. Every case above sits
# at the deeper depth, so without these two a regression to any other shape would pass.
while IFS='|' read -r skill script cwd args marker; do
  [ -n "$skill" ] || continue

  # A checkout that ships no ai/tools/ refuses, rather than reaching one that belongs to somebody else.
  # The decoy sits ABOVE the repository root, which is where two earlier shapes of this region reached.
  # Both ran a stranger's binary at exit 0; the shared region's own comment says how.
  #
  # Asserted from both ends: the refusal must name the missing resolver, AND the decoy must never have
  # run. Either alone passes for the wrong reason — a stub that died before resolving anything satisfies
  # the second, and one that ran the decoy and then failed could satisfy the first.
  escape="$base/escape-$script"
  mkdir -p "$escape/tools" "$escape/root/skills/$skill/scripts"
  printf '#!/usr/bin/env bash\necho "decoy resolver reached" >&2\nexit 2\n' > "$escape/tools/resolve.sh"
  chmod 755 "$escape/tools/resolve.sh"
  cp "$skills/$skill/scripts/$script" "$escape/root/skills/$skill/scripts/$script"
  chmod 755 "$escape/root/skills/$skill/scripts/$script"
  out=$(CDPATH= cd "$cwd" && "$escape/root/skills/$skill/scripts/$script" "$args" 2>&1)
  status=$?
  expect_status "$script exits 2 rather than reaching a tools/ outside the checkout" 2
  expect_out "$script names the resolver it could not find" "no resolver at"
  case "$out" in
    *"decoy resolver reached"*)
      record_fail "$script never reaches a resolver outside its own checkout" \
        "it resolved past the path it names and ran $escape/tools/resolve.sh: $out"
      ;;
    *) record_pass "$script never reaches a resolver outside its own checkout" ;;
  esac
done <<<"$(stubs)"

# Every stub's declared offset, checked against where that stub actually sits. The region names one
# path, so a stub's depth is its own property, and the thing worth holding is that each one declares
# the depth it really has.
#
# Discovered rather than listed, so a stub added tomorrow is checked without a row here; finding none
# is a failure, because a scan over nothing is green for the wrong reason.
offsets_checked=0
# `-z`, because `git ls-files` C-quotes a path holding a non-ASCII byte, a control character, a quote
# or a backslash — `"caf\303\251.sh"`, quotes and all. That name reaches no file, so the `grep` below
# `continue`s and the stub goes unscanned in silence, which is the one outcome this scan must not
# have. NUL-delimited output cannot survive a `$(...)`, hence the redirect rather than a here-string.
while IFS= read -r -d '' listed; do
  [ -n "$listed" ] || continue
  # `git ls-files` prints paths relative to the repository root, and this suite runs from whatever
  # directory the runner happens to be in. Anchored here rather than used as given: unanchored, every
  # `grep` below misses its file and `continue`s, so the scan checks nothing and only the zero-counted
  # guard at the bottom says so.
  stub_file="$repo/$listed"
  # This file carries the marker as a string it searches FOR, not as a region it holds. Excluded by
  # name rather than by a cleverer pattern: a scan that tried to tell the two apart by shape would be
  # one more thing to get wrong, and there is exactly one file in this position.
  case "$stub_file" in */tool-stub-test.sh) continue ;; esac
  grep -q -e '--- shared:tool-stub ---' "$stub_file" || continue
  offset=$(sed -n 's/^tools_offset="\(.*\)"$/\1/p' "$stub_file")
  if [ -z "$offset" ]; then
    record_fail "$stub_file declares a tools_offset" "it carries the shared region and names no offset, so it resolves nothing"
    continue
  fi
  stub_dir=$(CDPATH= cd -P "$(dirname "$stub_file")" && pwd -P)
  offsets_checked=$((offsets_checked + 1))
  if [ -x "$stub_dir/$offset/tools/resolve.sh" ]; then
    record_pass "${stub_file##*/} declares the offset that reaches its own tools directory"
  else
    record_fail "${stub_file##*/} declares the offset that reaches its own tools directory" \
      "tools_offset=$offset resolves to $stub_dir/$offset/tools/resolve.sh, which is not executable"
  fi
done < <(cd "$repo" && git ls-files -z '*.sh')
if [ "$offsets_checked" -eq 0 ]; then
  record_fail "the offset scan found stubs to read" "no file carried a tools_offset, so that scan checked nothing"
else
  record_pass "the offset scan found $offsets_checked stub(s) to read"
fi

# argv[0] survives the exec. The tools derive the skill directory from argv[0], so a stub that let the
# binary's own path through would send every write to the tools directory instead. Proven with the
# ledger, because it is the one write whose destination is visible. Asserted from both ends, since a
# run that wrote nowhere would satisfy the first half on its own.
ledger_name=stats.md
real_ledger="$skills/kk-reduce/$ledger_name"
# Copied and compared with cmp rather than hashed, so the case needs no particular checksum tool.
before="$base/ledger-before"
cp "$real_ledger" "$before" || {
  echo "tool-stub-test: could not copy $real_ledger, so the case that proves this checkout's own" >&2
  echo "  ledger stays untouched has no before-image to compare against, and nothing was tested" >&2
  exit 2
}

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
stubs_outside_skills=0
while IFS= read -r stub_file; do
  [ -n "$stub_file" ] || continue
  grep -q -e "$region" "$stub_file" || continue
  stubs_scanned=$((stubs_scanned + 1))
  case "$stub_file" in
    "$skills"/*) ;;
    *) stubs_outside_skills=$((stubs_outside_skills + 1)) ;;
  esac
  stub_base=${stub_file##*/}
  name="$stub_base documents itself in the lowercase form eco-check anchors on"
  grep -q "usage: $stub_base" "$stub_file" &&
    record_pass "$name" ||
    record_fail "$name" "no 'usage: $stub_base' line — a capitalised Usage: is outside both scans in subcommands.go"
done <<<"$(find "$ai" -name '*.sh' -type f -not -name '*-test.sh' | sort)"

if [ "$stubs_scanned" -gt 0 ]; then
  record_pass "the usage-form scan found $stubs_scanned stub(s) to read"
else
  record_fail "the usage-form scan found stub(s) to read" "nothing under $ai carries the shared region, so the form went unchecked"
fi

# Rooted at ai/, not at ai/skills/, and this case is what holds it there. A stub outside the skills
# tree goes unread if the walk starts inside it, and the count above cannot see that — one stub short
# of all of them is still above zero. So the observable is whether the walk got past the skills tree at
# all. Not a row naming the stub that lives out there: a row is what the comment above says should
# never be needed, and it would go stale the day that file moves.
if [ "$stubs_outside_skills" -gt 0 ]; then
  record_pass "and the walk reached past the skills tree, where $stubs_outside_skills of them live"
else
  record_fail "and the walk reached past the skills tree" \
    "every stub it found is under $skills, so one outside it is unscanned — which is how the one stub at kk-flavor/scripts/ went unread"
fi

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]
